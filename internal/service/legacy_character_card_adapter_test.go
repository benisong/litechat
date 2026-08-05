package service

import (
	"context"
	"strings"
	"testing"
)

func TestLegacyCharacterCardAdapterPreservesExistingDraftFields(t *testing.T) {
	raw := []byte("```json\n" + `{"character_card":{"name":"云舒","description":"身份","personality":"性格","scenario":"场景","first_msg":"开场","tags":["仙侠","旧约"],"pov":"third"}}` + "\n```")
	card, err := NewLegacyCharacterCardParser().Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("legacy adapter returned error: %v", err)
	}
	if card.CardVersion != "legacy" || card.Character.Name != "云舒" || card.Character.FirstMessage != "开场" {
		t.Fatalf("unexpected adapted card: %+v", card)
	}
	if !strings.Contains(strings.Join(card.Tags, ","), "仙侠") || card.WorldBook.GlobalEnabled {
		t.Fatalf("legacy card received unexpected worldbook/tags: %+v", card)
	}
}
