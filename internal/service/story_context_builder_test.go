package service

import (
	"litechat/internal/model"
	"strings"
	"testing"
)

func TestStoryContextBuilderExcludesCompileOnlyWorldbooks(t *testing.T) {
	builder := StoryContextBuilder{}
	messages := []model.ChatCompletionMessage{
		{Role: "system", Content: "固定角色规则"},
		{Role: "user", Content: "我拒绝让出资源"},
	}
	worldbooks := []*model.WorldBook{
		{ID: "static", Name: "客观规则", RuntimeMode: "static", Entries: []model.WorldBookEntry{{Enabled: true, Content: "低阶修士面对威压会恐惧"}}},
		{ID: "plot", Name: "隐藏剧情", RuntimeMode: "compile_only", Entries: []model.WorldBookEntry{{Enabled: true, Content: "用户未来会被夺舍"}}},
	}
	got := builder.Build(messages, worldbooks, "当前场景：执法殿；资源争议尚未解决")
	if len(got) != 2 || got[0].Role != "system" {
		t.Fatalf("unexpected messages: %+v", got)
	}
	if !containsText(got[0].Content, "低阶修士面对威压会恐惧") {
		t.Fatal("static worldbook content was not included")
	}
	if !containsText(got[0].Content, "当前场景：执法殿") {
		t.Fatal("runtime context was not included")
	}
	if containsText(got[0].Content, "用户未来会被夺舍") {
		t.Fatal("compile-only worldbook leaked into runtime prompt")
	}
}

func TestStoryContextBuilderKeepsOneLeadingSystemMessage(t *testing.T) {
	builder := StoryContextBuilder{}
	messages := []model.ChatCompletionMessage{
		{Role: "user", Content: "第一句"},
		{Role: "system", Content: "后置规则"},
		{Role: "assistant", Content: "回复"},
	}
	got := builder.Build(messages, nil, "")
	if len(got) != 3 || got[0].Role != "system" {
		t.Fatalf("unexpected normalized messages: %+v", got)
	}
	if !containsText(got[0].Content, "后置规则") {
		t.Fatal("post-system content was not merged")
	}
	for i, message := range got {
		if i > 0 && message.Role == "system" {
			t.Fatalf("found second system message at %d: %+v", i, got)
		}
	}
}

func containsText(text, needle string) bool {
	return strings.Contains(text, needle)
}
