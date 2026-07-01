package service

import (
	"encoding/json"
	"litechat/internal/model"
	"testing"
)

// 排查:各种预设配置下，buildMessages 产出的最终 messages 是否可能出现【多条 system】
// (多 system 会被多数 OpenAI 兼容中转丢弃，是 bug2 同类隐患)。
func countSystem(msgs []model.ChatCompletionMessage) int {
	n := 0
	for _, m := range msgs {
		if m.Role == "system" {
			n++
		}
	}
	return n
}

func hasAdjacentSameRole(msgs []model.ChatCompletionMessage) bool {
	for i := 1; i < len(msgs); i++ {
		if msgs[i-1].Role == msgs[i].Role {
			return true
		}
	}
	return false
}

func buildPresetMessages(t *testing.T, entries []model.PromptEntry, systemPrompt string) []model.ChatCompletionMessage {
	t.Helper()
	svc, wbStore, userID := newTestChatServiceWithStore(t)
	_ = wbStore
	char := &model.Character{ID: "c1", Name: "小雨", FirstMsg: "你好呀"}
	preset := &model.Preset{SystemPrompt: systemPrompt}
	if len(entries) > 0 {
		b, _ := json.Marshal(entries)
		preset.Prompts = string(b)
	}
	history := []*model.Message{
		{Role: "user", Content: "第一句"},
		{Role: "assistant", Content: "第一句回复"},
	}
	return svc.buildMessages("chat-1", preset, char, history, "新的一句", userID)
}

func TestPreset_MultipleSystemTrueEntries_MergeToOneSystem(t *testing.T) {
	// 多个 system_prompt=true 的条目：应被 Step A 合并为开头一条 system
	entries := []model.PromptEntry{
		{ID: "1", Content: "规则A", Role: "system", Enabled: true, SystemPrompt: true, Order: 0},
		{ID: "2", Content: "规则B", Role: "system", Enabled: true, SystemPrompt: true, Order: 1},
		{ID: "3", Content: "规则C", Role: "system", Enabled: true, SystemPrompt: true, Order: 2},
	}
	msgs := buildPresetMessages(t, entries, "")
	if n := countSystem(msgs); n > 1 {
		t.Errorf("多个 system_prompt=true 条目产生了 %d 条 system(应合并为 1)", n)
	}
	if hasAdjacentSameRole(msgs) {
		t.Errorf("出现相邻同 role 消息: %+v", roles(msgs))
	}
}

func TestPreset_SystemRoleButNotSystemPrompt_NoStrayLeadingSystem(t *testing.T) {
	// system_prompt=false 但 role=system 的条目：Step C 追加后应被 Step D 压成 user，
	// 不应形成第二条 system。
	entries := []model.PromptEntry{
		{ID: "1", Content: "主系统", Role: "system", Enabled: true, SystemPrompt: true, Order: 0},
		{ID: "2", Content: "附加system(非systemprompt)", Role: "system", Enabled: true, SystemPrompt: false, Order: 1},
	}
	msgs := buildPresetMessages(t, entries, "")
	if n := countSystem(msgs); n > 1 {
		t.Errorf("system_prompt=false+role=system 的条目造成了 %d 条 system", n)
	}
}

func TestPreset_NoSystemPromptEntries_JumpMessagesRolesValid(t *testing.T) {
	// 没有任何 system_prompt=true：开头不生成 system 块。验证不会出现相邻同 role / 多 system。
	entries := []model.PromptEntry{
		{ID: "1", Content: "尾部注入(user)", Role: "user", Enabled: true, SystemPrompt: false, Order: 0},
	}
	msgs := buildPresetMessages(t, entries, "")
	if n := countSystem(msgs); n > 1 {
		t.Errorf("产生了 %d 条 system", n)
	}
	if hasAdjacentSameRole(msgs) {
		t.Errorf("出现相邻同 role: %+v", roles(msgs))
	}
}

func TestPreset_PlusGlobalTextFix_StillSingleSystem(t *testing.T) {
	// 预设有 system + 全局文字修正条目一起：最终仍应只有一条 system(bug2 修复覆盖)
	svc, wbStore, userID := newTestChatServiceWithStore(t)
	gwb := &model.WorldBook{Name: "全局", CharacterID: ""}
	if err := wbStore.Create(gwb, userID); err != nil {
		t.Fatal(err)
	}
	fix := &model.WorldBookEntry{
		WorldBookID: gwb.ID, Keys: "文字问题修正", Content: "[Writing Style Correction]",
		Enabled: true, Constant: true, InjectionPos: 1, Role: "system",
	}
	if err := wbStore.CreateEntry(fix, userID); err != nil {
		t.Fatal(err)
	}
	char := &model.Character{ID: "c1", Name: "小雨", FirstMsg: "hi"}
	b, _ := json.Marshal([]model.PromptEntry{
		{ID: "1", Content: "主系统规则", Role: "system", Enabled: true, SystemPrompt: true, Order: 0},
	})
	preset := &model.Preset{Prompts: string(b)}
	history := []*model.Message{{Role: "user", Content: "hey"}}
	msgs := svc.buildMessages("chat-1", preset, char, history, "在吗", userID)

	if n := countSystem(msgs); n > 1 {
		t.Errorf("预设 system + 全局文字修正 造成 %d 条 system(bug2 未完全修复)", n)
	}
	if hasAdjacentSameRole(msgs) {
		t.Errorf("出现相邻同 role: %+v", roles(msgs))
	}
}

func roles(msgs []model.ChatCompletionMessage) []string {
	r := make([]string, len(msgs))
	for i, m := range msgs {
		r[i] = m.Role
	}
	return r
}
