package api

import (
	"bytes"
	"encoding/json"
	"litechat/internal/service"
	"litechat/internal/store"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestImportJSONCharacterCardHidesSchedulerEntriesFromResponse(t *testing.T) {
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
	raw := []byte(`{"card_version":"1.0","character":{"name":"重生之玄幻之旅","pov":"second","description":"公开身份","personality":"性格","scenario":"场景","first_message":"开场"},"worldbook":{"id":"w","version":"1.0","global_enabled":true,"main_entries":[{"id":"public","title":"公开","content":"公开内容","user_visible":true,"scheduler_enabled":false},{"id":"hidden","title":"隐藏","content":"隐藏调度内容","user_visible":false,"scheduler_enabled":true}]}}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/characters/import/json", bytes.NewReader(raw))
	ctx.Set("user_id", "user-1")
	h.ImportJSONCharacterCard(ctx)
	if recorder.Code != 201 {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "公开内容") || strings.Contains(body, "隐藏调度内容") {
		t.Fatalf("response leaked or omitted worldbook content: %s", body)
	}
	var response struct {
		Character struct {
			ID string `json:"id"`
		} `json:"character"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Character.ID == "" {
		t.Fatal("response did not include created character")
	}
}
