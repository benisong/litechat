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
	worldBooks *store.WorldBookStore
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

func NewJSONCharacterCardImportService(characters *store.CharacterStore, documents *store.CharacterCardDocumentStore, worldBooks ...*store.WorldBookStore) *JSONCharacterCardImportService {
	var worldBookStore *store.WorldBookStore
	if len(worldBooks) > 0 {
		worldBookStore = worldBooks[0]
	}
	return &JSONCharacterCardImportService{characters: characters, documents: documents, worldBooks: worldBookStore}
}

func (s *JSONCharacterCardImportService) GetPublic(ctx context.Context, userID, characterID string) (*JSONCharacterCardPublicView, error) {
	if s == nil || s.documents == nil {
		return nil, fmt.Errorf("json character card import service is not configured")
	}
	doc, err := s.documents.GetByCharacterID(characterID, userID)
	if err != nil {
		return nil, fmt.Errorf("load character card document: %w", err)
	}
	plan, err := BuildCharacterCardImportPlan(ctx, []byte(doc.SourceJSON))
	if err != nil {
		return nil, err
	}
	return &JSONCharacterCardPublicView{CharacterID: characterID, CardVersion: plan.CardVersion, WorldBook: plan.PublicWorldBook}, nil
}

func (s *JSONCharacterCardImportService) Import(ctx context.Context, userID string, input []byte) (*JSONCharacterCardImportResult, error) {
	plan, err := BuildJSONCharacterCardImportPlan(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.importPlan(userID, input, plan)
}

func (s *JSONCharacterCardImportService) ImportAny(ctx context.Context, userID string, input []byte) (*JSONCharacterCardImportResult, error) {
	plan, err := BuildCharacterCardImportPlan(ctx, input)
	if err != nil {
		return nil, err
	}
	return s.importPlan(userID, input, plan)
}

func (s *JSONCharacterCardImportService) importPlan(userID string, input []byte, plan *JSONCharacterCardImportPlan) (*JSONCharacterCardImportResult, error) {
	if s == nil || s.characters == nil || s.documents == nil {
		return nil, fmt.Errorf("json character card import service is not configured")
	}
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user id is required")
	}
	character := &model.Character{
		Name: plan.Character.Name, Description: plan.Character.Description,
		Personality: plan.Character.Personality, Scenario: plan.Character.Scenario,
		FirstMsg: plan.Character.FirstMessage, Tags: strings.Join(plan.Tags, ","), POV: plan.Character.POV,
	}
	if err := s.characters.Create(character, userID); err != nil {
		return nil, fmt.Errorf("create imported character: %w", err)
	}
	worldBookID, worldBookVersion := plan.PublicWorldBook.ID, plan.PublicWorldBook.Version
	if worldBookID == "" {
		worldBookID = "legacy-worldbook"
	}
	if worldBookVersion == "" {
		worldBookVersion = "legacy"
	}
	var linkedWorldBookID string
	if s.worldBooks != nil && (len(plan.PublicWorldBook.MainEntries) > 0 || len(plan.PublicWorldBook.SubEntries) > 0 || plan.PublicWorldBook.Name != "") {
		worldBook := &model.WorldBook{Name: plan.PublicWorldBook.Name, CharacterID: character.ID, RuntimeMode: "static"}
		if err := s.worldBooks.Create(worldBook, userID); err != nil {
			_ = s.characters.Delete(character.ID, userID)
			return nil, fmt.Errorf("create imported worldbook: %w", err)
		}
		linkedWorldBookID = worldBook.ID
		for _, entry := range publicWorldBookEntries(plan.PublicWorldBook, linkedWorldBookID) {
			if err := s.worldBooks.CreateEntry(&entry, userID); err != nil {
				_ = s.worldBooks.Delete(linkedWorldBookID, userID)
				_ = s.characters.Delete(character.ID, userID)
				return nil, fmt.Errorf("create imported worldbook entry: %w", err)
			}
		}
		worldBookID = linkedWorldBookID
	}
	document := &model.CharacterCardDocument{
		UserID: userID, CharacterID: character.ID, CardVersion: plan.CardVersion,
		WorldBookID: worldBookID, WorldBookVersion: worldBookVersion, SourceJSON: string(input),
	}
	if err := s.documents.Create(document); err != nil {
		if linkedWorldBookID != "" {
			_ = s.worldBooks.Delete(linkedWorldBookID, userID)
		}
		_ = s.characters.Delete(character.ID, userID)
		return nil, fmt.Errorf("save imported character document: %w", err)
	}
	return &JSONCharacterCardImportResult{Character: character, Document: document, Plan: plan}, nil
}

func publicWorldBookEntries(book ParsedWorldBook, worldBookID string) []model.WorldBookEntry {
	entries := make([]model.WorldBookEntry, 0, len(book.MainEntries)+len(book.SubEntries))
	for _, source := range append(append([]ParsedWorldBookEntry{}, book.MainEntries...), book.SubEntries...) {
		if !source.Enabled || !source.UserVisible {
			continue
		}
		entries = append(entries, model.WorldBookEntry{
			WorldBookID: worldBookID, Keys: source.Keys, SecondaryKeys: source.SecondaryKeys,
			Content: source.Content, Enabled: source.Enabled, Constant: source.Constant,
			Priority: source.Priority, InjectionPos: source.InjectionPosition, InjectionDepth: source.InjectionDepth,
			ScanDepth: source.ScanDepth, CaseSensitive: source.CaseSensitive, Order: source.Order, Role: source.Role,
		})
	}
	return entries
}
