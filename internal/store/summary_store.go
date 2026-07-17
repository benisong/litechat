package store

import (
	"database/sql"
	"errors"
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// SummaryStore 管理摘要状态与分片。
type SummaryStore struct {
	db *DB
}

var ErrSummaryStateChanged = errors.New("摘要状态已变化，请刷新后重试")

func NewSummaryStore(db *DB) *SummaryStore {
	return &SummaryStore{db: db}
}

func (s *SummaryStore) EnsureState(chatID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO chat_summary_state (chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq, updated_at)
		VALUES (?, 0, '', 0, ?)`, chatID, time.Now())
	return err
}

func (s *SummaryStore) GetState(chatID string) (*model.ChatSummaryState, error) {
	state := &model.ChatSummaryState{}
	load := func() error {
		var currentBig sql.NullString
		err := s.db.QueryRow(`
			SELECT chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq, updated_at
			FROM chat_summary_state WHERE chat_id = ?`, chatID,
		).Scan(&state.ChatID, &state.AppliedCutoffSeq, &currentBig, &state.DirtyFromSeq, &state.UpdatedAt)
		if err == nil && currentBig.Valid {
			state.CurrentBigSummary = currentBig.String
		}
		return err
	}

	if err := load(); err == nil {
		return state, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err := s.EnsureState(chatID); err != nil {
		return nil, err
	}
	if err := load(); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *SummaryStore) ApplySmallSummary(chatID string, cutoffSeq int) error {
	if err := s.EnsureState(chatID); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET applied_cutoff_seq = ?,
		    dirty_from_seq = CASE WHEN dirty_from_seq > 0 AND dirty_from_seq <= ? THEN 0 ELSE dirty_from_seq END,
		    updated_at = ?
		WHERE chat_id = ?`,
		cutoffSeq, cutoffSeq, time.Now(), chatID,
	)
	return err
}

// CommitRollingSummaryAndExchange 在所有模型调用完成后，短暂开启事务并原子写入摘要与本轮消息。
// expectedCutoffSeq / expectedLatestSeq 用于防止等待模型期间同一会话被其他请求修改。
func (s *SummaryStore) CommitRollingSummaryAndExchange(
	chatID string,
	expectedCutoffSeq int,
	expectedLatestSeq int,
	expectedLatestID string,
	summaryToSeq int,
	summaryContent string,
	userContent string,
	assistantContent string,
) error {
	if summaryToSeq <= expectedCutoffSeq || summaryToSeq > expectedLatestSeq {
		return fmt.Errorf("无效的摘要范围: cutoff=%d to=%d latest=%d", expectedCutoffSeq, summaryToSeq, expectedLatestSeq)
	}
	if summaryContent == "" || userContent == "" || assistantContent == "" {
		return errors.New("摘要或本轮消息为空")
	}

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO chat_summary_state
			(chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq, updated_at)
		VALUES (?, 0, '', 0, ?)`, chatID, now); err != nil {
		return err
	}

	var currentCutoff int
	if err := tx.QueryRow(`
		SELECT applied_cutoff_seq FROM chat_summary_state WHERE chat_id = ?`, chatID,
	).Scan(&currentCutoff); err != nil {
		return err
	}
	latestSeq := 0
	latestID := ""
	err = tx.QueryRow(`
		SELECT seq, id FROM messages
		WHERE chat_id = ?
		ORDER BY seq DESC, created_at DESC LIMIT 1`, chatID,
	).Scan(&latestSeq, &latestID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if currentCutoff != expectedCutoffSeq || latestSeq != expectedLatestSeq || latestID != expectedLatestID {
		return ErrSummaryStateChanged
	}

	summaryID := uuid.New().String()
	if _, err := tx.Exec(`
		UPDATE chat_summary_chunks
		SET status = 'dirty', updated_at = ?
		WHERE chat_id = ? AND status != 'dirty'`, now, chatID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO chat_summary_chunks
			(id, chat_id, level, from_seq, to_seq, content, status, merged_into_id, created_at, updated_at)
		VALUES (?, ?, 'big', 1, ?, ?, 'active', '', ?, ?)`,
		summaryID, chatID, summaryToSeq, summaryContent, now, now); err != nil {
		return err
	}

	userSeq := expectedLatestSeq + 1
	assistantSeq := expectedLatestSeq + 2
	if _, err := tx.Exec(`
		INSERT INTO messages (id, chat_id, seq, role, content, tokens, created_at)
		VALUES (?, ?, ?, 'user', ?, 0, ?)`,
		uuid.New().String(), chatID, userSeq, userContent, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO messages (id, chat_id, seq, role, content, tokens, created_at)
		VALUES (?, ?, ?, 'assistant', ?, 0, ?)`,
		uuid.New().String(), chatID, assistantSeq, assistantContent, now); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE chat_summary_state
		SET applied_cutoff_seq = ?, current_big_summary_id = ?, dirty_from_seq = 0, updated_at = ?
		WHERE chat_id = ?`, summaryToSeq, summaryID, now, chatID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SummaryStore) SetCurrentBigSummary(chatID, chunkID string) error {
	if err := s.EnsureState(chatID); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET current_big_summary_id = ?, updated_at = ?
		WHERE chat_id = ?`,
		chunkID, time.Now(), chatID,
	)
	return err
}

func (s *SummaryStore) RollbackCutoff(chatID string, cutoffSeq, dirtyFromSeq int) error {
	if err := s.EnsureState(chatID); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET applied_cutoff_seq = ?,
		    dirty_from_seq = CASE
		        WHEN dirty_from_seq = 0 OR dirty_from_seq > ? THEN ?
		        ELSE dirty_from_seq
		    END,
		    updated_at = ?
		WHERE chat_id = ?`,
		cutoffSeq, dirtyFromSeq, dirtyFromSeq, time.Now(), chatID,
	)
	return err
}

func (s *SummaryStore) CreateChunk(chunk *model.ChatSummaryChunk) error {
	chunk.ID = uuid.New().String()
	chunk.CreatedAt = time.Now()
	chunk.UpdatedAt = chunk.CreatedAt
	if chunk.Status == "" {
		chunk.Status = "active"
	}

	_, err := s.db.Exec(`
		INSERT INTO chat_summary_chunks
			(id, chat_id, level, from_seq, to_seq, content, status, merged_into_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		chunk.ID, chunk.ChatID, chunk.Level, chunk.FromSeq, chunk.ToSeq, chunk.Content,
		chunk.Status, chunk.MergedIntoID, chunk.CreatedAt, chunk.UpdatedAt,
	)
	return err
}

func (s *SummaryStore) GetActiveBigChunk(chatID string) (*model.ChatSummaryChunk, error) {
	row := s.db.QueryRow(`
		SELECT id, chat_id, level, from_seq, to_seq, content, status, merged_into_id, created_at, updated_at
		FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'big' AND status = 'active'
		ORDER BY to_seq DESC LIMIT 1`, chatID)
	return scanSummaryChunk(row)
}

func (s *SummaryStore) ListActiveSmallChunks(chatID string) ([]*model.ChatSummaryChunk, error) {
	rows, err := s.db.Query(`
		SELECT id, chat_id, level, from_seq, to_seq, content, status, merged_into_id, created_at, updated_at
		FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'small' AND status = 'active'
		ORDER BY from_seq ASC, to_seq DESC`, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.ChatSummaryChunk
	for rows.Next() {
		chunk, err := scanSummaryChunk(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, chunk)
	}
	return list, nil
}

func (s *SummaryStore) GetLatestUsableBigChunk(chatID string, maxToSeq int) (*model.ChatSummaryChunk, error) {
	if maxToSeq <= 0 {
		return nil, nil
	}

	row := s.db.QueryRow(`
		SELECT id, chat_id, level, from_seq, to_seq, content, status, merged_into_id, created_at, updated_at
		FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'big' AND status IN ('active', 'superseded')
		  AND from_seq <= 1 AND to_seq <= ?
		ORDER BY to_seq DESC LIMIT 1`, chatID, maxToSeq)
	return scanSummaryChunk(row)
}

func (s *SummaryStore) ListUsableSmallChunks(chatID string, maxToSeq int) ([]*model.ChatSummaryChunk, error) {
	if maxToSeq <= 0 {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT id, chat_id, level, from_seq, to_seq, content, status, merged_into_id, created_at, updated_at
		FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'small' AND status IN ('active', 'merged') AND to_seq <= ?
		ORDER BY from_seq ASC, to_seq DESC`, chatID, maxToSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*model.ChatSummaryChunk
	for rows.Next() {
		chunk, err := scanSummaryChunk(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, chunk)
	}
	return list, nil
}

func (s *SummaryStore) CountActiveSmallChunks(chatID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'small' AND status = 'active'`, chatID,
	).Scan(&count)
	return count, err
}

func (s *SummaryStore) SupersedeBigChunk(chatID string) error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_chunks
		SET status = 'superseded', updated_at = ?
		WHERE chat_id = ? AND level = 'big' AND status = 'active'`,
		time.Now(), chatID,
	)
	return err
}

func (s *SummaryStore) MarkSmallChunksMerged(chunkIDs []string, mergedIntoID string) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	for _, chunkID := range chunkIDs {
		if _, err := s.db.Exec(`
			UPDATE chat_summary_chunks
			SET status = 'merged', merged_into_id = ?, updated_at = ?
			WHERE id = ?`, mergedIntoID, time.Now(), chunkID); err != nil {
			return err
		}
	}
	return nil
}

func (s *SummaryStore) MarkChunksDirtyFromSeq(chatID string, fromSeq int) error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_chunks
		SET status = 'dirty', updated_at = ?
		WHERE chat_id = ? AND to_seq >= ? AND status != 'dirty'`,
		time.Now(), chatID, fromSeq,
	)
	return err
}

func (s *SummaryStore) ResetCurrentBigSummaryIfDirty(chatID string) error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET current_big_summary_id = '', updated_at = ?
		WHERE chat_id = ? AND current_big_summary_id IN (
			SELECT id FROM chat_summary_chunks
			WHERE chat_id = ? AND level = 'big' AND status = 'dirty'
		)`, time.Now(), chatID, chatID)
	return err
}

type summaryChunkScanner interface {
	Scan(dest ...any) error
}

func scanSummaryChunk(scanner summaryChunkScanner) (*model.ChatSummaryChunk, error) {
	chunk := &model.ChatSummaryChunk{}
	var mergedInto sql.NullString
	err := scanner.Scan(
		&chunk.ID,
		&chunk.ChatID,
		&chunk.Level,
		&chunk.FromSeq,
		&chunk.ToSeq,
		&chunk.Content,
		&chunk.Status,
		&mergedInto,
		&chunk.CreatedAt,
		&chunk.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if mergedInto.Valid {
		chunk.MergedIntoID = mergedInto.String
	}
	return chunk, nil
}
