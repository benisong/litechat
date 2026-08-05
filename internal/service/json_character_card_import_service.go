package service

import (
	"context"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
)

type JSONCharacterCardImportService struct {
	characters *store.CharacterStore
	documents  *store.CharacterCardDocumentStore
}

type JSONCharacterCardImportResult struct {
	Character *model.Character
	Document  *model.CharacterCardDocument
	Plan      *JSONCharacterCardImportPlan
}

type JSONCharacterCardPublicView struct {
	CharacterID string          `json:"character_id"`
	CardVersion string          `json:"card_version"`
	WorldBook   ParsedWorldBook `json:"worldbook"`
}

func NewJSONCharacterCardImportService(characters *store.CharacterStore, documents *store.CharacterCardDocumentStore) *JSONCharacterCardImportService {
	return &JSONCharacterCardImportService{characters: characters, documents: documents}
}

func (s *JSONCharacterCardImportService) GetPublic(ctx context.Context, userID, characterID string) (*JSONCharacterCardPublicView, error) {
	if s == nil || s.documents == nil {
		return nil, fmt.Errorf("json character card import service is not configured")
	}
	doc, err := s.documents.GetByCharacterID(characterID, userID)
	if err != nil {
		return nil, fmt.Errorf("load character card document: %w", err)
	}
	plan, err := BuildJSONCharacterCardImportPlan(ctx, []byte(doc.SourceJSON))
	if err != nil {
		return nil, err
	}
	return &JSONCharacterCardPublicView{CharacterID: characterID, CardVersion: plan.CardVersion, WorldBook: plan.PublicWorldBook}, nil
}

func (s *JSONCharacterCardImportService) Import(ctx context.Context, userID string, input []byte) (*JSONCharacterCardImportResult, error) {
	if s == nil || s.characters == nil || s.documents == nil {
		return nil, fmt.Errorf("json character card import service is not configured")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	plan, err := BuildJSONCharacterCardImportPlan(ctx, input)
	if err != nil {
		return nil, err
	}

	character := &model.Character{
		Name:        plan.Character.Name,
		Description: plan.Character.Description,
		Personality: plan.Character.Personality,
		Scenario:    plan.Character.Scenario,
		FirstMsg:    plan.Character.FirstMessage,
		Tags:        strings.Join(plan.Tags, ","),
		POV:         plan.Character.POV,
	}
	if err := s.characters.Create(character, userID); err != nil {
		return nil, fmt.Errorf("create imported character: %w", err)
	}

	document := &model.CharacterCardDocument{
		UserID:           userID,
		CharacterID:      character.ID,
		CardVersion:      plan.CardVersion,
		WorldBookID:      plan.PublicWorldBook.ID,
		WorldBookVersion: plan.PublicWorldBook.Version,
		SourceJSON:       string(input),
	}
	if err := s.documents.Create(document); err != nil {
		_ = s.characters.Delete(character.ID, userID)
		return nil, fmt.Errorf("save imported character document: %w", err)
	}
	return &JSONCharacterCardImportResult{Character: character, Document: document, Plan: plan}, nil
}
