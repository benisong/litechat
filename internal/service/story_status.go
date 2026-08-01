package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"litechat/internal/model"
	"time"
)

type StoryChatStatus struct {
	Chat          *model.Chat
	Manifest      *model.StoryManifest
	State         *model.ChatStoryState
	LatestSuccess *model.ChatSchedulerRecord
	CheckedAt     time.Time
}

func (i *StoryChatInitializer) GetStatus(ctx context.Context, userID, chatID string) (*StoryChatStatus, error) {
	if i == nil || i.chatStore == nil || i.storyStore == nil {
		return nil, fmt.Errorf("story status service is not configured")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	chat, err := i.chatStore.GetByID(chatID, userID)
	if err != nil {
		return nil, err
	}
	if !chat.SchedulerEnabled {
		return nil, fmt.Errorf("chat is not a story chat")
	}
	state, err := i.storyStore.GetStoryState(chatID)
	if err != nil {
		return nil, err
	}
	manifest, err := i.storyStore.GetManifest(state.ManifestID)
	if err != nil {
		return nil, err
	}
	latest, err := i.storyStore.LatestSuccessfulRecord(chatID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return &StoryChatStatus{Chat: chat, Manifest: manifest, State: state, LatestSuccess: latest, CheckedAt: time.Now()}, nil
}
