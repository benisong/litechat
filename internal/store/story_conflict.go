package store

import (
	"fmt"
	"litechat/internal/model"
	"time"
)

func (s *SchedulerStore) MarkStoryTurnConflict(chatID, recordID, message string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("scheduler store is not configured")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	if _, err := tx.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, error_code = ?, error_message = ?, finished_at = ?
		WHERE id = ? AND chat_id = ? AND status != ?`,
		model.SchedulerStatusConflict, "state_conflict", message, now, recordID, chatID, model.SchedulerStatusSuccess); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE chat_story_states SET scheduler_status = ?, updated_at = ? WHERE chat_id = ?`, model.SchedulerStatusConflict, now, chatID); err != nil {
		return err
	}
	return tx.Commit()
}
