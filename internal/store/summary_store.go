package store

import (
	"database/sql"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// SummaryStore 摘要状态 / 分片 / 任务队列
type SummaryStore struct {
	db *DB
}

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
	if err := s.EnsureState(chatID); err != nil {
		return nil, err
	}

	state := &model.ChatSummaryState{}
	var currentBig sql.NullString
	if err := s.db.QueryRow(`
		SELECT chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq, updated_at
		FROM chat_summary_state WHERE chat_id = ?`, chatID,
	).Scan(&state.ChatID, &state.AppliedCutoffSeq, &currentBig, &state.DirtyFromSeq, &state.UpdatedAt); err != nil {
		return nil, err
	}
	if currentBig.Valid {
		state.CurrentBigSummary = currentBig.String
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
		ORDER BY from_seq ASC`, chatID)
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
		WHERE chat_id = ? AND level = 'big' AND status IN ('active', 'superseded') AND to_seq <= ?
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
		ORDER BY from_seq ASC`, chatID, maxToSeq)
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

func (s *SummaryStore) ScheduleSmallJob(chatID string, fromSeq, toSeq, baseCutoffSeq int) error {
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE chat_summary_jobs
		SET from_seq = ?, to_seq = ?, base_cutoff_seq = ?, status = 'pending',
		    attempt_count = 0, next_run_at = ?, last_error = '', updated_at = ?
		WHERE chat_id = ? AND job_type = 'small' AND status IN ('pending', 'failed')`,
		fromSeq, toSeq, baseCutoffSeq, now, now, chatID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows > 0 {
		return nil
	}

	job := &model.ChatSummaryJob{
		ID:            uuid.New().String(),
		ChatID:        chatID,
		JobType:       "small",
		FromSeq:       fromSeq,
		ToSeq:         toSeq,
		BaseCutoffSeq: baseCutoffSeq,
		Status:        "pending",
		NextRunAt:     now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = s.db.Exec(`
		INSERT INTO chat_summary_jobs
			(id, chat_id, job_type, from_seq, to_seq, base_cutoff_seq, status, attempt_count, next_run_at, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.ChatID, job.JobType, job.FromSeq, job.ToSeq, job.BaseCutoffSeq,
		job.Status, job.AttemptCount, job.NextRunAt, job.LastError, job.CreatedAt, job.UpdatedAt,
	)
	return err
}

func (s *SummaryStore) ScheduleBigJob(chatID string, fromSeq, toSeq, baseCutoffSeq int) error {
	var count int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM chat_summary_jobs
		WHERE chat_id = ? AND job_type = 'big' AND status IN ('pending', 'running')`, chatID,
	).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	_, err := s.db.Exec(`
		INSERT INTO chat_summary_jobs
			(id, chat_id, job_type, from_seq, to_seq, base_cutoff_seq, status, attempt_count, next_run_at, last_error, created_at, updated_at)
		VALUES (?, ?, 'big', ?, ?, ?, 'pending', 0, ?, '', ?, ?)`,
		uuid.New().String(), chatID, fromSeq, toSeq, baseCutoffSeq, now, now, now,
	)
	return err
}

func (s *SummaryStore) ClaimNextJob() (*model.ChatSummaryJob, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	job := &model.ChatSummaryJob{}
	err = tx.QueryRow(`
		SELECT id, chat_id, job_type, from_seq, to_seq, base_cutoff_seq, status, attempt_count, next_run_at, last_error, created_at, updated_at
		FROM chat_summary_jobs
		WHERE status IN ('pending', 'failed') AND next_run_at <= ?
		ORDER BY created_at ASC
		LIMIT 1`, time.Now(),
	).Scan(&job.ID, &job.ChatID, &job.JobType, &job.FromSeq, &job.ToSeq, &job.BaseCutoffSeq,
		&job.Status, &job.AttemptCount, &job.NextRunAt, &job.LastError, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if _, err := tx.Exec(`
		UPDATE chat_summary_jobs
		SET status = 'running', updated_at = ?
		WHERE id = ?`, time.Now(), job.ID); err != nil {
		return nil, err
	}
	job.Status = "running"

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *SummaryStore) CompleteJob(jobID string) error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_jobs
		SET status = 'succeeded', updated_at = ?
		WHERE id = ?`, time.Now(), jobID)
	return err
}

func (s *SummaryStore) MarkJobStale(jobID string, reason string) error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_jobs
		SET status = 'stale', last_error = ?, updated_at = ?
		WHERE id = ?`, reason, time.Now(), jobID)
	return err
}

func (s *SummaryStore) FailJob(jobID string, attemptCount int, nextRunAt time.Time, lastError string) error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_jobs
		SET status = 'failed', attempt_count = ?, next_run_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?`, attemptCount, nextRunAt, lastError, time.Now(), jobID)
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
