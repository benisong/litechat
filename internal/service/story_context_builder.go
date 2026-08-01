package service

import (
	"litechat/internal/model"
	"strings"
)

// StoryContextBuilder 负责复杂剧情运行时的提示词边界。
// 它只注入 static 世界书和已确认的动态上下文，永不直接注入 compile_only 原文。
type StoryContextBuilder struct{}

func (StoryContextBuilder) Build(
	messages []model.ChatCompletionMessage,
	worldbooks []*model.WorldBook,
	runtimeContext string,
) []model.ChatCompletionMessage {
	var systemParts []string
	for _, message := range messages {
		if message.Role == "system" && strings.TrimSpace(message.Content) != "" {
			systemParts = append(systemParts, message.Content)
		}
	}

	for _, worldbook := range worldbooks {
		if worldbook == nil || (worldbook.RuntimeMode != "" && worldbook.RuntimeMode != "static") {
			continue
		}
		for _, entry := range worldbook.Entries {
			if !entry.Enabled || strings.TrimSpace(entry.Content) == "" {
				continue
			}
			systemParts = append(systemParts, entry.Content)
		}
	}
	if strings.TrimSpace(runtimeContext) != "" {
		systemParts = append(systemParts, "[Current Story Context]\n"+strings.TrimSpace(runtimeContext))
	}

	result := make([]model.ChatCompletionMessage, 0, len(messages)+1)
	if len(systemParts) > 0 {
		result = append(result, model.ChatCompletionMessage{
			Role:    "system",
			Content: strings.Join(systemParts, "\n\n"),
		})
	}
	for _, message := range messages {
		if message.Role != "system" {
			result = append(result, message)
		}
	}
	return result
}
