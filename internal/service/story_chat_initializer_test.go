package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

func TestStoryChatInitializerCreatesEnabledChatAndInitialState(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	user := &model.User{Username: "init-user", Role: "user", Mode: "self"}
	if err := store.NewUserStore(db).Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "复杂角色"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	compiler := NewManifestCompiler(store.NewSchedulerStore(db), fakeSchedulerCompletionClient{response: `{"manifest_version":1,"fields":{"trust":{"type":"integer","writable":true}},"observation_rules":[]}`})
	initializer := NewStoryChatInitializer(store.NewChatStore(db), store.NewSchedulerStore(db), store.NewCharacterStore(db), compiler)
	result, err := initializer.Initialize(context.Background(), StoryChatInitializeInput{UserID: user.ID, CharacterID: character.ID, CharacterVersion: "v1", WorldbookVersionHash: "hash-1", Title: "复杂剧情"})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if result.Chat == nil || !result.Chat.SchedulerEnabled || result.Manifest == nil || result.State == nil {
		t.Fatalf("unexpected initialization result: %+v", result)
	}
	if result.State.ChatID != result.Chat.ID || result.State.ManifestID != result.Manifest.ID || result.State.StateJSON != `{"trust":0}` {
		t.Fatalf("unexpected initial state: %+v", result.State)
	}
}

func TestStoryChatInitializerReusesManifestForSecondChat(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	user := &model.User{Username: "init-user-2", Role: "user", Mode: "self"}
	if err := store.NewUserStore(db).Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "复杂角色"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	client := &countingManifestCompilerClient{response: `{"manifest_version":1,"fields":{"flag":{"type":"boolean","writable":true}},"observation_rules":[]}`}
	storyStore := store.NewSchedulerStore(db)
	initializer := NewStoryChatInitializer(store.NewChatStore(db), storyStore, store.NewCharacterStore(db), NewManifestCompiler(storyStore, client))
	input := StoryChatInitializeInput{UserID: user.ID, CharacterID: character.ID, CharacterVersion: "v1", WorldbookVersionHash: "hash-1", Title: "剧情"}
	first, err := initializer.Initialize(context.Background(), input)
	if err != nil {
		t.Fatalf("first Initialize: %v", err)
	}
	second, err := initializer.Initialize(context.Background(), input)
	if err != nil {
		t.Fatalf("second Initialize: %v", err)
	}
	if first.Manifest.ID != second.Manifest.ID || client.calls.Load() != 1 {
		t.Fatalf("manifest was not reused: first=%s second=%s calls=%d", first.Manifest.ID, second.Manifest.ID, client.calls.Load())
	}
}
