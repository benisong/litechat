package api

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/service"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStoryMessageRuntime struct{}

func (fakeStoryMessageRuntime) SendMessageWithEvents(_ context.Context, _ service.ChatTurnInput, callback service.StreamCallback, statusCallback func(service.StoryRuntimeStatusEvent) error) (service.ChatRuntimeResult, error) {
	if err := callback("你好"); err != nil {
		return service.ChatRuntimeResult{}, err
	}
	if err := statusCallback(service.StoryRuntimeStatusEvent{Status: "processing", RecordID: "record-1"}); err != nil {
		return service.ChatRuntimeResult{}, err
	}
	if err := statusCallback(service.StoryRuntimeStatusEvent{Status: "success", RecordID: "record-1"}); err != nil {
		return service.ChatRuntimeResult{}, err
	}
	return service.ChatRuntimeResult{SchedulerStatus: "success", SchedulerRecordID: "record-1"}, nil
}

func TestSendStoryMessageWritesTokenAndSchedulerSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandlers(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h.SetStoryMessageRuntime(fakeStoryMessageRuntime{})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/story/chats/chat-1/messages", strings.NewReader(`{"content":"开始"}`))
	ctx.Set("user_id", "user-1")
	ctx.Params = gin.Params{{Key: "id", Value: "chat-1"}}
	h.SendStoryMessage(ctx)
	body := recorder.Body.String()
	for _, expected := range []string{`"token":"你好"`, `"scheduler_status":"processing"`, `"scheduler_status":"success"`, `"done":true`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("SSE missing %s: %s", expected, body)
		}
	}
	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
	}
	_ = model.SendMessageRequest{}
}

func TestSendStoryMessageReturns503WithoutRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandlers(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/story/chats/chat-1/messages", strings.NewReader(`{"content":"开始"}`))
	h.SendStoryMessage(ctx)
	if recorder.Code != 503 {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}
