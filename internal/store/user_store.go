package store

import (
	"fmt"
	"litechat/internal/auth"
	"litechat/internal/model"
	"log"
	"strings"
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
	if user.Mode == "" {
		user.Mode = "self"
	}
	if user.Role != "admin" && strings.TrimSpace(user.UserName) == "" {
		user.UserName = "user"
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO users (id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.PasswordHash, user.Role, user.Mode, user.UserName, user.UserDetail, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

// GetByID 按 ID 查询用户
func (s *UserStore) GetByID(id string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at
		FROM users WHERE id = ?`, id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Mode, &user.UserName, &user.UserDetail, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetByUsername 按用户名查询（admin 不区分 mode，普通用户按 mode 查询）
func (s *UserStore) GetByUsername(username string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at
		FROM users WHERE username = ? AND role = 'admin'`, username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Mode, &user.UserName, &user.UserDetail, &user.CreatedAt, &user.UpdatedAt)
	if err == nil {
		return user, nil
	}
	// 非 admin，需要知道当前模式才能查。先返回第一个匹配的
	err = s.db.QueryRow(`
		SELECT id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at
		FROM users WHERE username = ? ORDER BY created_at ASC LIMIT 1`, username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Mode, &user.UserName, &user.UserDetail, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

// GetByUsernameAndMode 按用户名+模式查询（登录时用）
func (s *UserStore) GetByUsernameAndMode(username, mode string) (*model.User, error) {
	user := &model.User{}
	// admin 用户不受 mode 限制
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at
		FROM users WHERE username = ? AND (role = 'admin' OR mode = ?)`, username, mode,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Mode, &user.UserName, &user.UserDetail, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetFirstNonAdminByMode returns the oldest non-admin user under a mode.
func (s *UserStore) GetFirstNonAdminByMode(mode string) (*model.User, error) {
	user := &model.User{}
	err := s.db.QueryRow(`
		SELECT id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at
		FROM users
		WHERE role != 'admin' AND mode = ?
		ORDER BY created_at ASC
		LIMIT 1`, mode,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Mode, &user.UserName, &user.UserDetail, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// List 查询所有用户（按当前模式过滤，admin 始终可见）
func (s *UserStore) List(mode string) ([]*model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password_hash, role, mode, user_name, user_detail, created_at, updated_at
		FROM users WHERE role = 'admin' OR mode = ?
		ORDER BY role DESC, created_at ASC`, mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.User
	for rows.Next() {
		user := &model.User{}
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Mode, &user.UserName, &user.UserDetail, &user.CreatedAt, &user.UpdatedAt); err != nil {
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

// UpdateProfile 更新当前用户资料
func (s *UserStore) UpdateProfile(id, userName, userDetail string) error {
	userName = strings.TrimSpace(userName)
	if userName == "" {
		userName = "user"
	}

	result, err := s.db.Exec(`UPDATE users SET user_name=?, user_detail=?, updated_at=? WHERE id=?`,
		userName, userDetail, time.Now(), id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("用户不存在: %s", id)
	}
	return nil
}

// UpdateUser 管理员更新用户信息（用户名、角色、密码，但不能改 role 为 admin）
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

// GetCurrentMode 获取当前系统运行模式
func (s *UserStore) GetCurrentMode() string {
	var mode string
	err := s.db.QueryRow(`SELECT value FROM configs WHERE key = 'service_mode'`).Scan(&mode)
	if err != nil || mode == "" {
		return "self"
	}
	return mode
}

// EnsureInitialUsers 确保初始用户存在
func (s *UserStore) EnsureInitialUsers() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return fmt.Errorf("查询用户数量失败: %w", err)
	}

	if count > 0 {
		return nil
	}

	log.Println("未发现用户，创建默认用户...")

	// admin 用户（mode 设为 self，admin 不受 mode 限制）
	adminHash, err := auth.HashPassword("admin")
	if err != nil {
		return fmt.Errorf("生成管理员密码哈希失败: %w", err)
	}
	adminUser := &model.User{
		Username:     "admin",
		PasswordHash: adminHash,
		Role:         "admin",
		Mode:         "self",
	}
	if err := s.Create(adminUser); err != nil {
		return fmt.Errorf("创建管理员用户失败: %w", err)
	}
	log.Printf("已创建管理员用户: admin (密码: admin)")

	// 自用模式普通用户
	userHash, err := auth.HashPassword("user")
	if err != nil {
		return fmt.Errorf("生成用户密码哈希失败: %w", err)
	}
	selfUser := &model.User{
		Username:     "user1",
		PasswordHash: userHash,
		Role:         "user",
		Mode:         "self",
		UserName:     "user",
	}
	if err := s.Create(selfUser); err != nil {
		return fmt.Errorf("创建自用模式用户失败: %w", err)
	}
	log.Printf("已创建自用模式用户: user1 (密码: user)")
	s.CreateDefaultCharacter(selfUser.ID)

	// 服务模式普通用户
	serviceUser := &model.User{
		Username:     "user1",
		PasswordHash: userHash,
		Role:         "user",
		Mode:         "service",
		UserName:     "user",
	}
	if err := s.Create(serviceUser); err != nil {
		return fmt.Errorf("创建服务模式用户失败: %w", err)
	}
	log.Printf("已创建服务模式用户: user1 (密码: user)")
	s.CreateDefaultCharacter(serviceUser.ID)

	qqSvcHash, err := auth.HashPassword("qqsvc")
	if err != nil {
		return fmt.Errorf("生成 qqsvc 密码哈希失败: %w", err)
	}
	qqSvcUser := &model.User{
		Username:     "qqsvc",
		PasswordHash: qqSvcHash,
		Role:         "user",
		Mode:         s.GetCurrentMode(),
		UserName:     "QQ用户",
	}
	if err := s.Create(qqSvcUser); err != nil {
		return fmt.Errorf("创建 qqsvc 用户失败: %w", err)
	}
	log.Printf("已创建 NapCat 默认承载用户: qqsvc (密码: qqsvc)")
	s.CreateDefaultCharacter(qqSvcUser.ID)
	_, _ = s.db.Exec(`
		INSERT OR IGNORE INTO configs (key, value, updated_at) VALUES (?, ?, ?)
	`, "napcat_owner_user_id", qqSvcUser.ID, time.Now())

	return nil
}

// CreateDefaultCharacter 为用户创建默认角色卡
func (s *UserStore) CreateDefaultCharacter(userID string) {
	now := time.Now()
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO characters (id, user_id, name, description, personality, scenario, first_msg, avatar_url, tags, pov, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userID,
		"小助手",
		"一个友善的 AI 聊天助手，喜欢帮助别人解决问题。",
		"温柔、耐心、幽默，说话简洁有条理。偶尔会开小玩笑活跃气氛。",
		"你正在和用户进行一对一的文字聊天。",
		"你好呀！我是小助手，有什么我可以帮你的吗？😊",
		"", "助手,默认", "second", now, now,
	)
	if err != nil {
		log.Printf("创建默认角色卡失败: %v", err)
	} else {
		log.Printf("已为用户 %s 创建默认角色卡「小助手」", userID)
	}
}
