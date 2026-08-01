package store

import (
	"fmt"
	"litechat/internal/model"
	"time"
)

func (s *SchedulerStore) MarkStoryTurnProcessing(chatID, recordID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("scheduler store is not configured")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	result, err := tx.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, attempt_count = attempt_count + 1, started_at = ?,
			finished_at = NULL, applied_at = NULL, error_code = '', error_message = ''
		WHERE id = ? AND status IN (?, ?)`, model.SchedulerStatusProcessing, now, recordID, model.SchedulerStatusFailed, model.SchedulerStatusConflict)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("scheduler record %s is not retryable", recordID)
	}
	if _, err := tx.Exec(`UPDATE chat_story_states SET scheduler_status = ?, updated_at = ? WHERE chat_id = ?`, model.SchedulerStatusProcessing, now, chatID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SchedulerStore) StartRecordRetry(recordID string) error {
	result, err := s.db.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, attempt_count = attempt_count + 1, started_at = ?,
			finished_at = NULL, applied_at = NULL, error_code = '', error_message = ''
		WHERE id = ? AND status IN (?, ?)`, model.SchedulerStatusProcessing, time.Now(), recordID, model.SchedulerStatusFailed, model.SchedulerStatusConflict)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("scheduler record %s is not retryable", recordID)
	}
	return nil
}
