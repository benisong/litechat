package service

import (
	"context"
	"errors"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

type sequenceManifestCompilerClient struct {
	responses []string
	calls     int
}

func (c *sequenceManifestCompilerClient) Complete(context.Context, string, []model.ChatCompletionMessage) (string, error) {
	response := c.responses[c.calls]
	c.calls++
	if response == "error" {
		return "", errors.New("compiler unavailable")
	}
	return response, nil
}

func TestManifestCompilerRetriesFailedManifestWithNewCompilation(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	client := &sequenceManifestCompilerClient{responses: []string{
		`{"manifest_version":1,"fields":{"bad":{"type":"invalid","writable":true}},"observation_rules":[]}`,
		`{"manifest_version":1,"fields":{"trust":{"type":"integer","writable":true}},"observation_rules":[]}`,
	}}
	storyStore := store.NewSchedulerStore(db)
	compiler := NewManifestCompiler(storyStore, client)
	input := ManifestCompileInput{CharacterID: "char-1", CharacterVersion: "v1", WorldbookVersionHash: "hash-1", CompilerModel: "smart-model", PromptVersion: "compiler-v1", CompileOnlyText: "剧情"}
	failed, err := compiler.Compile(context.Background(), input)
	if err == nil || failed.Status != model.ManifestStatusFailed {
		t.Fatalf("expected failed compilation: manifest=%+v err=%v", failed, err)
	}
	ready, err := compiler.Retry(context.Background(), failed.ID, input)
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}
	if ready.Status != model.ManifestStatusReady || ready.ID == failed.ID || client.calls != 2 {
		t.Fatalf("unexpected retry result: ready=%+v failed=%+v calls=%d", ready, failed, client.calls)
	}
}
