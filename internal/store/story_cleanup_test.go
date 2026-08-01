package store

import "testing"

func TestDeleteChatStoryDataRemovesChatScopedRowsButKeepsManifest(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO story_manifests (id, character_id, character_version, worldbook_version_hash, manifest_version, status, compiled_json)
		VALUES ('manifest-keep', 'character-1', 'v1', 'hash', 1, 'ready', '{}');
		INSERT INTO chat_story_states (chat_id, manifest_id, state_json) VALUES ('chat-delete', 'manifest-keep', '{}');
		INSERT INTO chat_scheduler_records (id, chat_id, user_message_id, assistant_message_id, turn_seq)
		VALUES ('record-delete', 'chat-delete', 'user-message', 'assistant-message', 1);
		INSERT INTO chat_story_events (id, chat_id, scheduler_record_id, event_key)
		VALUES ('event-delete', 'chat-delete', 'record-delete', 'event-1');`)
	if err != nil {
		t.Fatalf("seed story rows: %v", err)
	}
	if err := NewSchedulerStore(db).DeleteChatStoryData("chat-delete"); err != nil {
		t.Fatalf("DeleteChatStoryData: %v", err)
	}
	for _, table := range []string{"chat_story_states", "chat_scheduler_records", "chat_story_events"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE chat_id = ?", "chat-delete").Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s still has %d rows", table, count)
		}
	}
	var manifestCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM story_manifests WHERE id = ?", "manifest-keep").Scan(&manifestCount); err != nil {
		t.Fatal(err)
	}
	if manifestCount != 1 {
		t.Fatalf("manifest was deleted")
	}
}
