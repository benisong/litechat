package store

import (
	"fmt"
	"litechat/internal/model"
	"strings"
	"testing"
	"time"
)

func TestMigrateLegacySummariesToSeparateDatabase(t *testing.T) {
	dataDir := t.TempDir()
	mainDB, err := NewDB(dataDir)
	if err != nil {
		t.Fatalf("open main database: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	if err := mainDB.InitSchema(); err != nil {
		t.Fatalf("init main schema: %v", err)
	}
	insertSummaryDatabaseTestChat(t, mainDB, "migration-chat")

	legacyStore := NewSummaryStore(mainDB)
	if err := legacyStore.EnsureState("migration-chat"); err != nil {
		t.Fatalf("ensure legacy state: %v", err)
	}
	chunk := &model.ChatSummaryChunk{
		ChatID: "migration-chat", Level: "big", FromSeq: 1, ToSeq: 2,
		ToMessageID: "message-2", Content: "legacy summary", Status: "active",
	}
	if err := legacyStore.CreateChunk(chunk); err != nil {
		t.Fatalf("create legacy summary: %v", err)
	}
	if err := legacyStore.ApplySmallSummary("migration-chat", 2); err != nil {
		t.Fatalf("apply legacy summary: %v", err)
	}
	if err := legacyStore.SetCurrentBigSummary("migration-chat", chunk.ID); err != nil {
		t.Fatalf("set legacy summary: %v", err)
	}

	summaryDB, err := NewSummaryDB(dataDir)
	if err != nil {
		t.Fatalf("open summary database: %v", err)
	}
	t.Cleanup(func() { _ = summaryDB.Close() })
	if err := summaryDB.InitSummarySchema(); err != nil {
		t.Fatalf("init summary schema: %v", err)
	}
	if err := MigrateLegacySummaries(mainDB, summaryDB); err != nil {
		t.Fatalf("migrate legacy summaries: %v", err)
	}

	separateStore := NewSummaryStore(summaryDB, mainDB)
	state, err := separateStore.GetState("migration-chat")
	if err != nil {
		t.Fatalf("read migrated state: %v", err)
	}
	if state.AppliedCutoffSeq != 2 || state.CurrentBigSummary != chunk.ID {
		t.Fatalf("unexpected migrated state: %+v", state)
	}
	migrated, err := separateStore.GetActiveBigChunk("migration-chat")
	if err != nil || migrated == nil || migrated.Content != "legacy summary" {
		t.Fatalf("unexpected migrated chunk: chunk=%+v err=%v", migrated, err)
	}
	if _, err := summaryDB.Exec(`DELETE FROM chat_summary_chunks WHERE id = ?`, chunk.ID); err != nil {
		t.Fatalf("delete migrated chunk: %v", err)
	}
	// Later startups must not re-import summaries that the new system invalidated.
	if err := MigrateLegacySummaries(mainDB, summaryDB); err != nil {
		t.Fatalf("repeat legacy migration: %v", err)
	}
	var chunkCount int
	if err := summaryDB.QueryRow(`SELECT COUNT(*) FROM chat_summary_chunks WHERE id = ?`, chunk.ID).Scan(&chunkCount); err != nil {
		t.Fatalf("count chunk after repeated migration: %v", err)
	}
	if chunkCount != 0 {
		t.Fatal("completed migration re-imported an invalidated legacy summary")
	}
}

func TestSummaryWriterLockCannotBlockMainMessageWrites(t *testing.T) {
	dataDir := t.TempDir()
	mainDB, err := NewDB(dataDir)
	if err != nil {
		t.Fatalf("open main database: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	if err := mainDB.InitSchema(); err != nil {
		t.Fatalf("init main schema: %v", err)
	}
	insertSummaryDatabaseTestChat(t, mainDB, "isolation-chat")

	summaryDB, err := NewSummaryDB(dataDir)
	if err != nil {
		t.Fatalf("open summary database: %v", err)
	}
	t.Cleanup(func() { _ = summaryDB.Close() })
	if err := summaryDB.InitSummarySchema(); err != nil {
		t.Fatalf("init summary schema: %v", err)
	}

	lockTx, err := summaryDB.Begin()
	if err != nil {
		t.Fatalf("begin summary writer: %v", err)
	}
	if _, err := lockTx.Exec(`
		INSERT INTO chat_summary_state (chat_id, summary_required, updated_at)
		VALUES ('locked-summary-chat', 1, ?)`, time.Now()); err != nil {
		_ = lockTx.Rollback()
		t.Fatalf("acquire summary writer lock: %v", err)
	}

	writeDone := make(chan error, 1)
	go func() {
		writeDone <- NewMessageStore(mainDB).Create(&model.Message{
			ChatID: "isolation-chat", Role: "user", Content: "normal chat remains writable",
		})
	}()
	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("main message write failed: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		_ = lockTx.Rollback()
		t.Fatal("summary database writer lock blocked the main chat database")
	}
	if err := lockTx.Rollback(); err != nil {
		t.Fatalf("release summary writer lock: %v", err)
	}
}

func TestSeparateSummaryDatabaseDeleteRestoresSafeFallback(t *testing.T) {
	dataDir := t.TempDir()
	mainDB, err := NewDB(dataDir)
	if err != nil {
		t.Fatalf("open main database: %v", err)
	}
	t.Cleanup(func() { _ = mainDB.Close() })
	if err := mainDB.InitSchema(); err != nil {
		t.Fatalf("init main schema: %v", err)
	}
	insertSummaryDatabaseTestChat(t, mainDB, "delete-chat")
	messageStore := NewMessageStore(mainDB)
	for seq := 1; seq <= 4; seq++ {
		role := "user"
		if seq%2 == 0 {
			role = "assistant"
		}
		if err := messageStore.Create(&model.Message{
			ChatID: "delete-chat", Role: role, Content: strings.Repeat(fmt.Sprintf("%d", seq), 400),
		}); err != nil {
			t.Fatalf("create message %d: %v", seq, err)
		}
	}
	messages, err := messageStore.ListByChatID("delete-chat")
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}

	summaryDB, err := NewSummaryDB(dataDir)
	if err != nil {
		t.Fatalf("open summary database: %v", err)
	}
	t.Cleanup(func() { _ = summaryDB.Close() })
	if err := summaryDB.InitSummarySchema(); err != nil {
		t.Fatalf("init summary schema: %v", err)
	}
	summaryStore := NewSummaryStore(summaryDB, mainDB)
	safe := &model.ChatSummaryChunk{
		ChatID: "delete-chat", Level: "big", FromSeq: 1, ToSeq: 2,
		ToMessageID: messages[1].ID, Content: "safe summary", Status: "superseded",
	}
	latest := &model.ChatSummaryChunk{
		ChatID: "delete-chat", Level: "big", FromSeq: 1, ToSeq: 4,
		ToMessageID: messages[3].ID, Content: "contaminated summary", Status: "active",
	}
	for _, chunk := range []*model.ChatSummaryChunk{safe, latest} {
		if err := summaryStore.CreateChunk(chunk); err != nil {
			t.Fatalf("create summary to %d: %v", chunk.ToSeq, err)
		}
	}
	if err := summaryStore.ApplySmallSummary("delete-chat", 4); err != nil {
		t.Fatalf("apply latest summary: %v", err)
	}
	if err := summaryStore.SetCurrentBigSummary("delete-chat", latest.ID); err != nil {
		t.Fatalf("set latest summary: %v", err)
	}

	deleted, err := summaryStore.DeleteMessageAndRecalculate("delete-chat", messages[2].ID, false, 500)
	if err != nil || deleted != 1 {
		t.Fatalf("delete covered message: deleted=%d err=%v", deleted, err)
	}
	state, err := summaryStore.GetState("delete-chat")
	if err != nil {
		t.Fatalf("read recalculated state: %v", err)
	}
	if state.AppliedCutoffSeq != 2 || state.CurrentBigSummary != safe.ID || state.EligibilitySeq != 4 {
		t.Fatalf("safe fallback was not restored: %+v", state)
	}
	active, err := summaryStore.GetActiveBigChunk("delete-chat")
	if err != nil || active == nil || active.ID != safe.ID {
		t.Fatalf("unexpected active summary: chunk=%+v err=%v", active, err)
	}
	remaining, err := messageStore.ListByChatID("delete-chat")
	if err != nil || len(remaining) != 3 {
		t.Fatalf("unexpected remaining messages: count=%d err=%v", len(remaining), err)
	}
}

func insertSummaryDatabaseTestChat(t *testing.T, db *DB, chatID string) {
	t.Helper()
	characterID := "character-" + strings.TrimPrefix(chatID, "chat-")
	if _, err := db.Exec(`
		INSERT INTO characters (id, user_id, name) VALUES (?, 'summary-db-user', 'Summary DB Character')`,
		characterID,
	); err != nil {
		t.Fatalf("insert character: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO chats (id, user_id, character_id, title) VALUES (?, 'summary-db-user', ?, ?)`,
		chatID, characterID, fmt.Sprintf("Chat %s", chatID),
	); err != nil {
		t.Fatalf("insert chat: %v", err)
	}
}
