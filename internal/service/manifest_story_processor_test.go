package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

func TestManifestStoryTurnProcessorAppliesManifestRuleAtomically(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()

	storyStore := store.NewSchedulerStore(db)
	manifest := &model.StoryManifest{CharacterID: "character-1", CompiledJSON: `{
		"fields": {
			"facts.resource_request": {"type":"boolean","writable":true}
		},
		"observation_rules": [{
			"observation_key":"facts.resource_request",
			"value":true,
			"effects":[{"field":"facts.resource_request","operation":"set","value":true}],
			"events":[{"event_key":"resource_dispute_001","event_type":"major_event","summary":"发生资源争议","importance":"major"}]
		}]
	}`}
	if err := storyStore.CreateManifest(manifest); err != nil {
		t.Fatalf("CreateManifest: %v", err)
	}
	if err := storyStore.MarkManifestReady(manifest.ID, manifest.CompiledJSON, "prompt-v1", "compiler"); err != nil {
		t.Fatalf("MarkManifestReady: %v", err)
	}
	state := &model.ChatStoryState{ChatID: "chat-1", ManifestID: manifest.ID, StateJSON: `{}`}
	if err := storyStore.CreateStoryState(state); err != nil {
		t.Fatalf("CreateStoryState: %v", err)
	}
	record := &model.ChatSchedulerRecord{ChatID: "chat-1", UserMessageID: "user-1", AssistantMessageID: "assistant-1", TurnSeq: 2}
	if err := storyStore.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}

	completion := fakeSchedulerCompletionClient{response: `{"schema_version":1,"observations":[{"key":"facts.resource_request","value":true,"evidence":"柳如烟提出资源请求","confidence":0.98}]}`}
	scheduler := NewSchedulerService(storyStore, completion)
	processor := NewManifestStoryTurnProcessor(storyStore, scheduler, "cheap-model", "prompt-v1")
	result, err := processor.ProcessStoryTurn(context.Background(), record, state, nil, SchedulerValidationSpec{
		AllowedObservationKeys: map[string]bool{"facts.resource_request": true},
		AllowedEventIDs:        map[string]bool{"resource_dispute_001": true},
	})
	if err != nil {
		t.Fatalf("ProcessStoryTurn: %v", err)
	}
	if result.Status != string(model.SchedulerStatusSuccess) {
		t.Fatalf("unexpected result: %+v", result)
	}
	gotState, err := storyStore.GetStoryState("chat-1")
	if err != nil {
		t.Fatalf("GetStoryState: %v", err)
	}
	if gotState.StateVersion != 1 || gotState.StateJSON != `{"facts.resource_request":true}` {
		t.Fatalf("unexpected state: %+v", gotState)
	}
	var eventCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM chat_story_events WHERE event_key = ?`, "resource_dispute_001").Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one event, got %d", eventCount)
	}
}
