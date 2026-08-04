package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"litechat/internal/model"
)

func TestOpenAICompletionClientUsesConfiguredEndpointAndReturnsContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("User-Agent") != "LiteChat/1.0" {
			t.Errorf("missing completion compatibility headers: accept=%q user-agent=%q", r.Header.Get("Accept"), r.Header.Get("User-Agent"))
		}
		var request model.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if request.Model != "cheap-model" || request.Stream {
			t.Errorf("unexpected request: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema_version\":1}"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAICompletionClient(&model.AppSettings{
		APIEndpoint: server.URL + "/v1",
		APIKey:      "test-key",
	})
	content, err := client.Complete(context.Background(), "cheap-model", []model.ChatCompletionMessage{{Role: "user", Content: "test"}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if content != `{"schema_version":1}` {
		t.Fatalf("unexpected content: %s", content)
	}
}

func TestOpenAICompletionClientRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewOpenAICompletionClient(&model.AppSettings{APIEndpoint: server.URL})
	if _, err := client.Complete(context.Background(), "cheap-model", nil); err == nil {
		t.Fatal("expected upstream HTTP error")
	}
}
