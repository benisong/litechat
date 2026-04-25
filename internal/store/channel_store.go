package store

import (
	"database/sql"
	"fmt"
	"litechat/internal/model"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChannelStore manages provider-specific routing metadata.
type ChannelStore struct {
	db *DB
}

func NewChannelStore(db *DB) *ChannelStore {
	return &ChannelStore{db: db}
}

func (s *ChannelStore) ListWhitelist(provider string) ([]*model.ChannelWhitelistEntry, error) {
	rows, err := s.db.Query(`
		SELECT id, provider, self_id, external_user_id, display_name, note, enabled, created_at, updated_at
		FROM channel_whitelist
		WHERE provider = ?
		ORDER BY enabled DESC, updated_at DESC, created_at DESC`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.ChannelWhitelistEntry
	for rows.Next() {
		entry, err := scanChannelWhitelistEntry(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, entry)
	}
	return list, nil
}

func (s *ChannelStore) ReplaceWhitelist(provider string, entries []*model.ChannelWhitelistEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM channel_whitelist WHERE provider = ?`, provider); err != nil {
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if strings.TrimSpace(entry.ExternalUserID) == "" {
			continue
		}
		id := entry.ID
		if id == "" {
			id = uuid.New().String()
		}
		if _, err := tx.Exec(`
			INSERT INTO channel_whitelist (id, provider, self_id, external_user_id, display_name, note, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id,
			provider,
			strings.TrimSpace(entry.SelfID),
			strings.TrimSpace(entry.ExternalUserID),
			strings.TrimSpace(entry.DisplayName),
			strings.TrimSpace(entry.Note),
			boolToInt(entry.Enabled),
			now,
			now,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ChannelStore) GetWhitelistByID(id string) (*model.ChannelWhitelistEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, provider, self_id, external_user_id, display_name, note, enabled, created_at, updated_at
		FROM channel_whitelist
		WHERE id = ?`, id)
	return scanChannelWhitelistEntry(row)
}

func (s *ChannelStore) CreateWhitelistEntry(entry *model.ChannelWhitelistEntry) error {
	entry.ID = uuid.New().String()
	entry.Provider = strings.TrimSpace(entry.Provider)
	entry.SelfID = strings.TrimSpace(entry.SelfID)
	entry.ExternalUserID = strings.TrimSpace(entry.ExternalUserID)
	entry.DisplayName = strings.TrimSpace(entry.DisplayName)
	entry.Note = strings.TrimSpace(entry.Note)
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = entry.CreatedAt

	_, err := s.db.Exec(`
		INSERT INTO channel_whitelist (id, provider, self_id, external_user_id, display_name, note, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Provider, entry.SelfID, entry.ExternalUserID, entry.DisplayName, entry.Note,
		boolToInt(entry.Enabled), entry.CreatedAt, entry.UpdatedAt,
	)
	return err
}

func (s *ChannelStore) UpdateWhitelistEntry(entry *model.ChannelWhitelistEntry) error {
	entry.SelfID = strings.TrimSpace(entry.SelfID)
	entry.ExternalUserID = strings.TrimSpace(entry.ExternalUserID)
	entry.DisplayName = strings.TrimSpace(entry.DisplayName)
	entry.Note = strings.TrimSpace(entry.Note)
	entry.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		UPDATE channel_whitelist
		SET self_id = ?, external_user_id = ?, display_name = ?, note = ?, enabled = ?, updated_at = ?
		WHERE id = ? AND provider = ?`,
		entry.SelfID, entry.ExternalUserID, entry.DisplayName, entry.Note, boolToInt(entry.Enabled), entry.UpdatedAt,
		entry.ID, entry.Provider,
	)
	return err
}

func (s *ChannelStore) DeleteWhitelistEntry(id string) error {
	_, err := s.db.Exec(`DELETE FROM channel_whitelist WHERE id = ?`, id)
	return err
}

func (s *ChannelStore) FindEnabledWhitelist(provider, selfID, externalUserID string) (*model.ChannelWhitelistEntry, error) {
	row := s.db.QueryRow(`
		SELECT id, provider, self_id, external_user_id, display_name, note, enabled, created_at, updated_at
		FROM channel_whitelist
		WHERE provider = ? AND external_user_id = ? AND enabled = 1 AND (self_id = '' OR self_id = ?)
		ORDER BY CASE WHEN self_id = ? THEN 1 ELSE 0 END DESC, updated_at DESC
		LIMIT 1`,
		provider, externalUserID, selfID, selfID,
	)
	return scanChannelWhitelistEntry(row)
}

type channelWhitelistScanner interface {
	Scan(dest ...interface{}) error
}

func scanChannelWhitelistEntry(scanner channelWhitelistScanner) (*model.ChannelWhitelistEntry, error) {
	entry := &model.ChannelWhitelistEntry{}
	var enabled int
	err := scanner.Scan(
		&entry.ID,
		&entry.Provider,
		&entry.SelfID,
		&entry.ExternalUserID,
		&entry.DisplayName,
		&entry.Note,
		&enabled,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	entry.Enabled = enabled == 1
	return entry, nil
}

func IsNotFound(err error) bool {
	return err == sql.ErrNoRows
}

func (s *ChannelStore) GetOrCreateSession(provider, selfID, externalUserID, ownerUserID string) (*model.ChannelSession, error) {
	session, err := s.GetSession(provider, selfID, externalUserID)
	if err == nil {
		if session.OwnerUserID != ownerUserID {
			if err := s.UpdateSessionOwner(session.ID, ownerUserID); err != nil {
				return nil, err
			}
			session.OwnerUserID = ownerUserID
		}
		return session, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	now := time.Now()
	session = &model.ChannelSession{
		ID:             uuid.New().String(),
		Provider:       provider,
		SelfID:         strings.TrimSpace(selfID),
		ExternalUserID: strings.TrimSpace(externalUserID),
		OwnerUserID:    ownerUserID,
		State:          "idle",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_, err = s.db.Exec(`
		INSERT INTO channel_sessions (id, provider, self_id, external_user_id, owner_user_id, active_character_id, active_chat_id, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Provider, session.SelfID, session.ExternalUserID, session.OwnerUserID,
		session.ActiveCharacterID, session.ActiveChatID, session.State, session.CreatedAt, session.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ChannelStore) GetSession(provider, selfID, externalUserID string) (*model.ChannelSession, error) {
	session := &model.ChannelSession{}
	err := s.db.QueryRow(`
		SELECT id, provider, self_id, external_user_id, owner_user_id, active_character_id, active_chat_id, state, created_at, updated_at
		FROM channel_sessions
		WHERE provider = ? AND self_id = ? AND external_user_id = ?`,
		provider, selfID, externalUserID,
	).Scan(
		&session.ID,
		&session.Provider,
		&session.SelfID,
		&session.ExternalUserID,
		&session.OwnerUserID,
		&session.ActiveCharacterID,
		&session.ActiveChatID,
		&session.State,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ChannelStore) UpdateSessionOwner(id, ownerUserID string) error {
	_, err := s.db.Exec(`UPDATE channel_sessions SET owner_user_id = ?, updated_at = ? WHERE id = ?`, ownerUserID, time.Now(), id)
	return err
}

func (s *ChannelStore) UpdateSessionCharacter(id, characterID string) error {
	_, err := s.db.Exec(`
		UPDATE channel_sessions
		SET active_character_id = ?, active_chat_id = '', state = 'idle', updated_at = ?
		WHERE id = ?`,
		characterID, time.Now(), id,
	)
	return err
}

func (s *ChannelStore) UpdateSessionChat(id, characterID, chatID string) error {
	_, err := s.db.Exec(`
		UPDATE channel_sessions
		SET active_character_id = ?, active_chat_id = ?, state = 'chatting', updated_at = ?
		WHERE id = ?`,
		characterID, chatID, time.Now(), id,
	)
	return err
}

func (s *ChannelStore) ResetSession(id string) error {
	_, err := s.db.Exec(`
		UPDATE channel_sessions
		SET active_chat_id = '', state = 'idle', updated_at = ?
		WHERE id = ?`,
		time.Now(), id,
	)
	return err
}

func (s *ChannelStore) CreateChatLink(provider, selfID, externalUserID, ownerUserID, chatID string) error {
	_, err := s.db.Exec(`
		INSERT INTO channel_chat_links (id, provider, self_id, external_user_id, owner_user_id, chat_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), provider, strings.TrimSpace(selfID), strings.TrimSpace(externalUserID), ownerUserID, chatID, time.Now(),
	)
	return err
}

func (s *ChannelStore) ListChatLinks(provider, selfID, externalUserID, ownerUserID string) ([]*model.ChannelChatLinkView, error) {
	rows, err := s.db.Query(`
		SELECT c.id,
		       c.title,
		       c.character_id,
		       COALESCE(ch.name, ''),
		       COALESCE((SELECT content FROM messages WHERE chat_id = c.id ORDER BY seq DESC, created_at DESC LIMIT 1), ''),
		       COALESCE((SELECT COUNT(*) FROM messages WHERE chat_id = c.id), 0),
		       c.updated_at,
		       l.created_at
		FROM channel_chat_links l
		JOIN chats c ON c.id = l.chat_id
		LEFT JOIN characters ch ON ch.id = c.character_id
		WHERE l.provider = ? AND l.self_id = ? AND l.external_user_id = ? AND l.owner_user_id = ?
		ORDER BY c.updated_at DESC, l.created_at DESC`,
		provider, selfID, externalUserID, ownerUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.ChannelChatLinkView
	for rows.Next() {
		view := &model.ChannelChatLinkView{}
		var msgCount int64
		if err := rows.Scan(
			&view.ChatID,
			&view.Title,
			&view.CharacterID,
			&view.CharacterName,
			&view.LastMessage,
			&msgCount,
			&view.ChatUpdatedAt,
			&view.LinkedAt,
		); err != nil {
			return nil, err
		}
		view.MessageCount = int(msgCount)
		list = append(list, view)
	}
	return list, nil
}

func (s *ChannelStore) GetChatLinkByIndex(provider, selfID, externalUserID, ownerUserID string, index int) (*model.ChannelChatLinkView, error) {
	list, err := s.ListChatLinks(provider, selfID, externalUserID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if index < 1 || index > len(list) {
		return nil, sql.ErrNoRows
	}
	return list[index-1], nil
}

func (s *ChannelStore) GetChatLinkByID(provider, selfID, externalUserID, ownerUserID, chatID string) (*model.ChannelChatLinkView, error) {
	rows, err := s.ListChatLinks(provider, selfID, externalUserID, ownerUserID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ChatID == chatID {
			return row, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *ChannelStore) EnsureChatLink(provider, selfID, externalUserID, ownerUserID, chatID string) error {
	_, err := s.GetChatLinkByID(provider, selfID, externalUserID, ownerUserID, chatID)
	if err == nil {
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	if err := s.CreateChatLink(provider, selfID, externalUserID, ownerUserID, chatID); err != nil {
		return fmt.Errorf("create chat link: %w", err)
	}
	return nil
}
