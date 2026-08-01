package store

import (
	"litechat/internal/model"
	"testing"
)

func TestGetReadyManifestByCharacterAndWorldbookVersion(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	store := NewSchedulerStore(db)

	ready := &model.StoryManifest{CharacterID: "char-1", CharacterVersion: "v2", WorldbookVersionHash: "hash-2", CompiledJSON: `{"manifest_version":1,"fields":{"x":{"type":"boolean","writable":true}},"observation_rules":[]}`}
	if err := store.CreateManifest(ready); err != nil {
		t.Fatalf("CreateManifest ready: %v", err)
	}
	if err := store.MarkManifestReady(ready.ID, ready.CompiledJSON, "prompt-v1", "smart-model"); err != nil {
		t.Fatalf("MarkManifestReady: %v", err)
	}

	got, err := store.GetReadyManifest("char-1", "v2", "hash-2")
	if err != nil {
		t.Fatalf("GetReadyManifest: %v", err)
	}
	if got.ID != ready.ID || got.Status != model.ManifestStatusReady {
		t.Fatalf("unexpected cached manifest: %+v", got)
	}
	if _, err := store.GetReadyManifest("char-1", "v1", "hash-2"); err == nil {
		t.Fatal("expected version mismatch to miss cache")
	}
}
