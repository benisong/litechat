package store

import (
	"fmt"
	"litechat/internal/model"
	"strconv"
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

func (s *PresetStore) Create(p *model.Preset, userID string) error {
	p.ID = uuid.New().String()
	p.UserID = userID
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO presets (id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.UserID, p.Name, p.SystemPrompt, p.Prompts, p.Temperature, p.MaxTokens, p.TopP,
		boolToInt(p.IsDefault), p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *PresetStore) GetByID(id string, userID string) (*model.Preset, error) {
	p := &model.Preset{}
	var isDefault int
	// 先查自己的，再查任意的（服务模式下普通用户需要读 admin 的预设）
	err := s.db.QueryRow(`
		SELECT id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		// 回退：不限 user_id 查找（admin 创建的预设）
		err = s.db.QueryRow(`
			SELECT id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
			FROM presets WHERE id = ?`, id,
		).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
			&isDefault, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return nil, err
		}
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

func (s *PresetStore) GetDefault(userID string) (*model.Preset, error) {
	// 优先找自己的默认预设
	p := &model.Preset{}
	var isDefault int
	err := s.db.QueryRow(`
		SELECT id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE is_default = 1 AND user_id = ? LIMIT 1`, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err == nil {
		p.IsDefault = isDefault == 1
		return p, nil
	}
	// 回退：查找任意用户的默认预设（服务模式下 admin 创建的）
	err = s.db.QueryRow(`
		SELECT id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE is_default = 1 AND user_id != '' ORDER BY updated_at DESC LIMIT 1`,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err == nil {
		p.IsDefault = isDefault == 1
		return p, nil
	}
	// 最终回退：没有标记为默认的预设，取任意预设（服务模式下 admin 可能忘记勾选默认）
	err = s.db.QueryRow(`
		SELECT id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE user_id != '' ORDER BY updated_at DESC LIMIT 1`,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

// GetDefaultAdmin 服务模式专用：查找 admin 用户的预设
// 优先找 admin 的默认预设，找不到则取 admin 的任意预设
func (s *PresetStore) GetDefaultAdmin() (*model.Preset, error) {
	p := &model.Preset{}
	var isDefault int
	// 查找 admin 角色用户的默认预设
	err := s.db.QueryRow(`
		SELECT p.id, p.user_id, p.name, p.system_prompt, p.prompts, p.temperature, p.max_tokens, p.top_p, p.is_default, p.created_at, p.updated_at
		FROM presets p
		JOIN users u ON u.id = p.user_id
		WHERE u.role = 'admin' AND p.is_default = 1
		ORDER BY p.updated_at DESC LIMIT 1`,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err == nil {
		p.IsDefault = isDefault == 1
		return p, nil
	}
	// 回退：admin 没有标记默认，取 admin 的最新预设
	err = s.db.QueryRow(`
		SELECT p.id, p.user_id, p.name, p.system_prompt, p.prompts, p.temperature, p.max_tokens, p.top_p, p.is_default, p.created_at, p.updated_at
		FROM presets p
		JOIN users u ON u.id = p.user_id
		WHERE u.role = 'admin'
		ORDER BY p.updated_at DESC LIMIT 1`,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens, &p.TopP,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.IsDefault = isDefault == 1
	return p, nil
}

func (s *PresetStore) List(userID string) ([]*model.Preset, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, name, system_prompt, prompts, temperature, max_tokens, top_p, is_default, created_at, updated_at
		FROM presets WHERE user_id = ? ORDER BY is_default DESC, updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Preset
	for rows.Next() {
		p := &model.Preset{}
		var isDefault int
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.SystemPrompt, &p.Prompts, &p.Temperature, &p.MaxTokens,
			&p.TopP, &isDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsDefault = isDefault == 1
		list = append(list, p)
	}
	return list, nil
}

func (s *PresetStore) Update(p *model.Preset, userID string) error {
	p.UpdatedAt = time.Now()
	result, err := s.db.Exec(`
		UPDATE presets SET name=?, system_prompt=?, prompts=?, temperature=?, max_tokens=?, top_p=?, is_default=?, updated_at=?
		WHERE id=? AND user_id=?`,
		p.Name, p.SystemPrompt, p.Prompts, p.Temperature, p.MaxTokens, p.TopP,
		boolToInt(p.IsDefault), p.UpdatedAt, p.ID, userID,
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

func (s *PresetStore) Delete(id string, userID string) error {
	_, err := s.db.Exec(`DELETE FROM presets WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// WorldBookStore 世界书数据操作
type WorldBookStore struct {
	db *DB
}

func NewWorldBookStore(db *DB) *WorldBookStore {
	return &WorldBookStore{db: db}
}

func (s *WorldBookStore) Create(wb *model.WorldBook, userID string) error {
	wb.ID = uuid.New().String()
	wb.UserID = userID
	wb.CreatedAt = time.Now()
	wb.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO world_books (id, user_id, character_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		wb.ID, wb.UserID, wb.CharacterID, wb.Name, wb.Description, wb.CreatedAt, wb.UpdatedAt,
	)
	return err
}

func (s *WorldBookStore) GetByID(id string, userID string) (*model.WorldBook, error) {
	wb := &model.WorldBook{}
	err := s.db.QueryRow(`
		SELECT id, user_id, character_id, name, description, created_at, updated_at
		FROM world_books WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&wb.ID, &wb.UserID, &wb.CharacterID, &wb.Name, &wb.Description, &wb.CreatedAt, &wb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	entries, err := s.ListEntries(id, userID)
	if err != nil {
		return nil, err
	}
	wb.Entries = entries
	return wb, nil
}

func (s *WorldBookStore) List(userID string) ([]*model.WorldBook, error) {
	rows, err := s.db.Query(`
		SELECT wb.id, wb.user_id, wb.character_id, wb.name, wb.description, wb.created_at, wb.updated_at,
		       COALESCE(ch.name, '') as char_name
		FROM world_books wb
		LEFT JOIN characters ch ON ch.id = wb.character_id
		WHERE wb.user_id = ? ORDER BY wb.character_id ASC, wb.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.WorldBook
	for rows.Next() {
		wb := &model.WorldBook{}
		if err := rows.Scan(&wb.ID, &wb.UserID, &wb.CharacterID, &wb.Name, &wb.Description,
			&wb.CreatedAt, &wb.UpdatedAt, &wb.CharacterName); err != nil {
			return nil, err
		}
		list = append(list, wb)
	}
	return list, nil
}

func (s *WorldBookStore) Update(wb *model.WorldBook, userID string) error {
	wb.UpdatedAt = time.Now()
	_, err := s.db.Exec(`
		UPDATE world_books SET name=?, description=?, character_id=?, updated_at=? WHERE id=? AND user_id=?`,
		wb.Name, wb.Description, wb.CharacterID, wb.UpdatedAt, wb.ID, userID,
	)
	return err
}

func (s *WorldBookStore) Delete(id string, userID string) error {
	_, err := s.db.Exec(`DELETE FROM world_books WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// 世界书条目操作
func (s *WorldBookStore) CreateEntry(e *model.WorldBookEntry, userID string) error {
	e.ID = uuid.New().String()
	e.UserID = userID
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	if e.Role == "" {
		e.Role = "system"
	}

	_, err := s.db.Exec(`
		INSERT INTO world_book_entries
			(id, user_id, world_book_id, keys, secondary_keys, content, enabled, constant, priority,
			 injection_position, injection_depth, scan_depth, case_sensitive, order_num, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.UserID, e.WorldBookID, e.Keys, e.SecondaryKeys, e.Content,
		boolToInt(e.Enabled), boolToInt(e.Constant), e.Priority,
		e.InjectionPos, e.InjectionDepth, e.ScanDepth, boolToInt(e.CaseSensitive),
		e.Order, e.Role, e.CreatedAt, e.UpdatedAt,
	)
	return err
}

func (s *WorldBookStore) ListEntries(worldBookID string, userID string) ([]model.WorldBookEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, world_book_id, keys, secondary_keys, content, enabled, constant, priority,
		       injection_position, injection_depth, scan_depth, case_sensitive, order_num, role,
		       created_at, updated_at
		FROM world_book_entries WHERE world_book_id = ? AND user_id = ?
		ORDER BY priority DESC, order_num ASC`, worldBookID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.WorldBookEntry
	for rows.Next() {
		e := model.WorldBookEntry{}
		var enabled, constant, caseSensitive int
		if err := rows.Scan(&e.ID, &e.UserID, &e.WorldBookID, &e.Keys, &e.SecondaryKeys, &e.Content,
			&enabled, &constant, &e.Priority,
			&e.InjectionPos, &e.InjectionDepth, &e.ScanDepth, &caseSensitive,
			&e.Order, &e.Role, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Enabled = enabled == 1
		e.Constant = constant == 1
		e.CaseSensitive = caseSensitive == 1
		list = append(list, e)
	}
	return list, nil
}

// ListAllEntries 查询当前用户所有世界书的全部启用条目（用于聊天时扫描）
// ListAllEntries 查询全局 + 指定角色绑定的世界书条目（聊天时用）
func (s *WorldBookStore) ListAllEntries(userID string, characterID string) ([]model.WorldBookEntry, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.user_id, e.world_book_id, e.keys, e.secondary_keys, e.content, e.enabled, e.constant, e.priority,
		       e.injection_position, e.injection_depth, e.scan_depth, e.case_sensitive, e.order_num, e.role,
		       e.created_at, e.updated_at
		FROM world_book_entries e
		JOIN world_books wb ON wb.id = e.world_book_id
		WHERE e.enabled = 1 AND e.user_id = ?
		  AND (wb.character_id = '' OR wb.character_id = ?)
		ORDER BY e.priority DESC, e.order_num ASC`, userID, characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.WorldBookEntry
	for rows.Next() {
		e := model.WorldBookEntry{}
		var enabled, constant, caseSensitive int
		if err := rows.Scan(&e.ID, &e.UserID, &e.WorldBookID, &e.Keys, &e.SecondaryKeys, &e.Content,
			&enabled, &constant, &e.Priority,
			&e.InjectionPos, &e.InjectionDepth, &e.ScanDepth, &caseSensitive,
			&e.Order, &e.Role, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Enabled = enabled == 1
		e.Constant = constant == 1
		e.CaseSensitive = caseSensitive == 1
		list = append(list, e)
	}
	return list, nil
}

func (s *WorldBookStore) UpdateEntry(e *model.WorldBookEntry, userID string) error {
	e.UpdatedAt = time.Now()
	if e.Role == "" {
		e.Role = "system"
	}
	_, err := s.db.Exec(`
		UPDATE world_book_entries SET keys=?, secondary_keys=?, content=?, enabled=?, constant=?,
			priority=?, injection_position=?, injection_depth=?, scan_depth=?, case_sensitive=?,
			order_num=?, role=?, updated_at=?
		WHERE id=? AND user_id=?`,
		e.Keys, e.SecondaryKeys, e.Content, boolToInt(e.Enabled), boolToInt(e.Constant),
		e.Priority, e.InjectionPos, e.InjectionDepth, e.ScanDepth, boolToInt(e.CaseSensitive),
		e.Order, e.Role, e.UpdatedAt, e.ID, userID,
	)
	return err
}

func (s *WorldBookStore) DeleteEntry(id string, userID string) error {
	_, err := s.db.Exec(`DELETE FROM world_book_entries WHERE id = ? AND user_id = ?`, id, userID)
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
	settings := &model.AppSettings{
		UseDefaultModelForCharacterCard: true,
		ServiceMode:                     "self",
	}
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
		case "use_default_model_for_character_card":
			parsed, err := strconv.ParseBool(v)
			if err == nil {
				settings.UseDefaultModelForCharacterCard = parsed
			}
		case "character_card_model":
			settings.CharacterCardModel = v
		case "memory_prompt_suffix":
			settings.MemoryPromptSuffix = v
		case "theme":
			settings.Theme = v
		case "service_mode":
			settings.ServiceMode = v
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
