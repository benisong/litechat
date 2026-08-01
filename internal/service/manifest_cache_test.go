package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"sync/atomic"
	"testing"
)

type countingManifestCompilerClient struct {
	calls    atomic.Int32
	response string
}

func (c *countingManifestCompilerClient) Complete(context.Context, string, []model.ChatCompletionMessage) (string, error) {
	c.calls.Add(1)
	return c.response, nil
}

func TestManifestCompilerReusesReadyCacheWithoutCallingModel(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	client := &countingManifestCompilerClient{response: `{"manifest_version":1,"fields":{"x":{"type":"boolean","writable":true}},"observation_rules":[]}`}
	storyStore := store.NewSchedulerStore(db)
	compiler := NewManifestCompiler(storyStore, client)
	input := ManifestCompileInput{CharacterID: "char-1", CharacterVersion: "v1", WorldbookVersionHash: "hash-1", CompilerModel: "smart-model", PromptVersion: "compiler-v1", CompileOnlyText: "剧情"}
	first, err := compiler.CompileOrReuse(context.Background(), input)
	if err != nil {
		t.Fatalf("first CompileOrReuse: %v", err)
	}
	second, err := compiler.CompileOrReuse(context.Background(), input)
	if err != nil {
		t.Fatalf("second CompileOrReuse: %v", err)
	}
	if first.ID != second.ID || client.calls.Load() != 1 {
		t.Fatalf("cache was not reused: first=%s second=%s calls=%d", first.ID, second.ID, client.calls.Load())
	}
}
