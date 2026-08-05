package service

import (
	"context"
	"litechat/internal/store"
	"testing"
)

func TestJSONCharacterCardImportServiceCreatesCharacterAndDocument(t *testing.T) {
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	service := NewJSONCharacterCardImportService(store.NewCharacterStore(db), store.NewCharacterCardDocumentStore(db))
	raw := []byte(`{"card_version":"1.0","character":{"name":"重生之玄幻之旅","pov":"second","description":"身份","personality":"性格","scenario":"场景","first_message":"开场"},"worldbook":{"id":"w","version":"1.0","global_enabled":true,"main_entries":[{"id":"hidden","content":"调度规则","user_visible":false,"scheduler_enabled":true}]},"tags":["复杂剧情"]}`)

	result, err := service.Import(context.Background(), "user-1", raw)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Character.ID == "" || result.Character.Name != "重生之玄幻之旅" || result.Character.POV != "second" {
		t.Fatalf("unexpected character result: %+v", result.Character)
	}
	if result.Document.CharacterID != result.Character.ID || result.Document.SourceJSON != string(raw) {
		t.Fatalf("document is not bound to imported character: %+v", result.Document)
	}
	if len(result.Plan.SchedulerWorldBook.MainEntries) != 1 {
		t.Fatalf("scheduler worldbook was not retained in plan: %+v", result.Plan.SchedulerWorldBook)
	}
}
