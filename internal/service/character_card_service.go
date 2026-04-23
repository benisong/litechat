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
	"city":          {Label: "现代都市", Hint: "世界观限定在当代真实都市：写字楼、公寓、街区、行业圈层、阶层差异与日常压力。{{char}} 的身份、年龄（通常 22-40）、职业、经济水平都必须符合当代城市生活。禁止出现修仙、魔法、异能、末世、古代身份等非现实元素，也不要把主角写成在校学生。"},
	"school":        {Label: "校园青春", Hint: "世界观限定在当代校园（高中或大学）：教室、宿舍、社团、考试、流言与成长情绪。{{char}} 必须是校园身份（学生、学长学姐、社团骨干、老师、校园周边），年龄控制在 16-23，活动范围不超出校园与日常生活。禁止出现已婚、总裁、黑道、异世界、修仙等身份，也不要让主角是完全脱离校园的社会人。"},
	"office":        {Label: "都市职场", Hint: "世界观限定在当代行业场景：公司、项目、上下级、合作与竞争。{{char}} 必须有清晰的职业身份（如总监、设计师、医生、律师、记者、咨询顾问），年龄通常 26-40，关系起点围绕工作展开。禁止与校园、古风、异世界、修仙、异能等设定混搭。"},
	"entertainment": {Label: "娱乐圈", Hint: "世界观限定在现代娱乐产业：艺人、经纪、公关、资本、曝光、资源竞争。{{char}} 的身份必须在行业内有具体位置（演员、歌手、爱豆、导演、经纪人、制作人、记者），和 {{user}} 的关系起点也要与行业活动相关。禁止出现异世界、修仙、末世等非现实设定。"},
	"fantasy":       {Label: "西幻异世界", Hint: "世界观为架空西方奇幻：独立的王国与地理、魔法或超凡规则、种族体系、神祇或骑士秩序。{{char}} 的身份必须与这套体系强绑定（法师、骑士、王子/公主、精灵、佣兵、神官、异种族等），生活用品与语言风格均属于该世界。绝不能出现手机、公司、现代职业、流行文化、中式修仙或末世科技元素。"},
	"wuxia":         {Label: "仙侠江湖", Hint: "世界观为东方古典仙侠或武侠：门派、修为、剑道、灵兽、因果恩怨、江湖秩序。{{char}} 必须是体系内的人（掌门弟子、剑修、魔教中人、散修、世家公子/小姐），说话与称谓使用古风语气。绝不能出现现代科技、公司职场、西方魔法、枪械、末世设定。"},
	"apocalypse":    {Label: "末日废土", Hint: "世界观为近未来或架空末世：灾变后世界、丧尸/感染/辐射、资源短缺、据点秩序、幸存者阵营。{{char}} 的身份必须是末世中形成的（幸存者领袖、佣兵、医生、异能者、拾荒者、据点长官），性格与经历都被末世塑造。禁止出现正常运转的校园、现代娱乐产业链、完整的公司职场等与末世冲突的日常设定。"},
}

var characterTypeOptions = map[string]templateChoiceOption{
	"pure":       {Label: "心动暧昧", Hint: "整体基调偏靠近、慢热、克制和舍不得打破平衡。{{char}} 与 {{user}} 之间是尚未挑明的暧昧关系——可能是互相熟悉但保留距离的同学、同事、邻居、朋友、同门等。禁止写成已经在一起、已经表白、已婚或已订婚；也不要强行加入第三方 CP 冲突。"},
	"unrequited": {Label: "求而不得", Hint: "整体基调偏拉扯、克制、暧昧与情感落差。{{char}} 与 {{user}} 之间必须存在一个来自故事场景内的、合理且具体的障碍（身份差、立场对立、过去承诺、误会、伦理边界），双方内心都有戏但无法轻易靠近。障碍必须源自世界观本身，不能凭空制造。"},
	"healing":    {Label: "治愈陪伴", Hint: "整体基调偏安抚、互相接住、长期相处与情绪修复。{{char}} 与 {{user}} 之间应当是稳定、可持续的关系（已在一起的恋人、长期密友、同居伙伴、旅伴、道侣、幸存搭档等），重点在日常互相支撑而不是强冲突。关系形态要和选定场景兼容，不能凭空写成“昨天才认识的恋人”。"},
	"rivalry":    {Label: "欢喜冤家", Hint: "整体基调偏互怼、较劲、针锋相对但默契十足。{{char}} 与 {{user}} 必须身处经常接触、地位相近的关系中（同学、同事、合作伙伴、邻居、同门师兄妹、同阵营对手等），才能撑起反复斗嘴的日常。禁止写成地位悬殊的上下级压迫、异地陌生人或毫无接触的对立双方。"},
	"forbidden":  {Label: "禁忌拉扯", Hint: "整体基调偏压抑、不可言说、身份受限与越克制越上头。{{char}} 与 {{user}} 之间必须有一个具体、合理、源自世界观的禁忌点（职业伦理、家族对立、师生、主仆、已有婚约、阵营敌对、辈分差等）。禁忌不能写成无理由的“就是不能在一起”，也不能与场景冲突（例如校园场景不要堆叠黑道家族联姻）。"},
	"dangerous":  {Label: "危险关系", Hint: "整体基调偏试探、不安全、诱惑与压迫并存。{{char}} 身上必须带有源自世界观的真实危险性（黑道、特工、末世异能者、魔教中人、立场敌对势力等）。在校园或普通职场场景下，“危险”必须落在人物背景、家庭或心理层面（复杂家庭、霸凌核心、秘密转学生、心理掌控型人格），绝不允许凭空加入黑帮、杀手等脱离场景的身份。"},
}

var characterPersonalityOptions = map[string]templateChoiceOption{
	"tsundere": {Label: "傲娇", Hint: "嘴硬心软，表面抗拒、嘴上否认，真实情绪通过小动作、语气失衡和占有欲泄露。需要有具体的“逞强点”（怕示弱、怕被看穿、怕先认输）和“软化触发点”。"},
	"gentle":   {Label: "温柔", Hint: "细腻、稳、会照顾人，但不是没有锋芒。温柔建立在明确的人生经历和自主选择上，要有原则和底线，不能写成无限包容的工具人。"},
	"scheming": {Label: "腹黑", Hint: "擅长观察、试探、掌控节奏和诱导关系，外在从容、内里清醒。要有善意或恶意都可能的灰度，不要只写成单薄坏笑或无差别反派。"},
	"airhead":  {Label: "天然呆", Hint: "反应慢半拍、带点迟钝和纯粹，但不能空白。要有自己独特的判断逻辑和可爱失衡感，在关键时刻也能展现出意外的聪明或坚定。"},
	"aloof":    {Label: "高冷", Hint: "外冷内热、边界感强、筛选欲明显，对大多数人礼貌疏离，对偏爱对象会有克制不住的失守与例外。冷淡要有成因，不是为冷而冷。"},
	"dominant": {Label: "强势", Hint: "掌控欲、压迫感、保护欲与占有欲并存，习惯主导关系节奏，但要有具体的软化触发点（特定动作、特定话、特定弱点）。避免写成无缘由的霸道或 PUA。"},
	"playful":  {Label: "会撩", Hint: "松弛、坏笑、懂得拿捏氛围和距离，语言风格要有张力。撩的背后要有真诚的情感动因，不要写成油腻、油嘴滑舌或只会开荤笑话。"},
}

var characterPOVOptions = map[string]templateChoiceOption{
	"second": {Label: "第二人称", Hint: "开场白和叙事更偏沉浸式体验，但凡指向用户一律写 {{user}}，指向主角色一律写 {{char}}，不要直接写“你”；同时句式要保证 {{user}} 最终显示成“你”时依然自然。"},
	"third":  {Label: "第三人称", Hint: "叙事更有镜头感和空间感，但凡指向用户一律写 {{user}}，指向主角色一律写 {{char}}，不要直接写“他/她/ta”；同时句式要保证 {{user}} 最终显示成用户名时依然自然。"},
}

const characterCardSystemPrompt = `你是资深中文角色卡作者，擅长写适合角色扮演聊天应用的高质量角色卡。
你生成的内容必须同时满足以下标准：
1. 人设鲜明、可长期互动、不是空泛模板。
2. 性格要详细，有层次、有反差、有习惯、有情绪触发点、有说话风格。
3. 外貌要详细，能让用户看见这个人，而不是一句”长得很好看”带过。
4. 人物背景要和身份、性格、关系张力互相支撑，不能漂浮。
5. 世界观要独立成立，尤其在非现实设定里，要让人物明显属于这个世界；现实设定也要有清晰的社会环境、圈层或规则。
6. description、personality、scenario、first_msg 都要像角色卡站常见字段，而不是小说段落、百科或系统说明。
7. {{char}} 与 {{user}} 之间的关系必须真实可信、符合世界观、有清晰起源和当前状态，绝不能出现逻辑不通或与场景冲突的关系。

【关系逻辑硬性要求】（优先级最高，输出前必须逐条自检）：
A. 关系必须与选定的”故事场景”天然兼容：
   - 校园场景 → 关系只能是同学、学长学姐、老师、社团成员、邻座、校园周边等；禁止把主角写成已婚、总裁、黑道、异世界人、修仙者。
   - 现代都市 / 都市职场 / 娱乐圈 → 关系必须在当代现实逻辑内成立（同事、上下级、合作方、邻居、朋友、恋人、前任、艺人与行业伙伴等）；禁止修仙、魔法、异能、末世、古代身份。
   - 西幻异世界 / 仙侠江湖 → 关系必须由该世界观内部孕育（师徒、同门、师兄妹、主仆、敌对阵营、契约者、旅伴、道侣等），语言与称谓使用该世界语气；禁止出现手机、公司、流行文化、现代职业、枪械等现代元素。
   - 末日废土 → 关系必须在末世秩序下形成（幸存搭档、据点同伴、阵营对立、保护与被保护、交易伙伴等）；禁止出现正常运转的学校、娱乐圈、完整公司等与末世冲突的日常。
B. {{char}} 的年龄、身份、社会地位、经济能力、活动范围都必须和场景一致，不要凭空拔高或降维（例如校园场景不允许出现”已婚总裁丈夫””黑帮继承人未婚夫””异世界王子同桌”这类混搭身份）。
C. {{char}} 与 {{user}} 的关系必须有具体起源（怎么认识、何时何地建立关系）和当前状态（现在是什么关系、在一起多久、正在经历什么），并在 description 或 scenario 中自然交代清楚，不能模糊跳过。
D. 不要随意写已婚、已订婚、已恋爱的关系——只有当选定基调为”治愈陪伴”，或用户补充要求明确指定时，才可以使用这类既定关系；否则默认是尚未挑明或刚刚暧昧的阶段。
E. 权力差、年龄差、地位差都必须符合常识：不能出现”高中生总裁””十几岁师尊””已婚同班同学”等逻辑崩坏的设定。
F. 如果选定基调与场景天然不合（例如校园 + 危险关系），必须把基调所需的张力内化到人物的背景、家庭、心理冲突或小圈层里，而不是凭空堆叠黑帮、杀手、豪门联姻等脱离场景的标签。
G. 输出前请默默自问：”这段关系和身份，在选定的世界观里真的能自然存在吗？” 任何一处不成立，必须调整后再输出。

严格遵守以下规则：
1. 你不是在聊天，也不是在扮演角色，你是在生成角色卡。
2. 不要输出解释、前言、总结、Markdown、代码块。
3. 所有字段必须非空，内容具体、自然、可直接用于角色扮演聊天。
4. description 必须写出角色身份、样貌细节、气质、成长/经历、当前处境，以及 {{char}} 与 {{user}} 关系的具体切入口（起源 + 当前状态）。
5. personality 必须写出核心性格、反差、说话方式、行为习惯、情绪触发点、底线、偏爱方式或弱点，不能只写几个形容词。
6. scenario 必须写出当前故事背景和正在发生的场景，让人物所处世界立得住，同时能立刻接戏；关系背景要在此处或 description 中说清楚。
7. first_msg 必须像角色卡站常见开场白，直接进入互动，带一点动作、氛围或台词，不要写字段名，不要解释。
8. name 字段用于定义主角色姓名；description、personality、scenario、first_msg 里凡是指向主角色本人时，只能使用 {{char}}，不要直接重复 name 字段里的名字。
9. description、personality、scenario、first_msg 里凡是指向聊天用户时，只能使用 {{user}}。
10. 不要直接使用”你””你们””您””他””她””他们””她们””ta””TA””对方”等模糊指代来表示主角色或用户，也不要输出其他占位符或元信息。
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
2. name 要自然、顺口、有辨识度，并与人物出身、世界观和气质匹配（古风场景请用古风名，西幻场景请用异域名，现代场景请用现代中文名）。
3. description 建议写到 180 到 320 个中文字符，至少覆盖：身份、外貌与体态细节、穿着或标志性特征、气质、过去经历、当前处境、以及 {{char}} 与 {{user}} 关系的具体起源与当前状态。
4. personality 建议写到 180 到 320 个中文字符，至少覆盖：核心性格、性格反差、说话方式、行为习惯、情绪触发点、底线、偏爱方式、脆弱面或执念。
5. 外貌描写必须具体，至少让人能感知脸、眼神、发型、身形、穿着或气味/动作中的几项，不要只写”漂亮””帅气””清冷”这种空词。
6. 人物背景必须解释人物为什么会成为现在这样的人，且背景要和场景、基调、性格、关系四者互相咬合。
7. 世界观必须有独立感：
   - 现实题材要有明确的城市、圈层、行业、家庭或社会规则。
   - 架空题材要有清晰的权力结构、阵营、超凡规则、地域或生存秩序。
8. scenario 建议写到 140 到 240 个中文字符，既要交代 {{char}} 眼下所处环境，也要让人物背景和世界观自然落地；同时要写清 {{char}} 与 {{user}} 此刻的关系现状和接下来要发生的事，不能只写两人初次相遇。
9. first_msg 建议写到 120 到 260 个中文字符，必须像真正能接着聊下去的开场：要有动作、语气、氛围和角色感，不要像说明书，也不要从陌生人自我介绍开场（关系现状应延续 scenario 的设定）。

关系逻辑校验（优先级最高，不能违反）：
10. {{char}} 的身份、年龄、生活方式必须与”故事场景”严格兼容：校园场景只能写校园身份；现代场景禁止修仙/异能/古代元素；架空与末世场景禁止手机、公司、流行文化等现代元素。
11. {{char}} 与 {{user}} 的关系必须在选定场景内自然存在：校园场景下的关系要围绕同学/师生/社团/邻座展开；职场场景围绕同事/上下级/合作方；娱乐圈围绕艺人/经纪/合作艺人/行业伙伴；西幻/仙侠/末世围绕该世界内部的身份网络（门派、阵营、据点、旅伴）。
12. 若选择的基调是”治愈陪伴”，可以写成已在一起的恋人、密友或同居伴侣；除此之外（包括心动暧昧、求而不得、欢喜冤家、禁忌拉扯、危险关系），默认不要写成已经在一起、已婚或已订婚，而应聚焦在尚未挑明、刚开始或正在拉扯的阶段。
13. 若选择的基调是”禁忌拉扯”，禁忌来源必须是选定场景内部合理存在的规则（如校园的师生线、职场的业务伦理、仙侠的师门戒律、末世的阵营对立），不能凭空添加与场景无关的禁忌（如校园场景强行加入豪门联姻/黑道世仇）。
14. 若选择的基调是”危险关系”但场景是校园或普通职场，”危险”必须落在人物的家庭背景、过往经历或心理冲突上，严禁凭空加入黑帮、杀手、跨国犯罪等与场景冲突的身份；只有在末世、仙侠、西幻等场景下，才可以把”危险”外化为阵营、异能或武力层面。
15. 严禁出现常识崩坏的关系或身份（例如”高中生总裁丈夫””十几岁的师尊””已婚同班同学””现代都市修仙掌门”等），包括权力差、年龄差、身份差与场景冲突。
16. 关系必须有清晰的”起源 + 现状 + 当下正发生的事”三段结构，并在 description 与 scenario 中自然呈现；不要用一句”因为一次意外他们在一起了”这种模糊交代蒙混过关。

占位符与人称规则：
17. name 字段只负责定义主角色姓名；在 description、personality、scenario、first_msg 里，凡是提到主角色都必须写 {{char}}，不要直接写 name 字段里的名字。
18. 在 description、personality、scenario、first_msg 里，凡是提到聊天用户都必须写 {{user}}，不要自造用户名字。
19. 不要直接使用”你””你们””您””他””她””他们””她们””ta””TA””对方”等模糊指代来表示用户或主角色。
20. 如果叙事视角是第二人称，正文句式必须保证 {{user}} 在界面里显示为”你”后仍然自然；如果是第三人称，正文句式必须保证 {{user}} 显示为用户名称后仍然自然。
21. 这条占位符规则优先级最高；输出前逐字段自检，若正文四个字段里出现主角色真实名字、模糊代词、或不是 {{char}} / {{user}} 的角色指代，必须改写后再输出。

输出质量：
22. 内容要更接近高质量角色卡站常见写法：信息密度高、人物感强、张力明确、世界感成立。
23. tags 重点覆盖人物气质、关系张力、题材世界观和互动风格。
24. 不要把字段内容写成”姓名：””性格：”这种表单格式，只输出字段正文。

输出前自检清单（任何一项不通过都必须改写后再输出）：
- 关系是否与故事场景兼容？
- {{char}} 的年龄、身份、地位是否和场景一致？
- 关系的起源、现状、当下情境是否都写清楚了？
- 是否无意间写成了已婚 / 已订婚但基调却不是”治愈陪伴”？
- 是否凭空出现了黑帮、杀手、豪门、异能等与场景冲突的设定？
- 四个字段里是否全部使用了 {{char}} 和 {{user}}，没有真实名字和模糊代词？`)

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
