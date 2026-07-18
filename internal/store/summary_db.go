package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
)

// NewSummaryDB opens a physically separate SQLite file so summary writes can
// never hold the main chat database's writer lock.
func NewSummaryDB(dataDir string) (*DB, error) {
	dbPath := filepath.Join(dataDir, "litechat-summary.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("打开摘要数据库失败: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接摘要数据库失败: %w", err)
	}
	log.Printf("摘要数据库已连接: %s", dbPath)
	return &DB{DB: db, path: dbPath}, nil
}

func (db *DB) InitSummarySchema() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS chat_summary_state (
			chat_id                 TEXT PRIMARY KEY,
			applied_cutoff_seq      INTEGER DEFAULT 0,
			current_big_summary_id  TEXT DEFAULT '',
			dirty_from_seq          INTEGER DEFAULT 0,
			pending_to_seq          INTEGER DEFAULT 0,
			pending_status          TEXT DEFAULT '',
			pending_run_id          TEXT DEFAULT '',
			pending_attempts        INTEGER DEFAULT 0,
			pending_error           TEXT DEFAULT '',
			pending_started_at      DATETIME,
			summary_required        INTEGER DEFAULT 0,
			next_summary_floor      INTEGER DEFAULT 0,
			eligibility_seq         INTEGER DEFAULT 0,
			updated_at              DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS chat_summary_chunks (
			id             TEXT PRIMARY KEY,
			chat_id        TEXT NOT NULL,
			level          TEXT NOT NULL,
			from_seq       INTEGER NOT NULL,
			to_seq         INTEGER NOT NULL,
			to_message_id  TEXT DEFAULT '',
			content        TEXT NOT NULL,
			status         TEXT NOT NULL DEFAULT 'active',
			merged_into_id TEXT DEFAULT '',
			created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_summary_chunks_chat_status
		ON chat_summary_chunks(chat_id, status, level, from_seq);

		CREATE TABLE IF NOT EXISTS summary_meta (
			key        TEXT PRIMARY KEY,
			value      TEXT DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

// MigrateLegacySummaries copies summaries from the former main-database tables.
// INSERT OR IGNORE keeps the migration idempotent and preserves newer summary data.
func MigrateLegacySummaries(mainDB, summaryDB *DB) error {
	if mainDB == nil || summaryDB == nil || mainDB.path == "" {
		return nil
	}
	conn, err := summaryDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	var migrationComplete string
	err = conn.QueryRowContext(context.Background(), `
		SELECT value FROM summary_meta WHERE key = 'legacy_migration_complete'`,
	).Scan(&migrationComplete)
	if err == nil && migrationComplete == "1" {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if _, err := conn.ExecContext(context.Background(), `ATTACH DATABASE ? AS legacy_chat`, mainDB.path); err != nil {
		return fmt.Errorf("挂载旧摘要数据库失败: %w", err)
	}
	defer conn.ExecContext(context.Background(), `DETACH DATABASE legacy_chat`)

	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(context.Background(), `
		INSERT OR IGNORE INTO chat_summary_state (
			chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq,
			pending_to_seq, pending_status, pending_run_id, pending_attempts,
			pending_error, pending_started_at, summary_required, next_summary_floor,
			eligibility_seq, updated_at
		)
		SELECT chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq,
		       pending_to_seq, pending_status, pending_run_id, pending_attempts,
		       pending_error, pending_started_at, summary_required, next_summary_floor,
		       eligibility_seq, updated_at
		FROM legacy_chat.chat_summary_state`); err != nil {
		return fmt.Errorf("迁移旧摘要状态失败: %w", err)
	}
	if _, err := tx.ExecContext(context.Background(), `
		INSERT OR IGNORE INTO chat_summary_chunks (
			id, chat_id, level, from_seq, to_seq, to_message_id, content,
			status, merged_into_id, created_at, updated_at
		)
		SELECT id, chat_id, level, from_seq, to_seq, to_message_id, content,
		       status, merged_into_id, created_at, updated_at
		FROM legacy_chat.chat_summary_chunks`); err != nil {
		return fmt.Errorf("迁移旧摘要内容失败: %w", err)
	}
	if _, err := tx.ExecContext(context.Background(), `
		INSERT INTO summary_meta (key, value, updated_at)
		VALUES ('legacy_migration_complete', '1', CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`); err != nil {
		return fmt.Errorf("记录旧摘要迁移状态失败: %w", err)
	}
	return tx.Commit()
}
