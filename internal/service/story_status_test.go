package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

func TestStoryChatInitializerReturnsStoryStatusForOwnedChat(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	user := &model.User{Username: "status-user", Role: "user", Mode: "self"}
	if err := store.NewUserStore(db).Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "状态角色"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	storyStore := store.NewSchedulerStore(db)
	compiler := NewManifestCompiler(storyStore, fakeSchedulerCompletionClient{response: `{"manifest_version":1,"fields":{"trust":{"type":"integer","writable":true}},"observation_rules":[]}`})
	initializer := NewStoryChatInitializer(store.NewChatStore(db), storyStore, store.NewCharacterStore(db), compiler)
	result, err := initializer.Initialize(context.Background(), StoryChatInitializeInput{UserID: user.ID, CharacterID: character.ID, CharacterVersion: "v1", WorldbookVersionHash: "hash", CompileOnlyText: "剧情"})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	status, err := initializer.GetStatus(context.Background(), user.ID, result.Chat.ID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status.Chat.ID != result.Chat.ID || status.State.ManifestID != result.Manifest.ID || status.Manifest.Status != model.ManifestStatusReady || status.LatestSuccess != nil {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err = initializer.GetStatus(context.Background(), "other-user", result.Chat.ID); err == nil {
		t.Fatal("expected ownership check to fail")
	}
}
