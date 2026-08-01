package service

import (
	"testing"
)

func TestParseSchedulerOutputAcceptsPlainJSON(t *testing.T) {
	got, err := ParseSchedulerOutput(`{"schema_version":1,"observations":[]}`)
	if err != nil {
		t.Fatalf("ParseSchedulerOutput: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", got.SchemaVersion)
	}
}

func TestParseSchedulerOutputAcceptsFencedJSON(t *testing.T) {
	got, err := ParseSchedulerOutput("```json\n{\"schema_version\":1,\"observations\":[]}\n```")
	if err != nil {
		t.Fatalf("ParseSchedulerOutput: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", got.SchemaVersion)
	}
}

func TestParseSchedulerOutputExtractsJSONFromExplanation(t *testing.T) {
	got, err := ParseSchedulerOutput("本轮没有重大变化。\n{\"schema_version\":1,\"observations\":[]}")
	if err != nil {
		t.Fatalf("ParseSchedulerOutput: %v", err)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", got.SchemaVersion)
	}
}

func TestParseSchedulerOutputRejectsMalformedJSON(t *testing.T) {
	if _, err := ParseSchedulerOutput(`{"schema_version":1,"observations":[}`); err == nil {
		t.Fatal("expected malformed JSON to fail")
	}
}
