package service

import (
	"context"
	"testing"
)

type testChatRuntime struct{ name string }

func (r testChatRuntime) SendMessage(_ context.Context, _ ChatTurnInput, _ StreamCallback) (ChatRuntimeResult, error) {
	return ChatRuntimeResult{AssistantContent: r.name}, nil
}
func (r testChatRuntime) Regenerate(_ context.Context, _ ChatRegenerateInput, _ StreamCallback) (ChatRuntimeResult, error) {
	return ChatRuntimeResult{AssistantContent: r.name}, nil
}
func (r testChatRuntime) Retry(_ context.Context, _ ChatTurnInput, _ StreamCallback) (ChatRuntimeResult, error) {
	return ChatRuntimeResult{AssistantContent: r.name}, nil
}

func TestRuntimeFactorySelectsStoryOrLegacy(t *testing.T) {
	legacy := testChatRuntime{name: "legacy"}
	story := testChatRuntime{name: "story"}
	factory := RuntimeFactory{Legacy: legacy, Story: story}

	got, err := factory.Resolve(false)
	if err != nil {
		t.Fatalf("Resolve legacy: %v", err)
	}
	result, err := got.SendMessage(context.Background(), ChatTurnInput{}, nil)
	if err != nil || result.AssistantContent != "legacy" {
		t.Fatalf("unexpected legacy runtime: %+v, %v", result, err)
	}

	got, err = factory.Resolve(true)
	if err != nil {
		t.Fatalf("Resolve story: %v", err)
	}
	result, err = got.SendMessage(context.Background(), ChatTurnInput{}, nil)
	if err != nil || result.AssistantContent != "story" {
		t.Fatalf("unexpected story runtime: %+v, %v", result, err)
	}
}

func TestRuntimeFactoryRejectsMissingImplementation(t *testing.T) {
	factory := RuntimeFactory{}
	if _, err := factory.Resolve(false); err == nil {
		t.Fatal("expected missing legacy runtime to fail")
	}
	if _, err := (RuntimeFactory{Legacy: testChatRuntime{name: "legacy"}}).Resolve(true); err == nil {
		t.Fatal("expected missing story runtime to fail")
	}
}
