package store

import (
	"fmt"
	"litechat/internal/auth"
	"litechat/internal/model"
	"log"
	"time"

	"github.com/google/uuid"
)

// UserStore 用户数据操作
type UserStore struct {
	db *DB
}

func NewUserStore(db *DB) *UserStore {
	return &UserStore{db: db}
}

// Create 创建用户
func (s *UserStore) Create(user *model.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetByID 按 ID 查询用户
func (s *UserStore) GetByID(id string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetByUsername 按用户名查询用户
func (s *UserStore) GetByUsername(username string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users WHERE username = ?`, username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// List 查询所有用户
func (s *UserStore) List() ([]*model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.User
	for rows.Next() {
		user := &model.User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, user)
	}
	return list, nil
}

// Delete 删除用户
func (s *UserStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

// UpdatePassword 更新用户密码
func (s *UserStore) UpdatePassword(id, passwordHash string) error {
	result, err := s.db.Exec(`UPDATE users SET password_hash=?, updated_at=? WHERE id=?`,
		passwordHash, time.Now(), id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("用户不存在: %s", id)
	}
	return nil
}

// UpdateUsername 更新用户名
func (s *UserStore) UpdateUsername(id, username string) error {
	result, err := s.db.Exec(`UPDATE users SET username=?, updated_at=? WHERE id=?`,
		username, time.Now(), id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("用户不存在: %s", id)
	}
	return nil
}

// UpdateUser 管理员更新用户信息（用户名、角色、密码）
func (s *UserStore) UpdateUser(id, username, role, passwordHash string) error {
	if passwordHash != "" {
		_, err := s.db.Exec(`UPDATE users SET username=?, role=?, password_hash=?, updated_at=? WHERE id=?`,
			username, role, passwordHash, time.Now(), id)
		return err
	}
	_, err := s.db.Exec(`UPDATE users SET username=?, role=?, updated_at=? WHERE id=?`,
		username, role, time.Now(), id)
	return err
}

// EnsureInitialUsers 确保初始用户存在（如果没有任何用户，则创建默认管理员和普通用户）
func (s *UserStore) EnsureInitialUsers() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return fmt.Errorf("查询用户数量失败: %w", err)
	}

	if count > 0 {
		return nil // 已有用户，无需创建
	}

	log.Println("未发现用户，创建默认用户...")

	// 创建管理员用户
	adminHash, err := auth.HashPassword("admin")
	if err != nil {
		return fmt.Errorf("生成管理员密码哈希失败: %w", err)
	}
	adminUser := &model.User{
		Username:     "admin",
		PasswordHash: adminHash,
		Role:         "admin",
	}
	if err := s.Create(adminUser); err != nil {
		return fmt.Errorf("创建管理员用户失败: %w", err)
	}
	log.Printf("已创建管理员用户: admin (密码: admin)")

	// 创建普通用户
	userHash, err := auth.HashPassword("user")
	if err != nil {
		return fmt.Errorf("生成用户密码哈希失败: %w", err)
	}
	normalUser := &model.User{
		Username:     "user1",
		PasswordHash: userHash,
		Role:         "user",
	}
	if err := s.Create(normalUser); err != nil {
		return fmt.Errorf("创建普通用户失败: %w", err)
	}
	log.Printf("已创建普通用户: user1 (密码: user)")

	// 为普通用户创建默认角色卡
	s.CreateDefaultCharacter(normalUser.ID)

	return nil
}

// CreateDefaultCharacter 为用户创建默认角色卡
func (s *UserStore) CreateDefaultCharacter(userID string) {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO characters (id, user_id, name, description, personality, scenario, first_msg, avatar_url, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		userID,
		"小助手",
		"一个友善的 AI 聊天助手，喜欢帮助别人解决问题。",
		"温柔、耐心、幽默，说话简洁有条理。偶尔会开小玩笑活跃气氛。",
		"你正在和用户进行一对一的文字聊天。",
		"你好呀！我是小助手，有什么我可以帮你的吗？😊",
		"",
		"助手,默认",
		now, now,
	)
	if err != nil {
		log.Printf("创建默认角色卡失败: %v", err)
	} else {
		log.Printf("已为用户 %s 创建默认角色卡「小助手」", userID)
	}
}
