package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"net/http"
	"strings"
	"time"
)

// OpenAICompletionClient 调用 OpenAI Chat Completions 兼容接口。
// 它只服务调度/初始化等非流式请求，不改变旧聊天的流式客户端。
type OpenAICompletionClient struct {
	settings         *model.AppSettings
	httpClient       *http.Client
	settingsProvider func() *model.AppSettings
}

func NewOpenAICompletionClient(settings *model.AppSettings) *OpenAICompletionClient {
	return &OpenAICompletionClient{
		settings:   settings,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *OpenAICompletionClient) SetSettingsProvider(provider func() *model.AppSettings) {
	if c != nil {
		c.settingsProvider = provider
	}
}
func (c *OpenAICompletionClient) Complete(ctx context.Context, modelName string, messages []model.ChatCompletionMessage) (string, error) {
	if c == nil {
		return "", fmt.Errorf("completion client is not configured")
	}
	settings := c.settings
	if c.settingsProvider != nil {
		if current := c.settingsProvider(); current != nil {
			settings = current
		}
	}
	if c == nil || settings == nil {
		return "", fmt.Errorf("completion client is not configured")
	}
	if strings.TrimSpace(settings.APIEndpoint) == "" {
		return "", fmt.Errorf("API endpoint is empty")
	}
	if strings.TrimSpace(modelName) == "" {
		return "", fmt.Errorf("model name is empty")
	}

	requestBody := model.ChatCompletionRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: 0.1,
		MaxTokens:   2048,
		TopP:        0.9,
		Stream:      false,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal completion request: %w", err)
	}

	endpoint := strings.TrimRight(c.settings.APIEndpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(settings.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+settings.APIKey)
	}

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("completion request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errorBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
		return "", fmt.Errorf("completion upstream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errorBody)))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode completion response: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("completion response has no choices")
	}
	return response.Choices[0].Message.Content, nil
}
