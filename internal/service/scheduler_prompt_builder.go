package service

import (
	"encoding/json"
	"fmt"
	"litechat/internal/model"
	"strings"
)

type SchedulerPromptBuilder struct{}

func NewSchedulerPromptBuilder() *SchedulerPromptBuilder { return &SchedulerPromptBuilder{} }

func (b *SchedulerPromptBuilder) Build(userContent, assistantContent string, state *model.ChatStoryState, compiledManifest string, spec SchedulerValidationSpec) ([]model.ChatCompletionMessage, error) {
	if b == nil {
		return nil, fmt.Errorf("scheduler prompt builder is not configured")
	}
	var document manifestRuntimeDocument
	if err := json.Unmarshal([]byte(compiledManifest), &document); err != nil {
		return nil, fmt.Errorf("decode scheduler manifest: %w", err)
	}
	filtered := manifestRuntimeDocument{ManifestVersion: document.ManifestVersion, Fields: document.Fields}
	for _, rule := range document.ObservationRules {
		if len(spec.AllowedObservationKeys) == 0 || spec.AllowedObservationKeys[rule.ObservationKey] {
			filtered.ObservationRules = append(filtered.ObservationRules, rule)
		}
	}
	rulesJSON, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	stateJSON := "{}"
	if state != nil && strings.TrimSpace(state.StateJSON) != "" {
		stateJSON = state.StateJSON
	}
	metadata := ""
	if state != nil {
		metadata = fmt.Sprintf("\nCurrent scene: %s\nActive event: %s\nRoute: %s", state.CurrentScene, state.ActiveEvent, state.Route)
	}
	system := "You are the story scheduler. Extract candidate observations only. Do not output SQL, code, final numeric decisions, or unsupported fields. Every observation must include evidence. Return the declared JSON schema only.\n\nConfirmed state:\n" + stateJSON + metadata + "\n\nRelevant manifest rules:\n" + string(rulesJSON)
	turn := "USER MESSAGE:\n" + userContent + "\n\nASSISTANT REPLY:\n" + assistantContent
	return []model.ChatCompletionMessage{{Role: "system", Content: system}, {Role: "user", Content: turn}}, nil
}
