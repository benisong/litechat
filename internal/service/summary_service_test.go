package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"litechat/internal/model"
	"litechat/internal/store"
)

const summaryTestContent = `<chat_summary>
<plot>剧情</plot>
<relationship>关系</relationship>
<user_facts>事实</user_facts>
<world_state>世界</world_state>
<open_loops>待办</open_loops>
</chat_summary>`

func TestPlanSummaryForNextReplyWaitsBelowThreshold(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 700))
	history := listSummaryTestMessages(t, svc.messageStore, chatID)

	plan, err := svc.PlanSummaryForNextReply(chatID, history)
	if err != nil {
		t.Fatalf("plan summary: %v", err)
	}
	if plan != nil {
		t.Fatalf("expected no plan below threshold, got %+v", plan)
	}
}

func TestPlanSummaryForNextReplyUsesBoundedHistoryWithoutQueue(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for _, msg := range []struct {
		role    string
		content string
	}{
		{"user", strings.Repeat("甲", 1700)},
		{"assistant", strings.Repeat("乙", 1700)},
		{"user", strings.Repeat("丙", 1700)},
		{"assistant", strings.Repeat("丁", 1700)},
	} {
		createSummaryTestMessage(t, svc.messageStore, chatID, msg.role, msg.content)
	}
	history := listSummaryTestMessages(t, svc.messageStore, chatID)

	plan, err := svc.PlanSummaryForNextReply(chatID, history)
	if err != nil {
		t.Fatalf("plan summary: %v", err)
	}
	if plan == nil {
		t.Fatal("expected a summary plan")
	}
	if plan.baseCutoffSeq != 0 || plan.toSeq != 2 || plan.expectedLatestSeq != 4 {
		t.Fatalf("unexpected plan: cutoff=%d to=%d latest=%d", plan.baseCutoffSeq, plan.toSeq, plan.expectedLatestSeq)
	}
	if plan.expectedLatestID != history[len(history)-1].ID || len(plan.rawMessages) != 2 {
		t.Fatalf("plan does not match the history snapshot: %+v", plan)
	}
}

func TestGeneratePlannedSummaryFailureLeavesCurrentExchangeUnstored(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	history := listSummaryTestMessages(t, svc.messageStore, chatID)
	plan, err := svc.PlanSummaryForNextReply(chatID, history)
	if err != nil || plan == nil {
		t.Fatalf("plan summary: plan=%v err=%v", plan, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "summary unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	if err := svc.configStore.Set("api_endpoint", server.URL); err != nil {
		t.Fatalf("set API endpoint: %v", err)
	}

	if _, err := svc.GeneratePlannedSummary(plan); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("expected summary HTTP error, got %v", err)
	}
	assertSummaryTestStorage(t, db, chatID, 2, 0, 0)
}

func TestSummaryGateDoesNotGenerateOrDisplayReplyWhenSummaryFails(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		http.Error(w, "summary unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	if err := svc.configStore.Set("api_endpoint", server.URL); err != nil {
		t.Fatalf("set API endpoint: %v", err)
	}

	chatService := newSummaryGateChatService(db, svc)
	callbackCount := 0
	_, err := chatService.SendMessage(chatID, "held-user", "", "summary-user", func(string) error {
		callbackCount++
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "消息摘要返回错误") {
		t.Fatalf("expected summary gate error, got %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("summary failure must stop before chat generation, got %d API requests", requestCount)
	}
	if callbackCount != 0 {
		t.Fatalf("summary failure must not display a reply, got %d callbacks", callbackCount)
	}
	assertSummaryTestStorage(t, db, chatID, 2, 0, 0)
}

func TestSummaryModelWaitDoesNotBlockOtherDatabaseWrites(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(releaseRequest)
		}
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		http.Error(w, "summary unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	if err := svc.configStore.Set("api_endpoint", server.URL); err != nil {
		t.Fatalf("set API endpoint: %v", err)
	}

	chatService := newSummaryGateChatService(db, svc)
	sendDone := make(chan error, 1)
	go func() {
		_, err := chatService.SendMessage(chatID, "held-user", "", "summary-user", func(string) error { return nil })
		sendDone <- err
	}()
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("summary request did not start")
	}

	writeDone := make(chan error, 1)
	go func() {
		chatStore := store.NewChatStore(db)
		chat := &model.Chat{CharacterID: "summary-char", Title: "concurrent-chat"}
		if err := chatStore.Create(chat, "summary-user"); err != nil {
			writeDone <- fmt.Errorf("create chat while summary waits: %w", err)
			return
		}
		if err := svc.messageStore.Create(&model.Message{ChatID: chat.ID, Role: "user", Content: "concurrent-message"}); err != nil {
			writeDone <- fmt.Errorf("create message while summary waits: %w", err)
			return
		}
		writeDone <- chatStore.Delete(chat.ID, "summary-user")
	}()
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("database write failed while summary waited: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("database write was blocked by the summary request")
	}

	close(releaseRequest)
	released = true
	select {
	case err := <-sendDone:
		if err == nil || !strings.Contains(err.Error(), "消息摘要返回错误") {
			t.Fatalf("expected summary failure after release, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("summary request did not finish after release")
	}
	assertSummaryTestStorage(t, db, chatID, 2, 0, 0)
}

func TestSummaryGateCommitsBeforeDisplayingBufferedReply(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))

	requestOrder := make([]string, 0, 2)
	chatUsedPendingSummary := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req model.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode model request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if !req.Stream {
			requestOrder = append(requestOrder, "summary")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []any{map[string]any{"message": map[string]string{"content": summaryTestContent}}},
			})
			return
		}

		requestOrder = append(requestOrder, "chat")
		for _, message := range req.Messages {
			if strings.Contains(message.Content, "[Summary Memory]") && strings.Contains(message.Content, "剧情") {
				chatUsedPendingSummary = true
				break
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"她轻轻点头。\"}}]}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(server.Close)
	if err := svc.configStore.Set("api_endpoint", server.URL); err != nil {
		t.Fatalf("set API endpoint: %v", err)
	}

	chatService := newSummaryGateChatService(db, svc)
	callbackCount := 0
	response, err := chatService.SendMessage(chatID, "held-user", "", "summary-user", func(token string) error {
		callbackCount++
		if token != "她轻轻点头。" {
			return fmt.Errorf("unexpected buffered reply: %q", token)
		}
		var persisted int
		if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_id = ?`, chatID).Scan(&persisted); err != nil {
			return err
		}
		if persisted != 4 {
			return fmt.Errorf("reply displayed before commit: only %d messages persisted", persisted)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("send through summary gate: %v", err)
	}
	if response != "她轻轻点头。" || callbackCount != 1 {
		t.Fatalf("unexpected buffered response=%q callbackCount=%d", response, callbackCount)
	}
	if strings.Join(requestOrder, ",") != "summary,chat" {
		t.Fatalf("expected summary before chat, got %v", requestOrder)
	}
	if !chatUsedPendingSummary {
		t.Fatal("chat request did not use the pending summary context")
	}
	assertSummaryTestStorage(t, db, chatID, 4, 2, 1)
}

func TestBuildContextAfterPlannedSummaryUsesPendingSummary(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for i := 1; i <= 30; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		createSummaryTestMessage(t, svc.messageStore, chatID, role, strings.Repeat(fmt.Sprintf("%d", i%10), 1000))
	}
	history := listSummaryTestMessages(t, svc.messageStore, chatID)
	plan, err := svc.PlanSummaryForNextReply(chatID, history)
	if err != nil || plan == nil {
		t.Fatalf("plan summary: plan=%v err=%v", plan, err)
	}

	context, recent, err := svc.BuildContextAfterPlannedSummary(plan, summaryTestContent, history)
	if err != nil {
		t.Fatalf("build pending context: %v", err)
	}
	if !strings.Contains(context, "剧情") || !strings.Contains(context, "会话滚动摘要") {
		t.Fatalf("pending summary missing from context: %s", context)
	}
	if len(recent) >= len(history) {
		t.Fatalf("expected pending summary to trim history, got %d messages", len(recent))
	}
	if chars := countEffectiveChars(recent); chars > summaryMaxRawChars {
		t.Fatalf("expected at most %d effective chars, got %d", summaryMaxRawChars, chars)
	}
}

func TestCommitSummaryAndExchangeWritesAllRecordsAtomically(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for _, role := range []string{"user", "assistant", "user", "assistant"} {
		createSummaryTestMessage(t, svc.messageStore, chatID, role, strings.Repeat(role, 1700))
	}
	history := listSummaryTestMessages(t, svc.messageStore, chatID)
	plan, err := svc.PlanSummaryForNextReply(chatID, history)
	if err != nil || plan == nil {
		t.Fatalf("plan summary: plan=%v err=%v", plan, err)
	}
	if err := svc.CommitSummaryAndExchange(plan, summaryTestContent, "held-user", "held-assistant"); err != nil {
		t.Fatalf("commit summary gate: %v", err)
	}

	messages := listSummaryTestMessages(t, svc.messageStore, chatID)
	if len(messages) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(messages))
	}
	if messages[4].Seq != 5 || messages[4].Role != "user" || messages[4].Content != "held-user" {
		t.Fatalf("unexpected committed user message: %+v", messages[4])
	}
	if messages[5].Seq != 6 || messages[5].Role != "assistant" || messages[5].Content != "held-assistant" {
		t.Fatalf("unexpected committed assistant message: %+v", messages[5])
	}

	state, err := svc.summaryStore.GetState(chatID)
	if err != nil {
		t.Fatalf("get summary state: %v", err)
	}
	if state.AppliedCutoffSeq != plan.toSeq || state.CurrentBigSummary == "" || state.DirtyFromSeq != 0 {
		t.Fatalf("unexpected summary state: %+v", state)
	}
	chunk, err := svc.summaryStore.GetActiveBigChunk(chatID)
	if err != nil {
		t.Fatalf("get active summary: %v", err)
	}
	if chunk.FromSeq != 1 || chunk.ToSeq != plan.toSeq || chunk.Content != summaryTestContent {
		t.Fatalf("unexpected active summary: %+v", chunk)
	}
}

func TestCommitSummaryAndExchangeRejectsChangedHistory(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	plan, err := svc.PlanSummaryForNextReply(chatID, listSummaryTestMessages(t, svc.messageStore, chatID))
	if err != nil || plan == nil {
		t.Fatalf("plan summary: plan=%v err=%v", plan, err)
	}
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", "concurrent-message")

	err = svc.CommitSummaryAndExchange(plan, summaryTestContent, "held-user", "held-assistant")
	if !errors.Is(err, store.ErrSummaryStateChanged) {
		t.Fatalf("expected changed-state error, got %v", err)
	}
	assertSummaryTestStorage(t, db, chatID, 3, 0, 0)
}

func TestCommitSummaryAndExchangeRollsBackEveryWrite(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	plan, err := svc.PlanSummaryForNextReply(chatID, listSummaryTestMessages(t, svc.messageStore, chatID))
	if err != nil || plan == nil {
		t.Fatalf("plan summary: plan=%v err=%v", plan, err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER fail_summary_gate_assistant
		BEFORE INSERT ON messages
		WHEN NEW.role = 'assistant' AND NEW.content = 'held-assistant'
		BEGIN
			SELECT RAISE(ABORT, 'forced assistant insert failure');
		END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if err := svc.CommitSummaryAndExchange(plan, summaryTestContent, "held-user", "held-assistant"); err == nil {
		t.Fatal("expected atomic commit to fail")
	}
	assertSummaryTestStorage(t, db, chatID, 2, 0, 0)
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

func TestResolveCoverageUsesLongestOverlappingPrefix(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	for _, chunk := range []*model.ChatSummaryChunk{
		{ChatID: chatID, Level: "small", FromSeq: 1, ToSeq: 2, Content: summaryTestContent, Status: "active"},
		{ChatID: chatID, Level: "small", FromSeq: 1, ToSeq: 4, Content: summaryTestContent, Status: "active"},
	} {
		if err := svc.summaryStore.CreateChunk(chunk); err != nil {
			t.Fatalf("create overlapping chunk: %v", err)
		}
	}
	if err := svc.summaryStore.ApplySmallSummary(chatID, 4); err != nil {
		t.Fatalf("apply cutoff: %v", err)
	}

	_, smalls, coverageTo, err := svc.resolveUsableSummaryCoverage(chatID, 4)
	if err != nil {
		t.Fatalf("resolve overlap: %v", err)
	}
	if coverageTo != 4 || len(smalls) != 1 || smalls[0].ToSeq != 4 {
		t.Fatalf("expected longest prefix through 4, got coverage=%d chunks=%+v", coverageTo, smalls)
	}
}

func TestBuildContextCapsRawHistoryWithoutSummary(t *testing.T) {
	svc, _, chatID := newSummaryServiceTest(t)
	history := make([]*model.Message, 0, 40)
	for i := 1; i <= 40; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		history = append(history, &model.Message{Seq: i, Role: role, Content: strings.Repeat("长", 1000)})
	}

	context, trimmed := svc.BuildServiceModeContext(chatID, history)
	if context != "" {
		t.Fatalf("expected no summary context, got %q", context)
	}
	if len(trimmed) >= len(history) {
		t.Fatalf("expected raw history to be capped, got %d messages", len(trimmed))
	}
	if chars := countEffectiveChars(trimmed); chars > summaryMaxRawChars {
		t.Fatalf("expected at most %d effective chars, got %d", summaryMaxRawChars, chars)
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
		configStore,
		store.NewUserStore(db),
	), db, chatID
}

func newSummaryGateChatService(db *store.DB, summaryService *SummaryService) *ChatService {
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

func assertSummaryTestStorage(t *testing.T, db *store.DB, chatID string, messageCount, cutoff, chunkCount int) {
	t.Helper()
	var actualMessages int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_id = ?`, chatID).Scan(&actualMessages); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if actualMessages != messageCount {
		t.Fatalf("expected %d messages, got %d", messageCount, actualMessages)
	}

	var actualCutoff int
	if err := db.QueryRow(`SELECT applied_cutoff_seq FROM chat_summary_state WHERE chat_id = ?`, chatID).Scan(&actualCutoff); err != nil {
		t.Fatalf("read summary cutoff: %v", err)
	}
	if actualCutoff != cutoff {
		t.Fatalf("expected cutoff %d, got %d", cutoff, actualCutoff)
	}

	var actualChunks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_summary_chunks WHERE chat_id = ?`, chatID).Scan(&actualChunks); err != nil {
		t.Fatalf("count summary chunks: %v", err)
	}
	if actualChunks != chunkCount {
		t.Fatalf("expected %d chunks, got %d", chunkCount, actualChunks)
	}
}
