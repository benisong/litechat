package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"net/http"
	"strings"
)

type OpenAIStoryPrimaryClient struct {
	settings *model.AppSettings
	http     *http.Client
}

func NewOpenAIStoryPrimaryClient(settings *model.AppSettings) *OpenAIStoryPrimaryClient {
	return &OpenAIStoryPrimaryClient{settings: settings, http: &http.Client{}}
}

func (c *OpenAIStoryPrimaryClient) Stream(ctx context.Context, modelName string, messages []model.ChatCompletionMessage, callback StreamCallback) (string, error) {
	if c == nil || c.settings == nil {
		return "", fmt.Errorf("story primary client is not configured")
	}
	if strings.TrimSpace(c.settings.APIEndpoint) == "" {
		return "", fmt.Errorf("story primary API endpoint is not configured")
	}
	body, err := json.Marshal(model.ChatCompletionRequest{Model: modelName, Messages: messages, Stream: true})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(c.settings.APIEndpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.settings.APIKey)
	}
	client := c.http
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("story primary request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("story primary HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		if data == "" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return content.String(), fmt.Errorf("story primary SSE JSON: %w", err)
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		token := chunk.Choices[0].Delta.Content
		content.WriteString(token)
		if callback != nil {
			if err := callback(token); err != nil {
				return content.String(), err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return content.String(), fmt.Errorf("story primary SSE: %w", err)
	}
	if strings.TrimSpace(content.String()) == "" {
		return "", fmt.Errorf("story primary returned empty content")
	}
	return content.String(), nil
}
