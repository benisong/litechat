package service

import (
	"fmt"
	"litechat/internal/model"
	"strings"
)

const (
	maxSchedulerObservations = 100
	maxSchedulerEvents       = 50
	maxSchedulerEvidenceLen  = 4000
)

// SchedulerValidationSpec 是由已校验的 Manifest 生成的运行时白名单。
type SchedulerValidationSpec struct {
	AllowedObservationKeys map[string]bool
	AllowedEventIDs        map[string]bool
}

// ValidateSchedulerOutput 只验证调度模型提出的候选，不执行任何状态变更。
func ValidateSchedulerOutput(output *model.SchedulerOutput, spec SchedulerValidationSpec) error {
	if output == nil {
		return fmt.Errorf("scheduler output is nil")
	}
	if output.SchemaVersion != schedulerOutputSchemaVersion {
		return fmt.Errorf("unsupported scheduler schema version: %d", output.SchemaVersion)
	}
	if len(output.Observations) > maxSchedulerObservations {
		return fmt.Errorf("too many scheduler observations: %d", len(output.Observations))
	}
	if len(output.EventCandidates) > maxSchedulerEvents {
		return fmt.Errorf("too many scheduler event candidates: %d", len(output.EventCandidates))
	}

	for i, observation := range output.Observations {
		if strings.TrimSpace(observation.Key) == "" {
			return fmt.Errorf("observation %d has empty key", i)
		}
		if !spec.AllowedObservationKeys[observation.Key] {
			return fmt.Errorf("observation %d uses undeclared key: %s", i, observation.Key)
		}
		if strings.TrimSpace(observation.Evidence) == "" {
			return fmt.Errorf("observation %s has no evidence", observation.Key)
		}
		if len([]rune(observation.Evidence)) > maxSchedulerEvidenceLen {
			return fmt.Errorf("observation %s evidence is too long", observation.Key)
		}
		if observation.Confidence < 0 || observation.Confidence > 1 {
			return fmt.Errorf("observation %s has invalid confidence: %v", observation.Key, observation.Confidence)
		}
	}

	for i, candidate := range output.EventCandidates {
		if strings.TrimSpace(candidate.EventID) == "" {
			return fmt.Errorf("event candidate %d has empty event_id", i)
		}
		if !spec.AllowedEventIDs[candidate.EventID] {
			return fmt.Errorf("event candidate %d uses undeclared event: %s", i, candidate.EventID)
		}
		if strings.TrimSpace(candidate.Evidence) == "" {
			return fmt.Errorf("event candidate %s has no evidence", candidate.EventID)
		}
		if len([]rune(candidate.Evidence)) > maxSchedulerEvidenceLen {
			return fmt.Errorf("event candidate %s evidence is too long", candidate.EventID)
		}
	}
	return nil
}
