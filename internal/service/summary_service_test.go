package service

import (
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

func TestScheduleSmallSummaryBacklogUsesBoundedContiguousRange(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
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

	if err := svc.scheduleSmallIfNeeded(chatID, false); err != nil {
		t.Fatalf("schedule small summary: %v", err)
	}

	var fromSeq, toSeq, baseCutoff int
	if err := db.QueryRow(`
		SELECT from_seq, to_seq, base_cutoff_seq
		FROM chat_summary_jobs
		WHERE chat_id = ? AND job_type = 'small' AND status = 'pending'`, chatID,
	).Scan(&fromSeq, &toSeq, &baseCutoff); err != nil {
		t.Fatalf("read scheduled job: %v", err)
	}
	if fromSeq != 1 || toSeq != 2 || baseCutoff != 0 {
		t.Fatalf("expected first bounded range 1-2 from cutoff 0, got %d-%d from %d", fromSeq, toSeq, baseCutoff)
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

func TestScheduleSmallJobDoesNotOverlapRunningJob(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	if err := svc.summaryStore.ScheduleSmallJob(chatID, 1, 2, 0); err != nil {
		t.Fatalf("schedule first job: %v", err)
	}
	job, err := svc.summaryStore.ClaimNextJob()
	if err != nil || job == nil {
		t.Fatalf("claim first job: job=%v err=%v", job, err)
	}

	if err := svc.summaryStore.ScheduleSmallJob(chatID, 1, 4, 0); err != nil {
		t.Fatalf("schedule while running: %v", err)
	}
	var openCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM chat_summary_jobs
		WHERE chat_id = ? AND job_type = 'small' AND status IN ('pending', 'running', 'failed')`, chatID,
	).Scan(&openCount); err != nil {
		t.Fatalf("count open jobs: %v", err)
	}
	if openCount != 1 {
		t.Fatalf("expected only the running job, got %d open jobs", openCount)
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

func TestFailedBigJobIsNotDuplicated(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	if err := svc.summaryStore.ScheduleBigJob(chatID, 1, 10, 0); err != nil {
		t.Fatalf("schedule big job: %v", err)
	}
	job, err := svc.summaryStore.ClaimNextJob()
	if err != nil || job == nil {
		t.Fatalf("claim big job: job=%v err=%v", job, err)
	}
	if err := svc.summaryStore.FailJob(job.ID, 1, time.Now().Add(time.Hour), "temporary failure"); err != nil {
		t.Fatalf("fail big job: %v", err)
	}
	if err := svc.summaryStore.ScheduleBigJob(chatID, 1, 10, 0); err != nil {
		t.Fatalf("reschedule big job: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_summary_jobs WHERE chat_id = ? AND job_type = 'big'`, chatID).Scan(&count); err != nil {
		t.Fatalf("count big jobs: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one retryable big job, got %d", count)
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

func TestSummaryFailureKeepsOriginalContinuousRange(t *testing.T) {
	svc, db, chatID := newSummaryServiceTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "summary unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	if _, err := db.Exec(`UPDATE configs SET value = ? WHERE key = 'api_endpoint'`, server.URL); err != nil {
		t.Fatalf("set API endpoint: %v", err)
	}
	createSummaryTestMessage(t, svc.messageStore, chatID, "user", strings.Repeat("甲", 1700))
	createSummaryTestMessage(t, svc.messageStore, chatID, "assistant", strings.Repeat("乙", 1700))
	if err := svc.scheduleSmallIfNeeded(chatID, false); err != nil {
		t.Fatalf("schedule job: %v", err)
	}

	processed, err := svc.processNextJob()
	if err != nil || !processed {
		t.Fatalf("process failed job: processed=%v err=%v", processed, err)
	}

	var fromSeq, toSeq, baseCutoff, attempts int
	var status string
	if err := db.QueryRow(`
		SELECT from_seq, to_seq, base_cutoff_seq, attempt_count, status
		FROM chat_summary_jobs WHERE chat_id = ? AND job_type = 'small'`, chatID,
	).Scan(&fromSeq, &toSeq, &baseCutoff, &attempts, &status); err != nil {
		t.Fatalf("read failed job: %v", err)
	}
	if fromSeq != 1 || toSeq != 2 || baseCutoff != 0 || attempts != 1 || status != "failed" {
		t.Fatalf("unexpected retry state: range=%d-%d base=%d attempts=%d status=%s", fromSeq, toSeq, baseCutoff, attempts, status)
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
	if _, err := db.Exec(`UPDATE configs SET value = 'service' WHERE key = 'service_mode'`); err != nil {
		t.Fatalf("enable service mode: %v", err)
	}
	if _, err := db.Exec(`UPDATE configs SET value = 'test-model' WHERE key = 'default_model'`); err != nil {
		t.Fatalf("set model: %v", err)
	}

	chatID := "summary-chat"
	if _, err := db.Exec(`INSERT INTO characters (id, name) VALUES ('summary-char', 'Summary Character')`); err != nil {
		t.Fatalf("insert character: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO chats (id, user_id, character_id, title)
		VALUES (?, 'summary-user', 'summary-char', 'Summary Chat')`, chatID); err != nil {
		t.Fatalf("insert chat: %v", err)
	}

	messageStore := store.NewMessageStore(db)
	summaryStore := store.NewSummaryStore(db)
	return &SummaryService{
		messageStore: messageStore,
		summaryStore: summaryStore,
		configStore:  store.NewConfigStore(db),
		userStore:    store.NewUserStore(db),
		wakeCh:       make(chan struct{}, 1),
	}, db, chatID
}

func createSummaryTestMessage(t *testing.T, messageStore *store.MessageStore, chatID, role, content string) {
	t.Helper()
	if err := messageStore.Create(&model.Message{ChatID: chatID, Role: role, Content: content}); err != nil {
		t.Fatalf("create %s message: %v", role, err)
	}
}
