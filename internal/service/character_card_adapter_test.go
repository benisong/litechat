package service

import (
	"context"
	"testing"
)

func TestAdaptParsedJSONCardKeepsHiddenWorldbookOutOfCharacterFields(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"x","pov":"second","description":"公开身份","personality":"公开性格","scenario":"公开场景","first_message":"开场"},"worldbook":{"id":"w","version":"1.0","main_entries":[{"id":"hidden","content":"隐藏调度规则","user_visible":false,"scheduler_enabled":true}]}}`)
	parsed, err := NewJSONCharacterCardParser().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	normalized, err := NormalizeParsedCharacterCard(parsed)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.Character.Description != "公开身份" || normalized.Character.Scenario != "公开场景" {
		t.Fatalf("character fields were polluted: %+v", normalized.Character)
	}
	if len(normalized.SchedulerWorldBook.MainEntries) != 1 || normalized.SchedulerWorldBook.MainEntries[0].Content != "隐藏调度规则" {
		t.Fatalf("scheduler worldbook lost hidden entry: %+v", normalized.SchedulerWorldBook)
	}
}
