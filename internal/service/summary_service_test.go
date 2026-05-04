package service

import (
	"strings"
	"testing"

	"litechat/internal/model"
)

func TestParseSummaryChunkRejectsMissingFields(t *testing.T) {
	raw := `<chat_summary>
<plot>用户拿到了钥匙。</plot>
<user_facts>无</user_facts>
<world_state>门还没打开。</world_state>
<open_loops>下次需要继续开门。</open_loops>
</chat_summary>`

	_, err := parseSummaryChunk(raw)
	if err == nil {
		t.Fatal("expected missing relationship field to be rejected")
	}
	if !strings.Contains(err.Error(), "relationship") {
		t.Fatalf("expected error to name missing relationship field, got %v", err)
	}
}

func TestParseSummaryChunkAcceptsExplicitNoneFields(t *testing.T) {
	raw := `<chat_summary>
<plot>用户拿到了钥匙。</plot>
<relationship>无</relationship>
<user_facts>无</user_facts>
<world_state>门还没打开。</world_state>
<open_loops>下次需要继续开门。</open_loops>
</chat_summary>`

	normalized, err := parseSummaryChunk(raw)
	if err != nil {
		t.Fatalf("expected explicit none fields to parse: %v", err)
	}
	if !strings.Contains(normalized, "<relationship>无</relationship>") {
		t.Fatalf("expected normalized summary to preserve explicit none field, got %s", normalized)
	}
}

func TestBuildSmallSummaryPromptIncludesDefaultMemoryRules(t *testing.T) {
	prompt := buildSmallSummaryPrompt(nil, nil, []*model.Message{
		{Seq: 1, Role: "user", Content: "我把银钥匙藏在书架后面。"},
	}, "")

	if !strings.Contains(prompt, "必须保留会影响后续回复的稳定事实") {
		t.Fatalf("expected default memory rules in prompt, got %s", prompt)
	}
	if !strings.Contains(prompt, "银钥匙") {
		t.Fatalf("expected raw message content in prompt, got %s", prompt)
	}
}
