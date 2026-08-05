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

	chars := store.NewCharacterStore(db)
	worldBooks := store.NewWorldBookStore(db)
	h := NewHandlers(chars, nil, nil, nil, nil, nil, nil, nil, nil)
	h.SetJSONCharacterCardImporter(service.NewJSONCharacterCardImportService(chars, store.NewCharacterCardDocumentStore(db), worldBooks))
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
	worldbookList, err := worldBooks.List("user-1")
	if err != nil || len(worldbookList) != 1 {
		t.Fatalf("public worldbook was not created: books=%+v err=%v", worldbookList, err)
	}
	linkedWorldBook, err := worldBooks.GetByID(worldbookList[0].ID, "user-1")
	if err != nil || len(linkedWorldBook.Entries) != 1 || linkedWorldBook.Entries[0].Content != "公开内容" {
		t.Fatalf("public worldbook entries were not linked: book=%+v err=%v", linkedWorldBook, err)
	}

	readRecorder := httptest.NewRecorder()
	readCtx, _ := gin.CreateTestContext(readRecorder)
	readCtx.Request = httptest.NewRequest("GET", "/api/characters/"+response.Character.ID+"/card-document", nil)
	readCtx.Set("user_id", "user-1")
	readCtx.Params = gin.Params{{Key: "id", Value: response.Character.ID}}
	h.GetJSONCharacterCardDocument(readCtx)
	if readRecorder.Code != 200 || !strings.Contains(readRecorder.Body.String(), `"character_id"`) || !strings.Contains(readRecorder.Body.String(), "公开内容") || strings.Contains(readRecorder.Body.String(), "隐藏调度内容") {
		t.Fatalf("public document response is invalid: status=%d body=%s", readRecorder.Code, readRecorder.Body.String())
	}
}
