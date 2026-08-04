package service

import (
	"context"
	"testing"
)

func TestBuildJSONCharacterCardImportPlanSeparatesCharacterAndWorldbookData(t *testing.T) {
	raw := []byte(`{"card_version":"1.0","character":{"name":"重生之玄幻之旅","pov":"second","description":"公开身份","personality":"公开性格","scenario":"初始场景","first_message":"开场"},"worldbook":{"id":"w","version":"1.0","global_enabled":true,"main_entries":[{"id":"public","title":"公开规则","content":"公开内容","user_visible":true,"scheduler_enabled":false},{"id":"hidden","title":"隐藏规则","content":"隐藏内容","user_visible":false,"scheduler_enabled":true}]},"tags":["复杂剧情"]}`)
	plan, err := BuildJSONCharacterCardImportPlan(context.Background(), raw)
	if err != nil {
		t.Fatalf("Build plan returned error: %v", err)
	}
	if plan.Character.Name != "重生之玄幻之旅" || plan.Character.FirstMessage != "开场" {
		t.Fatalf("unexpected character draft: %+v", plan.Character)
	}
	if len(plan.PublicWorldBook.MainEntries) != 1 || plan.PublicWorldBook.MainEntries[0].ID != "public" {
		t.Fatalf("unexpected public worldbook: %+v", plan.PublicWorldBook)
	}
	if len(plan.SchedulerWorldBook.MainEntries) != 1 || plan.SchedulerWorldBook.MainEntries[0].ID != "hidden" {
		t.Fatalf("unexpected scheduler worldbook: %+v", plan.SchedulerWorldBook)
	}
	if len(plan.Tags) != 1 || plan.Tags[0] != "复杂剧情" {
		t.Fatalf("unexpected tags: %+v", plan.Tags)
	}
}
