package store

import (
	"litechat/internal/model"
	"sync"
	"testing"
)

func TestMessageStoreConcurrentCreateAssignsUniqueOrderedSequences(t *testing.T) {
	db, err := NewDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.InitSchema(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO characters (id, user_id, name) VALUES ('char', 'user', '测试')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO chats (id, user_id, character_id, title) VALUES ('chat', 'user', 'char', '测试')`); err != nil {
		t.Fatal(err)
	}
	messages := NewMessageStore(db)
	const total = 20
	errs := make(chan error, total)
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- messages.Create(&model.Message{ChatID: "chat", Role: "user", Content: string(rune('a' + i))})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	list, err := messages.ListByChatID("chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != total {
		t.Fatalf("messages=%d, want %d", len(list), total)
	}
	for i, message := range list {
		if message.Seq != i+1 {
			t.Fatalf("position %d has seq %d, want %d", i, message.Seq, i+1)
		}
	}
}
