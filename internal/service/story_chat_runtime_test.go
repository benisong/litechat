package service

import (
	"context"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

type fakeStoryPromptBuilder struct{}

func (fakeStoryPromptBuilder) BuildStoryPrompt(context.Context, *model.Chat, []*model.Message, string, *model.ChatStoryState) ([]model.ChatCompletionMessage, SchedulerValidationSpec, error) {
	return []model.ChatCompletionMessage{{Role: "system", Content: "story context"}}, SchedulerValidationSpec{}, nil
}

type fakeStoryPrimaryClient struct{}

func (fakeStoryPrimaryClient) Stream(_ context.Context, modelName string, _ []model.ChatCompletionMessage, callback StreamCallback) (string, error) {
	if modelName != "story-model" {
		return "", fmt.Errorf("unexpected model: %s", modelName)
	}
	if callback != nil {
		if err := callback("回复"); err != nil {
			return "", err
		}
	}
	return "回复", nil
}

type fakeStoryTurnProcessor struct{}

func (fakeStoryTurnProcessor) ProcessStoryTurn(_ context.Context, record *model.ChatSchedulerRecord, _ *model.ChatStoryState, _ []model.ChatCompletionMessage, _ SchedulerValidationSpec) (StoryProcessResult, error) {
	return StoryProcessResult{Status: "success", RecordID: record.ID}, nil
}

func TestStoryChatRuntimeRunsIndependentTurn(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	userStore := store.NewUserStore(db)
	user := &model.User{Username: "story-user", Role: "user", Mode: "self"}
	if err := userStore.Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	characterStore := store.NewCharacterStore(db)
	character := &model.Character{Name: "复杂角色"}
	if err := characterStore.Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	chatStore := store.NewChatStore(db)
	chat := &model.Chat{CharacterID: character.ID, Title: "剧情会话", SchedulerEnabled: true}
	if err := chatStore.Create(chat, user.ID); err != nil {
		t.Fatalf("Create chat: %v", err)
	}
	storyStore := store.NewSchedulerStore(db)
	if err := storyStore.CreateStoryState(&model.ChatStoryState{ChatID: chat.ID, ManifestID: "manifest-1", StateJSON: `{}`}); err != nil {
		t.Fatalf("Create state: %v", err)
	}

	runtime := NewStoryChatRuntime(StoryChatRuntimeDeps{
		ChatStore: chatStore, MessageStore: store.NewMessageStore(db), StoryStore: storyStore,
		PromptBuilder: fakeStoryPromptBuilder{}, PrimaryClient: fakeStoryPrimaryClient{},
		TurnProcessor: fakeStoryTurnProcessor{}, PrimaryModel: "story-model",
	})
	events := make([]StoryRuntimeStatusEvent, 0, 2)
	got, err := runtime.SendMessageWithEvents(context.Background(), ChatTurnInput{ChatID: chat.ID, UserID: user.ID, Content: "开始剧情"}, nil, func(event StoryRuntimeStatusEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if got.AssistantContent != "回复" || got.SchedulerStatus != "success" || got.SchedulerRecordID == "" {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(events) != 2 || events[0].Status != "processing" || events[1].Status != "success" || events[0].RecordID != events[1].RecordID {
		t.Fatalf("unexpected runtime events: %+v", events)
	}
	messages, err := store.NewMessageStore(db).ListByChatID(chat.ID)
	if err != nil {
		t.Fatalf("ListByChatID: %v", err)
	}
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
}

func TestStoryChatRuntimeRejectsLegacyChat(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	userStore := store.NewUserStore(db)
	user := &model.User{Username: "legacy-user", Role: "user", Mode: "self"}
	if err := userStore.Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "普通角色"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	chat := &model.Chat{CharacterID: character.ID, Title: "普通会话", SchedulerEnabled: false}
	if err := store.NewChatStore(db).Create(chat, user.ID); err != nil {
		t.Fatalf("Create chat: %v", err)
	}
	runtime := NewStoryChatRuntime(StoryChatRuntimeDeps{
		ChatStore: store.NewChatStore(db), MessageStore: store.NewMessageStore(db), StoryStore: store.NewSchedulerStore(db),
		PromptBuilder: fakeStoryPromptBuilder{}, PrimaryClient: fakeStoryPrimaryClient{}, TurnProcessor: fakeStoryTurnProcessor{}, PrimaryModel: "story-model",
	})
	if _, err := runtime.SendMessage(context.Background(), ChatTurnInput{ChatID: chat.ID, UserID: user.ID, Content: "不应进入剧情流程"}, nil); err == nil {
		t.Fatal("expected legacy chat to be rejected")
	}
}
