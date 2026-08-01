package service

import (
	"litechat/internal/model"
	"strings"
	"testing"
)

func TestSchedulerPromptBuilderBuildsIndependentInput(t *testing.T) {
	builder := NewSchedulerPromptBuilder()
	manifest := `{"manifest_version":1,"fields":{"trust":{"type":"integer","writable":true}},"observation_rules":[{"observation_key":"seen","value":true,"effects":[]},{"observation_key":"hidden_future","value":true,"effects":[]}]}`
	messages, err := builder.Build("用户要求交出资源", "角色拒绝了要求", &model.ChatStoryState{StateJSON: `{"trust":10}`, CurrentScene: "山门", ActiveEvent: "opening", Route: "survival"}, manifest, SchedulerValidationSpec{AllowedObservationKeys: map[string]bool{"seen": true}, AllowedEventIDs: map[string]bool{}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(messages) != 2 || messages[0].Role != "system" || messages[1].Role != "user" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if !strings.Contains(messages[1].Content, "用户要求交出资源") || !strings.Contains(messages[1].Content, "角色拒绝了要求") {
		t.Fatalf("turn content missing: %+v", messages)
	}
	if !strings.Contains(messages[0].Content, "seen") || strings.Contains(messages[0].Content, "hidden_future") {
		t.Fatalf("manifest filtering failed: %s", messages[0].Content)
	}
	if strings.Contains(messages[0].Content, "主模型 system") {
		t.Fatalf("primary prompt leaked: %s", messages[0].Content)
	}
	if !strings.Contains(messages[0].Content, `"trust":`) {
		t.Fatalf("state missing: %s", messages[0].Content)
	}
}
