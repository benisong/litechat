package store

import (
	"litechat/internal/model"
	"testing"
)

func newSchedulerTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

func TestInitSchemaCreatesStoryRuntimeTables(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	for _, table := range []string{
		"story_manifests",
		"chat_story_states",
		"chat_scheduler_records",
		"chat_story_events",
	} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
		if name != table {
			t.Fatalf("expected table %s, got %s", table, name)
		}
	}
}

func TestSchedulerRecordLifecycleAndLatestSuccess(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewSchedulerStore(db)
	record := &model.ChatSchedulerRecord{
		ChatID:             "chat-1",
		UserMessageID:      "user-msg-1",
		AssistantMessageID: "assistant-msg-1",
		TurnSeq:            2,
		InputSnapshot:      `{"input":"hello"}`,
	}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if record.ID == "" || record.Status != model.SchedulerStatusPending {
		t.Fatalf("unexpected created record: %+v", record)
	}

	if err := store.MarkProcessing(record.ID); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	if err := store.MarkSuccess(record.ID, `{"events":[]}`, "当前场景", 0, 1); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}

	latest, err := store.LatestSuccessfulRecord("chat-1")
	if err != nil {
		t.Fatalf("LatestSuccessfulRecord: %v", err)
	}
	if latest.ID != record.ID || latest.Status != model.SchedulerStatusSuccess {
		t.Fatalf("unexpected latest record: %+v", latest)
	}
	if latest.ContextText != "当前场景" || latest.StateVersionAfter != 1 {
		t.Fatalf("unexpected success payload: %+v", latest)
	}
}

func TestSchedulerRecordFailureKeepsError(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewSchedulerStore(db)
	record := &model.ChatSchedulerRecord{
		ChatID:             "chat-1",
		UserMessageID:      "user-msg-1",
		AssistantMessageID: "assistant-msg-1",
		TurnSeq:            2,
	}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if err := store.MarkProcessing(record.ID); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}
	if err := store.MarkFailed(record.ID, "model_timeout", "调度模型超时"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	got, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if got.Status != model.SchedulerStatusFailed || got.ErrorCode != "model_timeout" || got.ErrorMessage != "调度模型超时" {
		t.Fatalf("unexpected failed record: %+v", got)
	}
}
