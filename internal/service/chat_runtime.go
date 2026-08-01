package service

import "context"

// ChatTurnInput 是运行时处理一轮聊天所需的最小输入。
type ChatTurnInput struct {
	ChatID   string
	Content  string
	PresetID string
	UserID   string
}

// ChatRegenerateInput 是重新生成所需的输入。
type ChatRegenerateInput struct {
	ChatID string
	UserID string
}

// ChatRuntimeResult 是两种运行时对外统一的结果。
type ChatRuntimeResult struct {
	AssistantContent   string
	AssistantMessageID string
	SchedulerStatus    string
	SchedulerRecordID  string
	SchedulerError     string
}

// ChatRuntime 定义聊天业务动作，不暴露具体存储、提示词和模型实现。
type ChatRuntime interface {
	SendMessage(ctx context.Context, input ChatTurnInput, callback StreamCallback) (ChatRuntimeResult, error)
	Regenerate(ctx context.Context, input ChatRegenerateInput, callback StreamCallback) (ChatRuntimeResult, error)
	Retry(ctx context.Context, input ChatTurnInput, callback StreamCallback) (ChatRuntimeResult, error)
}

// StoryMessageRuntime 是复杂剧情消息入口的最小接口。
type StorySchedulerRetryRuntime interface {
	RetryWithEvents(ctx context.Context, input ChatTurnInput, statusCallback func(StoryRuntimeStatusEvent) error) (ChatRuntimeResult, error)
}

// StoryMessageRuntime 是复杂剧情消息入口的最小接口。
type StoryMessageRuntime interface {
	SendMessageWithEvents(ctx context.Context, input ChatTurnInput, callback StreamCallback, statusCallback func(StoryRuntimeStatusEvent) error) (ChatRuntimeResult, error)
}
