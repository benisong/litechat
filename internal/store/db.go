package store

import (
	"database/sql"
	"fmt"
	"litechat/internal/statusbar"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无需 CGO
)

// DB 数据库连接封装
type DB struct {
	*sql.DB
	path string
}

// NewDB 创建数据库连接
func NewDB(dataDir string) (*DB, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "litechat.db")
	// WAL keeps reads available during short writes; busy_timeout absorbs brief writer contention.
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	log.Printf("数据库已连接: %s", dbPath)
	return &DB{DB: db, path: dbPath}, nil
}

// InitSchema 初始化数据库表结构
func (db *DB) InitSchema() error {
	schema := `
	-- 用户表（username + mode 组合唯一）
	CREATE TABLE IF NOT EXISTS users (
		id            TEXT PRIMARY KEY,
		username      TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		role          TEXT NOT NULL DEFAULT 'user',
		mode          TEXT NOT NULL DEFAULT 'self',
		user_name     TEXT DEFAULT '',
		user_detail   TEXT DEFAULT '',
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(username, mode)
	);

	-- 角色卡表
	CREATE TABLE IF NOT EXISTS characters (
		id              TEXT PRIMARY KEY,
		user_id         TEXT DEFAULT '',
		name            TEXT NOT NULL,
		description     TEXT DEFAULT '',
		personality     TEXT DEFAULT '',
		scenario        TEXT DEFAULT '',
		first_msg       TEXT DEFAULT '',
		avatar_url      TEXT DEFAULT '',
		tags            TEXT DEFAULT '',
		pov             TEXT DEFAULT 'third',
		use_custom_user INTEGER DEFAULT 0,
		user_name       TEXT DEFAULT '',
		user_detail     TEXT DEFAULT '',
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 新 JSON 角色卡原文及嵌入式世界书文档
	CREATE TABLE IF NOT EXISTS character_card_documents (
		id                TEXT PRIMARY KEY,
		user_id           TEXT NOT NULL,
		character_id      TEXT NOT NULL UNIQUE REFERENCES characters(id) ON DELETE CASCADE,
		card_version      TEXT NOT NULL,
		worldbook_id      TEXT NOT NULL,
		worldbook_version TEXT NOT NULL,
		source_json       TEXT NOT NULL,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 预设表
	CREATE TABLE IF NOT EXISTS presets (
		id            TEXT PRIMARY KEY,
		user_id       TEXT DEFAULT '',
		name          TEXT NOT NULL,
		system_prompt TEXT DEFAULT '',
		prompts       TEXT DEFAULT '',
		temperature   REAL DEFAULT 0.8,
		max_tokens    INTEGER DEFAULT 2048,
		top_p         REAL DEFAULT 0.9,
		is_default    INTEGER DEFAULT 0,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 世界书表
	CREATE TABLE IF NOT EXISTS world_books (
		id           TEXT PRIMARY KEY,
		user_id      TEXT DEFAULT '',
		character_id TEXT DEFAULT '',
		name         TEXT NOT NULL,
		description  TEXT DEFAULT '',
		runtime_mode TEXT NOT NULL DEFAULT 'static',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 世界书条目表
	CREATE TABLE IF NOT EXISTS world_book_entries (
		id                 TEXT PRIMARY KEY,
		user_id            TEXT DEFAULT '',
		world_book_id      TEXT NOT NULL REFERENCES world_books(id) ON DELETE CASCADE,
		keys               TEXT DEFAULT '',
		secondary_keys     TEXT DEFAULT '',
		content            TEXT DEFAULT '',
		enabled            INTEGER DEFAULT 1,
		constant           INTEGER DEFAULT 0,
		priority           INTEGER DEFAULT 0,
		injection_position INTEGER DEFAULT 0,
		injection_depth    INTEGER DEFAULT 4,
		scan_depth         INTEGER DEFAULT 0,
		case_sensitive     INTEGER DEFAULT 0,
		order_num          INTEGER DEFAULT 100,
		role               TEXT DEFAULT 'system',
		bg_color           TEXT DEFAULT '',
		font_color         TEXT DEFAULT '',
		created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at         DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 对话会话表
	CREATE TABLE IF NOT EXISTS chats (
		id           TEXT PRIMARY KEY,
		user_id      TEXT DEFAULT '',
		character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
		title        TEXT NOT NULL,
		preset_id    TEXT DEFAULT '',
		scheduler_enabled INTEGER NOT NULL DEFAULT 0,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 消息表
	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		seq        INTEGER DEFAULT 0,
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		status_bar TEXT NOT NULL DEFAULT '',
		tokens     INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);


	CREATE TABLE IF NOT EXISTS chat_summary_state (
		chat_id                 TEXT PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
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
		chat_id        TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
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

	CREATE INDEX IF NOT EXISTS idx_summary_chunks_chat_status ON chat_summary_chunks(chat_id, status, level, from_seq);

	-- 配置表
	CREATE TABLE IF NOT EXISTS chat_summary_jobs (
		id               TEXT PRIMARY KEY,
		chat_id          TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		job_type         TEXT NOT NULL,
		from_seq         INTEGER NOT NULL,
		to_seq           INTEGER NOT NULL,
		base_cutoff_seq  INTEGER DEFAULT 0,
		status           TEXT NOT NULL DEFAULT 'pending',
		attempt_count    INTEGER DEFAULT 0,
		next_run_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_error       TEXT DEFAULT '',
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_summary_jobs_status_runat ON chat_summary_jobs(status, next_run_at, created_at);

	-- 剧情运行时 Manifest（角色卡/剧情世界书编译结果）
	CREATE TABLE IF NOT EXISTS story_manifests (
		id                         TEXT PRIMARY KEY,
		character_id               TEXT NOT NULL,
		character_version          TEXT NOT NULL DEFAULT '',
		worldbook_version_hash     TEXT NOT NULL DEFAULT '',
		manifest_version           INTEGER NOT NULL DEFAULT 1,
		status                     TEXT NOT NULL DEFAULT 'pending',
		compiled_json              TEXT NOT NULL DEFAULT '',
		compiler_model             TEXT NOT NULL DEFAULT '',
		prompt_version             TEXT NOT NULL DEFAULT '',
		error_message              TEXT NOT NULL DEFAULT '',
		created_at                 DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at                 DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_story_manifests_cache_key
		ON story_manifests(character_id, character_version, worldbook_version_hash, compiler_model, prompt_version, status, updated_at);

	-- 每个复杂剧情聊天独立的动态状态
	CREATE TABLE IF NOT EXISTS chat_story_states (
		chat_id          TEXT PRIMARY KEY,
		manifest_id      TEXT NOT NULL,
		state_version    INTEGER NOT NULL DEFAULT 0,
		state_json       TEXT NOT NULL DEFAULT '{}',
		current_scene    TEXT NOT NULL DEFAULT '',
		active_event     TEXT NOT NULL DEFAULT '',
		route            TEXT NOT NULL DEFAULT '',
		scheduler_status TEXT NOT NULL DEFAULT 'ready',
		last_success_record_id TEXT NOT NULL DEFAULT '',
		failure_count    INTEGER NOT NULL DEFAULT 0,
		created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at       DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 每一轮调度模型处理记录
	CREATE TABLE IF NOT EXISTS chat_scheduler_records (
		id                   TEXT PRIMARY KEY,
		chat_id              TEXT NOT NULL,
		user_message_id      TEXT NOT NULL,
		assistant_message_id TEXT NOT NULL,
		turn_seq             INTEGER NOT NULL,
		status               TEXT NOT NULL DEFAULT 'pending',
		attempt_count        INTEGER NOT NULL DEFAULT 0,
		scheduler_model      TEXT NOT NULL DEFAULT '',
		prompt_version       TEXT NOT NULL DEFAULT '',
		input_snapshot       TEXT NOT NULL DEFAULT '',
		raw_output           TEXT NOT NULL DEFAULT '',
		parsed_output        TEXT NOT NULL DEFAULT '',
		applied_changes      TEXT NOT NULL DEFAULT '',
		context_text         TEXT NOT NULL DEFAULT '',
		state_version_before INTEGER NOT NULL DEFAULT 0,
		state_version_after  INTEGER NOT NULL DEFAULT 0,
		error_code           TEXT NOT NULL DEFAULT '',
		error_message        TEXT NOT NULL DEFAULT '',
		created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at           DATETIME,
		finished_at          DATETIME,
		applied_at           DATETIME,
		UNIQUE(chat_id, assistant_message_id)
	);
	CREATE INDEX IF NOT EXISTS idx_scheduler_records_chat_status_seq
		ON chat_scheduler_records(chat_id, status, turn_seq);

	-- 已确认发生的剧情事件，只追加不覆盖
	CREATE TABLE IF NOT EXISTS chat_story_events (
		id                   TEXT PRIMARY KEY,
		chat_id              TEXT NOT NULL,
		scheduler_record_id  TEXT NOT NULL,
		event_key            TEXT NOT NULL,
		event_type           TEXT NOT NULL DEFAULT '',
		summary              TEXT NOT NULL DEFAULT '',
		importance           TEXT NOT NULL DEFAULT 'normal',
		evidence             TEXT NOT NULL DEFAULT '',
		status               TEXT NOT NULL DEFAULT 'applied',
		created_at           DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(chat_id, event_key)
	);
	CREATE INDEX IF NOT EXISTS idx_story_events_chat_created
		ON chat_story_events(chat_id, created_at);

	-- 配置表（全局配置）
	CREATE TABLE IF NOT EXISTS configs (
		key        TEXT PRIMARY KEY,
		value      TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 插入内置备用预设（is_default=0，不会被自动选中，仅作最终回退）
	INSERT OR IGNORE INTO presets (id, user_id, name, system_prompt, temperature, max_tokens, top_p, is_default)
	VALUES (
		'default',
		'',
		'内置备用预设',
		'你是{{char}}。请根据角色设定进行扮演，保持角色一致性。

角色描述：{{description}}

性格：{{personality}}

场景：{{scenario}}',
		0.8,
		2048,
		0.9,
		0
	);

	-- 插入默认配置
	INSERT OR IGNORE INTO configs (key, value) VALUES ('api_endpoint', 'https://api.openai.com/v1');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('api_key', '');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('default_model', 'gpt-4o-mini');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('use_default_model_for_memory', 'true');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('memory_model', '');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('story_compiler_model', '');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('story_scheduler_model', '');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('memory_summary_char_limit', '3000');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('theme', 'dark');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('service_mode', 'self');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('memory_prompt_suffix', '');
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("初始化数据库结构失败: %w", err)
	}

	// 兼容旧数据库：添加新列（已存在则忽略）
	db.Exec(`ALTER TABLE presets ADD COLUMN prompts TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN secondary_keys TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN constant INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN injection_position INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN injection_depth INTEGER DEFAULT 4`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN scan_depth INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN case_sensitive INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN order_num INTEGER DEFAULT 100`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN role TEXT DEFAULT 'system'`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN bg_color TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN font_color TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_books ADD COLUMN character_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_books ADD COLUMN runtime_mode TEXT NOT NULL DEFAULT 'static'`)
	db.Exec(`ALTER TABLE users ADD COLUMN mode TEXT DEFAULT 'self'`)
	db.Exec(`ALTER TABLE users ADD COLUMN user_name TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE users ADD COLUMN user_detail TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE characters ADD COLUMN pov TEXT DEFAULT 'third'`)
	db.Exec(`ALTER TABLE characters ADD COLUMN use_custom_user INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_name TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_detail TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE messages ADD COLUMN seq INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE messages ADD COLUMN status_bar TEXT NOT NULL DEFAULT ''`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN pending_to_seq INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN pending_status TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN pending_run_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN pending_attempts INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN pending_error TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN pending_started_at DATETIME`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN summary_required INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN next_summary_floor INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE chat_summary_state ADD COLUMN eligibility_seq INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE chat_summary_chunks ADD COLUMN to_message_id TEXT DEFAULT ''`)

	// 兼容旧数据库：添加 user_id 列（已存在则忽略）
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_chat_seq ON messages(chat_id, seq) WHERE seq > 0`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chats ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chats ADD COLUMN scheduler_enabled INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE presets ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_books ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`UPDATE characters SET pov = 'third' WHERE pov = '' OR pov IS NULL`)
	db.Exec(`UPDATE users SET user_name = 'user' WHERE role = 'user' AND (user_name = '' OR user_name IS NULL)`)
	db.Exec(`DELETE FROM configs WHERE key IN ('default_user_name', 'default_user_detail')`)
	db.Exec(`
		WITH ranked AS (
			SELECT rowid AS rid,
			       ROW_NUMBER() OVER (PARTITION BY chat_id ORDER BY created_at ASC, rowid ASC) AS seq
			FROM messages
		)
		UPDATE messages
		SET seq = (
			SELECT ranked.seq FROM ranked WHERE ranked.rid = messages.rowid
		)
		WHERE COALESCE(seq, 0) = 0
	`)

	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_chat_seq ON messages(chat_id, seq)`); err != nil {
		return fmt.Errorf("创建消息顺序索引失败: %w", err)
	}
	if err := db.migrateMessageStatusBars(); err != nil {
		return fmt.Errorf("迁移历史状态栏失败: %w", err)
	}

	log.Println("数据库结构初始化完成")
	return nil
}

func (db *DB) migrateMessageStatusBars() error {
	var legacyTableCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'message_status_bars'`).Scan(&legacyTableCount); err != nil {
		return err
	}
	if legacyTableCount > 0 {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`
			UPDATE messages
			SET status_bar = (
				SELECT content FROM message_status_bars WHERE message_id = messages.id
			)
			WHERE TRIM(COALESCE(status_bar, '')) = ''
			  AND EXISTS (
				SELECT 1 FROM message_status_bars WHERE message_id = messages.id
			  )`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DROP TABLE message_status_bars`); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	type legacyMessage struct {
		id      string
		content string
	}

	rows, err := db.Query(`
		SELECT id, content
		FROM messages
		WHERE role = 'assistant' AND instr(content, ?) > 0`, statusbar.Marker)
	if err != nil {
		return err
	}
	var legacy []legacyMessage
	for rows.Next() {
		var message legacyMessage
		if err := rows.Scan(&message.id, &message.content); err != nil {
			_ = rows.Close()
			return err
		}
		legacy = append(legacy, message)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(legacy) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, message := range legacy {
		body, panel := statusbar.Split(message.content)
		if panel == "" {
			continue
		}
		if _, err := tx.Exec(`
			UPDATE messages
			SET content = ?,
				status_bar = CASE WHEN TRIM(COALESCE(status_bar, '')) = '' THEN ? ELSE status_bar END
			WHERE id = ?`, body, panel, message.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
