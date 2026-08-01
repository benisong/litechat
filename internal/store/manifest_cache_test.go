package store

import (
	"database/sql"
	"testing"
)

func TestGetReadyManifestStrictCompilerCriteria(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO story_manifests (id, character_id, character_version, worldbook_version_hash, manifest_version, status, compiled_json, compiler_model, prompt_version)
		VALUES
		('manifest-model-a', 'character-1', 'v1', 'hash-1', 1, 'ready', '{}', 'model-a', 'prompt-1'),
		('manifest-model-b', 'character-1', 'v1', 'hash-1', 1, 'ready', '{}', 'model-b', 'prompt-2');`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	store := NewSchedulerStore(db)
	manifest, err := store.GetReadyManifest("character-1", "v1", "hash-1", "model-b", "prompt-2")
	if err != nil {
		t.Fatalf("strict lookup: %v", err)
	}
	if manifest.ID != "manifest-model-b" {
		t.Fatalf("got %s, want manifest-model-b", manifest.ID)
	}
	if _, err := store.GetReadyManifest("character-1", "v1", "hash-1", "model-b", "prompt-2", "999"); err == nil {
		t.Fatal("unexpected cache hit for incompatible manifest schema")
	}
	if _, err := store.GetReadyManifest("character-1", "v1", "hash-1", "model-c", "prompt-2"); err == nil || err == sql.ErrNoRows {
		// sql.ErrNoRows is the expected miss; any other nil result is a cache bug.
		if err == nil {
			t.Fatal("unexpected cache hit for unknown compiler criteria")
		}
	}
}
