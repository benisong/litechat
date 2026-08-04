package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type CharacterCardParser interface {
	Parse(context.Context, []byte) (*ParsedCharacterCard, error)
	Format() string
}

type ParsedCharacterCard struct {
	CardVersion string                     `json:"card_version"`
	Character   ParsedCharacter            `json:"character"`
	WorldBook   ParsedWorldBook            `json:"worldbook"`
	Tags        []string                   `json:"tags,omitempty"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}

type ParsedCharacter struct {
	Name         string `json:"name"`
	POV          string `json:"pov"`
	Description  string `json:"description"`
	Personality  string `json:"personality"`
	Scenario     string `json:"scenario"`
	FirstMessage string `json:"first_message"`
}

type ParsedWorldBook struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Version       string                 `json:"version"`
	GlobalEnabled bool                   `json:"global_enabled"`
	MainEntries   []ParsedWorldBookEntry `json:"main_entries,omitempty"`
	SubEntries    []ParsedWorldBookEntry `json:"sub_entries,omitempty"`
}

type ParsedWorldBookEntry struct {
	ID                string                 `json:"id"`
	Title             string                 `json:"title"`
	Keys              string                 `json:"keys"`
	SecondaryKeys     string                 `json:"secondary_keys"`
	Content           string                 `json:"content"`
	Enabled           bool                   `json:"enabled"`
	Constant          bool                   `json:"constant"`
	UserVisible       bool                   `json:"user_visible"`
	SchedulerEnabled  bool                   `json:"scheduler_enabled"`
	Priority          int                    `json:"priority"`
	InjectionPosition int                    `json:"injection_position"`
	InjectionDepth    int                    `json:"injection_depth"`
	ScanDepth         int                    `json:"scan_depth"`
	CaseSensitive     bool                   `json:"case_sensitive"`
	Order             int                    `json:"order"`
	Role              string                 `json:"role"`
	Activation        *ParsedEntryActivation `json:"activation,omitempty"`
}

type ParsedEntryActivation struct {
	Keywords []string `json:"keywords,omitempty"`
	Requires []string `json:"requires,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
}

type jsonCharacterCardParser struct{}

func NewJSONCharacterCardParser() CharacterCardParser { return jsonCharacterCardParser{} }

func (jsonCharacterCardParser) Format() string { return "json-character-card" }

func (jsonCharacterCardParser) Parse(_ context.Context, input []byte) (*ParsedCharacterCard, error) {
	var envelope struct {
		CardVersion string                     `json:"card_version"`
		Character   ParsedCharacter            `json:"character"`
		WorldBook   ParsedWorldBook            `json:"worldbook"`
		Tags        []string                   `json:"tags"`
		Extensions  map[string]json.RawMessage `json:"extensions"`
	}
	if err := json.Unmarshal(input, &envelope); err != nil {
		return nil, fmt.Errorf("decode json character card: %w", err)
	}
	if strings.TrimSpace(envelope.CardVersion) == "" {
		return nil, fmt.Errorf("card_version is required")
	}
	if strings.TrimSpace(envelope.Character.Name) == "" {
		return nil, fmt.Errorf("character.name is required")
	}
	if envelope.Character.POV != "second" && envelope.Character.POV != "third" {
		return nil, fmt.Errorf("character.pov must be second or third")
	}
	if strings.TrimSpace(envelope.WorldBook.ID) == "" {
		return nil, fmt.Errorf("worldbook.id is required")
	}
	if strings.TrimSpace(envelope.WorldBook.Version) == "" {
		return nil, fmt.Errorf("worldbook.version is required")
	}
	if err := normalizeWorldBookEntries(&envelope.WorldBook); err != nil {
		return nil, err
	}
	return &ParsedCharacterCard{
		CardVersion: envelope.CardVersion,
		Character:   envelope.Character,
		WorldBook:   envelope.WorldBook,
		Tags:        envelope.Tags,
		Extensions:  envelope.Extensions,
	}, nil
}

func normalizeWorldBookEntries(book *ParsedWorldBook) error {
	seen := map[string]bool{}
	for _, entries := range [][]ParsedWorldBookEntry{book.MainEntries, book.SubEntries} {
		for _, entry := range entries {
			if strings.TrimSpace(entry.ID) == "" {
				return fmt.Errorf("worldbook entry id is required")
			}
			if seen[entry.ID] {
				return fmt.Errorf("duplicate worldbook entry id: %s", entry.ID)
			}
			seen[entry.ID] = true
			if entry.InjectionPosition != 0 && entry.InjectionPosition != 1 {
				return fmt.Errorf("entry %s has invalid injection_position", entry.ID)
			}
			if entry.InjectionDepth < 0 || entry.ScanDepth < 0 {
				return fmt.Errorf("entry %s has negative depth", entry.ID)
			}
		}
	}
	return nil
}
