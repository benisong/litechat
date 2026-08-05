package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const summaryTestContent = `<chat_summary>
<plot>剧情继续推进。</plot>
<relationship>双方更加信任。</relationship>
<user_facts>用户喜欢安静的地方。</user_facts>
<world_state>当前位于图书馆。</world_state>
<open_loops>约定明天见面。</open_loops>
</chat_summary>`

func TestQueueSummaryIfNeededWaitsBelowThreshold(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 700))

	queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID))
	if err != nil {
		t.Fatalf("queue summary: %v", err)
	}
	if queued {
		t.Fatal("summary was queued below the threshold")
	}
	state := getSummaryTestState(t, svc, chatID)
	if state.PendingToSeq != 0 || state.PendingStatus != "" || state.SummaryRequired {
		t.Fatalf("unexpected pending state: %+v", state)
	}
}

func TestSummaryLifecycleUsesSeparateDatabase(t *testing.T) {
	dataDir := t.TempDir()
	mainDB, err := store.NewDB(dataDir)
	if err != nil {
		t.Fatalf("new main DB: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	if err := mainDB.InitSchema(); err != nil {
		t.Fatalf("init main schema: %v", err)
	}
	summaryDB, err := store.NewSummaryDB(dataDir)
	if err != nil {
		t.Fatalf("new summary DB: %v", err)
	}
	t.Cleanup(func() { _ = summaryDB.Close() })
	if err := summaryDB.InitSummarySchema(); err != nil {
		t.Fatalf("init summary schema: %v", err)
	}

	configStore := store.NewConfigStore(mainDB)
	if err := configStore.Set("service_mode", "service"); err != nil {
		t.Fatalf("enable service mode: %v", err)
	}
	if err := configStore.Set("default_model", "test-model"); err != nil {
		t.Fatalf("set model: %v", err)
	}
	if _, err := mainDB.Exec(`
		INSERT INTO characters (id, user_id, name) VALUES ('separate-char', 'separate-user', 'Separate Character')`); err != nil {
		t.Fatalf("insert character: %v", err)
	}
	if _, err := mainDB.Exec(`
		INSERT INTO chats (id, user_id, character_id, title)
		VALUES ('separate-chat', 'separate-user', 'separate-char', 'Separate Chat')`); err != nil {
		t.Fatalf("insert chat: %v", err)
	}
	messageStore := store.NewMessageStore(mainDB)
	svc := NewSummaryService(
		messageStore,
		store.NewSummaryStore(summaryDB, mainDB),
		store.NewCharacterStore(mainDB),
		configStore,
		store.NewUserStore(mainDB),
	)
	createSummaryTestMessage(t, messageStore, "separate-chat", "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, messageStore, "separate-chat", "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded("separate-chat", listSummaryTestMessages(t, messageStore, "separate-chat")); err != nil || !queued {
		t.Fatalf("queue separate summary: queued=%v err=%v", queued, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeSummaryResponse(w, summaryTestContent)
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)
	if err := svc.runPendingSummary("separate-chat"); err != nil {
		t.Fatalf("run separate summary: %v", err)
	}
	state := getSummaryTestState(t, svc, "separate-chat")
	if state.AppliedCutoffSeq != 2 {
		t.Fatalf("separate summary did not advance: %+v", state)
	}
	context, raw := svc.BuildServiceModeContext(
		"separate-chat",
		listSummaryTestMessages(t, messageStore, "separate-chat"),
	)
	if !strings.Contains(context, "剧情继续推进") || len(raw) != 0 {
		t.Fatalf("separate summary context not applied: context=%q raw=%+v", context, raw)
	}
}

func TestConfiguredSummaryCharLimitControlsRequiredFlag(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	history := listSummaryTestMessages(t, svc.messageStore, chatID)

	if err := svc.configStore.Set("memory_summary_char_limit", "5000"); err != nil {
		t.Fatalf("set high summary limit: %v", err)
	}
	if queued, err := svc.QueueSummaryIfNeeded(chatID, history); err != nil || queued {
		t.Fatalf("summary queued below configured limit: queued=%v err=%v", queued, err)
	}
	if state := getSummaryTestState(t, svc, chatID); state.SummaryRequired {
		t.Fatalf("summary_required ignored configured limit: %+v", state)
	}

	if err := svc.configStore.Set("memory_summary_char_limit", "3000"); err != nil {
		t.Fatalf("set lower summary limit: %v", err)
	}
	if queued, err := svc.QueueSummaryIfNeeded(chatID, history); err != nil || !queued {
		t.Fatalf("summary not queued at configured limit: queued=%v err=%v", queued, err)
	}
	state := getSummaryTestState(t, svc, chatID)
	if !state.SummaryRequired || state.PendingToSeq != 2 {
		t.Fatalf("unexpected required state: %+v", state)
	}
}

func TestSummaryRequiredWaitsForAssistantBoundary(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 3500))
	queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID))
	if err != nil || queued {
		t.Fatalf("summary without assistant boundary: queued=%v err=%v", queued, err)
	}
	state := getSummaryTestState(t, svc, chatID)
	if !state.SummaryRequired || state.PendingToSeq != 0 {
		t.Fatalf("required flag was lost while waiting for assistant: %+v", state)
	}
}

func TestRollingSummaryPromptUsesCharacterOnlyAsScopedReference(t *testing.T) {
	character := &model.Character{
		Name:          "角色名-仅参考",
		Description:   "角色描述-仅参考",
		Personality:   "性格-仅参考",
		Scenario:      "场景设定-仅首次",
		FirstMsg:      "开场白-仅首次",
		AvatarURL:     "avatar-reference.png",
		Tags:          "标签-仅参考",
		POV:           "second",
		UseCustomUser: true,
		UserName:      "用户名称-仅参考",
		UserDetail:    "用户设定-仅参考",
	}
	firstMessages := []*model.Message{
		{Seq: 1, Role: "user", Content: "首次用户消息"},
		{Seq: 2, Role: "assistant", Content: "首次助手正文\n\n'''\n【状态栏】\n地点：不应进入摘要\n'''"},
	}
	firstPrompt := buildRollingSummaryPrompt(character, true, nil, nil, firstMessages, 1, 2, "")
	for _, expected := range []string{
		"角色名-仅参考", "角色描述-仅参考", "性格-仅参考", "场景设定-仅首次", "开场白-仅首次",
		"avatar-reference.png", "标签-仅参考", "second", "用户名称-仅参考", "用户设定-仅参考",
		"禁止复制到摘要", "[聊天记录｜消息序号 1-2]", "[1][user] 首次用户消息", "[2][assistant] 首次助手正文",
	} {
		if !strings.Contains(firstPrompt, expected) {
			t.Fatalf("first summary prompt missing %q:\n%s", expected, firstPrompt)
		}
	}
	if strings.Contains(firstPrompt, "地点：不应进入摘要") || strings.Contains(firstPrompt, "【状态栏】") {
		t.Fatalf("status bar leaked into first summary prompt:\n%s", firstPrompt)
	}

	previous := &model.ChatSummaryChunk{ToSeq: 2, Content: "上一条摘要内容"}
	laterMessages := []*model.Message{
		{Seq: 3, Role: "user", Content: "后续用户消息"},
		{Seq: 4, Role: "assistant", Content: "后续助手消息"},
	}
	laterPrompt := buildRollingSummaryPrompt(character, false, previous, nil, laterMessages, 3, 4, "")
	for _, expected := range []string{
		"角色名-仅参考", "角色描述-仅参考", "性格-仅参考", "avatar-reference.png", "标签-仅参考",
		"second", "用户名称-仅参考", "用户设定-仅参考", "上一条摘要内容",
		"[上一条摘要｜截至消息序号 2]", "[上一条摘要之后的聊天记录｜消息序号 3-4]",
		"[3][user] 后续用户消息", "[4][assistant] 后续助手消息",
	} {
		if !strings.Contains(laterPrompt, expected) {
			t.Fatalf("later summary prompt missing %q:\n%s", expected, laterPrompt)
		}
	}
	for _, excluded := range []string{"场景设定-仅首次", "开场白-仅首次", "首次用户消息", "首次助手正文"} {
		if strings.Contains(laterPrompt, excluded) {
			t.Fatalf("later summary prompt unexpectedly contains %q:\n%s", excluded, laterPrompt)
		}
	}
}

func TestQueueSummaryIfNeededRecordsFixedAssistantBoundary(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for _, message := range []struct {
		role    string
		content string
	}{
		{"user", strings.Repeat("甲", 1700)},
		{"assistant", strings.Repeat("乙", 1700)},
		{"user", strings.Repeat("丙", 1700)},
		{"assistant", strings.Repeat("丁", 1700)},
	} {
		createSummaryTestMessage(t, svc.messageStore, chatID, message.role, message.content)
	}

	queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID))
	if err != nil {
		t.Fatalf("queue summary: %v", err)
	}
	if !queued {
		t.Fatal("expected a pending summary")
	}
	state := getSummaryTestState(t, svc, chatID)
	if state.AppliedCutoffSeq != 0 || state.PendingToSeq != 4 || state.PendingStatus != "pending" || !state.SummaryRequired {
		t.Fatalf("unexpected pending state: %+v", state)
	}
}

func TestThresholdReplyQueuesSummaryForTheNextUserMessage(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	var summaryRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req model.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !req.Stream {
			summaryRequests.Add(1)
			writeSummaryResponse(w, summaryTestContent)
			return
		}
		writeChatStream(w, strings.Repeat("乙", 1700))
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)

	chatService := newAsyncSummaryChatService(db, svc)
	if _, err := chatService.SendMessage(chatID, strings.Repeat("甲", 1700), "", "summary-user", func(string) error { return nil }); err != nil {
		t.Fatalf("send message: %v", err)
	}

	state := waitForSummaryState(t, svc, chatID, func(state *model.ChatSummaryState) bool {
		return state.PendingToSeq == 2 && state.PendingStatus == "pending"
	})
	if state.PendingToSeq != 2 || state.PendingStatus != "pending" {
		t.Fatalf("threshold reply did not leave a pending job: %+v", state)
	}
	time.Sleep(100 * time.Millisecond)
	if summaryRequests.Load() != 0 {
		t.Fatalf("summary started in the threshold turn: %d requests", summaryRequests.Load())
	}
}

func TestPendingSummaryDoesNotBlockChatAndOnlyAffectsLaterContext(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}

	summaryStarted := make(chan struct{})
	releaseSummary := make(chan struct{})
	var startOnce sync.Once
	var chatSawPendingSummary atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req model.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !req.Stream {
			startOnce.Do(func() { close(summaryStarted) })
			<-releaseSummary
			writeSummaryResponse(w, summaryTestContent)
			return
		}
		for _, message := range req.Messages {
			if strings.Contains(message.Content, "[Summary Memory]") {
				chatSawPendingSummary.Store(true)
			}
		}
		writeChatStream(w, "她轻轻点头。")
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)

	chatService := newAsyncSummaryChatService(db, svc)
	sendDone := make(chan error, 1)
	go func() {
		_, err := chatService.SendMessage(chatID, "下一条用户消息", "", "summary-user", func(string) error { return nil })
		sendDone <- err
	}()

	select {
	case <-summaryStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("background summary did not start after the next user message")
	}
	select {
	case err := <-sendDone:
		if err != nil {
			t.Fatalf("chat failed while summary was waiting: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("chat waited for the background summary")
	}

	historyWhilePending := listSummaryTestMessages(t, svc.messageStore, chatID)
	contextWhilePending, rawWhilePending := svc.BuildServiceModeContext(chatID, historyWhilePending)
	if contextWhilePending != "" || len(rawWhilePending) != len(historyWhilePending) {
		t.Fatalf("pending summary was used before success: context=%q raw=%d history=%d", contextWhilePending, len(rawWhilePending), len(historyWhilePending))
	}
	if chatSawPendingSummary.Load() {
		t.Fatal("current chat request used a summary that had not returned")
	}
	if len(historyWhilePending) != 4 {
		t.Fatalf("current exchange was not stored independently: %d messages", len(historyWhilePending))
	}

	close(releaseSummary)
	waitForSummaryState(t, svc, chatID, func(state *model.ChatSummaryState) bool {
		if state.AppliedCutoffSeq != 2 || state.PendingToSeq != 0 || state.PendingStatus != "" {
			return false
		}
		context, _ := svc.BuildServiceModeContext(chatID, listSummaryTestMessages(t, svc.messageStore, chatID))
		return strings.Contains(context, "剧情继续推进")
	})

	historyAfterSuccess := listSummaryTestMessages(t, svc.messageStore, chatID)
	contextAfterSuccess, rawAfterSuccess := svc.BuildServiceModeContext(chatID, historyAfterSuccess)
	if !strings.Contains(contextAfterSuccess, "剧情继续推进") {
		t.Fatalf("successful summary was not promoted: %q", contextAfterSuccess)
	}
	if len(rawAfterSuccess) != 2 || rawAfterSuccess[0].Seq != 3 || rawAfterSuccess[1].Seq != 4 {
		t.Fatalf("expected only messages after summary seq 2, got %+v", rawAfterSuccess)
	}
	activeSummary, err := svc.summaryStore.GetActiveBigChunk(chatID)
	if err != nil || activeSummary == nil {
		t.Fatalf("read active summary: chunk=%+v err=%v", activeSummary, err)
	}
	if activeSummary.ToSeq != 2 || activeSummary.ToMessageID != historyAfterSuccess[1].ID {
		t.Fatalf("summary did not record its exact ending message: %+v", activeSummary)
	}
}

func TestDelayedTurnEvaluationUsesLatestHistory(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "短消息")

	summaryStarted := make(chan struct{})
	releaseSummary := make(chan struct{})
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		startOnce.Do(func() { close(summaryStarted) })
		<-releaseSummary
		writeSummaryResponse(w, summaryTestContent)
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)

	stage := svc.StartTurnSummaryAsync(chatID, 3)
	select {
	case <-summaryStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("summary request did not start")
	}
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "短回复")
	// This delayed evaluator starts with an older turn, but must reload history
	// after the summary stage rather than publish that old snapshot.
	evaluationDone := svc.FinishTurnSummaryAsync(chatID, stage)

	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("丙", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("丁", 1700))
	close(releaseSummary)
	select {
	case <-evaluationDone:
	case <-time.After(2 * time.Second):
		t.Fatal("delayed turn evaluation did not finish")
	}

	state := getSummaryTestState(t, svc, chatID)
	if !state.SummaryRequired {
		t.Fatalf("delayed older turn cleared the latest summary requirement: %+v", state)
	}
	if state.EligibilitySeq != 6 {
		t.Fatalf("latest eligibility boundary was not recorded: %+v", state)
	}
	if _, err := svc.summaryStore.UpdateSummaryEligibility(chatID, false, 2, 0, 4); err != nil {
		t.Fatalf("submit stale eligibility snapshot: %v", err)
	}
	state = getSummaryTestState(t, svc, chatID)
	if !state.SummaryRequired || state.EligibilitySeq != 6 {
		t.Fatalf("stale eligibility snapshot overwrote newer state: %+v", state)
	}
}

func TestSummaryFailureKeepsCurrentExchangeAndPreviousSummary(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "旧用户消息")
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "旧助手回复")
	oldSummary := strings.Replace(summaryTestContent, "剧情继续推进", "旧摘要仍然可用", 1)
	createSuccessfulSummary(t, svc, chatID, 2, oldSummary)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("丙", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("丁", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}

	var chatSawOldSummary atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req model.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !req.Stream {
			http.Error(w, "summary unavailable", http.StatusBadGateway)
			return
		}
		for _, message := range req.Messages {
			if strings.Contains(message.Content, "旧摘要仍然可用") {
				chatSawOldSummary.Store(true)
			}
		}
		writeChatStream(w, "聊天回复照常返回。")
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)

	chatService := newAsyncSummaryChatService(db, svc)
	if _, err := chatService.SendMessage(chatID, "即使摘要失败也继续", "", "summary-user", func(string) error { return nil }); err != nil {
		t.Fatalf("chat was blocked by summary failure: %v", err)
	}
	waitForSummaryState(t, svc, chatID, func(state *model.ChatSummaryState) bool {
		return state.PendingStatus == "failed" && state.PendingToSeq == 6
	})

	state := getSummaryTestState(t, svc, chatID)
	if state.AppliedCutoffSeq != 2 || state.PendingToSeq != 6 || state.PendingError == "" {
		t.Fatalf("unexpected failed summary state: %+v", state)
	}
	if !chatSawOldSummary.Load() {
		t.Fatal("chat did not keep using the previous successful summary")
	}
	if messages := listSummaryTestMessages(t, svc.messageStore, chatID); len(messages) != 6 {
		t.Fatalf("summary failure lost the current exchange: %d messages", len(messages))
	}
	active, err := svc.summaryStore.GetActiveBigChunk(chatID)
	if err != nil || active == nil || active.ToSeq != 2 || active.Content != oldSummary {
		t.Fatalf("previous summary was replaced after failure: chunk=%+v err=%v", active, err)
	}
}

func TestFailedSummaryWaitsTenAssistantFloorsBeforeRetry(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}

	firstJob, err := svc.summaryStore.ClaimPendingSummary(chatID, time.Now().Add(-summaryJobLease))
	if err != nil || firstJob == nil {
		t.Fatalf("claim first summary: job=%+v err=%v", firstJob, err)
	}
	if err := svc.summaryStore.FailPendingSummary(firstJob, errors.New("summary failed")); err != nil {
		t.Fatalf("fail first summary: %v", err)
	}
	state := getSummaryTestState(t, svc, chatID)
	if state.NextSummaryFloor != 11 || !state.SummaryRequired || state.PendingStatus != "failed" {
		t.Fatalf("summary attempt did not add ten floors: %+v", state)
	}

	for floor := 2; floor <= 10; floor++ {
		createSummaryTestMessage(t, svc.messageStore, chatID, "user", fmt.Sprintf("user-%d", floor))
		createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", fmt.Sprintf("assistant-%d", floor))
		if _, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil {
			t.Fatalf("update floor %d: %v", floor, err)
		}
	}
	job, err := svc.summaryStore.ClaimPendingSummary(chatID, time.Now().Add(-summaryJobLease))
	if err != nil || job != nil {
		t.Fatalf("summary retried before ten floors elapsed: job=%+v err=%v", job, err)
	}

	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "user-11")
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "assistant-11")
	if _, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil {
		t.Fatalf("update floor 11: %v", err)
	}
	job, err = svc.summaryStore.ClaimPendingSummary(chatID, time.Now().Add(-summaryJobLease))
	if err != nil || job == nil {
		t.Fatalf("summary was not retryable after ten floors: job=%+v err=%v", job, err)
	}
	if job.ToSeq != 22 {
		t.Fatalf("retry did not advance to the latest assistant boundary: %+v", job)
	}
	if state := getSummaryTestState(t, svc, chatID); state.NextSummaryFloor != 21 {
		t.Fatalf("second attempt did not advance another ten floors: %+v", state)
	}
}

func TestSummaryNetworkWaitDoesNotLockChatDeletion(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var startOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		startOnce.Do(func() { close(requestStarted) })
		<-releaseRequest
		http.Error(w, "summary unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)

	runDone := make(chan error, 1)
	go func() { runDone <- svc.runPendingSummary(chatID) }()
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("summary request did not start")
	}

	deleteDone := make(chan error, 1)
	go func() { deleteDone <- store.NewChatStore(db).Delete(chatID, "summary-user") }()
	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatalf("delete chat while summary waits: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("chat deletion was blocked by the summary network request")
	}
	close(releaseRequest)
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("summary worker did not stop after request release")
	}
}

func TestSummaryContextAndWarningNeverWaitForDatabase(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "旧用户消息")
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "旧助手回复")
	createSuccessfulSummary(t, svc, chatID, 2, summaryTestContent)
	if err := svc.refreshRuntimeCache(chatID); err != nil {
		t.Fatalf("prime runtime cache: %v", err)
	}
	history := listSummaryTestMessages(t, svc.messageStore, chatID)

	db.SetMaxOpenConns(1)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("hold database connection: %v", err)
	}
	defer tx.Rollback()

	result := make(chan string, 1)
	go func() {
		context, _ := svc.BuildServiceModeContext(chatID, history)
		_ = svc.Warning(chatID)
		result <- context
	}()
	select {
	case context := <-result:
		if !strings.Contains(context, "剧情继续推进") {
			t.Fatalf("cached summary was not returned: %q", context)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("chat-path summary read waited for a database connection")
	}
}

func TestSummarySchedulingReturnsWhileDatabaseIsBusy(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "旧用户消息")
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "旧助手回复")
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "当前用户消息")
	current := listSummaryTestMessages(t, svc.messageStore, chatID)

	db.SetMaxOpenConns(1)
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("hold database connection: %v", err)
	}

	started := time.Now()
	stage := svc.StartTurnSummaryAsync(chatID, current[len(current)-1].Seq)
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		_ = tx.Rollback()
		t.Fatalf("summary scheduling blocked its caller for %s", elapsed)
	}
	select {
	case <-stage:
		t.Fatal("background database stage unexpectedly completed while its only connection was held")
	case <-time.After(50 * time.Millisecond):
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("release database connection: %v", err)
	}
	select {
	case <-stage:
	case <-time.After(2 * time.Second):
		t.Fatal("background summary stage did not resume after database release")
	}
}

func TestBuildContextWithoutSummaryKeepsFullHistoryAndWarnsAfter100Messages(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for i := 1; i <= 100; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		createSummaryTestMessage(t, svc.messageStore, chatID, role, fmt.Sprintf("message-%d", i))
	}
	if warning := svc.Warning(chatID); warning != "" {
		t.Fatalf("warning appeared at 100 messages: %q", warning)
	}
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "message-101")

	history := listSummaryTestMessages(t, svc.messageStore, chatID)
	context, raw := svc.BuildServiceModeContext(chatID, history)
	if context != "" || len(raw) != 101 {
		t.Fatalf("history was silently trimmed without a summary: context=%q raw=%d", context, len(raw))
	}
	if _, err := svc.QueueSummaryIfNeeded(chatID, history); err != nil {
		t.Fatalf("evaluate summary warning: %v", err)
	}
	warning := svc.Warning(chatID)
	if !strings.Contains(warning, "101 条消息") || !strings.Contains(warning, "完整历史") {
		t.Fatalf("unexpected warning: %q", warning)
	}
}

func TestInvalidateCancelsRunningSummaryAndKeepsOlderCoverage(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "旧用户消息")
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "旧助手回复")
	createSuccessfulSummary(t, svc, chatID, 2, summaryTestContent)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("丙", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("丁", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}
	job, err := svc.summaryStore.ClaimPendingSummary(chatID, time.Now().Add(-summaryJobLease))
	if err != nil || job == nil {
		t.Fatalf("claim summary: job=%+v err=%v", job, err)
	}

	svc.InvalidateFromSeq(chatID, 3)
	if err := svc.summaryStore.CompletePendingSummary(job, summaryTestContent, 3000); !errors.Is(err, store.ErrSummaryStateChanged) {
		t.Fatalf("stale summary was allowed to commit: %v", err)
	}
	state := getSummaryTestState(t, svc, chatID)
	if state.AppliedCutoffSeq != 2 || state.PendingToSeq != 0 || state.PendingStatus != "" {
		t.Fatalf("invalidation damaged older coverage or left a job: %+v", state)
	}
	context := waitForSummaryContext(t, svc, chatID, func(context string, _ []*model.Message) bool {
		return strings.Contains(context, "剧情继续推进")
	})
	if !strings.Contains(context, "剧情继续推进") {
		t.Fatalf("older valid summary was not retained: %q", context)
	}
}

func TestDeleteMessageRemovesContaminatedSummariesAndRestoresSafeFallback(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	if err := svc.configStore.Set("memory_summary_char_limit", "500"); err != nil {
		t.Fatalf("set summary limit: %v", err)
	}
	for seq := 1; seq <= 6; seq++ {
		role := "user"
		if seq%2 == 0 {
			role = "assistant"
		}
		createSummaryTestMessage(t, svc.messageStore, chatID, role, strings.Repeat(fmt.Sprintf("%d", seq), 300))
	}
	messages := listSummaryTestMessages(t, svc.messageStore, chatID)

	safeSummary := strings.Replace(summaryTestContent, "剧情继续推进", "安全摘要二", 1)
	contaminatedFour := strings.Replace(summaryTestContent, "剧情继续推进", "污染摘要四", 1)
	contaminatedSix := strings.Replace(summaryTestContent, "剧情继续推进", "污染摘要六", 1)
	chunks := []*model.ChatSummaryChunk{
		{ChatID: chatID, Level: "big", FromSeq: 1, ToSeq: 2, ToMessageID: messages[1].ID, Content: safeSummary, Status: "superseded"},
		{ChatID: chatID, Level: "big", FromSeq: 1, ToSeq: 4, ToMessageID: messages[3].ID, Content: contaminatedFour, Status: "superseded"},
		{ChatID: chatID, Level: "big", FromSeq: 1, ToSeq: 6, ToMessageID: messages[5].ID, Content: contaminatedSix, Status: "active"},
	}
	for _, chunk := range chunks {
		if err := svc.summaryStore.CreateChunk(chunk); err != nil {
			t.Fatalf("create summary to %d: %v", chunk.ToSeq, err)
		}
	}
	if err := svc.summaryStore.ApplySmallSummary(chatID, 6); err != nil {
		t.Fatalf("apply latest summary: %v", err)
	}
	if err := svc.summaryStore.SetCurrentBigSummary(chatID, chunks[2].ID); err != nil {
		t.Fatalf("set latest summary: %v", err)
	}

	deleted, err := svc.DeleteMessageAndRecalculate(chatID, messages[2].ID, false)
	if err != nil || deleted != 1 {
		t.Fatalf("delete covered message: deleted=%d err=%v", deleted, err)
	}
	state := getSummaryTestState(t, svc, chatID)
	if state.AppliedCutoffSeq != 2 || state.CurrentBigSummary != chunks[0].ID {
		t.Fatalf("did not restore the nearest safe summary: %+v", state)
	}
	if !state.SummaryRequired || state.NextSummaryFloor != 3 {
		t.Fatalf("summary trigger parameters were not recalculated: %+v", state)
	}

	var chunkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_summary_chunks WHERE chat_id = ?`, chatID).Scan(&chunkCount); err != nil {
		t.Fatalf("count remaining summaries: %v", err)
	}
	if chunkCount != 1 {
		t.Fatalf("contaminated summaries were not physically deleted: %d remain", chunkCount)
	}
	active, err := svc.summaryStore.GetActiveBigChunk(chatID)
	if err != nil || active == nil || active.ID != chunks[0].ID {
		t.Fatalf("safe fallback is not active: chunk=%+v err=%v", active, err)
	}

	history := listSummaryTestMessages(t, svc.messageStore, chatID)
	context := waitForSummaryContext(t, svc, chatID, func(context string, raw []*model.Message) bool {
		return strings.Contains(context, "安全摘要二") && len(raw) == 3
	})
	_, raw := svc.BuildServiceModeContext(chatID, history)
	if !strings.Contains(context, "安全摘要二") || strings.Contains(context, "污染摘要") {
		t.Fatalf("polluted summary leaked into context: %q", context)
	}
	if len(raw) != 3 || raw[0].Seq != 4 {
		t.Fatalf("unexpected raw history after fallback: %+v", raw)
	}
	if queued, err := svc.QueueSummaryIfNeeded(chatID, history); err != nil || !queued {
		t.Fatalf("recalculated parameters did not allow a new safe summary: queued=%v err=%v", queued, err)
	}
}

func TestDeleteMessageRollsBackWhenSummaryCleanupFails(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "user")
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", "assistant")
	messages := listSummaryTestMessages(t, svc.messageStore, chatID)
	createSuccessfulSummary(t, svc, chatID, 2, summaryTestContent)
	if _, err := db.Exec(`
		CREATE TRIGGER fail_summary_cleanup
		BEFORE DELETE ON chat_summary_chunks
		BEGIN
			SELECT RAISE(ABORT, 'forced summary cleanup failure');
		END`); err != nil {
		t.Fatalf("create cleanup trigger: %v", err)
	}

	if _, err := svc.DeleteMessageAndRecalculate(chatID, messages[0].ID, false); err == nil {
		t.Fatal("expected atomic delete to fail")
	}
	if remaining := listSummaryTestMessages(t, svc.messageStore, chatID); len(remaining) != 2 {
		t.Fatalf("message deletion was not rolled back: %+v", remaining)
	}
	var chunkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_summary_chunks WHERE chat_id = ?`, chatID).Scan(&chunkCount); err != nil {
		t.Fatalf("count summaries after rollback: %v", err)
	}
	if chunkCount != 1 {
		t.Fatalf("summary cleanup was not rolled back: %d", chunkCount)
	}
}

func TestCascadeDeletionRejectsInFlightEligibilitySnapshot(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for seq := 1; seq <= 4; seq++ {
		role := "user"
		if seq%2 == 0 {
			role = "assistant"
		}
		createSummaryTestMessage(t, svc.messageStore, chatID, role, fmt.Sprintf("message-%d", seq))
	}
	messages := listSummaryTestMessages(t, svc.messageStore, chatID)
	if _, err := svc.DeleteMessageAndRecalculate(chatID, messages[2].ID, true); err != nil {
		t.Fatalf("cascade delete: %v", err)
	}

	if _, err := svc.summaryStore.UpdateSummaryEligibility(chatID, true, 2, 4, 4); err != nil {
		t.Fatalf("submit deleted eligibility snapshot: %v", err)
	}
	state := getSummaryTestState(t, svc, chatID)
	if state.EligibilitySeq != 2 || state.PendingToSeq != 0 || state.SummaryRequired {
		t.Fatalf("deleted snapshot was allowed to repopulate summary state: %+v", state)
	}
}

func TestRegenerateFailureStillInvalidatesPendingSummary(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "chat unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, svc, server.URL)

	chatService := newAsyncSummaryChatService(db, svc)
	if _, err := chatService.Regenerate(chatID, "summary-user", func(string) error { return nil }); err == nil {
		t.Fatal("expected regeneration to fail")
	}
	state := waitForSummaryState(t, svc, chatID, func(state *model.ChatSummaryState) bool {
		return state.PendingToSeq == 0 && state.PendingStatus == ""
	})
	if state.PendingToSeq != 0 || state.PendingStatus != "" {
		t.Fatalf("failed regeneration left a stale summary job: %+v", state)
	}
	messages := listSummaryTestMessages(t, svc.messageStore, chatID)
	if len(messages) != 1 || messages[0].Role != "user" {
		t.Fatalf("unexpected messages after failed regeneration: %+v", messages)
	}
}

func TestNewSummaryServiceRecoversInterruptedJob(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if queued, err := svc.QueueSummaryIfNeeded(chatID, listSummaryTestMessages(t, svc.messageStore, chatID)); err != nil || !queued {
		t.Fatalf("queue summary: queued=%v err=%v", queued, err)
	}
	if job, err := svc.summaryStore.ClaimPendingSummary(chatID, time.Now().Add(-summaryJobLease)); err != nil || job == nil {
		t.Fatalf("claim summary: job=%+v err=%v", job, err)
	}

	restarted := NewSummaryService(svc.messageStore, svc.summaryStore, svc.characterStore, svc.configStore, svc.userStore)
	state := getSummaryTestState(t, restarted, chatID)
	if state.PendingStatus != "failed" || state.PendingRunID != "" || !strings.Contains(state.PendingError, "服务重启") {
		t.Fatalf("interrupted job was not made retryable: %+v", state)
	}
}

func TestReconcileSummaryStateRepairsNonPrefixCoverage(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	for i := 0; i < 4; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		createSummaryTestMessage(t, svc.messageStore, chatID, role, fmt.Sprintf("message-%d", i+1))
	}

	chunk := &model.ChatSummaryChunk{
		ChatID: chatID, Level: "small", FromSeq: 3, ToSeq: 4,
		Content: summaryTestContent, Status: "active",
	}
	if err := svc.summaryStore.CreateChunk(chunk); err != nil {
		t.Fatalf("create non-prefix chunk: %v", err)
	}
	if err := svc.summaryStore.ApplySmallSummary(chatID, 4); err != nil {
		t.Fatalf("apply invalid cutoff: %v", err)
	}

	state, err := svc.reconcileSummaryState(chatID)
	if err != nil {
		t.Fatalf("reconcile state: %v", err)
	}
	if state.AppliedCutoffSeq != 0 || state.DirtyFromSeq != 1 {
		t.Fatalf("expected cutoff=0 dirty_from=1, got cutoff=%d dirty_from=%d", state.AppliedCutoffSeq, state.DirtyFromSeq)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM chat_summary_chunks WHERE id = ?`, chunk.ID).Scan(&status); err != nil {
		t.Fatalf("read repaired chunk: %v", err)
	}
	if status != "dirty" {
		t.Fatalf("expected non-prefix chunk dirty, got %s", status)
	}
}

func TestParseSummaryChunkAcceptsPlainTextFallback(t *testing.T) {
	parsed, err := parseSummaryChunk("剧情继续推进，双方约定明天见面。")
	if err != nil {
		t.Fatalf("parse plain summary: %v", err)
	}
	if !strings.Contains(parsed, "剧情继续推进") || !strings.Contains(parsed, "<relationship>无</relationship>") {
		t.Fatalf("unexpected fallback summary: %s", parsed)
	}
}

func newSummaryServiceTest(t *testing.T) (*SummaryService, *store.DB, string) {
	t.Helper()
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	configStore := store.NewConfigStore(db)
	if err := configStore.Set("service_mode", "service"); err != nil {
		t.Fatalf("enable service mode: %v", err)
	}
	if err := configStore.Set("default_model", "test-model"); err != nil {
		t.Fatalf("set model: %v", err)
	}

	chatID := "summary-chat"
	if _, err := db.Exec(`
		INSERT INTO characters (id, user_id, name)
		VALUES ('summary-char', 'summary-user', 'Summary Character')`); err != nil {
		t.Fatalf("insert character: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO chats (id, user_id, character_id, title)
		VALUES (?, 'summary-user', 'summary-char', 'Summary Chat')`, chatID); err != nil {
		t.Fatalf("insert chat: %v", err)
	}

	messageStore := store.NewMessageStore(db)
	return NewSummaryService(
		messageStore,
		store.NewSummaryStore(db),
		store.NewCharacterStore(db),
		configStore,
		store.NewUserStore(db),
	), db, chatID
}

func newAsyncSummaryChatService(db *store.DB, summaryService *SummaryService) *ChatService {
	return NewChatService(
		store.NewChatStore(db),
		summaryService.messageStore,
		store.NewCharacterStore(db),
		store.NewPresetStore(db),
		store.NewWorldBookStore(db),
		summaryService.configStore,
		summaryService.userStore,
		summaryService,
	)
}

func createSummaryTestMessage(t *testing.T, messageStore *store.MessageStore, chatID, role, content string) {
	t.Helper()
	if err := messageStore.Create(&model.Message{ChatID: chatID, Role: role, Content: content}); err != nil {
		t.Fatalf("create %s message: %v", role, err)
	}
}

func listSummaryTestMessages(t *testing.T, messageStore *store.MessageStore, chatID string) []*model.Message {
	t.Helper()
	messages, err := messageStore.ListByChatID(chatID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	return messages
}

func getSummaryTestState(t *testing.T, svc *SummaryService, chatID string) *model.ChatSummaryState {
	t.Helper()
	state, err := svc.summaryStore.GetState(chatID)
	if err != nil {
		t.Fatalf("get summary state: %v", err)
	}
	return state
}

func waitForSummaryState(t *testing.T, svc *SummaryService, chatID string, ready func(*model.ChatSummaryState) bool) *model.ChatSummaryState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		state := getSummaryTestState(t, svc, chatID)
		if ready(state) {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	state := getSummaryTestState(t, svc, chatID)
	t.Fatalf("summary state did not become ready: %+v", state)
	return nil
}

func waitForSummaryContext(
	t *testing.T,
	svc *SummaryService,
	chatID string,
	ready func(string, []*model.Message) bool,
) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		history := listSummaryTestMessages(t, svc.messageStore, chatID)
		context, raw := svc.BuildServiceModeContext(chatID, history)
		if ready(context, raw) {
			return context
		}
		time.Sleep(10 * time.Millisecond)
	}
	history := listSummaryTestMessages(t, svc.messageStore, chatID)
	context, raw := svc.BuildServiceModeContext(chatID, history)
	t.Fatalf("summary context did not become ready: context=%q raw=%+v", context, raw)
	return ""
}

func createSuccessfulSummary(t *testing.T, svc *SummaryService, chatID string, toSeq int, content string) {
	t.Helper()
	messages := listSummaryTestMessages(t, svc.messageStore, chatID)
	toMessageID := ""
	for _, message := range messages {
		if message.Seq == toSeq {
			toMessageID = message.ID
			break
		}
	}
	chunk := &model.ChatSummaryChunk{
		ChatID: chatID, Level: "big", FromSeq: 1, ToSeq: toSeq,
		ToMessageID: toMessageID, Content: content, Status: "active",
	}
	if err := svc.summaryStore.CreateChunk(chunk); err != nil {
		t.Fatalf("create summary chunk: %v", err)
	}
	if err := svc.summaryStore.ApplySmallSummary(chatID, toSeq); err != nil {
		t.Fatalf("apply summary cutoff: %v", err)
	}
	if err := svc.summaryStore.SetCurrentBigSummary(chatID, chunk.ID); err != nil {
		t.Fatalf("set current summary: %v", err)
	}
}

func setSummaryTestEndpoint(t *testing.T, svc *SummaryService, endpoint string) {
	t.Helper()
	if err := svc.configStore.Set("api_endpoint", endpoint); err != nil {
		t.Fatalf("set API endpoint: %v", err)
	}
}

func writeSummaryResponse(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
	})
}

func writeChatStream(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "text/event-stream")
	payload, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"delta": map[string]string{"content": content}}},
	})
	_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", payload)
}
