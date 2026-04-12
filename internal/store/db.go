package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 绾?Go SQLite 椹卞姩锛屾棤闇€ CGO
)

// DB 鏁版嵁搴撹繛鎺ュ皝瑁?
type DB struct {
	*sql.DB
}

// NewDB 鍒涘缓鏁版嵁搴撹繛鎺?
func NewDB(dataDir string) (*DB, error) {
	// 纭繚鏁版嵁鐩綍瀛樺湪
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("鍒涘缓鏁版嵁鐩綍澶辫触: %w", err)
	}

	dbPath := filepath.Join(dataDir, "litechat.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("鎵撳紑鏁版嵁搴撳け璐? %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("杩炴帴鏁版嵁搴撳け璐? %w", err)
	}

	log.Printf("鏁版嵁搴撳凡杩炴帴: %s", dbPath)
	return &DB{db}, nil
}

// InitSchema 鍒濆鍖栨暟鎹簱琛ㄧ粨鏋?
func (db *DB) InitSchema() error {
	schema := `
	-- 鐢ㄦ埛琛紙username + mode 缁勫悎鍞竴锛?
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

	-- 瑙掕壊鍗¤〃
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

	-- 棰勮琛?
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

	-- 涓栫晫涔﹁〃
	CREATE TABLE IF NOT EXISTS world_books (
		id           TEXT PRIMARY KEY,
		user_id      TEXT DEFAULT '',
		character_id TEXT DEFAULT '',
		name         TEXT NOT NULL,
		description  TEXT DEFAULT '',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 涓栫晫涔︽潯鐩〃
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

	-- 瀵硅瘽浼氳瘽琛?
	CREATE TABLE IF NOT EXISTS chats (
		id           TEXT PRIMARY KEY,
		user_id      TEXT DEFAULT '',
		character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
		title        TEXT NOT NULL,
		preset_id    TEXT DEFAULT '',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 娑堟伅琛?	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		chat_id    TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		seq        INTEGER DEFAULT 0,
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		tokens     INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chat_summary_state (
		chat_id                 TEXT PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
		applied_cutoff_seq      INTEGER DEFAULT 0,
		current_big_summary_id  TEXT DEFAULT '',
		dirty_from_seq          INTEGER DEFAULT 0,
		updated_at              DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chat_summary_chunks (
		id             TEXT PRIMARY KEY,
		chat_id        TEXT NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
		level          TEXT NOT NULL,
		from_seq       INTEGER NOT NULL,
		to_seq         INTEGER NOT NULL,
		content        TEXT NOT NULL,
		status         TEXT NOT NULL DEFAULT 'active',
		merged_into_id TEXT DEFAULT '',
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_summary_chunks_chat_status ON chat_summary_chunks(chat_id, status, level, from_seq);

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

	-- 閰嶇疆琛?	CREATE TABLE IF NOT EXISTS configs (
		key        TEXT PRIMARY KEY,
		value      TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 鎻掑叆鍐呯疆澶囩敤棰勮锛坕s_default=0锛屼笉浼氳鑷姩閫変腑锛屼粎浣滄渶缁堝洖閫€锛?
	INSERT OR IGNORE INTO presets (id, user_id, name, system_prompt, temperature, max_tokens, top_p, is_default)
	VALUES (
		'default',
		'',
		'鍐呯疆澶囩敤棰勮',
		'浣犳槸{{char}}銆傝鏍规嵁瑙掕壊璁惧畾杩涜鎵紨锛屼繚鎸佽鑹蹭竴鑷存€с€?

瑙掕壊鎻忚堪锛歿{description}}

鎬ф牸锛歿{personality}}

鍦烘櫙锛歿{scenario}}',
		0.8,
		2048,
		0.9,
		0
	);

	-- 鎻掑叆榛樿閰嶇疆
	INSERT OR IGNORE INTO configs (key, value) VALUES ('api_endpoint', 'https://api.openai.com/v1');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('api_key', '');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('default_model', 'gpt-4o-mini');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('theme', 'dark');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('service_mode', 'self');
	INSERT OR IGNORE INTO configs (key, value) VALUES ('memory_prompt_suffix', '');
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("鍒濆鍖栨暟鎹簱缁撴瀯澶辫触: %w", err)
	}

	// 鍏煎鏃ф暟鎹簱锛氭坊鍔犳柊鍒楋紙宸插瓨鍦ㄥ垯蹇界暐锛?
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
	db.Exec(`ALTER TABLE users ADD COLUMN user_name TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE users ADD COLUMN user_detail TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE characters ADD COLUMN use_custom_user INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_name TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE characters ADD COLUMN user_detail TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE messages ADD COLUMN seq INTEGER DEFAULT 0`)

	// 鍏煎鏃ф暟鎹簱锛氭坊鍔?user_id 鍒楋紙宸插瓨鍦ㄥ垯蹇界暐锛?
	db.Exec(`ALTER TABLE characters ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE chats ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE presets ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_books ADD COLUMN user_id TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE world_book_entries ADD COLUMN user_id TEXT DEFAULT ''`)
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

	log.Println("鏁版嵁搴撶粨鏋勫垵濮嬪寲瀹屾垚")
	return nil
}
