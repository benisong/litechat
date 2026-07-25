package service

import (
	"errors"
	"litechat/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newResilienceChatService(db *store.DB, summaryService *SummaryService) *ChatService {
	return NewChatService(
		store.NewChatStore(db),
		summaryService.messageStore,
		store.NewCharacterStore(db),
		store.NewPresetStore(db),
		store.NewWorldBookStore(db),
		summaryService.configStore,
		summaryService.userStore,
		nil,
	)
}

func TestSendMessagePersistsReplyAfterClientDisconnect(t *testing.T) {
	summaryService, db, chatID := newSummaryServiceTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeChatStream(w, "她轻轻点头。")
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, summaryService, server.URL)

	chatService := newResilienceChatService(db, summaryService)
	response, err := chatService.SendMessage(
		chatID,
		"你还在吗？",
		"",
		"summary-user",
		func(string) error { return errors.New("client disconnected") },
	)
	if err != nil {
		t.Fatalf("send message after client disconnect: %v", err)
	}
	if response != "她轻轻点头。" {
		t.Fatalf("unexpected response: %q", response)
	}

	messages := listSummaryTestMessages(t, summaryService.messageStore, chatID)
	if len(messages) != 2 {
		t.Fatalf("expected the complete exchange to be persisted, got %+v", messages)
	}
	if messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("unexpected persisted roles: %+v", messages)
	}
}

func TestConcurrentSendIsRejectedBeforeCreatingDuplicateMessage(t *testing.T) {
	summaryService, db, chatID := newSummaryServiceTest(t)
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	defer func() {
		select {
		case <-releaseRequest:
		default:
			close(releaseRequest)
		}
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		writeChatStream(w, "第一条回复。")
	}))
	t.Cleanup(server.Close)
	setSummaryTestEndpoint(t, summaryService, server.URL)

	chatService := newResilienceChatService(db, summaryService)
	firstDone := make(chan error, 1)
	go func() {
		_, err := chatService.SendMessage(
			chatID,
			"第一条消息",
			"",
			"summary-user",
			func(string) error { return nil },
		)
		firstDone <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first request did not reach the model server")
	}

	_, err := chatService.SendMessage(
		chatID,
		"不应被保存的重复消息",
		"",
		"summary-user",
		func(string) error { return nil },
	)
	if !errors.Is(err, ErrChatBusy) {
		t.Fatalf("expected ErrChatBusy, got %v", err)
	}

	messages := listSummaryTestMessages(t, summaryService.messageStore, chatID)
	if len(messages) != 1 || messages[0].Content != "第一条消息" {
		t.Fatalf("concurrent request created a duplicate message: %+v", messages)
	}

	close(releaseRequest)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first request failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first request did not finish")
	}

	messages = listSummaryTestMessages(t, summaryService.messageStore, chatID)
	if len(messages) != 2 || messages[1].Role != "assistant" {
		t.Fatalf("first exchange was not completed normally: %+v", messages)
	}
}
