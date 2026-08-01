package store

import (
	"litechat/internal/model"
	"testing"
)

func TestWorldBookRuntimeModeRoundTrips(t *testing.T) {
	db := newSchedulerTestDB(t)
	defer db.Close()

	store := NewWorldBookStore(db)
	book := &model.WorldBook{
		Name:        "剧情节点",
		Description: "只用于初始化编译",
		RuntimeMode: "compile_only",
	}
	if err := store.Create(book, "user-1"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := store.GetByID(book.ID, "user-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.RuntimeMode != "compile_only" {
		t.Fatalf("expected compile_only, got %q", got.RuntimeMode)
	}

	got.RuntimeMode = "static"
	if err := store.Update(got, "user-1"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, err := store.GetByID(book.ID, "user-1")
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.RuntimeMode != "static" {
		t.Fatalf("expected static after update, got %q", updated.RuntimeMode)
	}
}
