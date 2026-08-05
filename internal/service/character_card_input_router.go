package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type CharacterCardInputRouter struct {
	registry *CharacterCardParserRegistry
}

func NewCharacterCardInputRouter() *CharacterCardInputRouter {
	return &CharacterCardInputRouter{registry: NewCharacterCardParserRegistry()}
}

func (r *CharacterCardInputRouter) Parse(ctx context.Context, input []byte) (*ParsedCharacterCard, error) {
	format, version, err := detectCharacterCardFormat(input)
	if err != nil {
		return nil, err
	}
	parser, err := r.registry.Resolve(format, version)
	if err != nil {
		return nil, err
	}
	return parser.Parse(ctx, input)
}

func detectCharacterCardFormat(input []byte) (string, string, error) {
	text := strings.TrimSpace(string(input))
	if text == "" {
		return "", "", fmt.Errorf("character card input is empty")
	}
	if strings.Contains(text, `"card_version"`) {
		var envelope struct {
			CardVersion string `json:"card_version"`
		}
		if err := json.Unmarshal(input, &envelope); err != nil {
			return "", "", fmt.Errorf("new json character card is malformed: %w", err)
		}
		if strings.TrimSpace(envelope.CardVersion) == "" {
			return "", "", fmt.Errorf("new json character card card_version is required")
		}
		return "json-character-card", envelope.CardVersion, nil
	}
	return "legacy-character-card", "legacy", nil
}
