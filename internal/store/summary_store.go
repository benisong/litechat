package store

import (
	"database/sql"
	"errors"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// SummaryStore 管理摘要状态与分片。
type SummaryStore struct {
	db        *DB
	messageDB *DB
}

var ErrSummaryStateChanged = errors.New("摘要状态已变化，请刷新后重试")

func NewSummaryStore(db *DB, messageDB ...*DB) *SummaryStore {
	mainDB := db
	if len(messageDB) > 0 && messageDB[0] != nil {
		mainDB = messageDB[0]
	}
	return &SummaryStore{db: db, messageDB: mainDB}
}

func (s *SummaryStore) ListChatIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT chat_id FROM chat_summary_state ORDER BY chat_id`)
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

func (s *SummaryStore) DeleteChat(chatID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM chat_summary_chunks WHERE chat_id = ?`, chatID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chat_summary_state WHERE chat_id = ?`, chatID); err != nil {
		return err
	}
	return tx.Commit()
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
		var currentBig, pendingStatus, pendingRunID, pendingError sql.NullString
		var pendingStartedAt sql.NullTime
		err := s.db.QueryRow(`
			SELECT chat_id, applied_cutoff_seq, current_big_summary_id, dirty_from_seq,
			       pending_to_seq, pending_status, pending_run_id, pending_attempts,
			       pending_error, pending_started_at, summary_required, next_summary_floor,
			       eligibility_seq, updated_at
			FROM chat_summary_state WHERE chat_id = ?`, chatID,
		).Scan(
			&state.ChatID, &state.AppliedCutoffSeq, &currentBig, &state.DirtyFromSeq,
			&state.PendingToSeq, &pendingStatus, &pendingRunID, &state.PendingAttempts,
			&pendingError, &pendingStartedAt, &state.SummaryRequired, &state.NextSummaryFloor,
			&state.EligibilitySeq, &state.UpdatedAt,
		)
		if err == nil {
			if currentBig.Valid {
				state.CurrentBigSummary = currentBig.String
			}
			if pendingStatus.Valid {
				state.PendingStatus = pendingStatus.String
			}
			if pendingRunID.Valid {
				state.PendingRunID = pendingRunID.String
			}
			if pendingError.Valid {
				state.PendingError = pendingError.String
			}
			if pendingStartedAt.Valid {
				startedAt := pendingStartedAt.Time
				state.PendingStartedAt = &startedAt
			}
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

type PendingSummaryJob struct {
	ChatID          string
	BaseCutoffSeq   int
	ToSeq           int
	TargetMessageID string
	RunID           string
	Attempt         int
}

// UpdateSummaryEligibility persists the two trigger parameters and queues an eligible boundary.
// Evaluations older than the latest committed snapshot are ignored.
func (s *SummaryStore) UpdateSummaryEligibility(chatID string, required bool, currentFloor, toSeq, evaluatedSeq int) (bool, error) {
	if err := s.EnsureState(chatID); err != nil {
		return false, err
	}
	var latestMessageSeq int
	if err := s.messageDB.QueryRow(`
		SELECT COALESCE(MAX(seq), 0) FROM messages WHERE chat_id = ?`, chatID,
	).Scan(&latestMessageSeq); err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var cutoff, nextFloor, eligibilitySeq int
	var pendingStatus string
	if err := tx.QueryRow(`
		SELECT applied_cutoff_seq, pending_status, next_summary_floor, eligibility_seq
		FROM chat_summary_state WHERE chat_id = ?`, chatID,
	).Scan(&cutoff, &pendingStatus, &nextFloor, &eligibilitySeq); err != nil {
		return false, err
	}
	if evaluatedSeq != latestMessageSeq || evaluatedSeq < eligibilitySeq {
		return false, tx.Commit()
	}

	now := time.Now()
	if !required {
		if pendingStatus == "running" {
			_, err = tx.Exec(`
				UPDATE chat_summary_state
				SET summary_required = 0, eligibility_seq = MAX(eligibility_seq, ?), updated_at = ?
				WHERE chat_id = ?`, evaluatedSeq, now, chatID)
		} else {
			_, err = tx.Exec(`
				UPDATE chat_summary_state
				SET summary_required = 0, pending_to_seq = 0, pending_status = '', pending_run_id = '',
				    pending_attempts = 0, pending_error = '', pending_started_at = NULL,
				    eligibility_seq = MAX(eligibility_seq, ?), updated_at = ?
				WHERE chat_id = ?`, evaluatedSeq, now, chatID)
		}
		if err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	if toSeq <= cutoff {
		if _, err := tx.Exec(`
			UPDATE chat_summary_state
			SET summary_required = 1, eligibility_seq = MAX(eligibility_seq, ?), updated_at = ?
			WHERE chat_id = ?`, evaluatedSeq, now, chatID); err != nil {
			return false, err
		}
		return false, tx.Commit()
	}

	if pendingStatus == "failed" {
		_, err = tx.Exec(`
			UPDATE chat_summary_state
			SET summary_required = 1, pending_to_seq = CASE WHEN pending_to_seq < ? THEN ? ELSE pending_to_seq END,
			    eligibility_seq = MAX(eligibility_seq, ?), updated_at = ? WHERE chat_id = ?`,
			toSeq, toSeq, evaluatedSeq, now, chatID)
		if err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	if pendingStatus == "pending" || pendingStatus == "running" || currentFloor < nextFloor {
		if _, err := tx.Exec(`
			UPDATE chat_summary_state
			SET summary_required = 1, eligibility_seq = MAX(eligibility_seq, ?), updated_at = ?
			WHERE chat_id = ?`, evaluatedSeq, now, chatID); err != nil {
			return false, err
		}
		return false, tx.Commit()
	}

	result, err := tx.Exec(`
		UPDATE chat_summary_state
		SET summary_required = 1, pending_to_seq = ?, pending_status = 'pending', pending_run_id = '',
		    pending_attempts = 0, pending_error = '', pending_started_at = NULL,
		    eligibility_seq = MAX(eligibility_seq, ?), updated_at = ?
		WHERE chat_id = ?`, toSeq, evaluatedSeq, now, chatID,
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ClaimPendingSummary leases one pending job. The transaction ends before any network request.
func (s *SummaryStore) ClaimPendingSummary(chatID string, staleBefore time.Time) (*PendingSummaryJob, error) {
	return s.claimPendingSummary(chatID, staleBefore, 0)
}

// ClaimPendingSummaryBefore prevents a delayed wake-up from claiming a summary
// that was queued by an assistant reply generated after the triggering user message.
func (s *SummaryStore) ClaimPendingSummaryBefore(chatID string, staleBefore time.Time, beforeSeq int) (*PendingSummaryJob, error) {
	return s.claimPendingSummary(chatID, staleBefore, beforeSeq)
}

func (s *SummaryStore) claimPendingSummary(chatID string, staleBefore time.Time, beforeSeq int) (*PendingSummaryJob, error) {
	if err := s.EnsureState(chatID); err != nil {
		return nil, err
	}
	var currentFloor int
	if err := s.messageDB.QueryRow(`
		SELECT COUNT(*) FROM messages WHERE chat_id = ? AND role = 'assistant'`, chatID,
	).Scan(&currentFloor); err != nil {
		return nil, err
	}

	now := time.Now()
	runID := uuid.New().String()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE chat_summary_state
		SET pending_status = 'running', pending_run_id = ?,
		    pending_attempts = pending_attempts + 1, pending_error = '',
		    pending_started_at = ?,
		    next_summary_floor = ? + 10,
		    updated_at = ?
		WHERE chat_id = ? AND summary_required = 1 AND pending_to_seq > applied_cutoff_seq
		  AND ? >= next_summary_floor
		  AND (? <= 0 OR pending_to_seq < ?)
		  AND (
		    pending_status IN ('pending', 'failed')
		    OR (pending_status = 'running' AND (pending_started_at IS NULL OR pending_started_at < ?))
		  )`, runID, now, currentFloor, now, chatID, currentFloor, beforeSeq, beforeSeq, staleBefore)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, tx.Commit()
	}

	job := &PendingSummaryJob{ChatID: chatID, RunID: runID}
	if err := tx.QueryRow(`
		SELECT applied_cutoff_seq, pending_to_seq, pending_attempts
		FROM chat_summary_state WHERE chat_id = ?`, chatID,
	).Scan(&job.BaseCutoffSeq, &job.ToSeq, &job.Attempt); err != nil {
		return nil, err
	}
	var targetRole string
	if err := s.messageDB.QueryRow(`
		SELECT id, role FROM messages WHERE chat_id = ? AND seq = ?`, chatID, job.ToSeq,
	).Scan(&job.TargetMessageID, &targetRole); err != nil {
		return nil, err
	}
	if targetRole != "assistant" || job.ToSeq <= job.BaseCutoffSeq {
		return nil, ErrSummaryStateChanged
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

// CompletePendingSummary promotes a successful rolling summary in one short transaction.
func (s *SummaryStore) CompletePendingSummary(job *PendingSummaryJob, summaryContent string, charLimit int) error {
	if job == nil || summaryContent == "" || job.ToSeq <= job.BaseCutoffSeq {
		return errors.New("摘要任务或摘要内容为空")
	}
	if charLimit <= 0 {
		charLimit = 3000
	}
	var targetID, targetRole string
	if err := s.messageDB.QueryRow(`
		SELECT id, role FROM messages WHERE chat_id = ? AND seq = ?`, job.ChatID, job.ToSeq,
	).Scan(&targetID, &targetRole); err != nil {
		return err
	}
	if targetID != job.TargetMessageID || targetRole != "assistant" {
		return ErrSummaryStateChanged
	}
	var unsummarizedChars, latestMessageSeq int
	if err := s.messageDB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN seq > ? THEN LENGTH(content) ELSE 0 END), 0),
		       COALESCE(MAX(seq), 0)
		FROM messages WHERE chat_id = ?`, job.ToSeq, job.ChatID,
	).Scan(&unsummarizedChars, &latestMessageSeq); err != nil {
		return err
	}
	summaryRequired := unsummarizedChars >= charLimit

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var cutoff, dirtyFrom, pendingTo int
	var pendingStatus, pendingRunID string
	if err := tx.QueryRow(`
		SELECT applied_cutoff_seq, dirty_from_seq, pending_to_seq, pending_status, pending_run_id
		FROM chat_summary_state WHERE chat_id = ?`, job.ChatID,
	).Scan(&cutoff, &dirtyFrom, &pendingTo, &pendingStatus, &pendingRunID); err != nil {
		return err
	}
	if cutoff != job.BaseCutoffSeq || pendingTo != job.ToSeq ||
		pendingStatus != "running" || pendingRunID != job.RunID ||
		(dirtyFrom > 0 && dirtyFrom <= job.ToSeq) {
		return ErrSummaryStateChanged
	}

	summaryID := uuid.New().String()
	if _, err := tx.Exec(`
		UPDATE chat_summary_chunks
		SET status = 'superseded', updated_at = ?
		WHERE chat_id = ? AND level = 'big' AND status = 'active'`, now, job.ChatID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE chat_summary_chunks
		SET status = 'merged', merged_into_id = ?, updated_at = ?
		WHERE chat_id = ? AND level = 'small' AND status = 'active'`, summaryID, now, job.ChatID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO chat_summary_chunks
			(id, chat_id, level, from_seq, to_seq, to_message_id, content, status, merged_into_id, created_at, updated_at)
		VALUES (?, ?, 'big', 1, ?, ?, ?, 'active', '', ?, ?)`,
		summaryID, job.ChatID, job.ToSeq, job.TargetMessageID, summaryContent, now, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE chat_summary_state
		SET applied_cutoff_seq = ?, current_big_summary_id = ?, dirty_from_seq = 0,
		    pending_to_seq = 0, pending_status = '', pending_run_id = '',
		    pending_attempts = 0, pending_error = '', pending_started_at = NULL,
		    summary_required = ?,
		    eligibility_seq = MAX(eligibility_seq, ?),
		    updated_at = ?
		WHERE chat_id = ?`, job.ToSeq, summaryID, summaryRequired, latestMessageSeq, now, job.ChatID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SummaryStore) FailPendingSummary(job *PendingSummaryJob, summaryErr error) error {
	if job == nil {
		return nil
	}
	errorText := "摘要生成失败"
	if summaryErr != nil {
		errorText = summaryErr.Error()
	}
	latestAssistantSeq := job.ToSeq
	if err := s.messageDB.QueryRow(`
		SELECT COALESCE(MAX(seq), ?) FROM messages WHERE chat_id = ? AND role = 'assistant'`,
		job.ToSeq, job.ChatID,
	).Scan(&latestAssistantSeq); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET pending_to_seq = MAX(pending_to_seq, ?),
		    pending_status = 'failed', pending_run_id = '', pending_error = ?,
		    pending_started_at = NULL, updated_at = ?
		WHERE chat_id = ? AND pending_status = 'running' AND pending_run_id = ?`,
		latestAssistantSeq, errorText, time.Now(), job.ChatID, job.RunID,
	)
	return err
}

func (s *SummaryStore) ResetPendingSummaryFromSeq(chatID string, fromSeq int) error {
	if fromSeq <= 0 {
		return nil
	}
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET pending_to_seq = 0, pending_status = '', pending_run_id = '',
		    pending_attempts = 0, pending_error = '', pending_started_at = NULL, updated_at = ?
		WHERE chat_id = ? AND pending_to_seq >= ?`, time.Now(), chatID, fromSeq)
	return err
}

func (s *SummaryStore) RecoverInterruptedSummaryJobs() error {
	_, err := s.db.Exec(`
		UPDATE chat_summary_state
		SET pending_status = 'failed', pending_run_id = '',
		    pending_error = '服务重启，等待下次消息重试', pending_started_at = NULL, updated_at = ?
		WHERE pending_status = 'running'`, time.Now())
	return err
}

// DeleteMessageAndRecalculate invalidates summary data before deleting from the
// main database. Across separate SQLite files this ordering guarantees that a
// running summary either commits first and is removed, or fails state validation.
func (s *SummaryStore) DeleteMessageAndRecalculate(chatID, messageID string, cascade bool, charLimit int) (int64, error) {
	if charLimit <= 0 {
		charLimit = 3000
	}
	var fromSeq int
	if err := s.messageDB.QueryRow(`
		SELECT seq FROM messages WHERE id = ? AND chat_id = ?`, messageID, chatID,
	).Scan(&fromSeq); err != nil {
		return 0, err
	}
	resetNextFloor, err := s.invalidateSummaryCoverage(chatID, fromSeq)
	if err != nil {
		return 0, err
	}

	var result sql.Result
	if cascade {
		result, err = s.messageDB.Exec(`DELETE FROM messages WHERE chat_id = ? AND seq >= ?`, chatID, fromSeq)
	} else {
		result, err = s.messageDB.Exec(`DELETE FROM messages WHERE chat_id = ? AND id = ?`, chatID, messageID)
	}
	if err != nil {
		_ = s.recalculateAfterMutation(chatID, charLimit, resetNextFloor)
		return 0, err
	}
	deletedMessages, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := s.recalculateAfterMutation(chatID, charLimit, resetNextFloor); err != nil {
		return 0, err
	}
	return deletedMessages, nil
}

func (s *SummaryStore) InvalidateSummariesFromSeq(chatID string, fromSeq, charLimit int) error {
	if fromSeq <= 0 {
		return nil
	}
	if charLimit <= 0 {
		charLimit = 3000
	}
	resetNextFloor, err := s.invalidateSummaryCoverage(chatID, fromSeq)
	if err != nil {
		return err
	}
	return s.recalculateAfterMutation(chatID, charLimit, resetNextFloor)
}

func (s *SummaryStore) invalidateSummaryCoverage(chatID string, fromSeq int) (bool, error) {
	if err := s.EnsureState(chatID); err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`DELETE FROM chat_summary_chunks WHERE chat_id = ? AND to_seq >= ?`, chatID, fromSeq)
	if err != nil {
		return false, err
	}
	deletedSummaries, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if _, err := tx.Exec(`
		UPDATE chat_summary_chunks SET status = 'superseded', updated_at = ?
		WHERE chat_id = ? AND level = 'big' AND status IN ('active', 'superseded')`, time.Now(), chatID); err != nil {
		return false, err
	}

	currentSummaryID := ""
	cutoffSeq := 0
	err = tx.QueryRow(`
		SELECT id, to_seq FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'big' AND from_seq <= 1 AND status = 'superseded'
		ORDER BY to_seq DESC LIMIT 1`, chatID,
	).Scan(&currentSummaryID, &cutoffSeq)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if currentSummaryID != "" {
		if _, err := tx.Exec(`
			UPDATE chat_summary_chunks SET status = 'active', updated_at = ? WHERE id = ?`,
			time.Now(), currentSummaryID); err != nil {
			return false, err
		}
	}

	var pendingTo int
	if err := tx.QueryRow(`
		SELECT pending_to_seq FROM chat_summary_state WHERE chat_id = ?`, chatID,
	).Scan(&pendingTo); err != nil {
		return false, err
	}

	pendingInvalidated := pendingTo >= fromSeq
	resetNextFloor := deletedSummaries > 0 || pendingInvalidated
	if _, err := tx.Exec(`
		UPDATE chat_summary_state
		SET applied_cutoff_seq = ?, current_big_summary_id = ?, dirty_from_seq = 0,
		    pending_to_seq = CASE WHEN ? THEN 0 ELSE pending_to_seq END,
		    pending_status = CASE WHEN ? THEN '' ELSE pending_status END,
		    pending_run_id = CASE WHEN ? THEN '' ELSE pending_run_id END,
		    pending_attempts = CASE WHEN ? THEN 0 ELSE pending_attempts END,
		    pending_error = CASE WHEN ? THEN '' ELSE pending_error END,
		    pending_started_at = CASE WHEN ? THEN NULL ELSE pending_started_at END,
		    summary_required = 1,
		    next_summary_floor = CASE WHEN ? THEN 0 ELSE next_summary_floor END,
		    eligibility_seq = 0,
		    updated_at = ?
		WHERE chat_id = ?`,
		cutoffSeq, currentSummaryID,
		pendingInvalidated, pendingInvalidated, pendingInvalidated,
		pendingInvalidated, pendingInvalidated, pendingInvalidated,
		resetNextFloor, time.Now(), chatID,
	); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return resetNextFloor, nil
}

func (s *SummaryStore) recalculateAfterMutation(chatID string, charLimit int, resetNextFloor bool) error {
	state, err := s.GetState(chatID)
	if err != nil {
		return err
	}
	var unsummarizedChars, currentFloor, latestSeq int
	if err := s.messageDB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN seq > ? THEN LENGTH(content) ELSE 0 END), 0),
		       COUNT(CASE WHEN role = 'assistant' THEN 1 END),
		       COALESCE(MAX(seq), 0)
		FROM messages WHERE chat_id = ?`, state.AppliedCutoffSeq, chatID,
	).Scan(&unsummarizedChars, &currentFloor, &latestSeq); err != nil {
		return err
	}
	summaryRequired := unsummarizedChars >= charLimit
	_, err = s.db.Exec(`
		UPDATE chat_summary_state
		SET summary_required = ?,
		    next_summary_floor = CASE WHEN ? THEN ? ELSE next_summary_floor END,
		    eligibility_seq = ?, updated_at = ?
		WHERE chat_id = ?`,
		summaryRequired, resetNextFloor, currentFloor, latestSeq, time.Now(), chatID,
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
			(id, chat_id, level, from_seq, to_seq, to_message_id, content, status, merged_into_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		chunk.ID, chunk.ChatID, chunk.Level, chunk.FromSeq, chunk.ToSeq, chunk.ToMessageID, chunk.Content,
		chunk.Status, chunk.MergedIntoID, chunk.CreatedAt, chunk.UpdatedAt,
	)
	return err
}

func (s *SummaryStore) GetActiveBigChunk(chatID string) (*model.ChatSummaryChunk, error) {
	row := s.db.QueryRow(`
		SELECT id, chat_id, level, from_seq, to_seq, to_message_id, content, status, merged_into_id, created_at, updated_at
		FROM chat_summary_chunks
		WHERE chat_id = ? AND level = 'big' AND status = 'active'
		ORDER BY to_seq DESC LIMIT 1`, chatID)
	return scanSummaryChunk(row)
}

func (s *SummaryStore) ListActiveSmallChunks(chatID string) ([]*model.ChatSummaryChunk, error) {
	rows, err := s.db.Query(`
		SELECT id, chat_id, level, from_seq, to_seq, to_message_id, content, status, merged_into_id, created_at, updated_at
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
		SELECT id, chat_id, level, from_seq, to_seq, to_message_id, content, status, merged_into_id, created_at, updated_at
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
		SELECT id, chat_id, level, from_seq, to_seq, to_message_id, content, status, merged_into_id, created_at, updated_at
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
		&chunk.ToMessageID,
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
