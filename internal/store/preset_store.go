package store

import (
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// PresetStore 预设数据操作
type PresetStore struct {
	db *DB
}

func NewPresetStore(db *DB) *PresetStore {
	return &PresetStore{db: db}
}

func (s *PresetStore) Create(p *model.Preset) error {
	p.ID = uuid.New().String()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO presets (id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.SystemPrompt, p.Prompts, p.Temperature, p.MaxTokens, p.TopP,
		boolToInt(p.IsDefault), p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PresetStore) GetByID(id string) (*model.Preset, error) {
	p := &model.Preset{}
	var isDefault int
	err := s.db.QueryRow(`
		SELECT id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

func (s *PresetStore) GetDefault() (*model.Preset, error) {
	p := &model.Preset{}
	var isDefault int
	err := s.db.QueryRow(`
		SELECT id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE is_default = 1 LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

func (s *PresetStore) List() ([]*model.Preset, error) {
	rows, err := s.db.Query(`
		SELECT id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets ORDER BY is_default DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Preset
	for rows.Next() {
		p := &model.Preset{}
		var isDefault int
		if err := rows.Scan(&p.ID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens,
			&p.TopP, &isDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsDefault = isDefault == 1
		list = append(list, p)
	}
	return list, nil
}

func (s *PresetStore) Update(p *model.Preset) error {
	p.UpdatedAt = time.Now()
	result, err := s.db.Exec(`
		UPDATE presets SET name=?, system_prompt=?, prompts=?, temperature=?, max_tokens=?, top_p=?, is_default=?, updated_at=?
		WHERE id=?`,
		p.Name, p.SystemPrompt, p.Prompts, p.Temperature, p.MaxTokens, p.TopP,
		boolToInt(p.IsDefault), p.UpdatedAt, p.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("预设不存在: %s", p.ID)
	}
	return nil
}

func (s *PresetStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM presets WHERE id = ?`, id)
	return err
}

// WorldBookStore 世界书数据操作
type WorldBookStore struct {
	db *DB
}

func NewWorldBookStore(db *DB) *WorldBookStore {
	return &WorldBookStore{db: db}
}

func (s *WorldBookStore) Create(wb *model.WorldBook) error {
	wb.ID = uuid.New().String()
	wb.CreatedAt = time.Now()
	wb.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO world_books (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		wb.ID, wb.Name, wb.Description, wb.CreatedAt, wb.UpdatedAt,
	)
	return err
}

func (s *WorldBookStore) GetByID(id string) (*model.WorldBook, error) {
	wb := &model.WorldBook{}
	err := s.db.QueryRow(`
		SELECT id, name, description, created_at, updated_at
		FROM world_books WHERE id = ?`, id,
	).Scan(&wb.ID, &wb.Name, &wb.Description, &wb.CreatedAt, &wb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// 加载条目
	entries, err := s.ListEntries(id)
	if err != nil {
		return nil, err
	}
	wb.Entries = entries
	return wb, nil
}

func (s *WorldBookStore) List() ([]*model.WorldBook, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, created_at, updated_at
		FROM world_books ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.WorldBook
	for rows.Next() {
		wb := &model.WorldBook{}
		if err := rows.Scan(&wb.ID, &wb.Name, &wb.Description, &wb.CreatedAt, &wb.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, wb)
	}
	return list, nil
}

func (s *WorldBookStore) Update(wb *model.WorldBook) error {
	wb.UpdatedAt = time.Now()
	_, err := s.db.Exec(`
		UPDATE world_books SET name=?, description=?, updated_at=? WHERE id=?`,
		wb.Name, wb.Description, wb.UpdatedAt, wb.ID,
	)
	return err
}

func (s *WorldBookStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM world_books WHERE id = ?`, id)
	return err
}

// 世界书条目操作
func (s *WorldBookStore) CreateEntry(e *model.WorldBookEntry) error {
	e.ID = uuid.New().String()
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO world_book_entries (id, world_book_id, keys, content, enabled, priority, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.WorldBookID, e.Keys, e.Content, boolToInt(e.Enabled), e.Priority, e.CreatedAt, e.UpdatedAt,
	)
	return err
}

func (s *WorldBookStore) ListEntries(worldBookID string) ([]model.WorldBookEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, world_book_id, keys, content, enabled, priority, created_at, updated_at
		FROM world_book_entries WHERE world_book_id = ?
		ORDER BY priority DESC`, worldBookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.WorldBookEntry
	for rows.Next() {
		e := model.WorldBookEntry{}
		var enabled int
		if err := rows.Scan(&e.ID, &e.WorldBookID, &e.Keys, &e.Content, &enabled, &e.Priority, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Enabled = enabled == 1
		list = append(list, e)
	}
	return list, nil
}

func (s *WorldBookStore) UpdateEntry(e *model.WorldBookEntry) error {
	e.UpdatedAt = time.Now()
	_, err := s.db.Exec(`
		UPDATE world_book_entries SET keys=?, content=?, enabled=?, priority=?, updated_at=?
		WHERE id=?`,
		e.Keys, e.Content, boolToInt(e.Enabled), e.Priority, e.UpdatedAt, e.ID,
	)
	return err
}

func (s *WorldBookStore) DeleteEntry(id string) error {
	_, err := s.db.Exec(`DELETE FROM world_book_entries WHERE id = ?`, id)
	return err
}

// ConfigStore 配置数据操作
type ConfigStore struct {
	db *DB
}

func NewConfigStore(db *DB) *ConfigStore {
	return &ConfigStore{db: db}
}

func (s *ConfigStore) Get(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM configs WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (s *ConfigStore) Set(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO configs (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, time.Now(),
	)
	return err
}

func (s *ConfigStore) GetSettings() (*model.AppSettings, error) {
	settings := &model.AppSettings{}
	rows, err := s.db.Query(`SELECT key, value FROM configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		switch k {
		case "api_endpoint":
			settings.APIEndpoint = v
		case "api_key":
			settings.APIKey = v
		case "default_model":
			settings.DefaultModel = v
		case "theme":
			settings.Theme = v
		}
	}
	return settings, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
