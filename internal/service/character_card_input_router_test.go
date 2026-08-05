package service

import (
	"context"
	"testing"
)

func TestCharacterCardInputRouterSelectsJSONParser(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"新卡","pov":"second"},"worldbook":{"id":"w","version":"1.0"}}`)
	card, err := NewCharacterCardInputRouter().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if card.CardVersion != "1.0" || card.Character.Name != "新卡" {
		t.Fatalf("unexpected card: %+v", card)
	}
}

func TestCharacterCardInputRouterSelectsLegacyParser(t *testing.T) {
	raw := []byte("```json\n" + `{"character_card":{"name":"旧卡","description":"身份","personality":"性格","scenario":"场景","first_msg":"开场","tags":["旧"]}}` + "\n```")
	card, err := NewCharacterCardInputRouter().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if card.CardVersion != "legacy" || card.Character.Name != "旧卡" {
		t.Fatalf("unexpected card: %+v", card)
	}
}

func TestCharacterCardInputRouterDoesNotFallbackBrokenNewCardToLegacy(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"不完整"}}`)
	_, err := NewCharacterCardInputRouter().Parse(context.Background(), raw)
	if err == nil {
		t.Fatal("expected broken new card to fail without legacy fallback")
	}
}
