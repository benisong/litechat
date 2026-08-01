package service

import (
	"encoding/json"
	"fmt"
	"litechat/internal/model"
	"strings"
)

const schedulerOutputSchemaVersion = 1

// ParseSchedulerOutput 解析调度模型输出。
// 允许模型在 JSON 外包裹说明文字或 markdown code fence，但最终必须得到一个合法的固定版本 JSON。
func ParseSchedulerOutput(raw string) (*model.SchedulerOutput, error) {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return nil, fmt.Errorf("scheduler output is empty")
	}
	candidate = stripJSONFence(candidate)

	jsonText, err := extractSchedulerJSONObject(candidate)
	if err != nil {
		return nil, err
	}

	output := &model.SchedulerOutput{}
	if err := json.Unmarshal([]byte(jsonText), output); err != nil {
		return nil, fmt.Errorf("decode scheduler output: %w", err)
	}
	if output.SchemaVersion != schedulerOutputSchemaVersion {
		return nil, fmt.Errorf("unsupported scheduler schema version: %d", output.SchemaVersion)
	}
	if output.Observations == nil {
		output.Observations = []model.SchedulerObservation{}
	}
	if output.EventCandidates == nil {
		output.EventCandidates = []model.SchedulerEventCandidate{}
	}
	if output.Inferences == nil {
		output.Inferences = []map[string]any{}
	}
	if output.Warnings == nil {
		output.Warnings = []string{}
	}
	return output, nil
}

func stripJSONFence(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "```") {
		return value
	}
	firstNewline := strings.IndexByte(value, '\n')
	if firstNewline < 0 {
		return value
	}
	value = value[firstNewline+1:]
	if end := strings.LastIndex(value, "```"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func extractSchedulerJSONObject(value string) (string, error) {
	start := strings.IndexByte(value, '{')
	if start < 0 {
		return "", fmt.Errorf("scheduler output does not contain a JSON object")
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(value); i++ {
		ch := value[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return value[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("scheduler output contains an incomplete JSON object")
}
