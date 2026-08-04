package service

import (
	"context"
	"testing"
)

func TestJSONCharacterCardParserAppliesWorldbookDefaults(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"x","pov":"second"},"worldbook":{"id":"w","version":"1.0","main_entries":[{"id":"entry"}]}}`)
	card, err := NewJSONCharacterCardParser().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	entry := card.WorldBook.MainEntries[0]
	if !card.WorldBook.GlobalEnabled || !entry.Enabled || entry.InjectionDepth != 4 || entry.Role != "system" {
		t.Fatalf("unexpected defaults: book=%+v entry=%+v", card.WorldBook, entry)
	}
}
