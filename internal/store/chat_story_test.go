package store

import (
	"litechat/internal/model"
	"testing"
)

func TestChatSchedulerEnabledRoundTrips(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewChatStore(db)
	characterStore := NewCharacterStore(db)
	character := &model.Character{Name: "剧情角色"}
	if err := characterStore.Create(character, "user-1"); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	chat := &model.Chat{
		CharacterID:      character.ID,
		Title:            "剧情测试",
		SchedulerEnabled: true,
	}
	if err := store.Create(chat, "user-1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := store.GetByID(chat.ID, "user-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !got.SchedulerEnabled {
		t.Fatalf("expected scheduler enabled, got %+v", got)
	}
}
