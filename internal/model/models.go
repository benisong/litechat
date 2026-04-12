package model

import "time"

// Character 角色卡模型
type Character struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	Personality   string    `json:"personality" db:"personality"`
	Scenario      string    `json:"scenario" db:"scenario"`
	FirstMsg      string    `json:"first_msg" db:"first_msg"`
	AvatarURL     string    `json:"avatar_url" db:"avatar_url"`
	Tags          string    `json:"tags" db:"tags"`
	UseCustomUser bool      `json:"use_custom_user" db:"use_custom_user"`
	UserName      string    `json:"user_name" db:"user_name"`
	UserDetail    string    `json:"user_detail" db:"user_detail"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// CharacterDraft 角色卡草稿
type CharacterDraft struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Personality   string `json:"personality"`
	Scenario      string `json:"scenario"`
	FirstMsg      string `json:"first_msg"`
	AvatarURL     string `json:"avatar_url"`
	Tags          string `json:"tags"`
	UseCustomUser bool   `json:"use_custom_user"`
	UserName      string `json:"user_name"`
	UserDetail    string `json:"user_detail"`
}

// Chat 对话会话模型
type Chat struct {
	ID          string     `json:"id" db:"id"`
	UserID      string     `json:"user_id" db:"user_id"`
	CharacterID string     `json:"character_id" db:"character_id"`
	Title       string     `json:"title" db:"title"`
	PresetID    string     `json:"preset_id" db:"preset_id"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	Character   *Character `json:"character,omitempty" db:"-"`
	LastMessage string     `json:"last_message,omitempty" db:"-"`
	MsgCount    int        `json:"msg_count,omitempty" db:"-"`
}

// Message 消息模型
type Message struct {
	ID        string    `json:"id" db:"id"`
	ChatID    string    `json:"chat_id" db:"chat_id"`
	Seq       int       `json:"seq" db:"seq"`
	Role      string    `json:"role" db:"role"`
	Content   string    `json:"content" db:"content"`
	Tokens    int       `json:"tokens" db:"tokens"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Preset 预设（系统提示词模板）
type Preset struct {
	ID           string    `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Name         string    `json:"name" db:"name"`
	SystemPrompt string    `json:"system_prompt" db:"system_prompt"`
	Prompts      string    `json:"prompts" db:"prompts"`
	Temperature  float64   `json:"temperature" db:"temperature"`
	MaxTokens    int       `json:"max_tokens" db:"max_tokens"`
	TopP         float64   `json:"top_p" db:"top_p"`
	IsDefault    bool      `json:"is_default" db:"is_default"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PromptEntry 多段提示词中的单个条目
type PromptEntry struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Content        string `json:"content"`
	Role           string `json:"role"`
	Enabled        bool   `json:"enabled"`
	SystemPrompt   bool   `json:"system_prompt"`
	InjectionPos   int    `json:"injection_position"`
	InjectionDepth int    `json:"injection_depth"`
	Order          int    `json:"order"`
}

// WorldBook 世界书（知识库）
type WorldBook struct {
	ID            string           `json:"id" db:"id"`
	UserID        string           `json:"user_id" db:"user_id"`
	CharacterID   string           `json:"character_id" db:"character_id"`
	Name          string           `json:"name" db:"name"`
	Description   string           `json:"description" db:"description"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
	Entries       []WorldBookEntry `json:"entries,omitempty" db:"-"`
	CharacterName string           `json:"character_name,omitempty" db:"-"`
}

// WorldBookEntry 世界书条目
type WorldBookEntry struct {
	ID             string    `json:"id" db:"id"`
	UserID         string    `json:"user_id" db:"user_id"`
	WorldBookID    string    `json:"world_book_id" db:"world_book_id"`
	Keys           string    `json:"keys" db:"keys"`
	SecondaryKeys  string    `json:"secondary_keys" db:"secondary_keys"`
	Content        string    `json:"content" db:"content"`
	Enabled        bool      `json:"enabled" db:"enabled"`
	Constant       bool      `json:"constant" db:"constant"`
	Priority       int       `json:"priority" db:"priority"`
	InjectionPos   int       `json:"injection_position" db:"injection_position"`
	InjectionDepth int       `json:"injection_depth" db:"injection_depth"`
	ScanDepth      int       `json:"scan_depth" db:"scan_depth"`
	CaseSensitive  bool      `json:"case_sensitive" db:"case_sensitive"`
	Order          int       `json:"order" db:"order_num"`
	Role           string    `json:"role" db:"role"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Config 全局配置
type Config struct {
	Key       string    `json:"key" db:"key"`
	Value     string    `json:"value" db:"value"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// AppSettings 应用设置
type AppSettings struct {
	APIEndpoint                     string `json:"api_endpoint"`
	APIKey                          string `json:"api_key"`
	DefaultModel                    string `json:"default_model"`
	UseDefaultModelForCharacterCard bool   `json:"use_default_model_for_character_card"`
	CharacterCardModel              string `json:"character_card_model"`
	MemoryPromptSuffix              string `json:"memory_prompt_suffix"`
	Theme                           string `json:"theme"`
	ServiceMode                     string `json:"service_mode"`
}

// ChatSummaryState 会话摘要状态
type ChatSummaryState struct {
	ChatID            string    `json:"chat_id" db:"chat_id"`
	AppliedCutoffSeq  int       `json:"applied_cutoff_seq" db:"applied_cutoff_seq"`
	CurrentBigSummary string    `json:"current_big_summary_id" db:"current_big_summary_id"`
	DirtyFromSeq      int       `json:"dirty_from_seq" db:"dirty_from_seq"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

// ChatSummaryChunk 摘要分片（小摘要 / 大摘要）
type ChatSummaryChunk struct {
	ID           string    `json:"id" db:"id"`
	ChatID        string    `json:"chat_id" db:"chat_id"`
	Level         string    `json:"level" db:"level"`
	FromSeq       int       `json:"from_seq" db:"from_seq"`
	ToSeq         int       `json:"to_seq" db:"to_seq"`
	Content       string    `json:"content" db:"content"`
	Status        string    `json:"status" db:"status"`
	MergedIntoID  string    `json:"merged_into_id" db:"merged_into_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// ChatSummaryJob 摘要后台任务
type ChatSummaryJob struct {
	ID            string    `json:"id" db:"id"`
	ChatID         string    `json:"chat_id" db:"chat_id"`
	JobType        string    `json:"job_type" db:"job_type"`
	FromSeq        int       `json:"from_seq" db:"from_seq"`
	ToSeq          int       `json:"to_seq" db:"to_seq"`
	BaseCutoffSeq  int       `json:"base_cutoff_seq" db:"base_cutoff_seq"`
	Status         string    `json:"status" db:"status"`
	AttemptCount   int       `json:"attempt_count" db:"attempt_count"`
	NextRunAt      time.Time `json:"next_run_at" db:"next_run_at"`
	LastError      string    `json:"last_error" db:"last_error"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	Content  string `json:"content" binding:"required"`
	PresetID string `json:"preset_id"`
}

// GenerateCharacterCardRequest 模板生成角色卡请求
type GenerateCharacterCardRequest struct {
	Gender            string `json:"gender" binding:"required"`
	Setting           string `json:"setting" binding:"required"`
	Type              string `json:"type" binding:"required"`
	Personality       string `json:"personality" binding:"required"`
	POV               string `json:"pov" binding:"required"`
	CustomPersonality string `json:"custom_personality"`
}

// GenerateCharacterCardResponse 模板生成角色卡响应
type GenerateCharacterCardResponse struct {
	Draft CharacterDraft `json:"draft"`
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
