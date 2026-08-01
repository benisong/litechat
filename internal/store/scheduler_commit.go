package store

import (
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

// CommitSchedulerTurn 将一轮调度的状态、事件和记录结果原子提交。
func (s *SchedulerStore) CommitSchedulerTurn(
	recordID string,
	state *model.ChatStoryState,
	expectedVersion int,
	parsedOutput string,
	appliedChanges string,
	contextText string,
	events []model.ChatStoryEvent,
) error {
	if state == nil {
		return fmt.Errorf("story state is nil")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	nextVersion := expectedVersion + 1
	result, err := tx.Exec(`
		UPDATE chat_story_states
		SET state_version = ?, state_json = ?, current_scene = ?, active_event = ?,
		    route = ?, scheduler_status = ?, last_success_record_id = ?,
		    failure_count = ?, updated_at = ?
		WHERE chat_id = ? AND state_version = ?`,
		nextVersion, state.StateJSON, state.CurrentScene, state.ActiveEvent,
		state.Route, "ready", recordID, 0, now, state.ChatID, expectedVersion)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("story state version conflict for chat %s", state.ChatID)
	}

	for _, event := range events {
		if event.ChatID == "" {
			event.ChatID = state.ChatID
		}
		if event.SchedulerRecordID == "" {
			event.SchedulerRecordID = recordID
		}
		if event.EventKey == "" {
			return fmt.Errorf("story event key is empty")
		}
		if event.Status == "" {
			event.Status = "applied"
		}
		if event.Importance == "" {
			event.Importance = "normal"
		}
		if event.ID == "" {
			event.ID = uuid.New().String()
		}
		if event.CreatedAt.IsZero() {
			event.CreatedAt = now
		}
		if _, err := tx.Exec(`
			INSERT INTO chat_story_events (
				id, chat_id, scheduler_record_id, event_key, event_type,
				summary, importance, evidence, status, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			event.ID, event.ChatID, event.SchedulerRecordID, event.EventKey,
			event.EventType, event.Summary, event.Importance, event.Evidence,
			event.Status, event.CreatedAt,
		); err != nil {
			return err
		}
	}

	result, err = tx.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, parsed_output = ?, applied_changes = ?, context_text = ?,
		    state_version_before = ?, state_version_after = ?,
		    finished_at = ?, applied_at = ?, error_code = '', error_message = ''
		WHERE id = ? AND status = ?`,
		model.SchedulerStatusSuccess, parsedOutput, appliedChanges, contextText,
		expectedVersion, nextVersion, now, now, recordID,
		model.SchedulerStatusProcessing)
	if err != nil {
		return err
	}
	rows, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("scheduler record is not processing: %s", recordID)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	state.StateVersion = nextVersion
	state.SchedulerStatus = "ready"
	state.LastSuccessRecordID = recordID
	state.FailureCount = 0
	state.UpdatedAt = now
	return nil
}
