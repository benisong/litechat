package service

import (
	"litechat/internal/model"
	"strings"
	"testing"
)

func TestBuildRoleIdentityPromptAnchorsFemaleGenderAndSecondPerson(t *testing.T) {
	svc := &ChatService{}
	char := &model.Character{
		Name:          "林小雨",
		Description:   "性别女。外表温柔，但行动果断。",
		POV:           "second",
		UseCustomUser: true,
		UserName:      "阿明",
	}

	prompt := svc.buildRoleIdentityPrompt(char, "user-1")

	wantParts := []string{
		"角色卡性别锚点：林小雨 的性别/称谓倾向是女性",
		"必须使用“她”和相符称谓，不要写成“他”",
		"描写用户的动作、感受、处境或对话称呼时用“你”",
		"不要反复用 阿明 当主语",
		"不要用“我”替代 林小雨",
		"“我”只允许出现在 林小雨 的直接台词",
	}
	for _, part := range wantParts {
		if !strings.Contains(prompt, part) {
			t.Fatalf("identity prompt missing %q\nprompt:\n%s", part, prompt)
		}
	}
}

func TestInferCharacterGenderHintHandlesExplicitGender(t *testing.T) {
	female := inferCharacterGenderHint(&model.Character{Description: "性别：女\n职业：侦探"})
	if female.Label != "女性" || female.Pronoun != "她" || female.OppositePronoun != "他" {
		t.Fatalf("unexpected female hint: %+v", female)
	}

	male := inferCharacterGenderHint(&model.Character{Description: "性别男，年轻剑士"})
	if male.Label != "男性" || male.Pronoun != "他" || male.OppositePronoun != "她" {
		t.Fatalf("unexpected male hint: %+v", male)
	}
}

func TestInferCharacterGenderHintPrioritizesExplicitGender(t *testing.T) {
	hint := inferCharacterGenderHint(&model.Character{Description: "性别女，但长期以男性身份潜伏。"})
	if hint.Label != "女性" || hint.Pronoun != "她" {
		t.Fatalf("expected explicit female gender to win, got %+v", hint)
	}
}
