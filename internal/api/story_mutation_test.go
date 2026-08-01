package api

import (
	"litechat/internal/model"
	"litechat/internal/service"
	"litechat/internal/store"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newStoryMutationTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

func TestDeleteChatAllowsSchedulerEnabledChat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newStoryMutationTestDB(t)
	defer db.Close()
	user := &model.User{Username: "story-owner", PasswordHash: "hash", Role: "user", Mode: "self"}
	if err := store.NewUserStore(db).Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "Story Character"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	chat := &model.Chat{CharacterID: character.ID, SchedulerEnabled: true}
	chatStore := store.NewChatStore(db)
	if err := chatStore.Create(chat, user.ID); err != nil {
		t.Fatalf("Create chat: %v", err)
	}
	storyStore := store.NewSchedulerStore(db)
	initializer := service.NewStoryChatInitializer(chatStore, storyStore, store.NewCharacterStore(db), nil)
	h := NewHandlers(nil, chatStore, store.NewMessageStore(db), nil, nil, nil, nil, nil, nil, initializer)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: chat.ID}}
	ctx.Set("user_id", user.ID)
	h.DeleteChat(ctx)
	if recorder.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, err := chatStore.GetByID(chat.ID, user.ID); err == nil {
		t.Fatal("story chat was not deleted")
	}
}

func TestDeleteChatKeepsLegacyBehavior(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newStoryMutationTestDB(t)
	defer db.Close()
	user := &model.User{Username: "legacy-owner", PasswordHash: "hash", Role: "user", Mode: "self"}
	if err := store.NewUserStore(db).Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "Legacy Character"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	chat := &model.Chat{CharacterID: character.ID, SchedulerEnabled: false}
	chatStore := store.NewChatStore(db)
	if err := chatStore.Create(chat, user.ID); err != nil {
		t.Fatalf("Create chat: %v", err)
	}
	h := NewHandlers(nil, chatStore, store.NewMessageStore(db), nil, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: chat.ID}}
	ctx.Set("user_id", user.ID)
	h.DeleteChat(ctx)
	if recorder.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if _, err := chatStore.GetByID(chat.ID, user.ID); err == nil {
		t.Fatal("legacy chat was not deleted")
	}
}
