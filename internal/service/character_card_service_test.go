package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseCharacterCardDraftAcceptsGeminiJSONCodeFence(t *testing.T) {
	raw := "```json\n" + `{
  "character_card": {
    "name": "云舒",
    "description": "{{char}} 是守着山城旧钟楼的女修，和 {{user}} 有一段未说出口的旧约。",
    "personality": "{{char}} 温和克制，遇到危险时会先护住 {{user}}，说话慢而坚定。",
    "scenario": "暴雨夜里，{{char}} 在钟楼下等到迟来的 {{user}}，旧约的期限只剩最后一炷香。",
    "first_msg": "{{char}} 抬手拂去肩头雨水，看向 {{user}}：这一次，{{user}} 还要假装不记得吗？",
    "tags": ["仙侠", "旧约", "克制"]
  }
}` + "\n```"

	draft, err := parseCharacterCardDraft(raw)
	if err != nil {
		t.Fatalf("parseCharacterCardDraft returned error: %v", err)
	}
	if draft.Name != "云舒" {
		t.Fatalf("unexpected name: %s", draft.Name)
	}
	if draft.Tags != "仙侠,旧约,克制" {
		t.Fatalf("unexpected tags: %s", draft.Tags)
	}
	if !strings.Contains(draft.FirstMsg, "{{user}}") {
		t.Fatalf("first message should preserve placeholders: %s", draft.FirstMsg)
	}
}

func TestCompletionMessageContentAcceptsPartsArray(t *testing.T) {
	var content completionMessageContent
	raw := []byte(`[
  {"type": "text", "text": "<character_card>"},
  {"type": "text", "text": "<name>云舒</name>"}
]`)

	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	got := content.String()
	if !strings.Contains(got, "<character_card>") || !strings.Contains(got, "<name>云舒</name>") {
		t.Fatalf("unexpected content: %s", got)
	}
}
