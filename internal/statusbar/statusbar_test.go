package statusbar

import "testing"

func TestSplitStatusBarPanel(t *testing.T) {
	content := "正文第一段。\n\n'''\n【状态栏】\n地点：图书馆\n关系：信任\n'''"
	body, panel := Split(content)
	if body != "正文第一段。" {
		t.Fatalf("unexpected body: %q", body)
	}
	if panel != "【状态栏】\n地点：图书馆\n关系：信任" {
		t.Fatalf("unexpected panel: %q", panel)
	}
}

func TestSplitUsesFinalStatusBarMarker(t *testing.T) {
	content := "她提到‘【状态栏】只是界面名称’。\n\n```text\n【状态栏】\n地点：庭院\n```"
	body, panel := Split(content)
	if body != "她提到‘【状态栏】只是界面名称’。" {
		t.Fatalf("earlier marker was removed from body: %q", body)
	}
	if panel != "【状态栏】\n地点：庭院" {
		t.Fatalf("unexpected panel: %q", panel)
	}
}

func TestSplitLeavesOrdinaryContentUnchanged(t *testing.T) {
	content := "普通回复，没有状态面板。\n"
	body, panel := Split(content)
	if body != content || panel != "" {
		t.Fatalf("ordinary content changed: body=%q panel=%q", body, panel)
	}
}
