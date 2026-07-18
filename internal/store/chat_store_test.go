package store

import (
	"litechat/internal/model"
	"strings"
	"testing"
)

func TestMessageStorePersistsStatusBarOnMessageRow(t *testing.T) {
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

	var storedBody, storedPanel string
	if err := db.QueryRow(`SELECT content, status_bar FROM messages WHERE id = ?`, message.ID).Scan(&storedBody, &storedPanel); err != nil {
		t.Fatalf("read stored body: %v", err)
	}
	if storedBody != "她轻轻点头。" || strings.Contains(storedBody, "状态栏") {
		t.Fatalf("status bar leaked into messages.content: %q", storedBody)
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
	var messageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE id = ?`, message.ID).Scan(&messageCount); err != nil {
		t.Fatalf("count messages after delete: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("message row was not deleted: %d", messageCount)
	}
	messages, err := messageStore.ListForContext(chatID)
	if err != nil {
		t.Fatalf("list context after delete: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("deleted assistant remained in cache: %+v", messages)
	}
}

func TestListForContextUsesCachedLatestAssistant(t *testing.T) {
	db, chatID := newMessageStoreTestDB(t)
	messageStore := NewMessageStore(db)
	userMessage := &model.Message{ChatID: chatID, Role: "user", Content: "hello"}
	if err := messageStore.Create(userMessage); err != nil {
		t.Fatalf("create user message: %v", err)
	}
	assistant := &model.Message{
		ChatID:  chatID,
		Role:    "assistant",
		Content: "cached body\n\n【状态栏】\n地点：缓存地点",
	}
	if err := messageStore.Create(assistant); err != nil {
		t.Fatalf("create assistant message: %v", err)
	}
	if _, err := db.Exec(`UPDATE messages SET content = 'database body' WHERE id = ?`, assistant.ID); err != nil {
		t.Fatalf("change stored assistant for cache assertion: %v", err)
	}

	messages, err := messageStore.ListForContext(chatID)
	if err != nil {
		t.Fatalf("list context: %v", err)
	}
	if len(messages) != 2 || messages[1].Content != "cached body" {
		t.Fatalf("latest assistant was not supplied by cache: %+v", messages)
	}
	if !strings.Contains(messages[1].StatusBar, "缓存地点") {
		t.Fatalf("cached status bar missing: %+v", messages[1])
	}
	rangeMessages, err := messageStore.ListByChatIDRange(chatID, assistant.Seq, assistant.Seq)
	if err != nil {
		t.Fatalf("list summary range: %v", err)
	}
	if len(rangeMessages) != 1 || rangeMessages[0].Content != "cached body" {
		t.Fatalf("summary range did not use cached assistant: %+v", rangeMessages)
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
