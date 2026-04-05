package model

import "time"

// Character 角色卡模型
type Character struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`   // 角色描述
	Personality string    `json:"personality" db:"personality"`   // 性格设定
	Scenario    string    `json:"scenario" db:"scenario"`         // 场景设定
	FirstMsg    string    `json:"first_msg" db:"first_msg"`       // 开场白
	AvatarURL   string    `json:"avatar_url" db:"avatar_url"`     // 头像URL
	Tags        string    `json:"tags" db:"tags"`                 // 标签，逗号分隔
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// Chat 对话会话模型
type Chat struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	CharacterID string    `json:"character_id" db:"character_id"`
	Title       string    `json:"title" db:"title"`
	PresetID    string    `json:"preset_id" db:"preset_id"`   // 使用的预设ID
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	// 关联数据（非数据库字段）
	Character   *Character `json:"character,omitempty" db:"-"`
	LastMessage string     `json:"last_message,omitempty" db:"-"`
	MsgCount    int        `json:"msg_count,omitempty" db:"-"`
}

// Message 消息模型
type Message struct {
	ID        string    `json:"id" db:"id"`
	ChatID    string    `json:"chat_id" db:"chat_id"`
	Role      string    `json:"role" db:"role"`       // user / assistant / system
	Content   string    `json:"content" db:"content"`
	Tokens    int       `json:"tokens" db:"tokens"`   // token 计数
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Preset 预设（系统提示词模板）
type Preset struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Name         string    `json:"name" db:"name"`
	SystemPrompt string    `json:"system_prompt" db:"system_prompt"` // 简单模式：单段系统提示词
	Prompts      string    `json:"prompts" db:"prompts"`             // 高级模式：多段提示词 JSON 数组
	Temperature  float64   `json:"temperature" db:"temperature"`
	MaxTokens    int       `json:"max_tokens" db:"max_tokens"`
	TopP         float64   `json:"top_p" db:"top_p"`
	IsDefault    bool      `json:"is_default" db:"is_default"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PromptEntry 多段���示词中的单个条目（SillyTavern 兼容格式）
type PromptEntry struct {
	ID             string `json:"id"`                       // 唯一标识
	Name           string `json:"name"`                     // 显示名称
	Content        string `json:"content"`                  // 提示词内容（支持 {{char}} 等变量）
	Role           string `json:"role"`                     // system / user / assistant
	Enabled        bool   `json:"enabled"`                  // 是否启用
	InjectionPos   int    `json:"injection_position"`       // 0=相对末尾(默认), 1=绝对位置
	InjectionDepth int    `json:"injection_depth"`          // 注入深度（0=最前/最后）
	Order          int    `json:"order"`                    // 同深度排序优先级
}

// WorldBook 世界书（知识库）
type WorldBook struct {
	ID          string    `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	// 关联条目（非数据库字段）
	Entries     []WorldBookEntry `json:"entries,omitempty" db:"-"`
}

// WorldBookEntry 世界书条目（SillyTavern Lorebook 兼容）
type WorldBookEntry struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"user_id" db:"user_id"`
	WorldBookID    string    `json:"world_book_id" db:"world_book_id"`
	Keys           string    `json:"keys" db:"keys"`                     // 主关键词，逗号分隔（OR 逻辑）
	SecondaryKeys  string    `json:"secondary_keys" db:"secondary_keys"` // 次关键词，逗号分隔（AND 逻辑，需同时命中）
	Content        string    `json:"content" db:"content"`               // 注入内容
	Enabled        bool      `json:"enabled" db:"enabled"`               // 是否启用
	Constant       bool      `json:"constant" db:"constant"`             // 常驻注入（不需要关键词触发）
	Priority       int       `json:"priority" db:"priority"`             // 优先级（数值越大越优先）
	InjectionPos   int       `json:"injection_position" db:"injection_position"` // 0=相对末尾, 1=绝对位置
	InjectionDepth int       `json:"injection_depth" db:"injection_depth"`       // 注入深度
	ScanDepth      int       `json:"scan_depth" db:"scan_depth"`         // 扫描深度（往回扫描几条消息，0=全部）
	CaseSensitive  bool      `json:"case_sensitive" db:"case_sensitive"` // 关键词大小写敏感
	Order          int       `json:"order" db:"order_num"`               // 同深度排序
	Role           string    `json:"role" db:"role"`                     // 注入角色 system/user/assistant
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Config 全局配置
type Config struct {
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// AppSettings 应用设置（聚合配置）
type AppSettings struct {
	APIEndpoint  string `json:"api_endpoint"`   // OpenAI 兼容 API 地址
	APIKey       string `json:"api_key"`        // API 密钥
	DefaultModel string `json:"default_model"`  // 默认模型
	Theme        string `json:"theme"`          // light / dark
	ServiceMode  string `json:"service_mode"`   // self=自用模式, service=服务模式
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	Content     string `json:"content" binding:"required"`
	PresetID    string `json:"preset_id"`
}

// ChatCompletionMessage OpenAI 兼容消息格式
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest OpenAI 兼容请求格式
type ChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	Temperature float64                 `json:"temperature,omitempty"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
	TopP        float64                 `json:"top_p,omitempty"`
	Stream      bool                    `json:"stream"`
}
