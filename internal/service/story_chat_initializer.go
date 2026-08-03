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
	chatStore            *store.ChatStore
	storyStore           *store.SchedulerStore
	characterStore       *store.CharacterStore
	compiler             *ManifestCompiler
	sourceProvider       StorySourceProvider
	defaultCompilerModel string
}

func NewStoryChatInitializer(chatStore *store.ChatStore, storyStore *store.SchedulerStore, characterStore *store.CharacterStore, compiler *ManifestCompiler, providers ...StorySourceProvider) *StoryChatInitializer {
	initializer := &StoryChatInitializer{chatStore: chatStore, storyStore: storyStore, characterStore: characterStore, compiler: compiler}
	if len(providers) > 0 {
		initializer.sourceProvider = providers[0]
	}
	return initializer
}

func (i *StoryChatInitializer) SetDefaultCompilerModel(modelName string) {
	if i != nil && strings.TrimSpace(modelName) != "" {
		i.defaultCompilerModel = strings.TrimSpace(modelName)
	}
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
		compilerModel = i.defaultCompilerModel
	}
	if compilerModel == "" {
		compilerModel = "story-compiler"
	}
	promptVersion := input.PromptVersion
	if promptVersion == "" {
		promptVersion = "compiler-v1"
	}
	worldbookHash := input.WorldbookVersionHash
	compileOnlyText := input.CompileOnlyText
	if i.sourceProvider != nil {
		source, sourceErr := i.sourceProvider.Load(ctx, input.UserID, input.CharacterID)
		if sourceErr != nil {
			return nil, sourceErr
		}
		worldbookHash = source.VersionHash
		compileOnlyText = source.Text
		if strings.TrimSpace(compileOnlyText) == "" {
			compileOnlyText = strings.Join([]string{character.Description, character.Personality, character.Scenario, character.FirstMsg}, "\n\n")
		}
	} else if strings.TrimSpace(compileOnlyText) == "" {
		compileOnlyText = strings.Join([]string{character.Description, character.Personality, character.Scenario, character.FirstMsg}, "\n\n")
	}
	manifest, err := i.compiler.CompileOrReuse(ctx, ManifestCompileInput{
		CharacterID: input.CharacterID, CharacterVersion: input.CharacterVersion,
		WorldbookVersionHash: worldbookHash, CompilerModel: compilerModel,
		PromptVersion: promptVersion, CompileOnlyText: compileOnlyText,
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

func (i *StoryChatInitializer) DeleteChatData(chatID string) error {
	if i == nil || i.storyStore == nil {
		return fmt.Errorf("story initializer is not configured")
	}
	return i.storyStore.DeleteChatStoryData(chatID)
}
func (i *StoryChatInitializer) RetryManifest(ctx context.Context, userID, manifestID string, input ManifestCompileInput) (*model.StoryManifest, error) {
	if i == nil || i.compiler == nil || i.storyStore == nil || i.characterStore == nil {
		return nil, fmt.Errorf("story chat initializer is not configured")
	}
	manifest, err := i.storyStore.GetManifest(manifestID)
	if err != nil {
		return nil, err
	}
	if _, err := i.characterStore.GetByID(manifest.CharacterID, userID); err != nil {
		return nil, err
	}
	if i.sourceProvider != nil {
		source, sourceErr := i.sourceProvider.Load(ctx, userID, manifest.CharacterID)
		if sourceErr != nil {
			return nil, sourceErr
		}
		input.WorldbookVersionHash = source.VersionHash
		input.CompileOnlyText = source.Text
	}
	return i.compiler.Retry(ctx, manifestID, input)
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
