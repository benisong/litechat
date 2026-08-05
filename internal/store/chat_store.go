package store

import (
	"database/sql"
	"litechat/internal/model"
	"litechat/internal/statusbar"
	"strings"
	"sync"
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
		INSERT INTO chats (id, user_id, character_id, title, preset_id, scheduler_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		chat.ID, chat.UserID, chat.CharacterID, chat.Title, chat.PresetID, boolToInt(chat.SchedulerEnabled), chat.CreatedAt, chat.UpdatedAt,
	)
	return err
}

// GetByID 按 ID 查询对话（限定用户）
func (s *ChatStore) GetByID(id string, userID string) (*model.Chat, error) {
	chat := &model.Chat{}
	err := s.db.QueryRow(`
		SELECT id, user_id, character_id, title, preset_id, scheduler_enabled, created_at, updated_at
		FROM chats WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&chat.ID, &chat.UserID, &chat.CharacterID, &chat.Title, &chat.PresetID, &chat.SchedulerEnabled, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return chat, nil
}

// ListByCharacter 查询某角色的所有对话（限定用户）
func (s *ChatStore) ListByCharacter(characterID string, userID string) ([]*model.Chat, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.user_id, c.character_id, c.title, c.preset_id, c.scheduler_enabled, c.created_at, c.updated_at,
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
		if err := rows.Scan(&chat.ID, &chat.UserID, &chat.CharacterID, &chat.Title, &chat.PresetID, &chat.SchedulerEnabled,
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
		SELECT c.id, c.user_id, c.character_id, c.title, c.preset_id, c.scheduler_enabled, c.created_at, c.updated_at,
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
		if err := rows.Scan(&chat.ID, &chat.UserID, &chat.CharacterID, &chat.Title, &chat.PresetID, &chat.SchedulerEnabled,
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
	db              *DB
	cacheMu         sync.RWMutex
	latestAssistant map[string]*model.Message
}

func NewMessageStore(db *DB) *MessageStore {
	return &MessageStore{
		db:              db,
		latestAssistant: make(map[string]*model.Message),
	}
}

const messageColumnsWithStatus = `
	m.id, m.chat_id, m.seq, m.role, m.content, m.status_bar, m.tokens, m.created_at`

const messageColumnsWithoutStatus = `
	m.id, m.chat_id, m.seq, m.role, m.content, '', m.tokens, m.created_at`

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessage(scanner messageScanner, msg *model.Message) error {
	return scanner.Scan(
		&msg.ID, &msg.ChatID, &msg.Seq, &msg.Role, &msg.Content, &msg.StatusBar, &msg.Tokens, &msg.CreatedAt,
	)
}

func cloneMessage(msg *model.Message) *model.Message {
	if msg == nil {
		return nil
	}
	copy := *msg
	return &copy
}

func (s *MessageStore) cacheLatestAssistantMessage(msg *model.Message) {
	if msg == nil || msg.Role != "assistant" {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	current := s.latestAssistant[msg.ChatID]
	if current == nil || msg.Seq >= current.Seq {
		s.latestAssistant[msg.ChatID] = cloneMessage(msg)
	}
}

func (s *MessageStore) cachedLatestAssistant(chatID string) *model.Message {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	return cloneMessage(s.latestAssistant[chatID])
}

func (s *MessageStore) InvalidateLatestAssistant(chatID string) {
	s.cacheMu.Lock()
	delete(s.latestAssistant, chatID)
	s.cacheMu.Unlock()
}

func (s *MessageStore) invalidateAssistantByID(id string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	for chatID, message := range s.latestAssistant {
		if message.ID == id {
			delete(s.latestAssistant, chatID)
			return
		}
	}
}

func insertMessageBySeq(messages []*model.Message, message *model.Message) []*model.Message {
	if message == nil {
		return messages
	}
	messages = append(messages, nil)
	i := len(messages) - 1
	for i > 0 && messages[i-1].Seq > message.Seq {
		messages[i] = messages[i-1]
		i--
	}
	messages[i] = cloneMessage(message)
	return messages
}

// Create 创建消息
func (s *MessageStore) Create(msg *model.Message) error {
	if msg.Role == "assistant" {
		body, panel := statusbar.Split(msg.Content)
		if panel != "" {
			msg.Content = body
			msg.StatusBar = panel
		}
		msg.StatusBar = strings.TrimSpace(msg.StatusBar)
	} else {
		msg.StatusBar = ""
	}
	msg.ID = uuid.New().String()
	msg.CreatedAt = time.Now()

	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		tx, err := s.db.Begin()
		if err == nil {
			if err = tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE chat_id = ?`, msg.ChatID).Scan(&msg.Seq); err == nil {
				_, err = tx.Exec(`
					INSERT INTO messages (id, chat_id, seq, role, content, status_bar, tokens, created_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					msg.ID, msg.ChatID, msg.Seq, msg.Role, msg.Content, msg.StatusBar, msg.Tokens, msg.CreatedAt,
				)
			}
			if err == nil {
				err = tx.Commit()
				if err != nil {
					_ = tx.Rollback()
				}
			} else {
				_ = tx.Rollback()
			}
		}
		if err == nil {
			s.cacheLatestAssistantMessage(msg)
			return nil
		}
		lastErr = err
		if !isSQLiteBusy(err) || attempt == 7 {
			return err
		}
		time.Sleep(time.Duration(5*(attempt+1)) * time.Millisecond)
	}
	return lastErr
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") || strings.Contains(text, "database is busy") || strings.Contains(text, "sqlite_busy")
}

// ListByChatID 查询对话的所有消息
func (s *MessageStore) ListByChatID(chatID string) ([]*model.Message, error) {
	rows, err := s.db.Query(`
		SELECT `+messageColumnsWithStatus+`
		FROM messages m
		WHERE m.chat_id = ?
		ORDER BY m.seq ASC, m.created_at ASC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		if err := scanMessage(rows, msg); err != nil {
			return nil, err
		}
		list = append(list, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Role == "assistant" {
			s.cacheLatestAssistantMessage(list[i])
			break
		}
	}
	return list, nil
}

// ListForContext reads previous messages from SQLite and supplies the latest
// assistant reply from the write-through cache.
func (s *MessageStore) ListForContext(chatID string) ([]*model.Message, error) {
	latest := s.cachedLatestAssistant(chatID)
	if latest == nil {
		messages, err := s.ListByChatID(chatID)
		if err != nil {
			return nil, err
		}
		latest = s.cachedLatestAssistant(chatID)
		for _, message := range messages {
			if latest == nil || message.ID != latest.ID {
				message.StatusBar = ""
			}
		}
		return messages, nil
	}

	rows, err := s.db.Query(`
		SELECT `+messageColumnsWithoutStatus+`
		FROM messages m
		WHERE m.chat_id = ? AND m.id <> ?
		ORDER BY m.seq ASC, m.created_at ASC`, chatID, latest.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*model.Message
	for rows.Next() {
		message := &model.Message{}
		if err := scanMessage(rows, message); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return insertMessageBySeq(messages, latest), nil
}

// ListByChatIDRange 查询对话中指定范围的消息
func (s *MessageStore) ListByChatIDRange(chatID string, fromSeq, toSeq int) ([]*model.Message, error) {
	latest := s.cachedLatestAssistant(chatID)
	var rows *sql.Rows
	var err error
	if latest != nil && latest.Seq >= fromSeq && latest.Seq <= toSeq {
		rows, err = s.db.Query(`
			SELECT `+messageColumnsWithoutStatus+`
			FROM messages m
			WHERE m.chat_id = ? AND m.seq >= ? AND m.seq <= ? AND m.id <> ?
			ORDER BY m.seq ASC, m.created_at ASC`, chatID, fromSeq, toSeq, latest.ID)
	} else {
		rows, err = s.db.Query(`
			SELECT `+messageColumnsWithoutStatus+`
			FROM messages m
			WHERE m.chat_id = ? AND m.seq >= ? AND m.seq <= ?
			ORDER BY m.seq ASC, m.created_at ASC`, chatID, fromSeq, toSeq)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		if err := scanMessage(rows, msg); err != nil {
			return nil, err
		}
		list = append(list, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if latest != nil && latest.Seq >= fromSeq && latest.Seq <= toSeq {
		list = insertMessageBySeq(list, latest)
	}
	return list, nil
}

// GetByID 查询单条消息
func (s *MessageStore) GetByID(id string) (*model.Message, error) {
	msg := &model.Message{}
	row := s.db.QueryRow(`
		SELECT `+messageColumnsWithStatus+`
		FROM messages m
		WHERE m.id = ?`, id)
	err := scanMessage(row, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// LatestSeq 获取当前对话的最新消息序号
func (s *MessageStore) LatestSeq(chatID string) (int, error) {
	var seq sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(seq) FROM messages WHERE chat_id = ?`, chatID).Scan(&seq); err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return int(seq.Int64), nil
}

// CountAfterSeq counts raw messages not covered by the latest successful summary.
func (s *MessageStore) CountAfterSeq(chatID string, afterSeq int) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM messages WHERE chat_id = ? AND seq > ?`, chatID, afterSeq,
	).Scan(&count)
	return count, err
}

// ListChatIDs 返回已有消息的会话，供启动时恢复摘要积压。
func (s *MessageStore) ListChatIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT chat_id FROM messages ORDER BY chat_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []string
	for rows.Next() {
		var chatID string
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, chatID)
	}
	return chatIDs, rows.Err()
}

func (s *MessageStore) LatestUserSeq(chatID string) (int, error) {
	var seq sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(seq) FROM messages WHERE chat_id = ? AND role = 'user'`, chatID).Scan(&seq); err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 0, nil
	}
	return int(seq.Int64), nil
}

// DeleteByID 删除单条消息
func (s *MessageStore) DeleteByID(id string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE id = ?`, id)
	if err == nil {
		s.invalidateAssistantByID(id)
	}
	return err
}

// DeleteFromID 删除指定消息及其之后的所有消息（级联删除）
func (s *MessageStore) DeleteFromID(id string, chatID string) (int64, error) {
	result, err := s.db.Exec(`
		DELETE FROM messages WHERE chat_id = ? AND seq >= (
			SELECT seq FROM messages WHERE id = ? AND chat_id = ?
		)`, chatID, id, chatID)
	if err != nil {
		return 0, err
	}
	s.InvalidateLatestAssistant(chatID)
	return result.RowsAffected()
}

// UpdateContent 更新消息内容（用于流式完成后更新）
func (s *MessageStore) UpdateContent(id, content string, tokens int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var chatID, role string
	var seq int
	if err := tx.QueryRow(`SELECT chat_id, seq, role FROM messages WHERE id = ?`, id).Scan(&chatID, &seq, &role); err != nil {
		return err
	}
	panel := ""
	if role == "assistant" {
		content, panel = statusbar.Split(content)
	}
	if _, err := tx.Exec(`UPDATE messages SET content = ?, status_bar = ?, tokens = ? WHERE id = ?`, content, panel, tokens, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if role == "assistant" {
		s.InvalidateLatestAssistant(chatID)
	}
	return nil
}
