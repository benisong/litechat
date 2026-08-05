package api

import (
	"bytes"
	"litechat/internal/service"
	"litechat/internal/store"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestImportCharacterCardAcceptsLegacyFormatThroughUnifiedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("new db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.InitSchema(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	h := NewHandlers(store.NewCharacterStore(db), nil, nil, nil, nil, nil, nil, nil, nil)
	h.SetJSONCharacterCardImporter(service.NewJSONCharacterCardImportService(store.NewCharacterStore(db), store.NewCharacterCardDocumentStore(db)))
	raw := []byte("```json\n" + `{"character_card":{"name":"统一旧卡","description":"身份","personality":"性格","scenario":"场景","first_msg":"开场","tags":["旧"]}}` + "\n```")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/characters/import", bytes.NewReader(raw))
	ctx.Set("user_id", "user-1")
	h.ImportCharacterCard(ctx)
	if recorder.Code != 201 || !strings.Contains(recorder.Body.String(), "统一旧卡") {
		t.Fatalf("unexpected unified import response: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
