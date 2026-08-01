package service

import (
	"context"
	"errors"
	"litechat/internal/model"
	"litechat/internal/store"
	"testing"
)

func newServiceSchedulerTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.NewDB(t.TempDir())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	return db
}

type fakeSchedulerCompletionClient struct {
	response string
	err      error
}

func (c fakeSchedulerCompletionClient) Complete(_ context.Context, _ string, _ []model.ChatCompletionMessage) (string, error) {
	return c.response, c.err
}

func TestSchedulerServiceProcessParsesAndValidatesWithoutCommittingState(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()

	store := store.NewSchedulerStore(db)
	record := &model.ChatSchedulerRecord{
		ChatID:             "chat-1",
		UserMessageID:      "user-1",
		AssistantMessageID: "assistant-1",
		TurnSeq:            2,
	}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}

	svc := NewSchedulerService(store, fakeSchedulerCompletionClient{
		response: `{"schema_version":1,"observations":[{"key":"facts.resource_request","value":true,"evidence":"现场提出资源请求","confidence":0.9}]}`,
	})
	output, err := svc.Process(context.Background(), record, "cheap-model", "prompt-v1", nil, SchedulerValidationSpec{
		AllowedObservationKeys: map[string]bool{"facts.resource_request": true},
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(output.Observations) != 1 {
		t.Fatalf("unexpected output: %+v", output)
	}
	got, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if got.Status != model.SchedulerStatusProcessing {
		t.Fatalf("successful analysis should await commit, got %s", got.Status)
	}
	if got.RawOutput == "" {
		t.Fatal("expected raw scheduler output to be persisted")
	}
}

func TestSchedulerServiceProcessMarksModelFailure(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()

	store := store.NewSchedulerStore(db)
	record := &model.ChatSchedulerRecord{ChatID: "chat-1", UserMessageID: "user-1", AssistantMessageID: "assistant-1", TurnSeq: 2}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	wantErr := errors.New("timeout")
	svc := NewSchedulerService(store, fakeSchedulerCompletionClient{err: wantErr})
	if _, err := svc.Process(context.Background(), record, "cheap-model", "prompt-v1", nil, SchedulerValidationSpec{}); err == nil {
		t.Fatal("expected model error")
	}
	got, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if got.Status != model.SchedulerStatusFailed || got.ErrorCode != "model_error" {
		t.Fatalf("unexpected failed record: %+v", got)
	}
}

func TestSchedulerServiceProcessMarksParseFailure(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()

	store := store.NewSchedulerStore(db)
	record := &model.ChatSchedulerRecord{ChatID: "chat-1", UserMessageID: "user-1", AssistantMessageID: "assistant-1", TurnSeq: 2}
	if err := store.CreateRecord(record); err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	svc := NewSchedulerService(store, fakeSchedulerCompletionClient{response: "not json"})
	if _, err := svc.Process(context.Background(), record, "cheap-model", "prompt-v1", nil, SchedulerValidationSpec{}); err == nil {
		t.Fatal("expected parse error")
	}
	got, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if got.Status != model.SchedulerStatusFailed || got.ErrorCode != "parse_error" {
		t.Fatalf("unexpected parse failure: %+v", got)
	}
}
