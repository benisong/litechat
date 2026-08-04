package service

import (
	"context"
	"encoding/json"
	"fmt"
)

type JSONCharacterCardImportPlan struct {
	CardVersion        string
	Character          ParsedCharacter
	Tags               []string
	PublicWorldBook    ParsedWorldBook
	SchedulerWorldBook ParsedWorldBook
	Extensions         map[string]json.RawMessage
}

func BuildJSONCharacterCardImportPlan(ctx context.Context, input []byte) (*JSONCharacterCardImportPlan, error) {
	parser := NewJSONCharacterCardParser()
	parsed, err := parser.Parse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("parse json character card: %w", err)
	}
	normalized, err := NormalizeParsedCharacterCard(parsed)
	if err != nil {
		return nil, fmt.Errorf("normalize json character card: %w", err)
	}
	return &JSONCharacterCardImportPlan{
		CardVersion:        normalized.CardVersion,
		Character:          normalized.Character,
		Tags:               append([]string(nil), normalized.Tags...),
		PublicWorldBook:    normalized.PublicWorldBook,
		SchedulerWorldBook: normalized.SchedulerWorldBook,
		Extensions:         normalized.Extensions,
	}, nil
}
