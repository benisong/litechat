package store

import (
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

type CharacterStore struct {
	db *DB
}

func NewCharacterStore(db *DB) *CharacterStore {
	return &CharacterStore{db: db}
}

// 角色卡字段列表（复用）
const charColumns = `id, user_id, name, description, personality, scenario, first_msg, avatar_url, tags, pov, use_custom_user, user_name, user_detail, created_at, updated_at`

func scanCharacter(scanner interface{ Scan(...interface{}) error }, c *model.Character) error {
	var useCustomUser int
	err := scanner.Scan(&c.ID, &c.UserID, &c.Name, &c.Description, &c.Personality, &c.Scenario,
		&c.FirstMsg, &c.AvatarURL, &c.Tags, &c.POV, &useCustomUser, &c.UserName, &c.UserDetail,
		&c.CreatedAt, &c.UpdatedAt)
	c.UseCustomUser = useCustomUser == 1
	c.POV = normalizeCharacterPOV(c.POV)
	return err
}

func (s *CharacterStore) Create(c *model.Character, userID string) error {
	c.ID = uuid.New().String()
	c.UserID = userID
	c.POV = normalizeCharacterPOV(c.POV)
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO characters (`+charColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.UserID, c.Name, c.Description, c.Personality, c.Scenario,
		c.FirstMsg, c.AvatarURL, c.Tags, c.POV, boolToInt(c.UseCustomUser), c.UserName, c.UserDetail,
		c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (s *CharacterStore) GetByID(id string, userID string) (*model.Character, error) {
	c := &model.Character{}
	err := s.db.QueryRow(`SELECT `+charColumns+` FROM characters WHERE id = ? AND user_id = ?`, id, userID)
	if e := scanCharacter(err, c); e != nil {
		return nil, e
	}
	return c, nil
}

func (s *CharacterStore) List(userID string) ([]*model.Character, error) {
	rows, err := s.db.Query(`SELECT `+charColumns+` FROM characters WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Character
	for rows.Next() {
		c := &model.Character{}
		if err := scanCharacter(rows, c); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

func (s *CharacterStore) Update(c *model.Character, userID string) error {
	c.POV = normalizeCharacterPOV(c.POV)
	c.UpdatedAt = time.Now()
	result, err := s.db.Exec(`
		UPDATE characters SET name=?, description=?, personality=?, scenario=?, first_msg=?, avatar_url=?, tags=?,
			pov=?, use_custom_user=?, user_name=?, user_detail=?, updated_at=?
		WHERE id=? AND user_id=?`,
		c.Name, c.Description, c.Personality, c.Scenario,
		c.FirstMsg, c.AvatarURL, c.Tags,
		c.POV, boolToInt(c.UseCustomUser), c.UserName, c.UserDetail,
		c.UpdatedAt, c.ID, userID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("角色卡不存在: %s", c.ID)
	}
	return nil
}

func normalizeCharacterPOV(pov string) string {
	switch pov {
	case "second", "third":
		return pov
	default:
		return "third"
	}
}

func (s *CharacterStore) Delete(id string, userID string) error {
	_, err := s.db.Exec(`DELETE FROM characters WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// boolToInt 在 preset_store.go 中定义
