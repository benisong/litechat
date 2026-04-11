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
	"female": {Label: "女性", Hint: "生成女性角色，气质和关系设计要自然可信，避免空泛模板化。"},
	"male":   {Label: "男性", Hint: "生成男性角色，互动风格要鲜明、稳定，并有明确的人物吸引力。"},
}

var characterSettingOptions = map[string]templateChoiceOption{
	"city":          {Label: "现代都市", Hint: "故事舞台是现实都市，细节贴近日常生活、现实社交和成年人关系推进。"},
	"school":        {Label: "校园青春", Hint: "故事舞台是校园，保留青春感、日常感和带点青涩的情绪流动。"},
	"office":        {Label: "职场办公室", Hint: "故事围绕工作关系展开，要有边界感、张力和克制感。"},
	"entertainment": {Label: "娱乐圈", Hint: "故事围绕名利场、曝光、资源和镜头外的真实情绪展开。"},
	"fantasy":       {Label: "西幻异世界", Hint: "世界观可带有魔法、骑士、学院、王城等元素，但仍要服务人物关系。"},
	"wuxia":         {Label: "仙侠江湖", Hint: "世界观可带宗门、江湖、师门、历练、宿命感，语言有适度古风气息。"},
	"apocalypse":    {Label: "末日废土", Hint: "环境危险、资源稀缺，关系推进要带生存压力和强绑定感。"},
}

var characterTypeOptions = map[string]templateChoiceOption{
	"pure":       {Label: "白月光", Hint: "整体氛围偏温柔、心动、慢热和可长期相处的陪伴感。"},
	"unrequited": {Label: "求而不得", Hint: "整体氛围偏克制、拉扯、若即若离，带明显的情感落差。"},
	"healing":    {Label: "治愈陪伴", Hint: "整体氛围偏安抚、陪伴、互相接住情绪，适合日常互动。"},
	"rivalry":    {Label: "欢喜冤家", Hint: "整体氛围偏互怼、较劲、互相拆台又默契十足。"},
	"forbidden":  {Label: "禁忌拉扯", Hint: "整体氛围偏不能说破、身份受限、越克制越上头。"},
	"dangerous":  {Label: "危险关系", Hint: "整体氛围偏不安全、试探、压迫感与吸引力并存。"},
}

var characterPersonalityOptions = map[string]templateChoiceOption{
	"tsundere": {Label: "傲娇", Hint: "嘴硬心软，说话会否认在意，但行动会暴露真实情绪。"},
	"gentle":   {Label: "温柔", Hint: "说话柔和耐心，擅长照顾人，情绪稳定但不是毫无个性。"},
	"scheming": {Label: "腹黑", Hint: "表面从容好相处，实则很会拿捏节奏、试探反应、引导关系。"},
	"airhead":  {Label: "天然呆", Hint: "反应慢半拍，常在无意间说出让人心动或失控的话。"},
	"aloof":    {Label: "高冷", Hint: "外冷内热，有距离感和筛选感，但偏爱时会明显失衡。"},
	"dominant": {Label: "强势", Hint: "掌控欲、压迫感和保护欲并存，习惯主导相处节奏。"},
	"playful":  {Label: "会撩", Hint: "有松弛感和坏心思，擅长用语言和氛围推进暧昧。"},
}

var characterPOVOptions = map[string]templateChoiceOption{
	"second": {Label: "第二人称", Hint: "开场白和叙事更贴近沉浸式体验，优先使用“你”。"},
	"third":  {Label: "第三人称", Hint: "叙事更像旁观描写，可自然使用 {{user}} 表示用户名字。"},
}

const characterCardSystemPrompt = `你是资深中文角色卡作者，擅长撰写适合角色扮演聊天应用的角色卡，风格接近常见角色卡站的高质量卡面文案。

你生成的内容必须具备这些特征：
1. 人设清晰，关系钩子明确，有能直接开聊的互动张力。
2. 文案像角色卡字段，而不是小说段落、设定百科或系统说明。
3. description、personality、scenario、first_msg 都要可直接放进角色卡站的对应栏位。
4. 语言自然、细节密度高，但不要堆砌辞藻，不要写空泛万能人设。

严格遵守以下规则：
1. 你不是在聊天，也不是在扮演角色，你是在生成角色卡。
2. 不要使用任何预设提示词口吻，不要输出解释、前言、总结、Markdown、代码块。
3. 所有字段必须非空，且内容具体、可用、适合长期聊天使用。
4. description 更像角色简介，需要写出身份、外貌气质、背景、与用户的关系切入口。
5. personality 更像角色卡站常见的人设描述，需要写出性格核心、说话风格、习惯、情绪触发点、偏爱方式或弱点。
6. scenario 只写“此刻正在发生什么”和“用户为什么会在这里”，不要写成世界观百科。
7. first_msg 必须像角色卡站常见开场白，直接进入场景，可带动作描写、情绪和对话，生成后就能继续聊天。
8. 保持字段之间的人物气质一致，不要互相矛盾。
9. 除 {{user}} 外，不要输出任何占位符或元信息。
10. tags 输出 4 到 7 个简短中文标签，用逗号分隔。
11. 不要输出 avatar_url、user_name、user_detail 等未要求字段。

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
		return nil, fmt.Errorf("不支持的故事场景")
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

	prompt := buildCharacterCardPrompt(gender, setting, storyType, personality, pov, req.CustomPersonality)
	messages := []model.ChatCompletionMessage{
		{Role: "system", Content: characterCardSystemPrompt},
		{Role: "user", Content: prompt},
	}

	raw, err := s.callOpenAICompletion(settings, modelName, messages, 1.0, 2200, 0.95)
	if err != nil {
		return nil, err
	}

	draft, err := parseCharacterCardDraft(raw)
	if err != nil {
		return nil, err
	}

	return draft, nil
}

func buildCharacterCardPrompt(gender, setting, storyType, personality, pov templateChoiceOption, customPersonality string) string {
	var builder strings.Builder
	builder.WriteString("请根据以下模板选项生成一张中文角色卡：\n")
	builder.WriteString(fmt.Sprintf("- 角色性别：%s。%s\n", gender.Label, gender.Hint))
	builder.WriteString(fmt.Sprintf("- 故事场景：%s。%s\n", setting.Label, setting.Hint))
	builder.WriteString(fmt.Sprintf("- 故事基调：%s。%s\n", storyType.Label, storyType.Hint))
	builder.WriteString(fmt.Sprintf("- 角色性格：%s。%s\n", personality.Label, personality.Hint))
	builder.WriteString(fmt.Sprintf("- 叙事视角：%s。%s\n", pov.Label, pov.Hint))
	if strings.TrimSpace(customPersonality) != "" {
		builder.WriteString(fmt.Sprintf("- 用户补充的性格要求：%s\n", strings.TrimSpace(customPersonality)))
	}

	builder.WriteString(`
额外要求：
1. 生成结果要更像角色卡站常见写法：人设鲜明、关系切入口明确、开场白可直接接戏。
2. name 要自然、顺口、辨识度强，并与世界观匹配；现代场景优先现代姓名，奇幻或古风场景可适度风格化。
3. description 建议控制在 120 到 220 个中文字符之间，重点写身份、外貌气质、过去经历、和用户的关系起点。
4. personality 建议控制在 120 到 220 个中文字符之间，写出性格核心、说话方式、习惯、小弱点、情绪触发点和对用户的偏爱方式。
5. scenario 建议控制在 80 到 160 个中文字符之间，只写当前关系阶段和眼下场景，不要写成长篇背景说明。
6. first_msg 建议控制在 80 到 220 个中文字符之间，要像角色卡站常见的第一条回复：直接进入互动状态，带一点动作描写、氛围和台词。
7. second 视角时优先使用“你”；third 视角时可以自然使用 {{user}} 表示用户名字。
8. 文风要适合角色扮演聊天，不要出现管理员、系统、模型、提示词、安全说明等元信息。
9. tags 使用简短中文标签，优先覆盖世界观、关系感、角色气质、互动张力。
10. 不要把字段内容写成“姓名：”“性格：”这种表单格式，只输出字段内容本身。`)

	return builder.String()
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
