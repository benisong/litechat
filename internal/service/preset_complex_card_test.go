package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPresetComplexCardUsesEmbeddedWorldbookTracks(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "character-design", "rebirth-fantasy-journey-complex-v1.json")
	input, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read preset: %v", err)
	}
	card, err := NewJSONCharacterCardParser().Parse(context.Background(), input)
	if err != nil {
		t.Fatalf("parse preset: %v", err)
	}
	if card.Character.Name != "重生之玄幻之旅" || card.Character.Description == "" {
		t.Fatalf("unexpected character: %+v", card.Character)
	}
	normalized, err := NormalizeParsedCharacterCard(card)
	if err != nil {
		t.Fatalf("normalize preset: %v", err)
	}
	if len(normalized.PublicWorldBook.MainEntries) != 4 {
		t.Fatalf("public entries=%d, want 4", len(normalized.PublicWorldBook.MainEntries))
	}
	if len(normalized.SchedulerWorldBook.SubEntries) != 3 {
		t.Fatalf("scheduler entries=%d, want 3", len(normalized.SchedulerWorldBook.SubEntries))
	}
	for _, entry := range normalized.PublicWorldBook.SubEntries {
		if !entry.UserVisible {
			t.Fatalf("hidden entry leaked to public track: %s", entry.ID)
		}
	}
}
