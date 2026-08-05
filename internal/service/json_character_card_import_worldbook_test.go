package service

import (
	"context"
	"litechat/internal/store"
	"testing"
)

func TestJSONImportCreatesLinkedPublicWorldBook(t *testing.T) {
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.InitSchema(); err != nil {
		t.Fatal(err)
	}
	chars := store.NewCharacterStore(db)
	docs := store.NewCharacterCardDocumentStore(db)
	worldBooks := store.NewWorldBookStore(db)
	svc := NewJSONCharacterCardImportService(chars, docs, worldBooks)
	input := []byte(`{"card_version":"1.0","character":{"name":"联动测试卡","pov":"second","description":"d"},"worldbook":{"id":"linked-wb","name":"公开设定","version":"1.0","main_entries":[{"id":"public","title":"公开","content":"公开世界观","user_visible":true,"enabled":true,"scheduler_enabled":true},{"id":"hidden","title":"隐藏","content":"隐藏调度","user_visible":false,"enabled":true,"scheduler_enabled":true}]}}`)
	result, err := svc.Import(context.Background(), "u1", input)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := worldBooks.GetByID(result.Document.WorldBookID, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if wb.CharacterID != result.Character.ID || len(wb.Entries) != 1 || wb.Entries[0].Content != "公开世界观" {
		t.Fatalf("unexpected linked worldbook: %+v", wb)
	}
}
