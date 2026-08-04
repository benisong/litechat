package store

import (
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

type CharacterCardDocumentStore struct{ db *DB }

func NewCharacterCardDocumentStore(db *DB) *CharacterCardDocumentStore {
	return &CharacterCardDocumentStore{db: db}
}

func (s *CharacterCardDocumentStore) Create(doc *model.CharacterCardDocument) error {
	if doc == nil {
		return fmt.Errorf("character card document is nil")
	}
	if doc.UserID == "" || doc.CharacterID == "" || doc.CardVersion == "" || doc.WorldBookID == "" || doc.WorldBookVersion == "" || doc.SourceJSON == "" {
		return fmt.Errorf("character card document has missing required fields")
	}
	doc.ID = uuid.New().String()
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt
	_, err := s.db.Exec(`
		INSERT INTO character_card_documents
		(id, user_id, character_id, card_version, worldbook_id, worldbook_version, source_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.UserID, doc.CharacterID, doc.CardVersion, doc.WorldBookID, doc.WorldBookVersion,
		doc.SourceJSON, doc.CreatedAt, doc.UpdatedAt)
	return err
}

func (s *CharacterCardDocumentStore) GetByCharacterID(characterID, userID string) (*model.CharacterCardDocument, error) {
	doc := &model.CharacterCardDocument{}
	err := s.db.QueryRow(`
		SELECT id, user_id, character_id, card_version, worldbook_id, worldbook_version, source_json, created_at, updated_at
		FROM character_card_documents WHERE character_id = ? AND user_id = ?`, characterID, userID).Scan(
		&doc.ID, &doc.UserID, &doc.CharacterID, &doc.CardVersion, &doc.WorldBookID, &doc.WorldBookVersion,
		&doc.SourceJSON, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *CharacterCardDocumentStore) DeleteByCharacterID(characterID, userID string) error {
	_, err := s.db.Exec(`DELETE FROM character_card_documents WHERE character_id = ? AND user_id = ?`, characterID, userID)
	return err
}
