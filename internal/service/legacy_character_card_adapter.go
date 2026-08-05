package service

import (
	"context"
	"fmt"
	"strings"
)

type legacyCharacterCardParser struct{}

func NewLegacyCharacterCardParser() CharacterCardParser { return legacyCharacterCardParser{} }

func (legacyCharacterCardParser) Format() string { return "legacy-character-card" }

func (legacyCharacterCardParser) Parse(_ context.Context, input []byte) (*ParsedCharacterCard, error) {
	draft, err := parseCharacterCardDraft(string(input))
	if err != nil {
		return nil, fmt.Errorf("parse legacy character card: %w", err)
	}
	pov := draft.POV
	if pov == "" {
		pov = "third"
	}
	return &ParsedCharacterCard{
		CardVersion: "legacy",
		Character: ParsedCharacter{
			Name: draft.Name, POV: pov, Description: draft.Description,
			Personality: draft.Personality, Scenario: draft.Scenario, FirstMessage: draft.FirstMsg,
		},
		Tags:      splitLegacyTags(draft.Tags),
		WorldBook: ParsedWorldBook{GlobalEnabled: false},
	}, nil
}

func splitLegacyTags(tags string) []string {
	parts := strings.Split(tags, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

var _ CharacterCardParser = legacyCharacterCardParser{}
