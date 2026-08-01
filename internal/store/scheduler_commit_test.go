package store

import (
	"litechat/internal/model"
	"testing"
)

func TestCommitSchedulerTurnAtomicallyUpdatesStateEventsAndRecord(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewSchedulerStore(db)
	state := &model.ChatStoryState{
		ChatID:     "chat-1",
		ManifestID: "manifest-1",
		StateJSON:  `{"trust":80}`,
		Route:      "survival",
	}
	if err := store.CreateStoryState(state); err != nil {
		t.Fatalf("CreateStoryState: %v", err)
	}
	record := &model.ChatSchedulerRecord{
		ChatID:             "chat-1",
		UserMessageID:      "user-1",
		AssistantMessageID: "assistant-1",
		TurnSeq:            2,
	}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if err := store.MarkProcessing(record.ID); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}

	state.StateJSON = `{"trust":85}`
	events := []model.ChatStoryEvent{{
		ChatID:            "chat-1",
		SchedulerRecordID: record.ID,
		EventKey:          "public_refusal_request",
		EventType:         "major_event",
		Summary:           "柳如烟公开要求让出资源",
		Importance:        "major",
		Evidence:          "现场出现资源让渡请求",
	}}
	if err := store.CommitSchedulerTurn(record.ID, state, 0, `{"observations":[]}`, `{"trust":5}`, "当前场景", events); err != nil {
		t.Fatalf("CommitSchedulerTurn: %v", err)
	}

	gotState, err := store.GetStoryState("chat-1")
	if err != nil {
		t.Fatalf("GetStoryState: %v", err)
	}
	if gotState.StateVersion != 1 || gotState.StateJSON != `{"trust":85}` {
		t.Fatalf("unexpected committed state: %+v", gotState)
	}
	gotRecord, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if gotRecord.Status != model.SchedulerStatusSuccess || gotRecord.StateVersionAfter != 1 {
		t.Fatalf("unexpected committed record: %+v", gotRecord)
	}
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_story_events WHERE chat_id = ?`, "chat-1").Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one event, got %d", eventCount)
	}
}

func TestCommitSchedulerTurnRollsBackOnStateVersionConflict(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewSchedulerStore(db)
	state := &model.ChatStoryState{ChatID: "chat-1", ManifestID: "manifest-1", StateJSON: `{}`}
	if err := store.CreateStoryState(state); err != nil {
		t.Fatalf("CreateStoryState: %v", err)
	}
	record := &model.ChatSchedulerRecord{ChatID: "chat-1", UserMessageID: "user-1", AssistantMessageID: "assistant-1", TurnSeq: 2}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if err := store.MarkProcessing(record.ID); err != nil {
		t.Fatalf("MarkProcessing: %v", err)
	}

	state.StateJSON = `{"unexpected":true}`
	err := store.CommitSchedulerTurn(record.ID, state, 99, `{}`, `{}`, "", []model.ChatStoryEvent{{
		ChatID: "chat-1", SchedulerRecordID: record.ID, EventKey: "must_not_commit", Summary: "must not commit",
	}})
	if err == nil {
		t.Fatal("expected version conflict")
	}
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_story_events WHERE event_key = ?`, "must_not_commit").Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("expected rollback, found %d events", eventCount)
	}
	gotRecord, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if gotRecord.Status != model.SchedulerStatusProcessing {
		t.Fatalf("record should remain processing after rollback, got %s", gotRecord.Status)
	}
}
