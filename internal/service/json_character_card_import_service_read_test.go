package service

import (
	"context"
	"litechat/internal/store"
	"testing"
)

func TestJSONCharacterCardImportServiceLoadsOnlyPublicWorldbook(t *testing.T) {
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	characters := store.NewCharacterStore(db)
	documents := store.NewCharacterCardDocumentStore(db)
	importer := NewJSONCharacterCardImportService(characters, documents)
	raw := []byte(`{"card_version":"1.0","character":{"name":"x","pov":"second"},"worldbook":{"id":"w","version":"1.0","global_enabled":true,"main_entries":[{"id":"public","content":"公开","user_visible":true,"scheduler_enabled":false},{"id":"hidden","content":"隐藏","user_visible":false,"scheduler_enabled":true}]}}`)
	created, err := importer.Import(context.Background(), "user-1", raw)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	view, err := importer.GetPublic(context.Background(), "user-1", created.Character.ID)
	if err != nil {
		t.Fatalf("get public card: %v", err)
	}
	if len(view.WorldBook.MainEntries) != 1 || view.WorldBook.MainEntries[0].Content != "公开" {
		t.Fatalf("unexpected public worldbook: %+v", view.WorldBook)
	}
}
