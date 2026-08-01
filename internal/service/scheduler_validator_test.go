package service

import (
	"litechat/internal/model"
	"testing"
)

func TestValidateSchedulerOutputAcceptsDeclaredObservationAndEvent(t *testing.T) {
	output := &model.SchedulerOutput{
		SchemaVersion: 1,
		Observations: []model.SchedulerObservation{
			{Key: "relationships.liu_ruyan.disappointment", Value: true, Evidence: "柳如烟公开要求让出资源", Confidence: 0.9},
		},
		EventCandidates: []model.SchedulerEventCandidate{
			{EventID: "resource_dispute_001", Reason: "前置事件出现", Evidence: "现场发生资源争议"},
		},
	}
	spec := SchedulerValidationSpec{
		AllowedObservationKeys: map[string]bool{"relationships.liu_ruyan.disappointment": true},
		AllowedEventIDs:        map[string]bool{"resource_dispute_001": true},
	}
	if err := ValidateSchedulerOutput(output, spec); err != nil {
		t.Fatalf("ValidateSchedulerOutput: %v", err)
	}
}

func TestValidateSchedulerOutputRejectsUnknownObservationKey(t *testing.T) {
	output := &model.SchedulerOutput{
		SchemaVersion: 1,
		Observations: []model.SchedulerObservation{
			{Key: "core_identity.knows_final_ending", Value: true, Evidence: "模型猜测", Confidence: 0.9},
		},
	}
	spec := SchedulerValidationSpec{AllowedObservationKeys: map[string]bool{}}
	if err := ValidateSchedulerOutput(output, spec); err == nil {
		t.Fatal("expected unknown observation key to fail")
	}
}

func TestValidateSchedulerOutputRejectsObservationWithoutEvidence(t *testing.T) {
	output := &model.SchedulerOutput{
		SchemaVersion: 1,
		Observations: []model.SchedulerObservation{
			{Key: "facts.resource_request", Value: true, Confidence: 0.9},
		},
	}
	spec := SchedulerValidationSpec{AllowedObservationKeys: map[string]bool{"facts.resource_request": true}}
	if err := ValidateSchedulerOutput(output, spec); err == nil {
		t.Fatal("expected missing evidence to fail")
	}
}

func TestValidateSchedulerOutputRejectsInvalidConfidenceAndEvent(t *testing.T) {
	output := &model.SchedulerOutput{
		SchemaVersion: 1,
		Observations: []model.SchedulerObservation{
			{Key: "facts.resource_request", Value: true, Evidence: "证据", Confidence: 1.5},
		},
		EventCandidates: []model.SchedulerEventCandidate{{EventID: "unknown", Evidence: "证据"}},
	}
	spec := SchedulerValidationSpec{
		AllowedObservationKeys: map[string]bool{"facts.resource_request": true},
		AllowedEventIDs:        map[string]bool{"known": true},
	}
	if err := ValidateSchedulerOutput(output, spec); err == nil {
		t.Fatal("expected invalid confidence/event to fail")
	}
}
