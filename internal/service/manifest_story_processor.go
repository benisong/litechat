package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"strconv"
)

type manifestFieldJSON struct {
	Type     string          `json:"type"`
	Writable bool            `json:"writable"`
	Initial  any             `json:"initial"`
	Min      *float64        `json:"min"`
	Max      *float64        `json:"max"`
	Allowed  map[string]bool `json:"allowed"`
}

func (f *manifestFieldJSON) UnmarshalJSON(data []byte) error {
	type alias manifestFieldJSON
	var decoded struct {
		*alias
		Values  []string `json:"values"`
		Default any      `json:"default"`
	}
	decoded.alias = (*alias)(f)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if f.Allowed == nil && len(decoded.Values) > 0 {
		f.Allowed = make(map[string]bool, len(decoded.Values))
		for _, value := range decoded.Values {
			f.Allowed[value] = true
		}
	}
	if f.Initial == nil && decoded.Default != nil {
		f.Initial = decoded.Default
	}
	return nil
}

type manifestEventJSON struct {
	EventKey   string `json:"event_key"`
	EventType  string `json:"event_type"`
	Summary    string `json:"summary"`
	Importance string `json:"importance"`
}

type manifestObservationRule struct {
	ObservationKey string              `json:"observation_key"`
	Value          any                 `json:"value"`
	Effects        []StateEffect       `json:"effects"`
	Events         []manifestEventJSON `json:"events"`
}

func (r *manifestObservationRule) UnmarshalJSON(data []byte) error {
	var raw struct {
		ObservationKey string          `json:"observation_key"`
		Value          any             `json:"value"`
		Effects        json.RawMessage `json:"effects"`
		Events         json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.ObservationKey, r.Value = raw.ObservationKey, raw.Value
	if len(raw.Effects) > 0 && string(raw.Effects) != "null" {
		if err := json.Unmarshal(raw.Effects, &r.Effects); err != nil {
			var values map[string]any
			if err := json.Unmarshal(raw.Effects, &values); err != nil {
				return err
			}
			for field, value := range values {
				r.Effects = append(r.Effects, StateEffect{Field: field, Operation: "set", Value: value})
			}
		}
	}
	if len(raw.Events) > 0 && string(raw.Events) != "null" {
		if err := json.Unmarshal(raw.Events, &r.Events); err != nil {
			r.Events = nil
			var names []string
			if err := json.Unmarshal(raw.Events, &names); err != nil {
				return err
			}
			for _, name := range names {
				r.Events = append(r.Events, manifestEventJSON{EventKey: name})
			}
		}
	}
	return nil
}

type manifestVersionJSON int

func (v *manifestVersionJSON) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("manifest_version is empty")
	}
	if data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || f != float64(int(f)) {
			return fmt.Errorf("invalid manifest_version %q", text)
		}
		*v = manifestVersionJSON(int(f))
		return nil
	}
	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*v = manifestVersionJSON(number)
	return nil
}

func (v manifestVersionJSON) MarshalJSON() ([]byte, error) { return json.Marshal(int(v)) }

type manifestRuntimeDocument struct {
	ManifestVersion  manifestVersionJSON          `json:"manifest_version"`
	Fields           map[string]manifestFieldJSON `json:"fields"`
	ObservationRules []manifestObservationRule    `json:"observation_rules"`
}

// ManifestStoryTurnProcessor 将已校验的候选事实按 Manifest 规则转换为状态和事件。
type ManifestStoryTurnProcessor struct {
	storyStore        *store.SchedulerStore
	scheduler         *SchedulerService
	modelName         string
	modelNameProvider func() string
	promptVersion     string
	messageStore      *store.MessageStore
	promptBuilder     *SchedulerPromptBuilder
}

func NewManifestStoryTurnProcessor(storyStore *store.SchedulerStore, scheduler *SchedulerService, modelName, promptVersion string) *ManifestStoryTurnProcessor {
	return &ManifestStoryTurnProcessor{storyStore: storyStore, scheduler: scheduler, modelName: modelName, promptVersion: promptVersion}
}

func NewManifestStoryTurnProcessorWithMessages(storyStore *store.SchedulerStore, messageStore *store.MessageStore, scheduler *SchedulerService, modelName, promptVersion string, promptBuilder *SchedulerPromptBuilder) *ManifestStoryTurnProcessor {
	processor := NewManifestStoryTurnProcessor(storyStore, scheduler, modelName, promptVersion)
	processor.messageStore = messageStore
	processor.promptBuilder = promptBuilder
	return processor
}

func (p *ManifestStoryTurnProcessor) SetModelNameProvider(provider func() string) {
	if p != nil {
		p.modelNameProvider = provider
	}
}

func (p *ManifestStoryTurnProcessor) ProcessStoryTurn(
	ctx context.Context,
	record *model.ChatSchedulerRecord,
	state *model.ChatStoryState,
	messages []model.ChatCompletionMessage,
	spec SchedulerValidationSpec,
) (StoryProcessResult, error) {
	if p == nil || p.storyStore == nil || p.scheduler == nil {
		return StoryProcessResult{}, fmt.Errorf("manifest story processor is not configured")
	}
	if record == nil || state == nil {
		return StoryProcessResult{}, fmt.Errorf("record and state are required")
	}
	manifest, err := p.storyStore.GetManifest(state.ManifestID)
	if err != nil {
		return StoryProcessResult{}, err
	}
	if manifest.Status != model.ManifestStatusReady {
		return StoryProcessResult{}, fmt.Errorf("story manifest is not ready: %s", manifest.Status)
	}
	var document manifestRuntimeDocument
	if err := json.Unmarshal([]byte(manifest.CompiledJSON), &document); err != nil {
		return p.fail(record.ID, "manifest_error", err)
	}
	fieldSpecs := make(map[string]FieldSpec, len(document.Fields))
	for key, field := range document.Fields {
		fieldSpecs[key] = FieldSpec{Type: field.Type, Writable: field.Writable, Allowed: field.Allowed, HasMin: field.Min != nil, HasMax: field.Max != nil}
		if field.Min != nil {
			fieldSpecs[key] = withMin(fieldSpecs[key], *field.Min)
		}
		if field.Max != nil {
			fieldSpecs[key] = withMax(fieldSpecs[key], *field.Max)
		}
	}

	schedulerMessages := messages
	if p.messageStore != nil && p.promptBuilder != nil {
		userMessage, userErr := p.messageStore.GetByID(record.UserMessageID)
		assistantMessage, assistantErr := p.messageStore.GetByID(record.AssistantMessageID)
		if userErr != nil || assistantErr != nil {
			if userErr != nil {
				return p.fail(record.ID, "message_error", userErr)
			}
			return p.fail(record.ID, "message_error", assistantErr)
		}
		schedulerMessages, err = p.promptBuilder.Build(userMessage.Content, assistantMessage.Content, state, manifest.CompiledJSON, spec)
		if err != nil {
			return p.fail(record.ID, "scheduler_prompt_error", err)
		}
	}
	modelName := p.modelName
	if p.modelNameProvider != nil {
		if current := p.modelNameProvider(); current != "" {
			modelName = current
		}
	}
	output, err := p.scheduler.Process(ctx, record, modelName, p.promptVersion, schedulerMessages, spec)
	if err != nil {
		return StoryProcessResult{Status: string(model.SchedulerStatusFailed), RecordID: record.ID, ErrorMessage: err.Error()}, err
	}
	var stateValues map[string]any
	if err := json.Unmarshal([]byte(state.StateJSON), &stateValues); err != nil {
		return p.fail(record.ID, "state_error", err)
	}
	if stateValues == nil {
		stateValues = map[string]any{}
	}

	var effects []StateEffect
	var events []model.ChatStoryEvent
	for _, observation := range output.Observations {
		for _, rule := range document.ObservationRules {
			if rule.ObservationKey != observation.Key || !valuesEqual(rule.Value, observation.Value) {
				continue
			}
			effects = append(effects, rule.Effects...)
			for _, event := range rule.Events {
				evidence := observation.Evidence
				events = append(events, model.ChatStoryEvent{ChatID: state.ChatID, SchedulerRecordID: record.ID, EventKey: event.EventKey, EventType: event.EventType, Summary: event.Summary, Importance: event.Importance, Evidence: evidence})
			}
		}
	}
	if err := ApplyStateEffects(stateValues, effects, fieldSpecs); err != nil {
		return p.fail(record.ID, "effect_error", err)
	}
	newState, err := json.Marshal(stateValues)
	if err != nil {
		return p.fail(record.ID, "state_encode_error", err)
	}
	state.StateJSON = string(newState)
	changes, err := json.Marshal(effects)
	if err != nil {
		return p.fail(record.ID, "effect_encode_error", err)
	}
	contextText := buildStoryContextText(state)
	if err := p.storyStore.CommitSchedulerTurn(record.ID, state, state.StateVersion, string(json.RawMessage(mustMarshal(output))), string(changes), contextText, events); err != nil {
		if errors.Is(err, store.ErrStoryStateConflict) {
			_ = p.storyStore.MarkStoryTurnConflict(record.ChatID, record.ID, err.Error())
			return StoryProcessResult{Status: string(model.SchedulerStatusConflict), RecordID: record.ID, ErrorMessage: err.Error()}, nil
		}
		return p.fail(record.ID, "commit_error", err)
	}
	return StoryProcessResult{Status: string(model.SchedulerStatusSuccess), RecordID: record.ID, ContextText: contextText}, nil
}

func (p *ManifestStoryTurnProcessor) fail(recordID, code string, err error) (StoryProcessResult, error) {
	_ = p.storyStore.MarkFailed(recordID, code, err.Error())
	return StoryProcessResult{Status: string(model.SchedulerStatusFailed), RecordID: recordID, ErrorMessage: err.Error()}, err
}

func withMin(spec FieldSpec, value float64) FieldSpec { spec.Min = value; return spec }
func withMax(spec FieldSpec, value float64) FieldSpec { spec.Max = value; return spec }

func buildStoryContextText(state *model.ChatStoryState) string {
	text := ""
	if state.CurrentScene != "" {
		text += "当前场景：" + state.CurrentScene + "\n"
	}
	if state.ActiveEvent != "" {
		text += "当前事件：" + state.ActiveEvent + "\n"
	}
	if state.Route != "" {
		text += "当前路线：" + state.Route
	}
	return text
}

func mustMarshal(value any) []byte { data, _ := json.Marshal(value); return data }
