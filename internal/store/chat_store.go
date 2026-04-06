package store

import (
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// ChatStore 对话数据操作
type ChatStore struct {
	db *DB
}

func NewChatStore(db *DB) *ChatStore {
	return &ChatStore{db: db}
}

// Create 创建对话
func (s *ChatStore) Create(chat *model.Chat, userID string) error {
	chat.ID = uuid.New().String()
	chat.UserID = userID
	chat.CreatedAt = time.Now()
	chat.UpdatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO chats (id, user_id, character_id, title, preset_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		chat.ID, chat.UserID, chat.CharacterID, chat.Title, chat.PresetID, chat.CreatedAt, chat.UpdatedAt,
	)
	return err
}

// GetByID 按 ID 查询对话（限定用户）
func (s *ChatStore) GetByID(id string, userID string) (*model.Chat, error) {
	chat := &model.Chat{}
	err := s.db.QueryRow(`
		SELECT id, user_id, character_id, title, preset_id, created_at, updated_at
		FROM chats WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&chat.ID, &chat.UserID, &chat.CharacterID, &chat.Title, &chat.PresetID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return chat, nil
}

// ListByCharacter 查询某角色的所有对话（限定用户）
func (s *ChatStore) ListByCharacter(characterID string, userID string) ([]*model.Chat, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.user_id, c.character_id, c.title, c.preset_id, c.created_at, c.updated_at,
			   (SELECT content FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1) as last_message,
			   (SELECT COUNT(*) FROM messages WHERE chat_id = c.id) as msg_count
		FROM chats c
		WHERE c.character_id = ? AND c.user_id = ?
		ORDER BY c.updated_at DESC`, characterID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Chat
	for rows.Next() {
		chat := &model.Chat{}
		var lastMsg, msgCount interface{}
		if err := rows.Scan(&chat.ID, &chat.UserID, &chat.CharacterID, &chat.Title, &chat.PresetID,
			&chat.CreatedAt, &chat.UpdatedAt, &lastMsg, &msgCount); err != nil {
			return nil, err
		}
		if lastMsg != nil {
			chat.LastMessage = lastMsg.(string)
		}
		if msgCount != nil {
			chat.MsgCount = int(msgCount.(int64))
		}
		list = append(list, chat)
	}
	return list, nil
}

// ListAll 查询所有对话（带角色信息，限定用户）
func (s *ChatStore) ListAll(userID string) ([]*model.Chat, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.user_id, c.character_id, c.title, c.preset_id, c.created_at, c.updated_at,
			   ch.name, ch.avatar_url,
			   (SELECT content FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1) as last_message,
			   (SELECT COUNT(*) FROM messages WHERE chat_id = c.id) as msg_count
		FROM chats c
		LEFT JOIN characters ch ON ch.id = c.character_id
		WHERE c.user_id = ?
		ORDER BY c.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Chat
	for rows.Next() {
		chat := &model.Chat{}
		char := &model.Character{}
		var lastMsg, msgCount interface{}
		if err := rows.Scan(&chat.ID, &chat.UserID, &chat.CharacterID, &chat.Title, &chat.PresetID,
			&chat.CreatedAt, &chat.UpdatedAt,
			&char.Name, &char.AvatarURL,
			&lastMsg, &msgCount); err != nil {
			return nil, err
		}
		chat.Character = char
		if lastMsg != nil {
			chat.LastMessage = lastMsg.(string)
		}
		if msgCount != nil {
			chat.MsgCount = int(msgCount.(int64))
		}
		list = append(list, chat)
	}
	return list, nil
}

// Delete 删除对话（级联删除消息，限定用户）
func (s *ChatStore) Delete(id string, userID string) error {
	_, err := s.db.Exec(`DELETE FROM chats WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// UpdateTitle 更新对话标题（限定用户）
func (s *ChatStore) UpdateTitle(id, title string, userID string) error {
	_, err := s.db.Exec(`UPDATE chats SET title=?, updated_at=? WHERE id=? AND user_id=?`,
		title, time.Now(), id, userID)
	return err
}

// Touch 更新对话的 updated_at（限定用户）
func (s *ChatStore) Touch(id string, userID string) error {
	_, err := s.db.Exec(`UPDATE chats SET updated_at=? WHERE id=? AND user_id=?`, time.Now(), id, userID)
	return err
}

// MessageStore 消息数据操作
type MessageStore struct {
	db *DB
}

func NewMessageStore(db *DB) *MessageStore {
	return &MessageStore{db: db}
}

// Create 创建消息
func (s *MessageStore) Create(msg *model.Message) error {
	msg.ID = uuid.New().String()
	msg.CreatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO messages (id, chat_id, role, content, tokens, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ChatID, msg.Role, msg.Content, msg.Tokens, msg.CreatedAt,
	)
	return err
}

// ListByChatID 查询对话的所有消息
func (s *MessageStore) ListByChatID(chatID string) ([]*model.Message, error) {
	rows, err := s.db.Query(`
		SELECT id, chat_id, role, content, tokens, created_at
		FROM messages WHERE chat_id = ?
		ORDER BY created_at ASC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.Role, &msg.Content, &msg.Tokens, &msg.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, msg)
	}
	return list, nil
}

// DeleteByID 删除单条消息
func (s *MessageStore) DeleteByID(id string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	return err
}

// DeleteFromID 删除指定消息及其之后的所有消息（级联删除）
func (s *MessageStore) DeleteFromID(id string, chatID string) (int64, error) {
	// 用 rowid 找到该消息的位置，删除它及之后的所有消息
	result, err := s.db.Exec(`
		DELETE FROM messages WHERE chat_id = ? AND rowid >= (
			SELECT rowid FROM messages WHERE id = ? AND chat_id = ?
		)`, chatID, id, chatID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// UpdateContent 更新消息内容（用于流式完成后更新）
func (s *MessageStore) UpdateContent(id, content string, tokens int) error {
	_, err := s.db.Exec(`UPDATE messages SET content=?, tokens=? WHERE id=?`, content, tokens, id)
	return err
}
