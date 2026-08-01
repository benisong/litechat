package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

func TestManifestCompilerCreatesReadyManifestFromModelOutput(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	storyStore := store.NewSchedulerStore(db)
	compiler := NewManifestCompiler(storyStore, fakeSchedulerCompletionClient{response: `{"manifest_version":1,"fields":{"facts.resource_request":{"type":"boolean","writable":true}},"observation_rules":[]}`})
	manifest, err := compiler.Compile(context.Background(), ManifestCompileInput{
		CharacterID: "char-1", CharacterVersion: "v1", WorldbookVersionHash: "hash-1",
		CompilerModel: "smart-model", PromptVersion: "compiler-v1", CompileOnlyText: "完整剧情世界书内容",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if manifest.Status != model.ManifestStatusReady || manifest.CompiledJSON == "" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	stored, err := storyStore.GetManifest(manifest.ID)
	if err != nil {
		t.Fatalf("GetManifest: %v", err)
	}
	if stored.Status != model.ManifestStatusReady {
		t.Fatalf("stored manifest is not ready: %+v", stored)
	}
}

func TestManifestCompilerMarksInvalidManifestFailed(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	storyStore := store.NewSchedulerStore(db)
	compiler := NewManifestCompiler(storyStore, fakeSchedulerCompletionClient{response: `{"manifest_version":1,"fields":{"secret":{"type":"unknown","writable":true}},"observation_rules":[]}`})
	manifest, err := compiler.Compile(context.Background(), ManifestCompileInput{
		CharacterID: "char-1", CompilerModel: "smart-model", PromptVersion: "compiler-v1", CompileOnlyText: "剧情",
	})
	if err == nil {
		t.Fatal("expected invalid manifest to fail")
	}
	if manifest == nil || manifest.Status != model.ManifestStatusFailed {
		t.Fatalf("unexpected failed manifest: %+v", manifest)
	}
}
