package service

import (
	"encoding/json"
	"fmt"
)

type NormalizedCharacterCard struct {
	CardVersion        string
	Character          ParsedCharacter
	Tags               []string
	PublicWorldBook    ParsedWorldBook
	SchedulerWorldBook ParsedWorldBook
	Extensions         map[string]json.RawMessage
}

func NormalizeParsedCharacterCard(card *ParsedCharacterCard) (*NormalizedCharacterCard, error) {
	if card == nil {
		return nil, fmt.Errorf("parsed character card is nil")
	}
	return &NormalizedCharacterCard{
		CardVersion:        card.CardVersion,
		Character:          card.Character,
		Tags:               append([]string(nil), card.Tags...),
		PublicWorldBook:    FilterParsedWorldBookForUser(card.WorldBook),
		SchedulerWorldBook: FilterParsedWorldBookForScheduler(card.WorldBook),
		Extensions:         card.Extensions,
	}, nil
}
