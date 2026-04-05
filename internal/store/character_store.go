package store

import (
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// CharacterStore 角色卡数据操作
type CharacterStore struct {
	db *DB
}

func NewCharacterStore(db *DB) *CharacterStore {
	return &CharacterStore{db: db}
}

// Create 创建角色卡
func (s *CharacterStore) Create(c *model.Character) error {
	c.ID = uuid.New().String()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO characters (id, name, description, personality, scenario, first_msg, avatar_url, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Description, c.Personality, c.Scenario,
		c.FirstMsg, c.AvatarURL, c.Tags, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

// GetByID 按 ID 查询角色卡
func (s *CharacterStore) GetByID(id string) (*model.Character, error) {
	c := &model.Character{}
	err := s.db.QueryRow(`
		SELECT id, name, description, personality, scenario, first_msg, avatar_url, tags, created_at, updated_at
		FROM characters WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Personality, &c.Scenario,
		&c.FirstMsg, &c.AvatarURL, &c.Tags, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// List 查询所有角色卡
func (s *CharacterStore) List() ([]*model.Character, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, personality, scenario, first_msg, avatar_url, tags, created_at, updated_at
		FROM characters ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Character
	for rows.Next() {
		c := &model.Character{}
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Personality, &c.Scenario,
			&c.FirstMsg, &c.AvatarURL, &c.Tags, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

// Update 更新角色卡
func (s *CharacterStore) Update(c *model.Character) error {
	c.UpdatedAt = time.Now()
	result, err := s.db.Exec(`
		UPDATE characters SET name=?, description=?, personality=?, scenario=?, first_msg=?, avatar_url=?, tags=?, updated_at=?
		WHERE id=?`,
		c.Name, c.Description, c.Personality, c.Scenario,
		c.FirstMsg, c.AvatarURL, c.Tags, c.UpdatedAt, c.ID,
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

// Delete 删除角色卡
func (s *CharacterStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM characters WHERE id = ?`, id)
	return err
}
