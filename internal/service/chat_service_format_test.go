package service

import (
	"strings"
	"testing"
)

func TestValidateAssistantReplyFormatAcceptsProseAndDialogue(t *testing.T) {
	valid := []string{
		`她轻轻点头，低声说：“我记得那把银钥匙。”`,
		`她把银钥匙从书架后取出，掌心微微发凉。`,
		`She paused at the door. "I remember the key."`,
	}

	for _, text := range valid {
		if err := validateAssistantReplyFormat(text); err != nil {
			t.Fatalf("expected valid formatted reply, got %v for %q", err, text)
		}
	}
}

func TestValidateAssistantReplyFormatRejectsStructuredOrMetaOutput(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "json", text: `{"reply":"我记得钥匙。"}`, want: "JSON"},
		{name: "markdown list", text: "- 我记得钥匙", want: "Markdown list"},
		{name: "markdown heading", text: "# 回复\n我记得钥匙", want: "Markdown heading"},
		{name: "role label", text: "助手：我记得钥匙。", want: "role label"},
		{name: "code fence", text: "```\n我记得钥匙\n```", want: "code fence"},
		{name: "meta", text: "分析：她应该记得钥匙。", want: "meta"},
		{name: "unbalanced quote", text: "她说：“我记得钥匙。", want: "unbalanced"},
		{name: "hidden only", text: "<think>hidden</think>", want: "empty"},
	}

	for _, tc := range tests {
		err := validateAssistantReplyFormat(tc.text)
		if err == nil {
			t.Fatalf("%s: expected invalid formatted reply", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: expected error to contain %q, got %v", tc.name, tc.want, err)
		}
	}
}
