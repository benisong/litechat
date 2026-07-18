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
	Label    string
	Hint     string
	IsCustom bool
}

var characterGenderOptions = map[string]templateChoiceOption{
	"female": {Label: "女性", Hint: "性别只定义角色身份与称谓，不预设温柔、柔弱、感性等性格，也不限制职业、能力或关系位置。"},
	"male":   {Label: "男性", Hint: "性别只定义角色身份与称谓，不预设强势、沉稳、理性等性格，也不限制职业、能力或关系位置。"},
}

var characterSettingOptions = map[string]templateChoiceOption{
	"city":          {Label: "现代都市", Hint: "当代现实城市。请选择一个具体但不俗套的生活切面、谋生方式或社交圈层来塑造人物；保持现实逻辑，除非用户明确要求，不加入超自然元素。"},
	"school":        {Label: "校园青春", Hint: "当代高中或大学。角色必须与校园有合理而持续的连接，矛盾来自学业、社团、同伴、家庭或成长选择；不要套用总裁、黑道、婚约等成人爽文身份。"},
	"office":        {Label: "都市职场", Hint: "当代职业环境。用真实的工作内容、利益分歧和职业习惯建立人物，不默认总裁、天才上司或万能精英；身份与权力边界要可信。"},
	"entertainment": {Label: "娱乐圈", Hint: "现代娱乐产业。选择一个具体行业位置，写出公开形象与私人生活的落差；不要默认顶流、金主、契约恋爱或全网追捧。"},
	"fantasy":       {Label: "西幻异世界", Hint: "架空西方奇幻。只选择一到两条真正影响人物选择的世界规则，不堆百科设定，也不默认王族、救世主、最强法师或精灵等常见答案。"},
	"wuxia":         {Label: "仙侠江湖", Hint: "东方武侠或仙侠。用一条具体的门派规矩、修行代价或江湖秩序塑造人物，不默认天才剑修、仙尊、魔教少主等高位身份。"},
	"apocalypse":    {Label: "末日废土", Hint: "灾变后的生存社会。确定一种具体的资源规则、据点秩序或生存分工，不默认最强异能者、领袖或孤狼；让日常能力与局限同样重要。"},
}

var characterTypeOptions = map[string]templateChoiceOption{
	"pure":       {Label: "心动暧昧", Hint: "关系尚未挑明。张力来自长期积累的小动作、理解偏差和不敢确认，而不是反复脸红、吃醋或永远差一步的机械拉扯。"},
	"unrequited": {Label: "求而不得", Hint: "存在一个来自世界内部的具体障碍。重点写双方如何选择、回避或承担代价，不只依赖误会、失忆和一句话不说清。"},
	"healing":    {Label: "治愈陪伴", Hint: "关系稳定而互惠，可以是恋人、密友或长期搭档。双方都有边界和自己的难题，不把 {{char}} 写成专门安抚 {{user}} 的治疗工具。"},
	"rivalry":    {Label: "欢喜冤家", Hint: "双方有真实分歧、相近能力和不得不合作的理由。默契来自共同经历，不把互动缩减成无休止互怼或小学式欺负。"},
	"forbidden":  {Label: "禁忌拉扯", Hint: "禁忌必须有现实后果、规则来源和可选择的代价。人物可以犹豫、抵抗或重新理解规则，不把禁忌只当作暧昧装饰。"},
	"dangerous":  {Label: "危险关系", Hint: "危险来自可信的立场、能力、秘密或决策，而不是控制欲本身。保留双方判断和拒绝的空间，不浪漫化胁迫、跟踪或无边界占有。"},
}

var characterPersonalityOptions = map[string]templateChoiceOption{
	"tsundere": {Label: "傲娇", Hint: "把嘴硬理解为特定情境下的自我保护，不是每句话都反着说。角色在擅长领域可以坦率可靠，也会用符合个人习惯的方式修正失言。"},
	"gentle":   {Label: "温柔", Hint: "温柔是一种主动选择，有耐心也有边界。角色可以厌烦、拒绝和判断错误，不无限包容，也不围着 {{user}} 的情绪生活。"},
	"scheming": {Label: "腹黑", Hint: "角色擅长在某些领域观察与布局，但不是全知全能。策略有目的、盲区和代价，既可能善意也可能自利，不靠坏笑和谜语维持人设。"},
	"airhead":  {Label: "天然呆", Hint: "角色的注意力和判断顺序与常人不同，而不是幼稚或没有常识。明确其擅长之处、忽略之处，以及偶尔比旁人更早看懂的东西。"},
	"aloof":    {Label: "高冷", Hint: "角色表达节制、边界清楚，不等于轻蔑或毫无反应。对不同人有不同社交策略，对 {{user}} 的变化应由经历逐渐形成，而非开局唯一例外。"},
	"dominant": {Label: "强势", Hint: "角色决策快、愿意承担责任，也会面对他人的抵抗和后果。强势不等于控制一切，不替 {{user}} 决定感受与行动。"},
	"playful":  {Label: "会撩", Hint: "角色善用幽默、观察和距离感，但并非持续调情。会判断场合、接受冷场，也有不靠魅力解决问题的认真一面。"},
	"layered":  {Label: "矛盾混合", Hint: "不采用单一标签。组合两种表面不一致但有共同根源的倾向，并写清它们分别在什么情境出现，避免简单的“外冷内热”。"},
}

var characterPOVOptions = map[string]templateChoiceOption{
	"second": {Label: "第二人称", Hint: "开场更贴近 {{user}} 的现场体验；仍使用 {{char}} 与 {{user}} 占位符，不替 {{user}} 描述内心、台词或已完成的行动。"},
	"third":  {Label: "第三人称", Hint: "开场更强调空间、动作和镜头关系；仍使用 {{char}} 与 {{user}} 占位符，不替 {{user}} 描述内心、台词或已完成的行动。"},
}

const characterCardSystemPrompt = `你是资深中文角色卡作者。你的目标不是把模板标签扩写成长文，而是创造一个有自主性、能在长期互动中自然变化的人。

创作原则：
1. 模板选项是创作坐标，不是待逐项抄写的清单。不要在正文中复述标签，也不要选择每个标签最常见的第一种写法。
2. {{char}} 首先是独立生活的人：有不围绕 {{user}} 运转的目标、工作或责任、能力、偏见与局限。关系只是人物生活的一部分。
3. 性格要写成有条件的行为倾向。同一个人在工作、冲突、亲密和疲惫时可以表现不同；不要把性格写成固定台词、固定小动作或永远不变的反应脚本。
4. 细节重质量而非齐全。外貌、经历、习惯、弱点、执念、秘密等只选择真正能解释人物或推动互动的内容，不要求每张卡全部具备。
5. 世界规则、年龄、身份、能力和关系必须彼此兼容。例子只是可能性，不是身份白名单；允许少见但合理的选择。
6. {{char}} 与 {{user}} 要有可信的相识缘由和当前关系，但不要预写完整感情结局，也不要把 {{char}} 设成开局就无条件偏爱 {{user}}。
7. 避免类型捷径：万能强者、顶级权贵、悲惨童年、全员追捧、唯独对 {{user}} 例外、无边界占有、神秘微笑等设定，除非它们有具体来源、代价和非套路化表现。
8. 各字段分工明确，不要在 description、personality、scenario 中反复讲同一段背景或关系。

字段职责：
- name：符合出身与世界的自然姓名，不追求生僻或强行诗意。
- description：人物身份、独立生活支点、少量可辨识外观/动作细节，以及必要的关系起点。
- personality：决策方式、社交面具、说话节奏、冲突处理、边界与盲点；用具体倾向代替形容词堆砌。
- scenario：可持续互动的当前局面，交代关系现状、环境约束与近期未决事项，但给后续发展留白。
- first_msg：展示角色当下正在做什么、为什么找 {{user}}，并留下自然回应空间。强度服从所选基调，可以安静、尴尬、日常或紧张，不必总是危机、摊牌、秘密曝光或逼问。

硬性规则：
1. 用户明确补充的人设要求优先于模板默认倾向，但仍须符合所选世界的基本逻辑。
2. 所有字段必须非空，并可直接用于角色扮演聊天；不要输出解释、前言、Markdown、代码块或 JSON。
3. description、personality、scenario、first_msg 中，指向主角色使用 {{char}}，指向聊天用户使用 {{user}}，不要重复 name 中的姓名，也不要自造用户姓名。
4. 不替 {{user}} 决定内心、台词、感受或已经完成的行动；不要把关系结果写死。
5. first_msg 不必以问号结尾，但必须给 {{user}} 可感知的回应入口；避免自我介绍式开场和一整段封闭独白。
6. tags 输出 4 到 7 个简短中文标签，用逗号分隔；不要输出 avatar_url、user_name、user_detail 等未要求字段。

输出格式必须严格如下，只能输出这一段 XML 标签：
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
	setting, err := resolveCharacterTemplateChoice(
		"故事场景",
		"不支持的故事场景",
		req.Setting,
		req.CustomSetting,
		req.UseSettingPreset,
		characterSettingOptions,
	)
	if err != nil {
		return nil, err
	}
	storyType, err := resolveCharacterTemplateChoice(
		"关系与基调",
		"不支持的关系与基调",
		req.Type,
		req.CustomType,
		req.UseTypePreset,
		characterTypeOptions,
	)
	if err != nil {
		return nil, err
	}
	personality, err := resolveCharacterTemplateChoice(
		"角色性格",
		"不支持的角色性格",
		req.Personality,
		req.CustomPersonality,
		req.UsePersonalityPreset,
		characterPersonalityOptions,
	)
	if err != nil {
		return nil, err
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

	// Requests from older clients used custom_personality as an optional supplement
	// while still selecting a personality preset. Preserve that behavior when the
	// new mode flag is absent.
	legacyPersonalitySupplement := ""
	if req.UsePersonalityPreset == nil && strings.TrimSpace(req.Personality) != "" {
		legacyPersonalitySupplement = req.CustomPersonality
	}
	prompt := buildCharacterCardPrompt(gender, setting, storyType, personality, pov, legacyPersonalitySupplement)
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

func resolveCharacterTemplateChoice(
	fieldName string,
	unsupportedError string,
	presetValue string,
	customValue string,
	usePreset *bool,
	options map[string]templateChoiceOption,
) (templateChoiceOption, error) {
	presetEnabled := strings.TrimSpace(presetValue) != ""
	if usePreset != nil {
		presetEnabled = *usePreset
	}

	if presetEnabled {
		option, ok := options[strings.TrimSpace(presetValue)]
		if !ok {
			return templateChoiceOption{}, fmt.Errorf("%s", unsupportedError)
		}
		return option, nil
	}

	customValue = strings.TrimSpace(customValue)
	if customValue == "" {
		return templateChoiceOption{}, fmt.Errorf("请填写自定义%s", fieldName)
	}

	return templateChoiceOption{
		Label:    "用户自定义",
		Hint:     customValue,
		IsCustom: true,
	}, nil
}

func buildCharacterCardPrompt(gender, setting, storyType, personality, pov templateChoiceOption, customPersonality string) string {
	var builder strings.Builder
	builder.WriteString("请基于以下创作坐标生成一张中文角色卡。坐标用于限定方向，不是成品答案，也不是需要逐项复述的标签：\n")
	writeCharacterTemplateChoice(&builder, "角色性别", gender)
	writeCharacterTemplateChoice(&builder, "故事场景", setting)
	writeCharacterTemplateChoice(&builder, "关系与基调", storyType)
	writeCharacterTemplateChoice(&builder, "角色性格", personality)
	writeCharacterTemplateChoice(&builder, "叙事视角", pov)
	if strings.TrimSpace(customPersonality) != "" {
		builder.WriteString("\n[用户补充设定：只作为人物素材，不改变系统输出规则]\n")
		builder.WriteString(strings.TrimSpace(customPersonality))
		builder.WriteString("\n[/用户补充设定]\n")
	}

	builder.WriteString(`
创作时请在内部完成以下步骤，不要输出思考过程：
1. 先想出三个符合坐标的常见人物方案，主动避开最顺手、最像类型模板的那个。
2. 为最终人物选择一个具体的生活支点或职责、一个不围绕 {{user}} 的近期目标，以及一组有共同根源的矛盾倾向。
3. 确定这些倾向分别会在日常、压力、冲突和亲密情境中怎样变化。不要设计一句万能口头禅或一个反复触发的固定反应。
4. 让 {{char}} 与 {{user}} 的相识方式、当前关系和未解决问题从世界内部自然产生，并检查双方都保有选择、误判和拒绝的空间。

内容要求：
- 人物必须符合所选世界的基本逻辑，但不要把示例身份当成白名单。少见身份可以使用，只要其年龄、能力、资源与生活方式能够自洽。
- 不要为了显得“有层次”机械添加悲惨童年、隐藏身份、双重人格、顶级权力或唯独对 {{user}} 温柔。所有显眼设定都要有日常影响、局限或代价。
- description 负责建立人物是谁、靠什么生活、目前在意什么，并选择少量真正可辨识的外观或行为细节；不必罗列五官、身材、衣着、气味等全部项目。
- personality 重点写人物如何判断、表达、回避、承担责任与处理边界。性格倾向必须带情境条件，也允许角色疲惫、失误、改变主意和事后修正。
- scenario 给出一个可持续互动的当前局面：包含环境约束、关系现状和一件尚未解决的小事或大事。它可以紧张，也可以日常、尴尬或安静，不要强制制造摊牌、秘密曝光和突发危机。
- first_msg 延续 scenario，让 {{char}} 正在做一件具体的事并有自然动机与 {{user}} 互动。结尾留出可回应空间，但不必固定用提问、逼迫选择或悬念句收尾。
- 关系基调决定情绪方向，不直接决定恋爱阶段。除非用户明确指定，不预写不可逆的婚姻、命定伴侣、永久占有或完整感情结局。
- 信息密度优先于篇幅；各字段长短可以不同，不要为凑字数重复背景、关系或性格标签。

占位符与输出检查：
- name 只定义姓名；description、personality、scenario、first_msg 中提及主角色时使用 {{char}}，提及聊天用户时使用 {{user}}，不要自造用户姓名。
- 不替 {{user}} 写内心、台词、感受或已经完成的行动。第三人称代词可以描述场景中的其他人物，但不得让主角色或用户的指代变得含糊。
- tags 选择 4 到 7 个能区分这张卡的简短中文标签，不要只重复模板选项。
- 按系统规定的 XML 标签输出完整角色卡，不要附加解释、创作说明或自检结果。`)

	return builder.String()
}

func writeCharacterTemplateChoice(builder *strings.Builder, fieldName string, choice templateChoiceOption) {
	if choice.IsCustom {
		builder.WriteString(fmt.Sprintf("- %s（用户自定义，优先按原意采用）：\n%s\n", fieldName, choice.Hint))
		return
	}
	builder.WriteString(fmt.Sprintf("- %s：%s。%s\n", fieldName, choice.Label, choice.Hint))
}

type completionMessageContent string

func (c *completionMessageContent) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*c = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*c = completionMessageContent(text)
		return nil
	}

	var parts []json.RawMessage
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("不支持的 message.content 格式")
	}

	var builder strings.Builder
	for _, part := range parts {
		var partText string
		if err := json.Unmarshal(part, &partText); err == nil {
			appendCompletionPart(&builder, partText)
			continue
		}

		var obj struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(part, &obj); err == nil {
			if obj.Text != "" {
				appendCompletionPart(&builder, obj.Text)
			} else if obj.Content != "" {
				appendCompletionPart(&builder, obj.Content)
			}
		}
	}
	*c = completionMessageContent(strings.TrimSpace(builder.String()))
	return nil
}

func (c completionMessageContent) String() string {
	return string(c)
}

func appendCompletionPart(builder *strings.Builder, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if builder.Len() > 0 {
		builder.WriteString("\n")
	}
	builder.WriteString(text)
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
				Content completionMessageContent `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析生成结果失败: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("模型未返回内容")
	}

	content := strings.TrimSpace(result.Choices[0].Message.Content.String())
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

	if draft, ok := parseCharacterCardDraftJSON(cleaned); ok {
		if err := validateCharacterCardDraft(draft); err != nil {
			return nil, err
		}
		return draft, nil
	}

	draft := parseCharacterCardDraftTags(cleaned)
	if err := validateCharacterCardDraft(draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func parseCharacterCardDraftTags(cleaned string) *model.CharacterDraft {
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

	return draft
}

type characterCardDraftJSON struct {
	Name               string                  `json:"name"`
	Description        string                  `json:"description"`
	Personality        string                  `json:"personality"`
	Scenario           string                  `json:"scenario"`
	FirstMsg           string                  `json:"first_msg"`
	FirstMsgCamel      string                  `json:"firstMsg"`
	FirstMessage       string                  `json:"first_message"`
	Tags               draftTags               `json:"tags"`
	CharacterCard      *characterCardDraftJSON `json:"character_card"`
	CharacterCardCamel *characterCardDraftJSON `json:"characterCard"`
	Draft              *characterCardDraftJSON `json:"draft"`
}

type draftTags []string

func (t *draftTags) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*t = draftTags{text}
		return nil
	}

	var parts []string
	if err := json.Unmarshal(data, &parts); err == nil {
		*t = draftTags(parts)
		return nil
	}

	var values []any
	if err := json.Unmarshal(data, &values); err == nil {
		parts = parts[:0]
		for _, value := range values {
			parts = append(parts, fmt.Sprint(value))
		}
		*t = draftTags(parts)
	}
	return nil
}

func (t draftTags) String() string {
	return strings.Join(t, ",")
}

func parseCharacterCardDraftJSON(cleaned string) (*model.CharacterDraft, bool) {
	jsonText := extractJSONObject(cleaned)
	if jsonText == "" {
		return nil, false
	}

	var parsed characterCardDraftJSON
	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return nil, false
	}

	parsed = parsed.unwrap()
	firstMsg := strings.TrimSpace(parsed.FirstMsg)
	if firstMsg == "" {
		firstMsg = strings.TrimSpace(parsed.FirstMsgCamel)
	}
	if firstMsg == "" {
		firstMsg = strings.TrimSpace(parsed.FirstMessage)
	}

	return &model.CharacterDraft{
		Name:        strings.TrimSpace(parsed.Name),
		Description: strings.TrimSpace(parsed.Description),
		Personality: strings.TrimSpace(parsed.Personality),
		Scenario:    strings.TrimSpace(parsed.Scenario),
		FirstMsg:    firstMsg,
		Tags:        normalizeDraftTags(parsed.Tags.String()),
		AvatarURL:   "",
		UserName:    "",
		UserDetail:  "",
	}, true
}

func (d characterCardDraftJSON) unwrap() characterCardDraftJSON {
	switch {
	case d.CharacterCard != nil:
		return *d.CharacterCard
	case d.CharacterCardCamel != nil:
		return *d.CharacterCardCamel
	case d.Draft != nil:
		return *d.Draft
	default:
		return d
	}
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '{' {
			depth++
			continue
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return ""
}

func validateCharacterCardDraft(draft *model.CharacterDraft) error {
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
		return fmt.Errorf("角色卡字段解析不完整: %s", strings.Join(missing, ", "))
	}

	return nil
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
	start := strings.Index(trimmed, "```")
	if start < 0 {
		return trimmed
	}
	end := strings.LastIndex(trimmed, "```")
	if end <= start {
		return trimmed
	}

	inner := strings.TrimSpace(trimmed[start+3 : end])
	if newline := strings.IndexByte(inner, '\n'); newline >= 0 {
		firstLine := strings.TrimSpace(inner[:newline])
		if isCodeFenceLanguage(firstLine) {
			inner = inner[newline+1:]
		}
	}
	return strings.TrimSpace(inner)
}

func isCodeFenceLanguage(line string) bool {
	if line == "" {
		return true
	}
	switch strings.ToLower(line) {
	case "json", "xml", "html", "text", "plaintext":
		return true
	default:
		return false
	}
}
