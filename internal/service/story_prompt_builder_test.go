package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
	"testing"
)

func TestStoryPromptBuilderIncludesCharacterAndStaticOnly(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	user := &model.User{Username: "prompt-user", Role: "user", Mode: "self"}
	if err := store.NewUserStore(db).Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	character := &model.Character{Name: "复杂角色", Description: "固定设定", Personality: "冷静"}
	if err := store.NewCharacterStore(db).Create(character, user.ID); err != nil {
		t.Fatalf("Create character: %v", err)
	}
	worldbooks := store.NewWorldBookStore(db)
	staticBook := &model.WorldBook{Name: "客观规则", CharacterID: character.ID, RuntimeMode: "static"}
	compileBook := &model.WorldBook{Name: "隐藏剧情", CharacterID: character.ID, RuntimeMode: "compile_only"}
	for _, book := range []*model.WorldBook{staticBook, compileBook} {
		if err := worldbooks.Create(book, user.ID); err != nil {
			t.Fatalf("Create book: %v", err)
		}
	}
	for _, entry := range []struct {
		book    *model.WorldBook
		content string
	}{{staticBook, "静态规则内容"}, {compileBook, "隐藏未来结局"}} {
		if err := worldbooks.CreateEntry(&model.WorldBookEntry{WorldBookID: entry.book.ID, Content: entry.content, Enabled: true}, user.ID); err != nil {
			t.Fatalf("Create entry: %v", err)
		}
	}
	builder := NewDefaultStoryPromptBuilder(store.NewCharacterStore(db), worldbooks, nil)
	messages, _, err := builder.BuildStoryPrompt(context.Background(), &model.Chat{CharacterID: character.ID, UserID: user.ID}, nil, "用户输入", &model.ChatStoryState{CurrentScene: "山门", Route: "survival"})
	if err != nil {
		t.Fatalf("BuildStoryPrompt: %v", err)
	}
	joined := ""
	for _, message := range messages {
		joined += message.Role + ":" + message.Content + "\n"
	}
	if !strings.Contains(joined, "固定设定") || !strings.Contains(joined, "静态规则内容") || !strings.Contains(joined, "当前场景：山门") || !strings.Contains(joined, "用户输入") {
		t.Fatalf("missing expected context: %s", joined)
	}
	if strings.Contains(joined, "隐藏未来结局") {
		t.Fatalf("compile_only leaked: %s", joined)
	}
	if strings.Count(joined, "system:") != 1 {
		t.Fatalf("expected one system message: %s", joined)
	}
}
