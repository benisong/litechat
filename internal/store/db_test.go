package store

import (
	"database/sql"
	"strings"
	"testing"
)

func TestInitSchemaMigratesAsyncSummaryStateColumns(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE chat_summary_state (
			chat_id                TEXT PRIMARY KEY,
			applied_cutoff_seq     INTEGER DEFAULT 0,
			current_big_summary_id TEXT DEFAULT '',
			dirty_from_seq         INTEGER DEFAULT 0,
			updated_at             DATETIME DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create legacy summary state: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE chat_summary_chunks (
			id             TEXT PRIMARY KEY,
			chat_id        TEXT NOT NULL,
			level          TEXT NOT NULL,
			from_seq       INTEGER NOT NULL,
			to_seq         INTEGER NOT NULL,
			content        TEXT NOT NULL,
			status         TEXT NOT NULL DEFAULT 'active',
			merged_into_id TEXT DEFAULT '',
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatalf("create legacy summary chunks: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	rows, err := db.Query(`PRAGMA table_info(chat_summary_state)`)
	if err != nil {
		t.Fatalf("read summary columns: %v", err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var id, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&id, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan summary column: %v", err)
		}
		columns[name] = true
	}
	for _, name := range []string{
		"pending_to_seq",
		"pending_status",
		"pending_run_id",
		"pending_attempts",
		"pending_error",
		"pending_started_at",
		"summary_required",
		"next_summary_floor",
		"eligibility_seq",
	} {
		if !columns[name] {
			t.Fatalf("legacy database was not migrated: missing %s", name)
		}
	}

	var hasToMessageID bool
	chunkRows, err := db.Query(`PRAGMA table_info(chat_summary_chunks)`)
	if err != nil {
		t.Fatalf("read summary chunk columns: %v", err)
	}
	defer chunkRows.Close()
	for chunkRows.Next() {
		var id, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := chunkRows.Scan(&id, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan summary chunk column: %v", err)
		}
		hasToMessageID = hasToMessageID || name == "to_message_id"
	}
	if !hasToMessageID {
		t.Fatal("legacy database was not migrated: missing to_message_id")
	}
	settings, err := NewConfigStore(db).GetSettings()
	if err != nil {
		t.Fatalf("read migrated settings: %v", err)
	}
	if settings.MemorySummaryCharLimit != 3000 {
		t.Fatalf("unexpected default summary char limit: %d", settings.MemorySummaryCharLimit)
	}
}

func TestInitSchemaMigratesLegacyStatusBars(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO characters (id, user_id, name) VALUES ('legacy-char', 'legacy-user', 'Legacy Character')`); err != nil {
		t.Fatalf("insert legacy character: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO chats (id, user_id, character_id, title)
		VALUES ('legacy-chat', 'legacy-user', 'legacy-char', 'Legacy Chat')`); err != nil {
		t.Fatalf("insert legacy chat: %v", err)
	}
	legacyContent := "旧正文。\n\n'''\n【状态栏】\n地点：旧城\n'''"
	if _, err := db.Exec(`
		INSERT INTO messages (id, chat_id, seq, role, content)
		VALUES ('legacy-message', 'legacy-chat', 1, 'assistant', ?)`, legacyContent); err != nil {
		t.Fatalf("insert legacy message: %v", err)
	}
	if _, err := db.Exec(`DROP TABLE message_status_bars`); err != nil {
		t.Fatalf("remove new status table: %v", err)
	}

	if err := db.InitSchema(); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	message, err := NewMessageStore(db).GetByID("legacy-message")
	if err != nil {
		t.Fatalf("load migrated message: %v", err)
	}
	if message.Content != "旧正文。" || !strings.Contains(message.StatusBar, "地点：旧城") {
		t.Fatalf("legacy status bar was not migrated: %+v", message)
	}
}
