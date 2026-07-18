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
	POV           string    `json:"pov" db:"pov"`
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
	POV           string `json:"pov"`
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
	// 状态栏本地渲染配色（来自角色绑定世界书的状态栏条目，供前端渲染用）
	StatusBarBg string `json:"status_bar_bg,omitempty" db:"-"`
	StatusBarFg string `json:"status_bar_fg,omitempty" db:"-"`
}

// Message 消息模型
type Message struct {
	ID        string    `json:"id" db:"id"`
	ChatID    string    `json:"chat_id" db:"chat_id"`
	Seq       int       `json:"seq" db:"seq"`
	Role      string    `json:"role" db:"role"`
	Content   string    `json:"content" db:"content"`
	StatusBar string    `json:"status_bar,omitempty" db:"-"`
	Tokens    int       `json:"tokens" db:"tokens"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// MessageStatusBar stores the separately persisted status panel for one assistant message.
type MessageStatusBar struct {
	MessageID  string    `json:"message_id" db:"message_id"`
	ChatID     string    `json:"chat_id" db:"chat_id"`
	MessageSeq int       `json:"message_seq" db:"message_seq"`
	Content    string    `json:"content" db:"content"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
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
	EnableTextFix bool             `json:"enable_text_fix,omitempty" db:"-"`
}

// WorldBookEntry 世界书条目
type WorldBookEntry struct {
	ID             string `json:"id" db:"id"`
	UserID         string `json:"user_id" db:"user_id"`
	WorldBookID    string `json:"world_book_id" db:"world_book_id"`
	Keys           string `json:"keys" db:"keys"`
	SecondaryKeys  string `json:"secondary_keys" db:"secondary_keys"`
	Content        string `json:"content" db:"content"`
	Enabled        bool   `json:"enabled" db:"enabled"`
	Constant       bool   `json:"constant" db:"constant"`
	Priority       int    `json:"priority" db:"priority"`
	InjectionPos   int    `json:"injection_position" db:"injection_position"`
	InjectionDepth int    `json:"injection_depth" db:"injection_depth"`
	ScanDepth      int    `json:"scan_depth" db:"scan_depth"`
	CaseSensitive  bool   `json:"case_sensitive" db:"case_sensitive"`
	Order          int    `json:"order" db:"order_num"`
	Role           string `json:"role" db:"role"`
	// 状态栏专用的本地渲染配色（仅状态栏特殊条目使用，普通条目留空）
	BgColor   string    `json:"bg_color" db:"bg_color"`
	FontColor string    `json:"font_color" db:"font_color"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
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
	UseDefaultModelForMemory        bool   `json:"use_default_model_for_memory"`
	MemoryModel                     string `json:"memory_model"`
	MemoryPromptSuffix              string `json:"memory_prompt_suffix"`
	MemorySummaryCharLimit          int    `json:"memory_summary_char_limit"`
	Theme                           string `json:"theme"`
	ServiceMode                     string `json:"service_mode"`
}

// ChatSummaryState 会话摘要状态
type ChatSummaryState struct {
	ChatID            string     `json:"chat_id" db:"chat_id"`
	AppliedCutoffSeq  int        `json:"applied_cutoff_seq" db:"applied_cutoff_seq"`
	CurrentBigSummary string     `json:"current_big_summary_id" db:"current_big_summary_id"`
	DirtyFromSeq      int        `json:"dirty_from_seq" db:"dirty_from_seq"`
	PendingToSeq      int        `json:"pending_to_seq" db:"pending_to_seq"`
	PendingStatus     string     `json:"pending_status" db:"pending_status"`
	PendingRunID      string     `json:"pending_run_id" db:"pending_run_id"`
	PendingAttempts   int        `json:"pending_attempts" db:"pending_attempts"`
	PendingError      string     `json:"pending_error" db:"pending_error"`
	PendingStartedAt  *time.Time `json:"pending_started_at,omitempty" db:"pending_started_at"`
	SummaryRequired   bool       `json:"summary_required" db:"summary_required"`
	NextSummaryFloor  int        `json:"next_summary_floor" db:"next_summary_floor"`
	EligibilitySeq    int        `json:"eligibility_seq" db:"eligibility_seq"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// ChatSummaryChunk 摘要分片（小摘要 / 大摘要）
type ChatSummaryChunk struct {
	ID           string    `json:"id" db:"id"`
	ChatID       string    `json:"chat_id" db:"chat_id"`
	Level        string    `json:"level" db:"level"`
	FromSeq      int       `json:"from_seq" db:"from_seq"`
	ToSeq        int       `json:"to_seq" db:"to_seq"`
	ToMessageID  string    `json:"to_message_id" db:"to_message_id"`
	Content      string    `json:"content" db:"content"`
	Status       string    `json:"status" db:"status"`
	MergedIntoID string    `json:"merged_into_id" db:"merged_into_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	Content  string `json:"content" binding:"required"`
	PresetID string `json:"preset_id"`
}

// GenerateCharacterCardRequest 模板生成角色卡请求
type GenerateCharacterCardRequest struct {
	Gender               string `json:"gender" binding:"required"`
	Setting              string `json:"setting"`
	Type                 string `json:"type"`
	Personality          string `json:"personality"`
	POV                  string `json:"pov" binding:"required"`
	UseSettingPreset     *bool  `json:"use_setting_preset"`
	UseTypePreset        *bool  `json:"use_type_preset"`
	UsePersonalityPreset *bool  `json:"use_personality_preset"`
	CustomSetting        string `json:"custom_setting"`
	CustomType           string `json:"custom_type"`
	CustomPersonality    string `json:"custom_personality"`
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
