package service

import (
	"context"
	"litechat/internal/store"
	"testing"
)

func TestJSONCharacterCardImportServiceImportAnyAcceptsLegacyCard(t *testing.T) {
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	service := NewJSONCharacterCardImportService(store.NewCharacterStore(db), store.NewCharacterCardDocumentStore(db))
	raw := []byte("```json\n" + `{"character_card":{"name":"旧导入卡","description":"身份","personality":"性格","scenario":"场景","first_msg":"开场","tags":["旧"]}}` + "\n```")
	result, err := service.ImportAny(context.Background(), "user-1", raw)
	if err != nil {
		t.Fatalf("ImportAny returned error: %v", err)
	}
	if result.Character.Name != "旧导入卡" || result.Plan.CardVersion != "legacy" {
		t.Fatalf("unexpected legacy import result: %+v", result)
	}
}
