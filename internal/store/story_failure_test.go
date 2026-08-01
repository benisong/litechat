package store

import "testing"

func TestMarkStoryTurnFailedPausesAfterThreshold(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO chat_story_states (chat_id, manifest_id, state_json, scheduler_status, failure_count)
		VALUES ('chat-pause', 'manifest-1', '{}', 'failed', 2);
		INSERT INTO chat_scheduler_records (id, chat_id, user_message_id, assistant_message_id, turn_seq, status)
		VALUES ('record-pause', 'chat-pause', 'user-1', 'assistant-1', 1, 'processing');`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := NewSchedulerStore(db).MarkStoryTurnFailed("chat-pause", "record-pause", "invalid_output", "bad output"); err != nil {
		t.Fatal(err)
	}
	var status string
	var failures int
	if err := db.QueryRow("SELECT scheduler_status, failure_count FROM chat_story_states WHERE chat_id = ?", "chat-pause").Scan(&status, &failures); err != nil {
		t.Fatal(err)
	}
	if status != "paused" || failures != 3 {
		t.Fatalf("unexpected pause state: %s %d", status, failures)
	}
}

func TestMarkStoryTurnFailedPreservesStateAndIncrementsFailureCount(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO chat_story_states (chat_id, manifest_id, state_version, state_json, scheduler_status, failure_count)
		VALUES ('chat-fail', 'manifest-1', 3, '{"trust":10}', 'processing', 1);
		INSERT INTO chat_scheduler_records (id, chat_id, user_message_id, assistant_message_id, turn_seq, status)
		VALUES ('record-fail', 'chat-fail', 'user-1', 'assistant-1', 1, 'processing');`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := NewSchedulerStore(db).MarkStoryTurnFailed("chat-fail", "record-fail", "invalid_output", "bad scheduler output"); err != nil {
		t.Fatalf("MarkStoryTurnFailed: %v", err)
	}
	var stateJSON, status string
	var version, failures int
	if err := db.QueryRow("SELECT state_json, state_version, scheduler_status, failure_count FROM chat_story_states WHERE chat_id = ?", "chat-fail").Scan(&stateJSON, &version, &status, &failures); err != nil {
		t.Fatal(err)
	}
	if stateJSON != `{"trust":10}` || version != 3 || status != "failed" || failures != 2 {
		t.Fatalf("unexpected state: %q %d %s %d", stateJSON, version, status, failures)
	}
}
