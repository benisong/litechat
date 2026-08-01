package store

import (
	"litechat/internal/model"
	"testing"
)

func TestManifestLifecycle(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewSchedulerStore(db)
	manifest := &model.StoryManifest{
		CharacterID:          "character-1",
		CharacterVersion:     "v1",
		WorldbookVersionHash: "hash-1",
		CompilerModel:        "compiler-model",
		PromptVersion:        "prompt-v1",
	}
	if err := store.CreateManifest(manifest); err != nil {
		t.Fatalf("CreateManifest: %v", err)
	}
	if manifest.Status != model.ManifestStatusPending || manifest.ID == "" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if err := store.MarkManifestReady(manifest.ID, `{"manifest_version":1}`, "prompt-v1", "compiler-model"); err != nil {
		t.Fatalf("MarkManifestReady: %v", err)
	}
	got, err := store.GetManifest(manifest.ID)
	if err != nil {
		t.Fatalf("GetManifest: %v", err)
	}
	if got.Status != model.ManifestStatusReady || got.CompiledJSON == "" {
		t.Fatalf("unexpected ready manifest: %+v", got)
	}
}

func TestStoryStateOptimisticVersionUpdate(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewSchedulerStore(db)
	state := &model.ChatStoryState{
		ChatID:     "chat-1",
		ManifestID: "manifest-1",
		StateJSON:  `{"route":"survival"}`,
		Route:      "survival",
	}
	if err := store.CreateStoryState(state); err != nil {
		t.Fatalf("CreateStoryState: %v", err)
	}
	if state.StateVersion != 0 {
		t.Fatalf("expected initial version 0, got %d", state.StateVersion)
	}
	if err := store.UpdateStoryState(state, 0); err != nil {
		t.Fatalf("UpdateStoryState: %v", err)
	}
	if state.StateVersion != 1 {
		t.Fatalf("expected version 1, got %d", state.StateVersion)
	}
	if err := store.UpdateStoryState(state, 0); err == nil {
		t.Fatal("expected stale version update to fail")
	}
}
