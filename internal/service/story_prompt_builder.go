package service

import (
	"context"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
)

type DefaultStoryPromptBuilder struct {
	characters *store.CharacterStore
	worldbooks *store.WorldBookStore
	presets    *store.PresetStore
	contexts   StoryContextBuilder
}

func NewDefaultStoryPromptBuilder(characters *store.CharacterStore, worldbooks *store.WorldBookStore, presets *store.PresetStore) *DefaultStoryPromptBuilder {
	return &DefaultStoryPromptBuilder{characters: characters, worldbooks: worldbooks, presets: presets}
}

func (b *DefaultStoryPromptBuilder) BuildStoryPrompt(ctx context.Context, chat *model.Chat, history []*model.Message, content string, state *model.ChatStoryState) ([]model.ChatCompletionMessage, SchedulerValidationSpec, error) {
	if b == nil || b.characters == nil || b.worldbooks == nil {
		return nil, SchedulerValidationSpec{}, fmt.Errorf("story prompt builder is not configured")
	}
	if chat == nil {
		return nil, SchedulerValidationSpec{}, fmt.Errorf("chat is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, SchedulerValidationSpec{}, err
	}
	character, err := b.characters.GetByID(chat.CharacterID, chat.UserID)
	if err != nil {
		return nil, SchedulerValidationSpec{}, err
	}
	base := []model.ChatCompletionMessage{{Role: "system", Content: buildStoryCharacterPrompt(character)}}
	if b.presets != nil {
		preset, presetErr := b.loadPreset(chat)
		if presetErr == nil && preset != nil && strings.TrimSpace(preset.SystemPrompt) != "" {
			base = append(base, model.ChatCompletionMessage{Role: "system", Content: preset.SystemPrompt})
		}
	}
	books, err := b.loadStaticWorldbooks(chat.UserID, chat.CharacterID)
	if err != nil {
		return nil, SchedulerValidationSpec{}, err
	}
	runtimeContext := storyRuntimeContext(state)
	messages := b.contexts.Build(base, books, runtimeContext)
	for _, message := range history {
		if message == nil || message.Role == "system" {
			continue
		}
		messages = append(messages, model.ChatCompletionMessage{Role: message.Role, Content: message.Content})
	}
	if strings.TrimSpace(content) != "" {
		messages = append(messages, model.ChatCompletionMessage{Role: "user", Content: content})
	}
	return messages, SchedulerValidationSpec{}, nil
}

func (b *DefaultStoryPromptBuilder) loadPreset(chat *model.Chat) (*model.Preset, error) {
	if chat.PresetID != "" {
		return b.presets.GetByID(chat.PresetID, chat.UserID)
	}
	return b.presets.GetDefault(chat.UserID)
}

func (b *DefaultStoryPromptBuilder) loadStaticWorldbooks(userID, characterID string) ([]*model.WorldBook, error) {
	books, err := b.worldbooks.List(userID)
	if err != nil {
		return nil, err
	}
	result := make([]*model.WorldBook, 0, len(books))
	for _, book := range books {
		if book == nil || book.RuntimeMode != "static" || (book.CharacterID != "" && book.CharacterID != characterID) {
			continue
		}
		entries, err := b.worldbooks.ListEntries(book.ID, userID)
		if err != nil {
			return nil, err
		}
		book.Entries = entries
		result = append(result, book)
	}
	return result, nil
}

func buildStoryCharacterPrompt(character *model.Character) string {
	parts := []string{"[Character]", "Name: " + character.Name}
	if strings.TrimSpace(character.Description) != "" {
		parts = append(parts, "Description: "+character.Description)
	}
	if strings.TrimSpace(character.Personality) != "" {
		parts = append(parts, "Personality: "+character.Personality)
	}
	if strings.TrimSpace(character.Scenario) != "" {
		parts = append(parts, "Scenario: "+character.Scenario)
	}
	if strings.TrimSpace(character.UserName) != "" {
		parts = append(parts, "User: "+character.UserName)
	}
	if strings.TrimSpace(character.UserDetail) != "" {
		parts = append(parts, "User Detail: "+character.UserDetail)
	}
	return strings.Join(parts, "\n")
}

func storyRuntimeContext(state *model.ChatStoryState) string {
	if state == nil {
		return ""
	}
	var parts []string
	if state.CurrentScene != "" {
		parts = append(parts, "当前场景："+state.CurrentScene)
	}
	if state.ActiveEvent != "" {
		parts = append(parts, "当前事件："+state.ActiveEvent)
	}
	if state.Route != "" {
		parts = append(parts, "当前路线："+state.Route)
	}
	return strings.Join(parts, "\n")
}
