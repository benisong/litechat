package store

import (
	"litechat/internal/model"
	"strings"
	"testing"
)

func TestMessageStorePersistsStatusBarSeparately(t *testing.T) {
	db, chatID := newMessageStoreTestDB(t)
	messageStore := NewMessageStore(db)
	combined := "她轻轻点头。\n\n'''\n【状态栏】\n地点：图书馆\n关系：信任\n'''"
	message := &model.Message{ChatID: chatID, Role: "assistant", Content: combined}
	if err := messageStore.Create(message); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}
	if message.Content != "她轻轻点头。" || !strings.Contains(message.StatusBar, "地点：图书馆") {
		t.Fatalf("message was not split in memory: %+v", message)
	}

	var storedBody string
	if err := db.QueryRow(`SELECT content FROM messages WHERE id = ?`, message.ID).Scan(&storedBody); err != nil {
		t.Fatalf("read stored body: %v", err)
	}
	if storedBody != "她轻轻点头。" || strings.Contains(storedBody, "状态栏") {
		t.Fatalf("status bar leaked into messages.content: %q", storedBody)
	}
	var storedPanel string
	if err := db.QueryRow(`SELECT content FROM message_status_bars WHERE message_id = ?`, message.ID).Scan(&storedPanel); err != nil {
		t.Fatalf("read stored panel: %v", err)
	}
	if storedPanel != message.StatusBar {
		t.Fatalf("unexpected stored panel: %q", storedPanel)
	}

	loaded, err := messageStore.GetByID(message.ID)
	if err != nil {
		t.Fatalf("load message: %v", err)
	}
	if loaded.Content != message.Content || loaded.StatusBar != message.StatusBar {
		t.Fatalf("separate panel was not hydrated: %+v", loaded)
	}

	if err := messageStore.DeleteByID(message.ID); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	var panelCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM message_status_bars WHERE message_id = ?`, message.ID).Scan(&panelCount); err != nil {
		t.Fatalf("count panels after delete: %v", err)
	}
	if panelCount != 0 {
		t.Fatalf("status panel did not cascade with message: %d", panelCount)
	}
}

func TestMessageStoreDoesNotSplitUserText(t *testing.T) {
	db, chatID := newMessageStoreTestDB(t)
	messageStore := NewMessageStore(db)
	content := "用户讨论【状态栏】这个词，但这不是助手状态面板。"
	message := &model.Message{ChatID: chatID, Role: "user", Content: content}
	if err := messageStore.Create(message); err != nil {
		t.Fatalf("create user message: %v", err)
	}
	if message.Content != content || message.StatusBar != "" {
		t.Fatalf("user text was incorrectly split: %+v", message)
	}
}

func newMessageStoreTestDB(t *testing.T) (*DB, string) {
	t.Helper()
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO characters (id, user_id, name) VALUES ('status-char', 'status-user', 'Status Character')`); err != nil {
		t.Fatalf("insert character: %v", err)
	}
	chatID := "status-chat"
	if _, err := db.Exec(`
		INSERT INTO chats (id, user_id, character_id, title)
		VALUES (?, 'status-user', 'status-char', 'Status Chat')`, chatID); err != nil {
		t.Fatalf("insert chat: %v", err)
	}
	return db, chatID
}
