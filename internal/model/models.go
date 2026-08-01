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
	ID               string     `json:"id" db:"id"`
	UserID           string     `json:"user_id" db:"user_id"`
	CharacterID      string     `json:"character_id" db:"character_id"`
	Title            string     `json:"title" db:"title"`
	PresetID         string     `json:"preset_id" db:"preset_id"`
	SchedulerEnabled bool       `json:"scheduler_enabled" db:"scheduler_enabled"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
	Character        *Character `json:"character,omitempty" db:"-"`
	LastMessage      string     `json:"last_message,omitempty" db:"-"`
	MsgCount         int        `json:"msg_count,omitempty" db:"-"`
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
	StatusBar string    `json:"status_bar,omitempty" db:"status_bar"`
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
	RuntimeMode   string           `json:"runtime_mode" db:"runtime_mode"`
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

// ChatSummaryJob 摘要后台任务
type ChatSummaryJob struct {
	ID            string    `json:"id" db:"id"`
	ChatID        string    `json:"chat_id" db:"chat_id"`
	JobType       string    `json:"job_type" db:"job_type"`
	FromSeq       int       `json:"from_seq" db:"from_seq"`
	ToSeq         int       `json:"to_seq" db:"to_seq"`
	BaseCutoffSeq int       `json:"base_cutoff_seq" db:"base_cutoff_seq"`
	Status        string    `json:"status" db:"status"`
	AttemptCount  int       `json:"attempt_count" db:"attempt_count"`
	NextRunAt     time.Time `json:"next_run_at" db:"next_run_at"`
	LastError     string    `json:"last_error" db:"last_error"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// SchedulerStatus 调度记录状态。
type SchedulerStatus string

const (
	SchedulerStatusPending    SchedulerStatus = "pending"
	SchedulerStatusProcessing SchedulerStatus = "processing"
	SchedulerStatusSuccess    SchedulerStatus = "success"
	SchedulerStatusFailed     SchedulerStatus = "failed"
	SchedulerStatusInvalid    SchedulerStatus = "invalid"
	SchedulerStatusConflict   SchedulerStatus = "conflict"
)

// ManifestStatus 剧情 Manifest 编译状态。
type ManifestStatus string

const (
	ManifestStatusPending    ManifestStatus = "pending"
	ManifestStatusProcessing ManifestStatus = "processing"
	ManifestStatusReady      ManifestStatus = "ready"
	ManifestStatusFailed     ManifestStatus = "failed"
	ManifestStatusStale      ManifestStatus = "stale"
)

// StoryManifest 角色卡剧情世界书的编译结果。
type StoryManifest struct {
	ID                   string         `json:"id" db:"id"`
	CharacterID          string         `json:"character_id" db:"character_id"`
	CharacterVersion     string         `json:"character_version" db:"character_version"`
	WorldbookVersionHash string         `json:"worldbook_version_hash" db:"worldbook_version_hash"`
	ManifestVersion      int            `json:"manifest_version" db:"manifest_version"`
	Status               ManifestStatus `json:"status" db:"status"`
	CompiledJSON         string         `json:"compiled_json" db:"compiled_json"`
	CompilerModel        string         `json:"compiler_model" db:"compiler_model"`
	PromptVersion        string         `json:"prompt_version" db:"prompt_version"`
	ErrorMessage         string         `json:"error_message" db:"error_message"`
	CreatedAt            time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at" db:"updated_at"`
}

// ChatStoryState 每个复杂剧情聊天独立的运行状态。
type ChatStoryState struct {
	ChatID              string    `json:"chat_id" db:"chat_id"`
	ManifestID          string    `json:"manifest_id" db:"manifest_id"`
	StateVersion        int       `json:"state_version" db:"state_version"`
	StateJSON           string    `json:"state_json" db:"state_json"`
	CurrentScene        string    `json:"current_scene" db:"current_scene"`
	ActiveEvent         string    `json:"active_event" db:"active_event"`
	Route               string    `json:"route" db:"route"`
	SchedulerStatus     string    `json:"scheduler_status" db:"scheduler_status"`
	LastSuccessRecordID string    `json:"last_success_record_id" db:"last_success_record_id"`
	FailureCount        int       `json:"failure_count" db:"failure_count"`
	CreatedAt           time.Time `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time `json:"updated_at" db:"updated_at"`
}

// SchedulerObservation 调度模型从本轮对话中提取的候选事实。
type SchedulerObservation struct {
	Key        string  `json:"key"`
	Value      any     `json:"value"`
	Evidence   string  `json:"evidence"`
	Confidence float64 `json:"confidence"`
}

// SchedulerEventCandidate 调度模型建议检查的事件。
type SchedulerEventCandidate struct {
	EventID  string `json:"event_id"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
}

// SchedulerOutput 调度模型的结构化候选输出。
type SchedulerOutput struct {
	SchemaVersion   int                       `json:"schema_version"`
	Observations    []SchedulerObservation    `json:"observations"`
	EventCandidates []SchedulerEventCandidate `json:"event_candidates"`
	Inferences      []map[string]any          `json:"inferences"`
	Warnings        []string                  `json:"warnings"`
}

// ChatStoryEvent 已确认发生的剧情事件，只追加不覆盖。
type ChatStoryEvent struct {
	ID                string    `json:"id" db:"id"`
	ChatID            string    `json:"chat_id" db:"chat_id"`
	SchedulerRecordID string    `json:"scheduler_record_id" db:"scheduler_record_id"`
	EventKey          string    `json:"event_key" db:"event_key"`
	EventType         string    `json:"event_type" db:"event_type"`
	Summary           string    `json:"summary" db:"summary"`
	Importance        string    `json:"importance" db:"importance"`
	Evidence          string    `json:"evidence" db:"evidence"`
	Status            string    `json:"status" db:"status"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// ChatSchedulerRecord 一轮剧情调度记录。
type ChatSchedulerRecord struct {
	ID                 string          `json:"id" db:"id"`
	ChatID             string          `json:"chat_id" db:"chat_id"`
	UserMessageID      string          `json:"user_message_id" db:"user_message_id"`
	AssistantMessageID string          `json:"assistant_message_id" db:"assistant_message_id"`
	TurnSeq            int             `json:"turn_seq" db:"turn_seq"`
	Status             SchedulerStatus `json:"status" db:"status"`
	AttemptCount       int             `json:"attempt_count" db:"attempt_count"`
	SchedulerModel     string          `json:"scheduler_model" db:"scheduler_model"`
	PromptVersion      string          `json:"prompt_version" db:"prompt_version"`
	InputSnapshot      string          `json:"input_snapshot" db:"input_snapshot"`
	RawOutput          string          `json:"raw_output" db:"raw_output"`
	ParsedOutput       string          `json:"parsed_output" db:"parsed_output"`
	AppliedChanges     string          `json:"applied_changes" db:"applied_changes"`
	ContextText        string          `json:"context_text" db:"context_text"`
	StateVersionBefore int             `json:"state_version_before" db:"state_version_before"`
	StateVersionAfter  int             `json:"state_version_after" db:"state_version_after"`
	ErrorCode          string          `json:"error_code" db:"error_code"`
	ErrorMessage       string          `json:"error_message" db:"error_message"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	StartedAt          *time.Time      `json:"started_at,omitempty" db:"started_at"`
	FinishedAt         *time.Time      `json:"finished_at,omitempty" db:"finished_at"`
	AppliedAt          *time.Time      `json:"applied_at,omitempty" db:"applied_at"`
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
