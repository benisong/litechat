package service

import (
	"errors"
	"strings"
	"testing"

	"litechat/internal/model"
	"litechat/internal/store"
)

const testSummaryContent = `<chat_summary>
<plot>plot</plot>
<relationship>relationship</relationship>
<user_facts>facts</user_facts>
<world_state>world</world_state>
<open_loops>loops</open_loops>
</chat_summary>`

func TestSummaryJobFailuresExhaustedReschedulesFromLatestUserNode(t *testing.T) {
	db := newSummaryTestDB(t)
	defer db.Close()

	chatID := "chat-retry"
	seedSummaryChat(t, db, chatID)

	messageStore := store.NewMessageStore(db)
	summaryStore := store.NewSummaryStore(db)
	service := &SummaryService{
		messageStore: messageStore,
		summaryStore: summaryStore,
		wakeCh:       make(chan struct{}, 1),
	}

	for _, msg := range []struct {
		role    string
		content string
	}{
		{"user", "first user"},
		{"assistant", "first assistant"},
		{"user", "latest user"},
		{"assistant", "latest assistant"},
	} {
		if err := messageStore.Create(&model.Message{ChatID: chatID, Role: msg.role, Content: msg.content}); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}

	if err := summaryStore.ApplySmallSummary(chatID, 2); err != nil {
		t.Fatalf("apply existing cutoff: %v", err)
	}
	if err := summaryStore.ScheduleSmallJob(chatID, 1, 4, 0); err != nil {
		t.Fatalf("schedule original job: %v", err)
	}

	job, err := summaryStore.ClaimNextJob()
	if err != nil {
		t.Fatalf("claim original job: %v", err)
	}
	if job == nil {
		t.Fatal("expected original job")
	}

	if err := service.handleSummaryJobFailuresExhausted(job, errors.New("bad summary format")); err != nil {
		t.Fatalf("handle exhausted failures: %v", err)
	}

	var oldStatus, oldError string
	if err := db.QueryRow(`SELECT status, last_error FROM chat_summary_jobs WHERE id = ?`, job.ID).Scan(&oldStatus, &oldError); err != nil {
		t.Fatalf("read old job: %v", err)
	}
	if oldStatus != "stale" {
		t.Fatalf("expected old job stale, got %s", oldStatus)
	}
	if !strings.Contains(oldError, "resummarizing from user seq 3") {
		t.Fatalf("expected old job error to mention user seq fallback, got %s", oldError)
	}

	var fromSeq, toSeq, baseCutoffSeq int
	var newStatus string
	if err := db.QueryRow(`
		SELECT from_seq, to_seq, base_cutoff_seq, status
		FROM chat_summary_jobs
		WHERE chat_id = ? AND job_type = 'small' AND id != ?`,
		chatID, job.ID,
	).Scan(&fromSeq, &toSeq, &baseCutoffSeq, &newStatus); err != nil {
		t.Fatalf("read rescheduled job: %v", err)
	}
	if fromSeq != 3 || toSeq != 4 || baseCutoffSeq != 2 || newStatus != "pending" {
		t.Fatalf("unexpected rescheduled job: from=%d to=%d base=%d status=%s", fromSeq, toSeq, baseCutoffSeq, newStatus)
	}

	state, err := summaryStore.GetState(chatID)
	if err != nil {
		t.Fatalf("read summary state: %v", err)
	}
	if state.AppliedCutoffSeq != 2 || state.DirtyFromSeq != 3 {
		t.Fatalf("unexpected summary state: cutoff=%d dirty=%d", state.AppliedCutoffSeq, state.DirtyFromSeq)
	}
}

func TestFallbackSummaryStartingAfterFirstMessageIsUsableWithoutTrimmingHistory(t *testing.T) {
	db := newSummaryTestDB(t)
	defer db.Close()

	chatID := "chat-context"
	seedSummaryChat(t, db, chatID)
	if _, err := db.Exec(`UPDATE configs SET value = 'service' WHERE key = 'service_mode'`); err != nil {
		t.Fatalf("enable service mode: %v", err)
	}

	summaryStore := store.NewSummaryStore(db)
	if err := summaryStore.CreateChunk(&model.ChatSummaryChunk{
		ChatID:  chatID,
		Level:   "small",
		FromSeq: 3,
		ToSeq:   4,
		Content: testSummaryContent,
		Status:  "active",
	}); err != nil {
		t.Fatalf("create fallback chunk: %v", err)
	}
	if err := summaryStore.ApplySmallSummary(chatID, 4); err != nil {
		t.Fatalf("apply fallback cutoff: %v", err)
	}

	service := &SummaryService{
		summaryStore: summaryStore,
		userStore:    store.NewUserStore(db),
	}
	history := []*model.Message{
		{Seq: 1, Role: "user", Content: "old user"},
		{Seq: 2, Role: "assistant", Content: "old assistant"},
		{Seq: 3, Role: "user", Content: "latest user"},
		{Seq: 4, Role: "assistant", Content: "latest assistant"},
	}

	context, trimmed := service.BuildServiceModeContext(chatID, history)
	if !strings.Contains(context, "plot") {
		t.Fatalf("expected fallback summary in context, got %q", context)
	}
	if len(trimmed) != len(history) {
		t.Fatalf("expected non-prefix fallback summary to keep full history, got %d messages", len(trimmed))
	}
}

func newSummaryTestDB(t *testing.T) *store.DB {
	t.Helper()

	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return db
}

func seedSummaryChat(t *testing.T, db *store.DB, chatID string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO characters (id, name) VALUES ('char-summary-test', 'Summary Test')`); err != nil {
		t.Fatalf("insert character: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO chats (id, user_id, character_id, title) VALUES (?, 'user-summary-test', 'char-summary-test', 'Summary Test')`,
		chatID,
	); err != nil {
		t.Fatalf("insert chat: %v", err)
	}
}
