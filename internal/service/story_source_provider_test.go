package service

import (
	"context"
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
	"testing"
)

func TestWorldBookStorySourceProviderLoadsOnlyCompileOnlyBooks(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	userStore := store.NewUserStore(db)
	user := &model.User{Username: "source-user", Role: "user", Mode: "self"}
	if err := userStore.Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	worldBookStore := store.NewWorldBookStore(db)
	global := &model.WorldBook{Name: "全局剧情", RuntimeMode: "compile_only"}
	characterBook := &model.WorldBook{Name: "角色静态规则", CharacterID: "char-1", RuntimeMode: "static"}
	hidden := &model.WorldBook{Name: "角色隐藏剧情", CharacterID: "char-1", RuntimeMode: "compile_only"}
	for _, book := range []*model.WorldBook{global, characterBook, hidden} {
		if err := worldBookStore.Create(book, user.ID); err != nil {
			t.Fatalf("Create worldbook: %v", err)
		}
	}
	for _, entry := range []struct {
		book         *model.WorldBook
		key, content string
	}{
		{global, "global", "全局设定"}, {characterBook, "static", "运行时规则"}, {hidden, "secret", "隐藏真相"},
	} {
		if err := worldBookStore.CreateEntry(&model.WorldBookEntry{WorldBookID: entry.book.ID, Keys: entry.key, Content: entry.content, Enabled: true}, user.ID); err != nil {
			t.Fatalf("Create entry: %v", err)
		}
	}
	provider := NewWorldBookStorySourceProvider(worldBookStore)
	source, err := provider.Load(context.Background(), user.ID, "char-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(source.Text, "全局设定") || !strings.Contains(source.Text, "隐藏真相") {
		t.Fatalf("compile-only text missing: %q", source.Text)
	}
	if strings.Contains(source.Text, "运行时规则") {
		t.Fatalf("static worldbook leaked: %q", source.Text)
	}
	if source.VersionHash == "" {
		t.Fatal("expected version hash")
	}
}

func TestWorldBookStorySourceProviderHashIsStable(t *testing.T) {
	db := newServiceSchedulerTestDB(t)
	defer db.Close()
	userStore := store.NewUserStore(db)
	user := &model.User{Username: "source-user-2", Role: "user", Mode: "self"}
	if err := userStore.Create(user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	worldBookStore := store.NewWorldBookStore(db)
	book := &model.WorldBook{Name: "剧情", CharacterID: "char-2", RuntimeMode: "compile_only"}
	if err := worldBookStore.Create(book, user.ID); err != nil {
		t.Fatalf("Create worldbook: %v", err)
	}
	for _, content := range []string{"第二条", "第一条"} {
		if err := worldBookStore.CreateEntry(&model.WorldBookEntry{WorldBookID: book.ID, Content: content, Enabled: true}, user.ID); err != nil {
			t.Fatalf("Create entry: %v", err)
		}
	}
	provider := NewWorldBookStorySourceProvider(worldBookStore)
	first, err := provider.Load(context.Background(), user.ID, "char-2")
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	second, err := provider.Load(context.Background(), user.ID, "char-2")
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if first.VersionHash != second.VersionHash || first.Text != second.Text {
		t.Fatalf("source is not stable: first=%+v second=%+v", first, second)
	}
}
