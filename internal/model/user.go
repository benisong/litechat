package model

import "time"

// User 用户模型
type User struct {
	ID            string    `json:"id" db:"id"`
	Username      string    `json:"username" db:"username"`
	PasswordHash  string    `json:"-" db:"password_hash"`
	Role          string    `json:"role" db:"role"`       // admin / user
	Mode          string    `json:"mode" db:"mode"`       // self / service（用户所属模式）
	Balance       int       `json:"balance" db:"balance"`              // 积分余额
	TotalTokens   int       `json:"total_tokens" db:"total_tokens"`    // 累计消耗 token
	TotalMessages int       `json:"total_messages" db:"total_messages"` // 累计发送消息条数
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"` // 默认 user
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}
