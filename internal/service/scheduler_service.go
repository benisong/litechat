package service

import (
	"context"
	"fmt"
	"litechat/internal/model"
	"litechat/internal/store"
)

// CompletionClient 是调度运行时使用的非流式模型调用抽象。
type CompletionClient interface {
	Complete(ctx context.Context, modelName string, messages []model.ChatCompletionMessage) (string, error)
}

// SchedulerService 负责调度模型调用、解析和候选结果校验。
// 它不直接提交剧情状态；成功结果等待 RuleEngine 和 SchedulerStore 事务提交。
type SchedulerService struct {
	store  *store.SchedulerStore
	client CompletionClient
}

func NewSchedulerService(schedulerStore *store.SchedulerStore, client CompletionClient) *SchedulerService {
	return &SchedulerService{store: schedulerStore, client: client}
}

func (s *SchedulerService) Process(
	ctx context.Context,
	record *model.ChatSchedulerRecord,
	modelName string,
	promptVersion string,
	messages []model.ChatCompletionMessage,
	spec SchedulerValidationSpec,
) (*model.SchedulerOutput, error) {
	if record == nil {
		return nil, fmt.Errorf("scheduler record is nil")
	}
	if s.store == nil || s.client == nil {
		return nil, fmt.Errorf("scheduler service is not configured")
	}
	if err := s.store.MarkProcessing(record.ID); err != nil {
		return nil, err
	}

	raw, err := s.client.Complete(ctx, modelName, messages)
	if err != nil {
		_ = s.store.MarkFailed(record.ID, "model_error", err.Error())
		return nil, fmt.Errorf("scheduler model: %w", err)
	}
	if err := s.store.UpdateRawOutput(record.ID, raw, modelName, promptVersion); err != nil {
		_ = s.store.MarkFailed(record.ID, "persistence_error", err.Error())
		return nil, fmt.Errorf("save scheduler output: %w", err)
	}

	output, err := ParseSchedulerOutput(raw)
	if err != nil {
		_ = s.store.MarkFailed(record.ID, "parse_error", err.Error())
		return nil, err
	}
	if err := ValidateSchedulerOutput(output, spec); err != nil {
		_ = s.store.MarkFailed(record.ID, "validation_error", err.Error())
		return nil, err
	}
	return output, nil
}
