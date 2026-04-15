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
	"female": {Label: "女性", Hint: "生成女性角色，气质、关系推进和情绪表达要自然可信，避免空泛模板化。"},
	"male":   {Label: "男性", Hint: "生成男性角色，魅力点、说话方式和互动节奏要鲜明稳定，不要写成空壳设定。"},
}

var characterSettingOptions = map[string]templateChoiceOption{
	"city":          {Label: "现代都市", Hint: "世界观要扎根现实都市生活，可带行业圈层、城市气质、阶层差异或地下暗线，不要只是普通日常背景板。"},
	"school":        {Label: "校园青春", Hint: "世界观要体现校园生态、社团、成绩竞争、人际流言、成长压力与青春情绪，而不是只有教室和放学。"},
	"office":        {Label: "职场办公室", Hint: "世界观要体现行业规则、权力结构、利益关系和职场边界，让人物背景与职业环境真正互相咬合。"},
	"entertainment": {Label: "娱乐圈", Hint: "世界观要包含曝光、资本、经纪体系、资源争夺与公众形象管理，让人物处境和情绪来源有行业支撑。"},
	"fantasy":       {Label: "西幻异世界", Hint: "世界观要有独立的权力体系、地理风貌、超凡规则或种族秩序，人物身份必须与这些设定强绑定。"},
	"wuxia":         {Label: "仙侠江湖", Hint: "世界观要有门派、修行体系、因果恩怨、地位秩序与江湖规矩，人物来历与立场要从世界中长出来。"},
	"apocalypse":    {Label: "末日废土", Hint: "世界观要有生存规则、资源体系、危险来源与据点秩序，人物过去和现在都要被末世塑造。"},
}

var characterTypeOptions = map[string]templateChoiceOption{
	"pure":       {Label: "白月光", Hint: "整体氛围偏心动、慢热、温柔和克制，关系张力来自靠近时的悸动与舍不得打破平衡。"},
	"unrequited": {Label: "求而不得", Hint: "整体氛围偏拉扯、克制、暧昧与情感落差，人物必须有无法轻易跨过的现实或心理障碍。"},
	"healing":    {Label: "治愈陪伴", Hint: "整体氛围偏安抚、互相接住、长期相处和情绪修复，人物需要有能支撑陪伴感的内在温度。"},
	"rivalry":    {Label: "欢喜冤家", Hint: "整体氛围偏互怼、较劲、针锋相对但默契十足，关系推进要带火花和反差。"},
	"forbidden":  {Label: "禁忌拉扯", Hint: "整体氛围偏压抑、不可言说、身份受限与越克制越上头，人物和背景都需要天然制造禁忌感。"},
	"dangerous":  {Label: "危险关系", Hint: "整体氛围偏试探、不安全、诱惑与压迫并存，角色必须具备让人想逃又忍不住靠近的魅力。"},
}

var characterPersonalityOptions = map[string]templateChoiceOption{
	"tsundere": {Label: "傲娇", Hint: "嘴硬心软，表面抗拒、嘴上否认，真实情绪会通过小动作、语气失衡和占有欲泄露。"},
	"gentle":   {Label: "温柔", Hint: "细腻、稳、会照顾人，但不是没有锋芒；温柔应该建立在明确的人生经历和选择上。"},
	"scheming": {Label: "腹黑", Hint: "擅长观察、试探、掌控节奏和诱导关系，外在从容，内里清醒，不能只写成单薄坏笑。"},
	"airhead":  {Label: "天然呆", Hint: "反应慢半拍、带点迟钝和纯粹，但不能空白；要有自己独特的判断逻辑和可爱失衡感。"},
	"aloof":    {Label: "高冷", Hint: "外冷内热、边界感强、筛选欲明显，但偏爱时会有克制不住的失守和例外。"},
	"dominant": {Label: "强势", Hint: "掌控欲、压迫感、保护欲与占有欲并存，习惯主导关系节奏，但也要有触发软化的内因。"},
	"playful":  {Label: "会撩", Hint: "会逗人、会试探、懂得拿捏氛围和距离，语言风格要有张力，不能只是油嘴滑舌。"},
}

var characterPOVOptions = map[string]templateChoiceOption{
	"second": {Label: "第二人称", Hint: "开场白和叙事更偏沉浸式体验，但凡指向用户一律写 {{user}}，指向主角色一律写 {{char}}，不要直接写“你”；同时句式要保证 {{user}} 最终显示成“你”时依然自然。"},
	"third":  {Label: "第三人称", Hint: "叙事更有镜头感和空间感，但凡指向用户一律写 {{user}}，指向主角色一律写 {{char}}，不要直接写“他/她/ta”；同时句式要保证 {{user}} 最终显示成用户名时依然自然。"},
}

const characterCardSystemPrompt = `你是资深中文角色卡作者，擅长写适合角色扮演聊天应用的高质量角色卡。
你生成的内容必须同时满足以下标准：
1. 人设鲜明、可长期互动、不是空泛模板。
2. 性格要详细，有层次、有反差、有习惯、有情绪触发点、有说话风格。
3. 外貌要详细，能让用户看见这个人，而不是一句“长得很好看”带过。
4. 人物背景要和身份、性格、关系张力互相支撑，不能漂浮。
5. 世界观要独立成立，尤其在非现实设定里，要让人物明显属于这个世界；现实设定也要有清晰的社会环境、圈层或规则。
6. description、personality、scenario、first_msg 都要像角色卡站常见字段，而不是小说段落、百科或系统说明。

严格遵守以下规则：
1. 你不是在聊天，也不是在扮演角色，你是在生成角色卡。
2. 不要输出解释、前言、总结、Markdown、代码块。
3. 所有字段必须非空，内容具体、自然、可直接用于角色扮演聊天。
4. description 必须写出角色身份、样貌细节、气质、成长/经历、当前处境，以及角色与用户关系的切入口。
5. personality 必须写出核心性格、反差、说话方式、行为习惯、情绪触发点、底线、偏爱方式或弱点，不能只写几个形容词。
6. scenario 必须写出当前故事背景和正在发生的场景，让人物所处世界立得住，同时能立刻接戏。
7. first_msg 必须像角色卡站常见开场白，直接进入互动，带一点动作、氛围或台词，不要写字段名，不要解释。
8. name 字段用于定义主角色姓名；description、personality、scenario、first_msg 里凡是指向主角色本人时，只能使用 {{char}}，不要直接重复 name 字段里的名字。
9. description、personality、scenario、first_msg 里凡是指向聊天用户时，只能使用 {{user}}。
10. 不要直接使用“你”“你们”“您”“他”“她”“他们”“她们”“ta”“TA”“对方”等模糊指代来表示主角色或用户，也不要输出其他占位符或元信息。
11. tags 输出 4 到 7 个简短中文标签，用逗号分隔。
12. 不要输出 avatar_url、user_name、user_detail 等未要求字段。

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

	raw, err := s.callOpenAICompletion(settings, modelName, messages, 1.0, 2800, 0.95)
	if err != nil {
		return nil, err
	}

	draft, err := parseCharacterCardDraft(raw)
	if err != nil {
		return nil, err
	}
	draft.POV = req.POV

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
		builder.WriteString(fmt.Sprintf("- 用户补充的人设要求：%s\n", strings.TrimSpace(customPersonality)))
	}

	builder.WriteString(`
额外要求：
1. 角色必须像一个真实存在、能长期互动的人，而不是单句标签的拼接。
2. name 要自然、顺口、有辨识度，并与人物出身、世界观和气质匹配。
3. description 建议写到 180 到 320 个中文字符，至少覆盖：身份、外貌与体态细节、穿着或标志性特征、气质、过去经历、当前处境、和用户之间的关系入口。
4. personality 建议写到 180 到 320 个中文字符，至少覆盖：核心性格、性格反差、说话方式、行为习惯、情绪触发点、底线、偏爱方式、脆弱面或执念。
5. 外貌描写必须具体，至少让人能感知脸、眼神、发型、身形、穿着或气味/动作中的几项，不要只写“漂亮”“帅气”“清冷”这种空词。
6. 人物背景必须解释人物为什么会成为现在这样的人，且背景要和场景、基调、性格互相咬合。
7. 世界观必须是有独立感的：
   - 现实题材也要有明确的城市、圈层、行业、家庭或社会规则。
   - 架空题材要有清晰的权力结构、阵营、超凡规则、地域或生存秩序。
8. scenario 建议写到 140 到 240 个中文字符，既要交代眼下所处环境，也要让人物背景和世界观自然落地，不能只是笼统写两人相遇。
9. first_msg 建议写到 120 到 260 个中文字符，必须像真正能接着聊下去的开场：要有动作、语气、氛围和角色感，不要像说明书。
10. name 字段只负责定义主角色姓名；在 description、personality、scenario、first_msg 里，凡是提到主角色都必须写 {{char}}，不要直接写 name 字段里的名字。
11. 在 description、personality、scenario、first_msg 里，凡是提到聊天用户都必须写 {{user}}，不要自造用户名字。
12. 不要直接使用“你”“你们”“您”“他”“她”“他们”“她们”“ta”“TA”“对方”等模糊指代来表示用户或主角色。
13. 如果叙事视角是第二人称，正文句式必须保证 {{user}} 在界面里显示为“你”后仍然自然；如果是第三人称，正文句式必须保证 {{user}} 显示为用户名称后仍然自然。
14. 这条占位符规则优先级最高；输出前逐字段自检，若正文四个字段里出现主角色真实名字、模糊代词、或不是 {{char}} / {{user}} 的角色指代，必须改写后再输出。
15. 内容要更接近高质量角色卡站常见写法：信息密度高、人物感强、张力明确、世界感成立。
16. tags 重点覆盖人物气质、关系张力、题材世界观和互动风格。
17. 不要把字段内容写成“姓名：”“性格：”这种表单格式，只输出字段正文。`)

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
