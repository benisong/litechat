package service

import (
	"context"
	"errors"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"sync"
)

type StoryPrimaryClient interface {
	Stream(ctx context.Context, modelName string, messages []model.ChatCompletionMessage, callback StreamCallback) (string, error)
}

type StoryPromptBuilder interface {
	BuildStoryPrompt(ctx context.Context, chat *model.Chat, history []*model.Message, content string, state *model.ChatStoryState) ([]model.ChatCompletionMessage, SchedulerValidationSpec, error)
}

type StoryRuntimeStatusEvent struct {
	Status       string
	RecordID     string
	ErrorMessage string
}

type StoryProcessResult struct {
	Status       string
	RecordID     string
	ContextText  string
	ErrorMessage string
}

type StoryTurnProcessor interface {
	ProcessStoryTurn(ctx context.Context, record *model.ChatSchedulerRecord, state *model.ChatStoryState, messages []model.ChatCompletionMessage, spec SchedulerValidationSpec) (StoryProcessResult, error)
}

// StoryChatRuntime 是复杂剧情独立运行时，不修改 LegacyChatRuntime 的业务流程。
type StoryChatRuntime struct {
	chatStore     *store.ChatStore
	messageStore  *store.MessageStore
	storyStore    *store.SchedulerStore
	promptBuilder StoryPromptBuilder
	primaryClient StoryPrimaryClient
	turnProcessor StoryTurnProcessor
	primaryModel  string
	turnMu        sync.Mutex
	activeTurns   map[string]struct{}
}

type StoryChatRuntimeDeps struct {
	ChatStore     *store.ChatStore
	MessageStore  *store.MessageStore
	StoryStore    *store.SchedulerStore
	PromptBuilder StoryPromptBuilder
	PrimaryClient StoryPrimaryClient
	TurnProcessor StoryTurnProcessor
	PrimaryModel  string
}

func NewStoryChatRuntime(deps StoryChatRuntimeDeps) *StoryChatRuntime {
	return &StoryChatRuntime{
		chatStore: deps.ChatStore, messageStore: deps.MessageStore, storyStore: deps.StoryStore,
		promptBuilder: deps.PromptBuilder, primaryClient: deps.PrimaryClient,
		turnProcessor: deps.TurnProcessor, primaryModel: deps.PrimaryModel,
	}
}

func (r *StoryChatRuntime) beginTurn(chatID string) bool {
	r.turnMu.Lock()
	defer r.turnMu.Unlock()
	if r.activeTurns == nil {
		r.activeTurns = make(map[string]struct{})
	}
	if _, exists := r.activeTurns[chatID]; exists {
		return false
	}
	r.activeTurns[chatID] = struct{}{}
	return true
}

func (r *StoryChatRuntime) finishTurn(chatID string) {
	r.turnMu.Lock()
	delete(r.activeTurns, chatID)
	r.turnMu.Unlock()
}

func (r *StoryChatRuntime) SendMessage(ctx context.Context, input ChatTurnInput, callback StreamCallback) (ChatRuntimeResult, error) {
	return r.SendMessageWithEvents(ctx, input, callback, nil)
}

func (r *StoryChatRuntime) SendMessageWithEvents(ctx context.Context, input ChatTurnInput, callback StreamCallback, statusCallback func(StoryRuntimeStatusEvent) error) (ChatRuntimeResult, error) {
	if err := r.validate(); err != nil {
		return ChatRuntimeResult{}, err
	}
	chat, err := r.chatStore.GetByID(input.ChatID, input.UserID)
	if err != nil {
		return ChatRuntimeResult{}, err
	}
	if !chat.SchedulerEnabled {
		return ChatRuntimeResult{}, errors.New("chat is not configured for story runtime")
	}
	if !r.beginTurn(input.ChatID) {
		return ChatRuntimeResult{}, fmt.Errorf("story chat is busy")
	}
	defer r.finishTurn(input.ChatID)
	state, err := r.storyStore.GetStoryState(input.ChatID)
	if err != nil {
		return ChatRuntimeResult{}, errors.New("story runtime is not initialized")
	}
	history, err := r.messageStore.ListByChatID(input.ChatID)
	if err != nil {
		return ChatRuntimeResult{}, err
	}
	userMessage := &model.Message{ChatID: input.ChatID, Role: "user", Content: input.Content}
	if err := r.messageStore.Create(userMessage); err != nil {
		return ChatRuntimeResult{}, err
	}
	messages, spec, err := r.promptBuilder.BuildStoryPrompt(ctx, chat, history, input.Content, state)
	if err != nil {
		return ChatRuntimeResult{}, err
	}
	assistantContent, err := r.primaryClient.Stream(ctx, r.primaryModel, messages, callback)
	if err != nil {
		return ChatRuntimeResult{}, err
	}
	assistantMessage := &model.Message{ChatID: input.ChatID, Role: "assistant", Content: assistantContent}
	if err := r.messageStore.Create(assistantMessage); err != nil {
		return ChatRuntimeResult{}, err
	}
	record := &model.ChatSchedulerRecord{ChatID: input.ChatID, UserMessageID: userMessage.ID, AssistantMessageID: assistantMessage.ID, TurnSeq: assistantMessage.Seq}
	if err := r.storyStore.CreateRecord(record); err != nil {
		return ChatRuntimeResult{}, err
	}
	if statusCallback != nil {
		if err := statusCallback(StoryRuntimeStatusEvent{Status: "processing", RecordID: record.ID}); err != nil {
			return ChatRuntimeResult{}, err
		}
	}
	processed, err := r.turnProcessor.ProcessStoryTurn(ctx, record, state, messages, spec)
	if err != nil {
		_ = r.storyStore.MarkStoryTurnFailed(record.ChatID, record.ID, "processor_error", err.Error())
		event := StoryRuntimeStatusEvent{Status: "failed", RecordID: record.ID, ErrorMessage: err.Error()}
		if statusCallback != nil {
			_ = statusCallback(event)
		}
		return ChatRuntimeResult{AssistantContent: assistantContent, AssistantMessageID: assistantMessage.ID, SchedulerStatus: "failed", SchedulerRecordID: record.ID, SchedulerError: err.Error()}, nil
	}
	if statusCallback != nil {
		_ = statusCallback(StoryRuntimeStatusEvent{Status: processed.Status, RecordID: processed.RecordID, ErrorMessage: processed.ErrorMessage})
	}
	return ChatRuntimeResult{AssistantContent: assistantContent, AssistantMessageID: assistantMessage.ID, SchedulerStatus: processed.Status, SchedulerRecordID: processed.RecordID, SchedulerError: processed.ErrorMessage}, nil
}

func (r *StoryChatRuntime) Regenerate(context.Context, ChatRegenerateInput, StreamCallback) (ChatRuntimeResult, error) {
	return ChatRuntimeResult{}, errors.New("story runtime regeneration is disabled in V1")
}

func (r *StoryChatRuntime) Retry(ctx context.Context, input ChatTurnInput, callback StreamCallback) (ChatRuntimeResult, error) {
	return r.RetryWithEvents(ctx, input, nil)
}

func (r *StoryChatRuntime) RetryWithEvents(ctx context.Context, input ChatTurnInput, statusCallback func(StoryRuntimeStatusEvent) error) (ChatRuntimeResult, error) {
	if err := r.validate(); err != nil {
		return ChatRuntimeResult{}, err
	}
	chat, err := r.chatStore.GetByID(input.ChatID, input.UserID)
	if err != nil {
		return ChatRuntimeResult{}, err
	}
	if !chat.SchedulerEnabled {
		return ChatRuntimeResult{}, errors.New("chat is not configured for story runtime")
	}
	if !r.beginTurn(input.ChatID) {
		return ChatRuntimeResult{}, fmt.Errorf("story chat is busy")
	}
	defer r.finishTurn(input.ChatID)
	state, err := r.storyStore.GetStoryState(input.ChatID)
	if err != nil {
		return ChatRuntimeResult{}, errors.New("story runtime is not initialized")
	}
	record, err := r.storyStore.LatestRetryableRecord(input.ChatID)
	if err != nil {
		return ChatRuntimeResult{}, fmt.Errorf("no failed scheduler record to retry: %w", err)
	}
	if err := r.storyStore.MarkStoryTurnProcessing(input.ChatID, record.ID); err != nil {
		return ChatRuntimeResult{}, err
	}
	if statusCallback != nil {
		_ = statusCallback(StoryRuntimeStatusEvent{Status: "processing", RecordID: record.ID})
	}
	processed, processErr := r.turnProcessor.ProcessStoryTurn(ctx, record, state, nil, SchedulerValidationSpec{})
	if processErr != nil {
		_ = r.storyStore.MarkStoryTurnFailed(input.ChatID, record.ID, "retry_error", processErr.Error())
		if statusCallback != nil {
			_ = statusCallback(StoryRuntimeStatusEvent{Status: "failed", RecordID: record.ID, ErrorMessage: processErr.Error()})
		}
		return ChatRuntimeResult{SchedulerStatus: "failed", SchedulerRecordID: record.ID, SchedulerError: processErr.Error()}, nil
	}
	if statusCallback != nil {
		_ = statusCallback(StoryRuntimeStatusEvent{Status: processed.Status, RecordID: processed.RecordID, ErrorMessage: processed.ErrorMessage})
	}
	return ChatRuntimeResult{SchedulerStatus: processed.Status, SchedulerRecordID: processed.RecordID, SchedulerError: processed.ErrorMessage}, nil
}

func (r *StoryChatRuntime) validate() error {
	switch {
	case r == nil:
		return errors.New("story runtime is nil")
	case r.chatStore == nil, r.messageStore == nil, r.storyStore == nil:
		return errors.New("story runtime stores are not configured")
	case r.promptBuilder == nil:
		return errors.New("story prompt builder is not configured")
	case r.primaryClient == nil:
		return errors.New("story primary client is not configured")
	case r.turnProcessor == nil:
		return errors.New("story turn processor is not configured")
	case r.primaryModel == "":
		return errors.New("story primary model is not configured")
	default:
		return nil
	}
}
