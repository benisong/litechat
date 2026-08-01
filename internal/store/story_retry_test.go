package store

import "testing"

func TestMarkStoryTurnProcessingAtomicallyRetriesConflict(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO chat_story_states (chat_id, manifest_id, state_json, scheduler_status)
		VALUES ('chat-retry', 'manifest-1', '{"trust":10}', 'conflict');
		INSERT INTO chat_scheduler_records (id, chat_id, user_message_id, assistant_message_id, turn_seq, status, attempt_count)
		VALUES ('record-retry', 'chat-retry', 'user-1', 'assistant-1', 1, 'conflict', 2);`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := NewSchedulerStore(db).MarkStoryTurnProcessing("chat-retry", "record-retry"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	var recordStatus, stateStatus string
	var attempts int
	if err := db.QueryRow("SELECT status, attempt_count FROM chat_scheduler_records WHERE id = ?", "record-retry").Scan(&recordStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT scheduler_status FROM chat_story_states WHERE chat_id = ?", "chat-retry").Scan(&stateStatus); err != nil {
		t.Fatal(err)
	}
	if recordStatus != "processing" || stateStatus != "processing" || attempts != 3 {
		t.Fatalf("unexpected retry state: record=%s state=%s attempts=%d", recordStatus, stateStatus, attempts)
	}
}
