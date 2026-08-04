package store

import (
	"database/sql"
	"errors"
	"fmt"
	"litechat/internal/model"
	"time"

	"github.com/google/uuid"
)

var ErrStoryStateConflict = errors.New("story state version conflict")

type SchedulerStore struct {
	db *DB
}

func NewSchedulerStore(db *DB) *SchedulerStore {
	return &SchedulerStore{db: db}
}

func (s *SchedulerStore) CreateManifest(manifest *model.StoryManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if manifest.CharacterID == "" {
		return fmt.Errorf("character_id is required")
	}
	manifest.ID = uuid.New().String()
	if manifest.ManifestVersion == 0 {
		manifest.ManifestVersion = 1
	}
	manifest.Status = model.ManifestStatusPending
	manifest.CreatedAt = time.Now()
	manifest.UpdatedAt = manifest.CreatedAt

	_, err := s.db.Exec(`
		INSERT INTO story_manifests (
			id, character_id, character_version, worldbook_version_hash,
			manifest_version, status, compiled_json, compiler_model,
			prompt_version, error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		manifest.ID, manifest.CharacterID, manifest.CharacterVersion,
		manifest.WorldbookVersionHash, manifest.ManifestVersion, manifest.Status,
		manifest.CompiledJSON, manifest.CompilerModel, manifest.PromptVersion,
		manifest.ErrorMessage, manifest.CreatedAt, manifest.UpdatedAt,
	)
	return err
}

func (s *SchedulerStore) GetManifest(id string) (*model.StoryManifest, error) {
	manifest := &model.StoryManifest{}
	var status string
	err := s.db.QueryRow(`
		SELECT id, character_id, character_version, worldbook_version_hash,
		       manifest_version, status, compiled_json, compiler_model,
		       prompt_version, error_message, created_at, updated_at
		FROM story_manifests WHERE id = ?`, id).Scan(
		&manifest.ID, &manifest.CharacterID, &manifest.CharacterVersion,
		&manifest.WorldbookVersionHash, &manifest.ManifestVersion, &status,
		&manifest.CompiledJSON, &manifest.CompilerModel, &manifest.PromptVersion,
		&manifest.ErrorMessage, &manifest.CreatedAt, &manifest.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	manifest.Status = model.ManifestStatus(status)
	return manifest, nil
}

func (s *SchedulerStore) GetReadyManifest(characterID, characterVersion, worldbookVersionHash string, compilerCriteria ...string) (*model.StoryManifest, error) {
	manifest := &model.StoryManifest{}
	query := `SELECT id, character_id, character_version, worldbook_version_hash, manifest_version,
		status, compiled_json, compiler_model, prompt_version, error_message, created_at, updated_at
		FROM story_manifests WHERE character_id = ? AND character_version = ? AND worldbook_version_hash = ? AND status = ?`
	args := []any{characterID, characterVersion, worldbookVersionHash, model.ManifestStatusReady}
	if len(compilerCriteria) > 0 && compilerCriteria[0] != "" {
		query += " AND compiler_model = ?"
		args = append(args, compilerCriteria[0])
	}
	if len(compilerCriteria) > 1 && compilerCriteria[1] != "" {
		query += " AND prompt_version = ?"
		args = append(args, compilerCriteria[1])
	}
	if len(compilerCriteria) > 2 && compilerCriteria[2] != "" {
		query += " AND manifest_version = ?"
		args = append(args, compilerCriteria[2])
	}
	query += " ORDER BY updated_at DESC LIMIT 1"
	err := s.db.QueryRow(query, args...).Scan(&manifest.ID, &manifest.CharacterID, &manifest.CharacterVersion, &manifest.WorldbookVersionHash,
		&manifest.ManifestVersion, &manifest.Status, &manifest.CompiledJSON, &manifest.CompilerModel,
		&manifest.PromptVersion, &manifest.ErrorMessage, &manifest.CreatedAt, &manifest.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func (s *SchedulerStore) MarkManifestReady(id, compiledJSON, promptVersion, compilerModel string) error {
	result, err := s.db.Exec(`
		UPDATE story_manifests
		SET status = ?, compiled_json = ?, prompt_version = ?, compiler_model = ?,
		    error_message = '', updated_at = ?
		WHERE id = ? AND status IN (?, ?)
	`, model.ManifestStatusReady, compiledJSON, promptVersion, compilerModel,
		time.Now(), id, model.ManifestStatusPending, model.ManifestStatusProcessing)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("manifest is not pending or processing: %s", id)
	}
	return nil
}

func (s *SchedulerStore) MarkManifestFailed(id, errorMessage string) error {
	result, err := s.db.Exec(`
		UPDATE story_manifests
		SET status = ?, error_message = ?, updated_at = ?
		WHERE id = ? AND status IN (?, ?)
	`, model.ManifestStatusFailed, errorMessage, time.Now(), id,
		model.ManifestStatusPending, model.ManifestStatusProcessing)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("manifest is not pending or processing: %s", id)
	}
	return nil
}

func (s *SchedulerStore) CreateStoryState(state *model.ChatStoryState) error {
	if state == nil {
		return fmt.Errorf("story state is nil")
	}
	if state.ChatID == "" || state.ManifestID == "" {
		return fmt.Errorf("chat_id and manifest_id are required")
	}
	if state.StateJSON == "" {
		state.StateJSON = "{}"
	}
	if state.SchedulerStatus == "" {
		state.SchedulerStatus = "ready"
	}
	state.CreatedAt = time.Now()
	state.UpdatedAt = state.CreatedAt

	_, err := s.db.Exec(`
		INSERT INTO chat_story_states (
			chat_id, manifest_id, state_version, state_json, current_scene,
			active_event, route, scheduler_status, last_success_record_id,
			failure_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		state.ChatID, state.ManifestID, state.StateVersion, state.StateJSON,
		state.CurrentScene, state.ActiveEvent, state.Route, state.SchedulerStatus,
		state.LastSuccessRecordID, state.FailureCount, state.CreatedAt, state.UpdatedAt,
	)
	return err
}

func (s *SchedulerStore) GetStoryState(chatID string) (*model.ChatStoryState, error) {
	state := &model.ChatStoryState{}
	err := s.db.QueryRow(`
		SELECT chat_id, manifest_id, state_version, state_json, current_scene,
		       active_event, route, scheduler_status, last_success_record_id,
		       failure_count, created_at, updated_at
		FROM chat_story_states WHERE chat_id = ?`, chatID).Scan(
		&state.ChatID, &state.ManifestID, &state.StateVersion, &state.StateJSON,
		&state.CurrentScene, &state.ActiveEvent, &state.Route, &state.SchedulerStatus,
		&state.LastSuccessRecordID, &state.FailureCount, &state.CreatedAt, &state.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// UpdateStoryState 使用 expectedVersion 做乐观锁，防止乱序调度覆盖新状态。
func (s *SchedulerStore) UpdateStoryState(state *model.ChatStoryState, expectedVersion int) error {
	if state == nil {
		return fmt.Errorf("story state is nil")
	}
	nextVersion := expectedVersion + 1
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE chat_story_states
		SET state_version = ?, state_json = ?, current_scene = ?, active_event = ?,
		    route = ?, scheduler_status = ?, last_success_record_id = ?,
		    failure_count = ?, updated_at = ?
		WHERE chat_id = ? AND state_version = ?`,
		nextVersion, state.StateJSON, state.CurrentScene, state.ActiveEvent,
		state.Route, state.SchedulerStatus, state.LastSuccessRecordID,
		state.FailureCount, now, state.ChatID, expectedVersion)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w for chat %s", ErrStoryStateConflict, state.ChatID)
	}
	state.StateVersion = nextVersion
	state.UpdatedAt = now
	return nil
}

func (s *SchedulerStore) CreateRecord(record *model.ChatSchedulerRecord) error {
	if record == nil {
		return fmt.Errorf("scheduler record is nil")
	}
	if record.ChatID == "" || record.AssistantMessageID == "" {
		return fmt.Errorf("chat_id and assistant_message_id are required")
	}

	record.ID = uuid.New().String()
	record.Status = model.SchedulerStatusPending
	record.AttemptCount = 0
	record.CreatedAt = time.Now()

	_, err := s.db.Exec(`
		INSERT INTO chat_scheduler_records (
			id, chat_id, user_message_id, assistant_message_id, turn_seq,
			status, attempt_count, scheduler_model, prompt_version,
			input_snapshot, raw_output, parsed_output, applied_changes,
			context_text, state_version_before, state_version_after,
			error_code, error_message, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.ChatID,
		record.UserMessageID,
		record.AssistantMessageID,
		record.TurnSeq,
		record.Status,
		record.AttemptCount,
		record.SchedulerModel,
		record.PromptVersion,
		record.InputSnapshot,
		record.RawOutput,
		record.ParsedOutput,
		record.AppliedChanges,
		record.ContextText,
		record.StateVersionBefore,
		record.StateVersionAfter,
		record.ErrorCode,
		record.ErrorMessage,
		record.CreatedAt,
	)
	return err
}

func (s *SchedulerStore) DeleteRecord(id string) error {
	if id == "" {
		return nil
	}
	if _, err := s.db.Exec(`DELETE FROM chat_story_events WHERE scheduler_record_id = ?`, id); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM chat_scheduler_records WHERE id = ?`, id)
	return err
}

func (s *SchedulerStore) GetRecord(id string) (*model.ChatSchedulerRecord, error) {
	return s.scanRecord(s.db.QueryRow(`
		SELECT id, chat_id, user_message_id, assistant_message_id, turn_seq,
		       status, attempt_count, scheduler_model, prompt_version,
		       input_snapshot, raw_output, parsed_output, applied_changes,
		       context_text, state_version_before, state_version_after,
		       error_code, error_message, created_at, started_at, finished_at, applied_at
		FROM chat_scheduler_records WHERE id = ?`, id))
}

func (s *SchedulerStore) LatestRetryableRecord(chatID string) (*model.ChatSchedulerRecord, error) {
	return s.scanRecord(s.db.QueryRow(`
		SELECT id, chat_id, user_message_id, assistant_message_id, turn_seq,
		       status, attempt_count, scheduler_model, prompt_version,
		       input_snapshot, raw_output, parsed_output, applied_changes,
		       context_text, state_version_before, state_version_after,
		       error_code, error_message, created_at, started_at, finished_at, applied_at
		FROM chat_scheduler_records
		WHERE chat_id = ? AND status IN (?, ?)
		ORDER BY turn_seq DESC, created_at DESC LIMIT 1`, chatID, model.SchedulerStatusFailed, model.SchedulerStatusConflict))
}
func (s *SchedulerStore) LatestFailedRecord(chatID string) (*model.ChatSchedulerRecord, error) {
	return s.scanRecord(s.db.QueryRow(`
		SELECT id, chat_id, user_message_id, assistant_message_id, turn_seq,
		       status, attempt_count, scheduler_model, prompt_version,
		       input_snapshot, raw_output, parsed_output, applied_changes,
		       context_text, state_version_before, state_version_after,
		       error_code, error_message, created_at, started_at, finished_at, applied_at
		FROM chat_scheduler_records
		WHERE chat_id = ? AND status = ?
		ORDER BY turn_seq DESC, created_at DESC LIMIT 1`, chatID, model.SchedulerStatusFailed))
}

func (s *SchedulerStore) LatestSuccessfulRecord(chatID string) (*model.ChatSchedulerRecord, error) {
	return s.scanRecord(s.db.QueryRow(`
		SELECT id, chat_id, user_message_id, assistant_message_id, turn_seq,
		       status, attempt_count, scheduler_model, prompt_version,
		       input_snapshot, raw_output, parsed_output, applied_changes,
		       context_text, state_version_before, state_version_after,
		       error_code, error_message, created_at, started_at, finished_at, applied_at
		FROM chat_scheduler_records
		WHERE chat_id = ? AND status = ?
		ORDER BY turn_seq DESC, created_at DESC
		LIMIT 1`, chatID, model.SchedulerStatusSuccess))
}

func (s *SchedulerStore) UpdateRawOutput(id, rawOutput, schedulerModel, promptVersion string) error {
	result, err := s.db.Exec(`
		UPDATE chat_scheduler_records
		SET raw_output = ?, scheduler_model = ?, prompt_version = ?
		WHERE id = ? AND status = ?
	`, rawOutput, schedulerModel, promptVersion, id, model.SchedulerStatusProcessing)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("scheduler record is not processing: %s", id)
	}
	return nil
}

func (s *SchedulerStore) MarkProcessing(id string) error {
	result, err := s.db.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, attempt_count = attempt_count + 1, started_at = ?,
		    finished_at = NULL, error_code = '', error_message = ?
		WHERE id = ? AND status IN (?, ?)
	`, model.SchedulerStatusProcessing, time.Now(), "", id,
		model.SchedulerStatusPending, model.SchedulerStatusFailed)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("scheduler record is not pending or failed: %s", id)
	}
	return nil
}

func (s *SchedulerStore) MarkSuccess(id, parsedOutput, contextText string, stateVersionBefore, stateVersionAfter int) error {
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, parsed_output = ?, context_text = ?,
		    state_version_before = ?, state_version_after = ?,
		    finished_at = ?, applied_at = ?, error_code = '', error_message = ''
		WHERE id = ? AND status = ?
	`, model.SchedulerStatusSuccess, parsedOutput, contextText,
		stateVersionBefore, stateVersionAfter, now, now, id,
		model.SchedulerStatusProcessing)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("scheduler record is not processing: %s", id)
	}
	return nil
}

func (s *SchedulerStore) MarkFailed(id, errorCode, errorMessage string) error {
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE chat_scheduler_records
		SET status = ?, error_code = ?, error_message = ?, finished_at = ?
		WHERE id = ? AND status = ?
	`, model.SchedulerStatusFailed, errorCode, errorMessage, now, id,
		model.SchedulerStatusProcessing)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("scheduler record is not processing: %s", id)
	}
	return nil
}

func (s *SchedulerStore) scanRecord(row *sql.Row) (*model.ChatSchedulerRecord, error) {
	record := &model.ChatSchedulerRecord{}
	var status string
	var startedAt, finishedAt, appliedAt sql.NullTime
	if err := row.Scan(
		&record.ID,
		&record.ChatID,
		&record.UserMessageID,
		&record.AssistantMessageID,
		&record.TurnSeq,
		&status,
		&record.AttemptCount,
		&record.SchedulerModel,
		&record.PromptVersion,
		&record.InputSnapshot,
		&record.RawOutput,
		&record.ParsedOutput,
		&record.AppliedChanges,
		&record.ContextText,
		&record.StateVersionBefore,
		&record.StateVersionAfter,
		&record.ErrorCode,
		&record.ErrorMessage,
		&record.CreatedAt,
		&startedAt,
		&finishedAt,
		&appliedAt,
	); err != nil {
		return nil, err
	}
	record.Status = model.SchedulerStatus(status)
	if startedAt.Valid {
		record.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		record.FinishedAt = &finishedAt.Time
	}
	if appliedAt.Valid {
		record.AppliedAt = &appliedAt.Time
	}
	return record, nil
}
