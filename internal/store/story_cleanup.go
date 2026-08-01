package store

import "fmt"

// DeleteChatStoryData 删除单个聊天专属的剧情调度数据。
// story_manifests 按角色卡/世界书版本共享，不在这里删除。
func (s *SchedulerStore) DeleteChatStoryData(chatID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("scheduler store is not configured")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		"DELETE FROM chat_story_events WHERE chat_id = ?",
		"DELETE FROM chat_scheduler_records WHERE chat_id = ?",
		"DELETE FROM chat_story_states WHERE chat_id = ?",
	} {
		if _, err := tx.Exec(statement, chatID); err != nil {
			return fmt.Errorf("delete story data for chat %s: %w", chatID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit story data deletion for chat %s: %w", chatID, err)
	}
	return nil
}
