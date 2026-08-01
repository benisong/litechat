package service

import (
	"context"
	"fmt"
	"litechat/internal/model"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIStoryPrimaryClientStreamsChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	client := NewOpenAIStoryPrimaryClient(&model.AppSettings{APIEndpoint: server.URL, APIKey: "[REDACTED]"})
	var tokens []string
	content, err := client.Stream(context.Background(), "story-model", []model.ChatCompletionMessage{{Role: "user", Content: "开始"}}, func(token string) error { tokens = append(tokens, token); return nil })
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if content != "你好" || strings.Join(tokens, "") != "你好" {
		t.Fatalf("content=%q tokens=%v", content, tokens)
	}
}

func TestOpenAIStoryPrimaryClientReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "upstream failed", http.StatusBadGateway) }))
	defer server.Close()
	client := NewOpenAIStoryPrimaryClient(&model.AppSettings{APIEndpoint: server.URL})
	if _, err := client.Stream(context.Background(), "story-model", nil, nil); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestOpenAIStoryPrimaryClientHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(time.Second) }))
	defer server.Close()
	client := NewOpenAIStoryPrimaryClient(&model.AppSettings{APIEndpoint: server.URL})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Stream(ctx, "story-model", nil, nil); err == nil {
		t.Fatal("expected cancellation error")
	}
}
