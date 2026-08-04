package service

import (
	"context"
	"encoding/json"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
)

const manifestSchemaVersion = 1

type ManifestCompileInput struct {
	CharacterID          string
	CharacterVersion     string
	WorldbookVersionHash string
	CompilerModel        string
	PromptVersion        string
	CompileOnlyText      string
}

type ManifestCompiler struct {
	storyStore *store.SchedulerStore
	client     CompletionClient
}

func NewManifestCompiler(storyStore *store.SchedulerStore, client CompletionClient) *ManifestCompiler {
	return &ManifestCompiler{storyStore: storyStore, client: client}
}

func (c *ManifestCompiler) Retry(ctx context.Context, manifestID string, input ManifestCompileInput) (*model.StoryManifest, error) {
	if c == nil || c.storyStore == nil {
		return nil, fmt.Errorf("manifest compiler is not configured")
	}
	if strings.TrimSpace(manifestID) == "" {
		return nil, fmt.Errorf("manifest id is required")
	}
	previous, err := c.storyStore.GetManifest(manifestID)
	if err != nil {
		return nil, err
	}
	if previous.Status != model.ManifestStatusFailed && previous.Status != model.ManifestStatusStale {
		return nil, fmt.Errorf("manifest %s is not retryable in status %s", manifestID, previous.Status)
	}
	if input.CharacterID == "" {
		input.CharacterID = previous.CharacterID
	}
	if input.CharacterVersion == "" {
		input.CharacterVersion = previous.CharacterVersion
	}
	if input.WorldbookVersionHash == "" {
		input.WorldbookVersionHash = previous.WorldbookVersionHash
	}
	if input.CompilerModel == "" {
		input.CompilerModel = previous.CompilerModel
	}
	if input.PromptVersion == "" {
		input.PromptVersion = previous.PromptVersion
	}
	return c.Compile(ctx, input)
}

func (c *ManifestCompiler) CompileOrReuse(ctx context.Context, input ManifestCompileInput) (*model.StoryManifest, error) {
	if c == nil || c.storyStore == nil {
		return nil, fmt.Errorf("manifest compiler is not configured")
	}
	if cached, err := c.storyStore.GetReadyManifest(input.CharacterID, input.CharacterVersion, input.WorldbookVersionHash, input.CompilerModel, input.PromptVersion, fmt.Sprint(manifestSchemaVersion)); err == nil {
		return cached, nil
	}
	return c.Compile(ctx, input)
}

func (c *ManifestCompiler) Compile(ctx context.Context, input ManifestCompileInput) (*model.StoryManifest, error) {
	manifest := &model.StoryManifest{
		CharacterID: input.CharacterID, CharacterVersion: input.CharacterVersion,
		WorldbookVersionHash: input.WorldbookVersionHash, ManifestVersion: manifestSchemaVersion,
		CompilerModel: input.CompilerModel, PromptVersion: input.PromptVersion,
	}
	if c == nil || c.storyStore == nil || c.client == nil {
		return manifest, fmt.Errorf("manifest compiler is not configured")
	}
	if strings.TrimSpace(input.CharacterID) == "" || strings.TrimSpace(input.CompilerModel) == "" {
		return manifest, fmt.Errorf("character id and compiler model are required")
	}
	if err := c.storyStore.CreateManifest(manifest); err != nil {
		return manifest, err
	}
	fail := func(code string, err error) (*model.StoryManifest, error) {
		_ = c.storyStore.MarkManifestFailed(manifest.ID, fmt.Sprintf("%s: %v", code, err))
		manifest.Status = model.ManifestStatusFailed
		manifest.ErrorMessage = err.Error()
		return manifest, err
	}
	messages := []model.ChatCompletionMessage{
		{Role: "system", Content: manifestCompilerSystemPrompt},
		{Role: "user", Content: input.CompileOnlyText},
	}
	raw, err := c.client.Complete(ctx, input.CompilerModel, messages)
	if err != nil {
		return fail("compiler_request", err)
	}
	jsonText, err := extractSchedulerJSONObject(raw)
	if err != nil {
		return fail("compiler_output", err)
	}
	if err := validateManifestJSON(jsonText); err != nil {
		return fail("manifest_validation", err)
	}
	if err := c.storyStore.MarkManifestReady(manifest.ID, jsonText, input.PromptVersion, input.CompilerModel); err != nil {
		return fail("manifest_persist", err)
	}
	stored, err := c.storyStore.GetManifest(manifest.ID)
	if err != nil {
		return manifest, err
	}
	return stored, nil
}

const manifestCompilerSystemPrompt = `你是剧情 Manifest 编译器。请把输入的完整剧情世界书编译成严格 JSON。
只允许输出 manifest_version、fields、observation_rules 三个顶层字段。
manifest_version 必须输出数字 1，不要输出字符串 "1.0"。
fields 的每个字段必须包含 type 和 writable；type 只能是 boolean、integer、number、string、enum、string_set、event_set，禁止使用 array 或其他类型。
observation_rules 只能声明 observation_key、value、effects、events。
不要输出 SQL、代码、解释文字或未声明字段。只输出 JSON。`

func validateManifestJSON(raw string) error {
	var document manifestRuntimeDocument
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if document.ManifestVersion != 1 {
		return fmt.Errorf("unsupported manifest_version: %d", document.ManifestVersion)
	}
	if len(document.Fields) == 0 {
		return fmt.Errorf("manifest fields are empty")
	}
	for key, field := range document.Fields {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("manifest contains empty field key")
		}
		if !validManifestFieldType(field.Type) {
			return fmt.Errorf("field %s has invalid type %q", key, field.Type)
		}
	}
	for index := range document.ObservationRules {
		rule := &document.ObservationRules[index]
		rule.ObservationKey = resolveManifestFieldName(rule.ObservationKey, document.Fields)
		for effectIndex := range rule.Effects {
			rule.Effects[effectIndex].Field = resolveManifestFieldName(rule.Effects[effectIndex].Field, document.Fields)
		}
	}
	seenEvents := map[string]bool{}
	for index, rule := range document.ObservationRules {
		if strings.TrimSpace(rule.ObservationKey) == "" {
			return fmt.Errorf("observation rule %d has empty key", index)
		}
		if _, ok := document.Fields[rule.ObservationKey]; !ok {
			return fmt.Errorf("observation rule %d references undeclared key %s", index, rule.ObservationKey)
		}
		for _, effect := range rule.Effects {
			field, ok := document.Fields[effect.Field]
			if !ok {
				return fmt.Errorf("effect references undeclared field %s", effect.Field)
			}
			if !field.Writable {
				return fmt.Errorf("effect references read-only field %s", effect.Field)
			}
			if effect.Operation != "set" && effect.Operation != "increment" && effect.Operation != "decrement" && effect.Operation != "append" {
				return fmt.Errorf("unsupported effect operation %s", effect.Operation)
			}
		}
		for _, event := range rule.Events {
			if strings.TrimSpace(event.EventKey) == "" {
				return fmt.Errorf("observation rule %d has empty event key", index)
			}
			if seenEvents[event.EventKey] {
				return fmt.Errorf("duplicate event key %s", event.EventKey)
			}
			seenEvents[event.EventKey] = true
		}
	}
	return nil
}

func resolveManifestFieldName(name string, fields map[string]manifestFieldJSON) string {
	if _, ok := fields[name]; ok {
		return name
	}
	best := ""
	for candidate := range fields {
		if levenshteinDistance(name, candidate) <= 1 {
			if best != "" {
				return name
			}
			best = candidate
		}
	}
	if best != "" {
		return best
	}
	return name
}

func levenshteinDistance(a, b string) int {
	runesA, runesB := []rune(a), []rune(b)
	prev := make([]int, len(runesB)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ca := range runesA {
		cur := make([]int, len(runesB)+1)
		cur[0] = i + 1
		for j, cb := range runesB {
			cost := 0
			if ca != cb {
				cost = 1
			}
			cur[j+1] = minInt(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(runesB)]
}

func minInt(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func validManifestFieldType(fieldType string) bool {
	switch fieldType {
	case "boolean", "integer", "number", "string", "enum", "string_set", "event_set":
		return true
	default:
		return false
	}
}
