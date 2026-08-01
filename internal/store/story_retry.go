package store

import (
	"fmt"
	"litechat/internal/model"
	"time"
)

func (s *SchedulerStore) MarkStoryTurnProcessing(chatID, recordID string) error {
	if err := s.StartRecordRetry(recordID); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE chat_story_states SET scheduler_status = ?, updated_at = ? WHERE chat_id = ?`, model.SchedulerStatusProcessing, time.Now(), chatID)
	return err
}

func (s *SchedulerStore) StartRecordRetry(recordID string) error {
	result, err := s.db.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, attempt_count = attempt_count + 1, started_at = ?,
			finished_at = NULL, applied_at = NULL, error_code = '', error_message = ''
		WHERE id = ? AND status = ?`, model.SchedulerStatusProcessing, time.Now(), recordID, model.SchedulerStatusFailed)
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
