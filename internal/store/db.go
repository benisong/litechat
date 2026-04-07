package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无需 CGO
)

// DB 数据库连接封装
type DB struct {
	*sql.DB
}

// NewDB 创建数据库连接
func NewDB(dataDir string) (*DB, error) {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "litechat.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	log.Printf("数据库已连接: %s", dbPath)
	return &DB{db}, nil
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
		use_custom_user INTEGER DEFAULT 0,
		user_name       TEXT DEFAULT '',
		user_detail     TEXT DEFAULT '',
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
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
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 消息表
	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		tokens     INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 配置表
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
	INSERT OR IGNORE INTO configs (key, value) VALUES ('theme', 'dark');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('service_mode', 'self');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('default_user_name', '');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('default_user_detail', '');
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
	db.Exec(`ALTER TABLE world_books ADD COLUMN character_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE users ADD COLUMN mode TEXT DEFAULT 'self'`)
	db.Exec(`ALTER TABLE characters ADD COLUMN use_custom_user INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_name TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_detail TEXT DEFAULT ''`)

	// 计费相关字段
	db.Exec(`ALTER TABLE users ADD COLUMN balance INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE users ADD COLUMN total_tokens INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE users ADD COLUMN total_messages INTEGER DEFAULT 0`)

	// 兼容旧数据库：添加 user_id 列（已存在则忽略）
	db.Exec(`ALTER TABLE characters ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chats ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE presets ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_books ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN user_id TEXT DEFAULT ''`)

	log.Println("数据库结构初始化完成")
	return nil
}
