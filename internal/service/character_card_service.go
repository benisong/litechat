package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type templateChoiceOption struct {
	Label string
	Hint  string
}

var characterGenderOptions = map[string]templateChoiceOption{
	"female": {Label: "女性", Hint: "生成女性角色，气质和关系设计要自然可信。"},
	"male":   {Label: "男性", Hint: "生成男性角色，互动风格要鲜明且有代入感。"},
}

var characterSettingOptions = map[string]templateChoiceOption{
	"city":   {Label: "都市", Hint: "故事舞台是现代都市，细节贴近日常生活和现实社交。"},
	"school": {Label: "校园", Hint: "故事舞台是校园，保留青春感、日常感和成长氛围。"},
}

var characterTypeOptions = map[string]templateChoiceOption{
	"pure":       {Label: "白月光", Hint: "整体氛围偏温柔、治愈、暧昧，适合长期陪伴式互动。"},
	"unrequited": {Label: "求而不得", Hint: "整体氛围偏拉扯、克制、带一点距离感和心动张力。"},
}

var characterPersonalityOptions = map[string]templateChoiceOption{
	"tsundere": {Label: "傲娇", Hint: "说话会嘴硬，但行动和真实情绪会泄露在意。"},
	"gentle":   {Label: "温柔", Hint: "说话柔和耐心，擅长照顾人，情绪稳定。"},
	"scheming": {Label: "腹黑", Hint: "表面从容好相处，内里有掌控欲和试探意味。"},
	"airhead":  {Label: "天然呆", Hint: "反应有点慢半拍，常在无意间制造心动感。"},
}

var characterPOVOptions = map[string]templateChoiceOption{
	"second": {Label: "第二人称", Hint: "开场白和叙事更贴近沉浸式体验，优先使用“你”。"},
	"third":  {Label: "第三人称", Hint: "叙事更像旁观描写，可自然使用 {{user}} 表示用户名字。"},
}

const characterCardSystemPrompt = `你是角色卡生成器。
你的任务是根据用户提供的模板选项，生成一张适用于角色扮演聊天应用的中文角色卡。

严格遵守以下规则：
1. 不要使用任何预设提示词思路，你现在不是在聊天，而是在生成角色卡。
2. 只输出指定标签，不要输出解释、前言、总结、Markdown、代码块。
3. 所有字段都必须非空，内容必须具体、自然、适合长期聊天使用。
4. first_msg 必须是一段可直接作为对话开场的内容，不要写“这是开场白”。
5. tags 输出 3 到 6 个短标签，使用逗号分隔。
6. 不要输出 avatar_url、user_name、user_detail 等未要求字段。

输出格式必须严格如下：
<character_card>
<name>...</name>
<description>...</description>
<personality>...</personality>
<scenario>...</scenario>
<first_msg>...</first_msg>
<tags>标签1,标签2,标签3</tags>
</character_card>`

func (s *ChatService) GenerateCharacterCardDraft(req model.GenerateCharacterCardRequest) (*model.CharacterDraft, error) {
	gender, ok := characterGenderOptions[req.Gender]
	if !ok {
		return nil, fmt.Errorf("不支持的角色性别")
	}
	setting, ok := characterSettingOptions[req.Setting]
	if !ok {
		return nil, fmt.Errorf("不支持的故事舞台")
	}
	storyType, ok := characterTypeOptions[req.Type]
	if !ok {
		return nil, fmt.Errorf("不支持的故事基调")
	}
	personality, ok := characterPersonalityOptions[req.Personality]
	if !ok {
		return nil, fmt.Errorf("不支持的角色性格")
	}
	pov, ok := characterPOVOptions[req.POV]
	if !ok {
		return nil, fmt.Errorf("不支持的叙事视角")
	}

	settings, err := s.configStore.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("读取设置失败: %w", err)
	}
	if strings.TrimSpace(settings.APIEndpoint) == "" {
		return nil, fmt.Errorf("未配置 API 端点")
	}
	if strings.TrimSpace(settings.APIKey) == "" {
		return nil, fmt.Errorf("未配置 API 密钥")
	}

	modelName := strings.TrimSpace(settings.DefaultModel)
	if !settings.UseDefaultModelForCharacterCard && strings.TrimSpace(settings.CharacterCardModel) != "" {
		modelName = strings.TrimSpace(settings.CharacterCardModel)
	}
	if modelName == "" {
		return nil, fmt.Errorf("未配置可用模型")
	}

	prompt := buildCharacterCardPrompt(gender, setting, storyType, personality, pov)
	messages := []model.ChatCompletionMessage{
		{Role: "system", Content: characterCardSystemPrompt},
		{Role: "user", Content: prompt},
	}

	raw, err := s.callOpenAICompletion(settings, modelName, messages, 0.9, 1800, 0.9)
	if err != nil {
		return nil, err
	}

	draft, err := parseCharacterCardDraft(raw)
	if err != nil {
		return nil, err
	}

	return draft, nil
}

func buildCharacterCardPrompt(gender, setting, storyType, personality, pov templateChoiceOption) string {
	return fmt.Sprintf(`请根据以下模板选项生成一张中文角色卡：
- 角色性别：%s。%s
- 故事舞台：%s。%s
- 故事基调：%s。%s
- 角色性格：%s。%s
- 叙事视角：%s。%s

额外要求：
1. 角色名称要贴合设定，尽量自然，不要过于套路。
2. description 要写清角色身份、外貌气质、背景和她/他与用户之间的关系切入点。
3. personality 要写出性格、说话风格、行为习惯和情绪表达方式。
4. scenario 要描述一个适合展开对话的当下场景。
5. first_msg 要直接进入互动状态，像真实开场，不要解释，不要加字段名。
6. second 视角时优先使用“你”；third 视角时可以自然使用 {{user}} 表示用户名字。
7. 内容应适合角色扮演聊天，不要出现管理员、系统、模型、提示词等元信息。
8. tags 使用简短中文标签，用逗号分隔。`,
		gender.Label, gender.Hint,
		setting.Label, setting.Hint,
		storyType.Label, storyType.Hint,
		personality.Label, personality.Hint,
		pov.Label, pov.Hint,
	)
}

func (s *ChatService) callOpenAICompletion(settings *model.AppSettings, modelName string, messages []model.ChatCompletionMessage, temperature float64, maxTokens int, topP float64) (string, error) {
	reqBody := model.ChatCompletionRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		TopP:        topP,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	apiURL := strings.TrimRight(settings.APIEndpoint, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.APIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析生成结果失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("模型未返回内容")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("模型未返回内容")
	}
	return content, nil
}

func parseCharacterCardDraft(raw string) (*model.CharacterDraft, error) {
	cleaned := stripMarkdownCodeFence(cleanAssistantContent(raw))
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return nil, fmt.Errorf("模型未返回可解析的角色卡内容")
	}

	draft := &model.CharacterDraft{
		Name:        extractTaggedContent(cleaned, "name"),
		Description: extractTaggedContent(cleaned, "description"),
		Personality: extractTaggedContent(cleaned, "personality"),
		Scenario:    extractTaggedContent(cleaned, "scenario"),
		FirstMsg:    extractTaggedContent(cleaned, "first_msg"),
		Tags:        normalizeDraftTags(extractTaggedContent(cleaned, "tags")),
		AvatarURL:   "",
		UserName:    "",
		UserDetail:  "",
	}

	var missing []string
	if draft.Name == "" {
		missing = append(missing, "name")
	}
	if draft.Description == "" {
		missing = append(missing, "description")
	}
	if draft.Personality == "" {
		missing = append(missing, "personality")
	}
	if draft.Scenario == "" {
		missing = append(missing, "scenario")
	}
	if draft.FirstMsg == "" {
		missing = append(missing, "first_msg")
	}
	if draft.Tags == "" {
		missing = append(missing, "tags")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("角色卡字段解析不完整: %s", strings.Join(missing, ", "))
	}

	return draft, nil
}

func extractTaggedContent(raw, tag string) string {
	pattern := regexp.MustCompile(fmt.Sprintf(`(?is)<%s>\s*(.*?)\s*</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag)))
	matches := pattern.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func normalizeDraftTags(raw string) string {
	raw = strings.ReplaceAll(raw, "，", ",")
	raw = strings.ReplaceAll(raw, "、", ",")
	raw = strings.ReplaceAll(raw, "\n", ",")
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		cleaned = append(cleaned, tag)
	}
	return strings.Join(cleaned, ",")
}

func stripMarkdownCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```xml")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}
