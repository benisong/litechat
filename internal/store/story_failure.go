package store

import (
	"fmt"
	"litechat/internal/model"
	"time"
)

const storyFailurePauseThreshold = 3

// MarkStoryTurnFailed 原子记录调度失败，并递增聊天连续失败计数。
// 不修改 state_json/state_version，因此上一份成功状态保持不变。
func (s *SchedulerStore) MarkStoryTurnFailed(chatID, recordID, code, message string) error {
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
		model.SchedulerStatusFailed, code, message, now, recordID, chatID, model.SchedulerStatusSuccess); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE chat_story_states
		SET scheduler_status = CASE WHEN failure_count + 1 >= ? THEN ? ELSE ? END,
			failure_count = failure_count + 1, updated_at = ?
		WHERE chat_id = ?`, storyFailurePauseThreshold, model.SchedulerStatusPaused, model.SchedulerStatusFailed, now, chatID); err != nil {
		return err
	}
	return tx.Commit()
}
