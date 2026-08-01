package service

import (
	"context"
	"encoding/json"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
)

type StoryChatInitializeInput struct {
	UserID               string
	CharacterID          string
	CharacterVersion     string
	WorldbookVersionHash string
	Title                string
	PresetID             string
	CompilerModel        string
	PromptVersion        string
	CompileOnlyText      string
}

type StoryChatInitializeResult struct {
	Chat     *model.Chat
	Manifest *model.StoryManifest
	State    *model.ChatStoryState
}

type StoryChatInitializer struct {
	chatStore      *store.ChatStore
	storyStore     *store.SchedulerStore
	characterStore *store.CharacterStore
	compiler       *ManifestCompiler
}

func NewStoryChatInitializer(chatStore *store.ChatStore, storyStore *store.SchedulerStore, characterStore *store.CharacterStore, compiler *ManifestCompiler) *StoryChatInitializer {
	return &StoryChatInitializer{chatStore: chatStore, storyStore: storyStore, characterStore: characterStore, compiler: compiler}
}

func (i *StoryChatInitializer) Initialize(ctx context.Context, input StoryChatInitializeInput) (*StoryChatInitializeResult, error) {
	if i == nil || i.chatStore == nil || i.storyStore == nil || i.characterStore == nil || i.compiler == nil {
		return nil, fmt.Errorf("story chat initializer is not configured")
	}
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.CharacterID) == "" {
		return nil, fmt.Errorf("user id and character id are required")
	}
	character, err := i.characterStore.GetByID(input.CharacterID, input.UserID)
	if err != nil {
		return nil, err
	}
	compilerModel := input.CompilerModel
	if compilerModel == "" {
		compilerModel = "story-compiler"
	}
	promptVersion := input.PromptVersion
	if promptVersion == "" {
		promptVersion = "compiler-v1"
	}
	manifest, err := i.compiler.CompileOrReuse(ctx, ManifestCompileInput{
		CharacterID: input.CharacterID, CharacterVersion: input.CharacterVersion,
		WorldbookVersionHash: input.WorldbookVersionHash, CompilerModel: compilerModel,
		PromptVersion: promptVersion, CompileOnlyText: input.CompileOnlyText,
	})
	if err != nil {
		return nil, err
	}
	stateJSON, err := initialStateJSON(manifest.CompiledJSON)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = character.Name
	}
	chat := &model.Chat{CharacterID: input.CharacterID, Title: title, PresetID: input.PresetID, SchedulerEnabled: true}
	if err := i.chatStore.Create(chat, input.UserID); err != nil {
		return nil, err
	}
	state := &model.ChatStoryState{ChatID: chat.ID, ManifestID: manifest.ID, StateJSON: stateJSON}
	if err := i.storyStore.CreateStoryState(state); err != nil {
		return nil, err
	}
	return &StoryChatInitializeResult{Chat: chat, Manifest: manifest, State: state}, nil
}

func initialStateJSON(compiled string) (string, error) {
	var document manifestRuntimeDocument
	if err := json.Unmarshal([]byte(compiled), &document); err != nil {
		return "", fmt.Errorf("decode manifest for initial state: %w", err)
	}
	values := make(map[string]any, len(document.Fields))
	for key, field := range document.Fields {
		if field.Initial != nil {
			values[key] = field.Initial
			continue
		}
		switch field.Type {
		case "boolean":
			values[key] = false
		case "integer":
			values[key] = 0
		case "number":
			values[key] = 0.0
		case "string", "enum":
			values[key] = ""
		case "string_set", "event_set":
			values[key] = []string{}
		default:
			return "", fmt.Errorf("unsupported initial field type %q", field.Type)
		}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode initial state: %w", err)
	}
	return string(data), nil
}
