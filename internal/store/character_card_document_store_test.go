package store

import (
	"litechat/internal/model"
	"testing"
)

func TestCharacterCardDocumentStorePersistsRawJSONAndWorldbookVersion(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	store := NewCharacterCardDocumentStore(db)
	characterStore := NewCharacterStore(db)
	character := &model.Character{Name: "重生之玄幻之旅", POV: "second"}
	if err := characterStore.Create(character, "user-1"); err != nil {
		t.Fatalf("create character: %v", err)
	}
	doc := &model.CharacterCardDocument{
		UserID: "user-1", CharacterID: character.ID, CardVersion: "1.0",
		WorldBookID: "worldbook-1", WorldBookVersion: "1.0",
		SourceJSON: `{"character":{"name":"重生之玄幻之旅"},"worldbook":{"global_enabled":true}}`,
	}
	if err := store.Create(doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if doc.ID == "" {
		t.Fatal("document ID was not generated")
	}

	loaded, err := store.GetByCharacterID(character.ID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if loaded.SourceJSON != doc.SourceJSON || loaded.WorldBookVersion != "1.0" {
		t.Fatalf("document was not persisted exactly: %+v", loaded)
	}
}
