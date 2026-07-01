package service

import (
	"litechat/internal/model"
	"litechat/internal/store"
	"strings"
	"testing"
)

// 复现 bug2：全局世界书里的「文字问题修正」常驻条目是否真的被注入到聊天 messages。
func newTestChatServiceWithStore(t *testing.T) (*ChatService, *store.WorldBookStore, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.NewDB(dir)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if err := db.InitSchema(); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	wbStore := store.NewWorldBookStore(db)
	userStore := store.NewUserStore(db)
	u := &model.User{Username: "u1", Role: "user", Mode: "self", UserName: "阿明"}
	if err := userStore.Create(u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := &ChatService{worldBookStore: wbStore, userStore: userStore}
	return svc, wbStore, u.ID
}

func TestGlobalTextFixEntryIsInjected(t *testing.T) {
	svc, wbStore, userID := newTestChatServiceWithStore(t)

	// 建一个全局世界书（CharacterID 为空）
	gwb := &model.WorldBook{Name: "全局纠错", CharacterID: ""}
	if err := wbStore.Create(gwb, userID); err != nil {
		t.Fatalf("create global wb: %v", err)
	}
	// 插入文字修正常驻条目（模拟 buildTextFixEntry 的字段）
	fix := &model.WorldBookEntry{
		WorldBookID:  gwb.ID,
		Keys:         "文字问题修正",
		Content:      "[Writing Style Correction] avoid clichés.",
		Enabled:      true,
		Constant:     true,
		InjectionPos: 1,
		Role:         "system",
	}
	if err := wbStore.CreateEntry(fix, userID); err != nil {
		t.Fatalf("create entry: %v", err)
	}

	// 一个绑定了某角色的会话（角色与全局世界书无绑定关系）
	char := &model.Character{ID: "char-xyz", Name: "测试角色"}
	messages := []model.ChatCompletionMessage{
		{Role: "system", Content: "你是测试角色。"},
		{Role: "user", Content: "你好"},
	}

	out := svc.injectWorldBookEntries(messages, messages, char, userID)

	found := false
	foundIdx := -1
	for i, m := range out {
		if strings.Contains(m.Content, "Writing Style Correction") {
			found = true
			foundIdx = i
			break
		}
	}
	if !found {
		t.Fatalf("全局文字修正条目未被注入！out=%+v", out)
	}
	t.Logf("注入成功：位置 idx=%d, role=%s, 共 %d 条消息", foundIdx, out[foundIdx].Role, len(out))
	for i, m := range out {
		preview := m.Content
		if len(preview) > 40 {
			preview = preview[:40]
		}
		t.Logf("  [%d] role=%s content=%q", i, m.Role, preview)
	}

	// bug2 复现：注入后是否出现【连续两条 system】(很多 OpenAI 兼容中转只认第一条 system，
	// 后续 system 被丢弃 → 文字修正指令不生效)。
	for i := 1; i < len(out); i++ {
		if out[i-1].Role == "system" && out[i].Role == "system" {
			t.Errorf("出现连续两条 system 消息(idx %d,%d)：多数中转会丢弃第二条，导致文字修正不生效", i-1, i)
		}
	}
}
