package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"litechat/internal/store"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ChatService 聊天服务：封装会话、角色卡、预设、世界书与摘要，串起整条聊天链路。
type ChatService struct {
	chatStore      *store.ChatStore
	messageStore   *store.MessageStore
	characterStore *store.CharacterStore
	presetStore    *store.PresetStore
	worldBookStore *store.WorldBookStore
	configStore    *store.ConfigStore
	userStore      *store.UserStore
	summaryService *SummaryService
}

func NewChatService(
	chatStore *store.ChatStore,
	messageStore *store.MessageStore,
	characterStore *store.CharacterStore,
	presetStore *store.PresetStore,
	worldBookStore *store.WorldBookStore,
	configStore *store.ConfigStore,
	userStore *store.UserStore,
	summaryService *SummaryService,
) *ChatService {
	return &ChatService{
		chatStore:      chatStore,
		messageStore:   messageStore,
		characterStore: characterStore,
		presetStore:    presetStore,
		worldBookStore: worldBookStore,
		configStore:    configStore,
		userStore:      userStore,
		summaryService: summaryService,
	}
}

// StreamCallback SSE 流式回调：每生成一个增量 token 就回调一次，用于边生成边推送。
type StreamCallback func(token string) error

// SendMessage 处理一次用户发送：落库用户消息、组装上下文、流式请求模型，再落库 AI 回复并触发摘要。
func (s *ChatService) SendMessage(chatID, content, presetID, userID string, callback StreamCallback) (string, error) {
	// 读取会话并校验归属当前用户
	chat, err := s.chatStore.GetByID(chatID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to load chat: %w", err)
	}

	// 读取会话对应的角色卡
	character, err := s.characterStore.GetByID(chat.CharacterID, userID)
	if err != nil {
		return "", fmt.Errorf("failed to load character: %w", err)
	}

	// 确定使用的预设：优先用请求指定的，其次用会话绑定的
	presetIDToUse := presetID
	if presetIDToUse == "" {
		presetIDToUse = chat.PresetID
	}

	// 判断当前是否为 service 运行模式
	isServiceMode := s.userStore.GetCurrentMode() == "service"

	var preset *model.Preset
	if presetIDToUse != "" {
		preset, err = s.presetStore.GetByID(presetIDToUse, userID)
		if err != nil {
			log.Printf("[preset] failed to load preset %s: %v", presetIDToUse, err)
			preset = nil
		}
	}
	if preset == nil {
		if isServiceMode {
			// service mode: load the admin default preset
			preset, err = s.presetStore.GetDefaultAdmin()
			if err != nil {
				log.Printf("[preset] failed to load admin preset in service mode: %v; using built-in fallback", err)
			}
		} else {
			preset, err = s.presetStore.GetDefault(userID)
			if err != nil {
				log.Printf("[preset] failed to load default preset: %v; using built-in fallback", err)
			}
		}
		if preset == nil {
			preset = &model.Preset{
				SystemPrompt: "You are {{char}}. Stay in character based on the role card.",
				Temperature:  0.8,
				MaxTokens:    2048,
				TopP:         0.9,
			}
		}
	}
	// 记录最终选用的预设信息，便于排查预设是否生效
	hasPrompts := preset.Prompts != ""
	log.Printf("[preset] 选用预设 name=%s id=%s user=%s 含多段提示词=%v prompts长度=%d",
		preset.Name, preset.ID, preset.UserID, hasPrompts, len(preset.Prompts))

	// 读取该会话的历史消息
	history, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("读取历史消息失败: %w", err)
	}

	// 首次对话且角色带开场白：先把开场白作为第一条 assistant 消息落库
	if len(history) == 0 && character.FirstMsg != "" {
		firstMsg := &model.Message{
			ChatID:  chatID,
			Role:    "assistant",
			Content: s.replaceRoleCardText(character.FirstMsg, character, userID),
		}
		if err := s.messageStore.Create(firstMsg); err != nil {
			log.Printf("[chat] 落库开场白失败: %v", err)
		} else {
			// 落库成功后把开场白补进历史，供后续上下文使用
			history = append(history, firstMsg)
		}
	}

	// 把本次用户输入落库
	userMsg := &model.Message{
		ChatID:  chatID,
		Role:    "user",
		Content: content,
	}
	if err := s.messageStore.Create(userMsg); err != nil {
		return "", fmt.Errorf("保存用户消息失败: %w", err)
	}

	// 组装发送给模型的完整消息列表
	messages := s.buildMessages(chatID, preset, character, history, content, userID)

	// 调试模式：把本次请求的完整消息 dump 到文件，便于排查
	if DebugEnabled {
		var msgDebug strings.Builder
		msgDebug.WriteString(fmt.Sprintf("=== %s 预设：%s (ID: %s) 消息条数: %d ===\n\n",
			time.Now().Format("15:04:05"), preset.Name, preset.ID, len(messages)))
		for i, m := range messages {
			msgDebug.WriteString(fmt.Sprintf("[%d] role=%s\n%s\n\n", i, m.Role, m.Content))
		}
		debugFile := fmt.Sprintf("data/debug_messages_%d.txt", time.Now().UnixMilli())
		os.WriteFile(debugFile, []byte(msgDebug.String()), 0644)
		log.Printf("[debug] 已写出请求消息到 %s（共 %d 条）", debugFile, len(messages))
	}

	// 读取 API 配置（endpoint / key / 模型）
	settings, err := s.configStore.GetSettings()
	if err != nil {
		return "", fmt.Errorf("读取设置失败: %w", err)
	}

	// 带格式校验地流式请求模型
	fullResponse, err := s.callFormattedOpenAIStream(settings, preset, messages, callback)

	// 记录本次响应（调试模式下会写文件）
	s.debugLogResponse(chatID, fullResponse, err)

	if err != nil {
		return "", err
	}

	// 把模型回复落库
	aiMsg := &model.Message{
		ChatID:  chatID,
		Role:    "assistant",
		Content: fullResponse,
	}
	if err := s.messageStore.Create(aiMsg); err != nil {
		return "", fmt.Errorf("保存 AI 回复失败: %w", err)
	}
	if s.summaryService != nil {
		s.summaryService.OnAssistantMessageStored(chatID)
	}

	// 刷新会话的 updated_at 时间
	_ = s.chatStore.Touch(chatID, userID)

	return fullResponse, nil
}

// Regenerate 重新生成最后一条 AI 回复：删除旧回复，用同一条用户输入重新请求模型。
func (s *ChatService) Regenerate(chatID, userID string, callback StreamCallback) (string, error) {
	// 读取全部消息
	allMessages, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("failed to load messages: %w", err)
	}
	if len(allMessages) == 0 {
		return "", fmt.Errorf("no messages available for regeneration")
	}

	// 从后往前找到最后一条 assistant 回复
	var lastAiIdx = -1
	for i := len(allMessages) - 1; i >= 0; i-- {
		if allMessages[i].Role == "assistant" {
			lastAiIdx = i
			break
		}
	}
	if lastAiIdx < 0 {
		return "", fmt.Errorf("no assistant reply available for regeneration")
	}

	oldAISeq := allMessages[lastAiIdx].Seq
	if err := s.messageStore.DeleteByID(allMessages[lastAiIdx].ID); err != nil {
		return "", fmt.Errorf("failed to delete previous assistant reply: %w", err)
	}

	// 找到该 AI 回复对应的上一条用户输入
	var lastUserContent string
	for i := lastAiIdx - 1; i >= 0; i-- {
		if allMessages[i].Role == "user" {
			lastUserContent = allMessages[i].Content
			break
		}
	}
	if lastUserContent == "" {
		return "", fmt.Errorf("could not find the matching user message")
	}

	// 读取会话
	chat, err := s.chatStore.GetByID(chatID, userID)
	if err != nil {
		return "", fmt.Errorf("读取会话失败: %w", err)
	}

	// 读取角色卡
	character, err := s.characterStore.GetByID(chat.CharacterID, userID)
	if err != nil {
		return "", fmt.Errorf("读取角色卡失败: %w", err)
	}

	// 加载预设
	preset := s.loadPreset(chat.PresetID, "", userID)

	// 读取历史消息（用于重建上下文）
	history, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("读取历史消息失败: %w", err)
	}
	// 若历史末尾是用户消息，先去掉（它会作为本次输入重新发送）
	if len(history) > 0 && history[len(history)-1].Role == "user" {
		history = history[:len(history)-1]
	}

	// 用上一条用户输入重新组装消息
	messages := s.buildMessages(chatID, preset, character, history, lastUserContent, userID)

	// 读取 API 配置
	settings, err := s.configStore.GetSettings()
	if err != nil {
		return "", fmt.Errorf("读取设置失败: %w", err)
	}

	// 重新流式请求模型
	fullResponse, err := s.callFormattedOpenAIStream(settings, preset, messages, callback)
	s.debugLogResponse(chatID, fullResponse, err)
	if err != nil {
		return "", err
	}

	// 把新的回复落库
	aiMsg := &model.Message{
		ChatID:  chatID,
		Role:    "assistant",
		Content: fullResponse,
	}
	if err := s.messageStore.Create(aiMsg); err != nil {
		return "", fmt.Errorf("保存 AI 回复失败: %w", err)
	}
	if s.summaryService != nil {
		s.summaryService.InvalidateFromSeq(chatID, oldAISeq)
		s.summaryService.OnAssistantMessageStored(chatID)
	}

	_ = s.chatStore.Touch(chatID, userID)
	return fullResponse, nil
}

// RetryLastOrRegenerate 在“重试”和“重新生成”之间自动选择：
// 末尾是 AI 回复就重新生成；末尾是用户消息就直接用它再请求一次。
func (s *ChatService) RetryLastOrRegenerate(chatID, userID string, callback StreamCallback) (string, error) {
	allMessages, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("failed to load messages: %w", err)
	}
	if len(allMessages) == 0 {
		return "", fmt.Errorf("no messages available for regeneration")
	}

	lastMessage := allMessages[len(allMessages)-1]
	if lastMessage.Role == "assistant" {
		return s.Regenerate(chatID, userID, callback)
	}
	if lastMessage.Role != "user" {
		return "", fmt.Errorf("最后一条消息既不是用户消息也不是 AI 回复，无法重试")
	}

	lastUserContent := lastMessage.Content

	chat, err := s.chatStore.GetByID(chatID, userID)
	if err != nil {
		return "", fmt.Errorf("读取会话失败: %w", err)
	}

	character, err := s.characterStore.GetByID(chat.CharacterID, userID)
	if err != nil {
		return "", fmt.Errorf("读取角色卡失败: %w", err)
	}

	preset := s.loadPreset(chat.PresetID, "", userID)

	history, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("读取历史消息失败: %w", err)
	}
	if len(history) > 0 && history[len(history)-1].Role == "user" {
		history = history[:len(history)-1]
	}

	messages := s.buildMessages(chatID, preset, character, history, lastUserContent, userID)

	settings, err := s.configStore.GetSettings()
	if err != nil {
		return "", fmt.Errorf("读取设置失败: %w", err)
	}

	fullResponse, err := s.callFormattedOpenAIStream(settings, preset, messages, callback)
	s.debugLogResponse(chatID, fullResponse, err)
	if err != nil {
		return "", err
	}

	aiMsg := &model.Message{
		ChatID:  chatID,
		Role:    "assistant",
		Content: fullResponse,
	}
	if err := s.messageStore.Create(aiMsg); err != nil {
		return "", fmt.Errorf("保存 AI 回复失败: %w", err)
	}
	if s.summaryService != nil {
		s.summaryService.OnAssistantMessageStored(chatID)
	}

	_ = s.chatStore.Touch(chatID, userID)
	return fullResponse, nil
}

func (s *ChatService) loadPreset(chatPresetID, requestPresetID, userID string) *model.Preset {
	presetIDToUse := requestPresetID
	if presetIDToUse == "" {
		presetIDToUse = chatPresetID
	}
	var preset *model.Preset
	var err error
	if presetIDToUse != "" {
		preset, err = s.presetStore.GetByID(presetIDToUse, userID)
		if err != nil {
			preset = nil
		}
	}
	if preset == nil {
		if s.userStore.GetCurrentMode() == "service" {
			preset, err = s.presetStore.GetDefaultAdmin()
		} else {
			preset, err = s.presetStore.GetDefault(userID)
		}
		if err != nil {
			preset = &model.Preset{
				SystemPrompt: "You are {{char}}. Stay in character based on the role card.",
				Temperature:  0.8,
				MaxTokens:    2048,
				TopP:         0.9,
			}
		}
	}
	return preset
}

// replaceVars helper: get the current user profile used by template variables
func (s *ChatService) getDefaultUserProfile(userID string) (string, string, bool) {
	user, err := s.userStore.GetByID(userID)
	if err != nil {
		return "user", "", false
	}

	userName := strings.TrimSpace(user.UserName)
	userDetail := strings.TrimSpace(user.UserDetail)
	if userName == "" {
		userName = "user"
	}

	isCustom := !strings.EqualFold(userName, "user") || userDetail != ""
	return userName, userDetail, isCustom
}

// getUserName returns the user name used by template variables
func (s *ChatService) getUserName(char *model.Character, userID string) string {
	if char.UseCustomUser && char.UserName != "" {
		return char.UserName
	}
	userName, _, _ := s.getDefaultUserProfile(userID)
	return userName
}

// getUserDetail returns the user detail used by template variables
func (s *ChatService) getUserDetail(char *model.Character, userID string) string {
	if char.UseCustomUser {
		return char.UserDetail
	}
	_, userDetail, _ := s.getDefaultUserProfile(userID)
	return userDetail
}

func (s *ChatService) getCharacterPOV(char *model.Character) string {
	if char == nil {
		return "third"
	}
	if strings.EqualFold(strings.TrimSpace(char.POV), "second") {
		return "second"
	}
	return "third"
}

type characterGenderHint struct {
	Label           string
	Pronoun         string
	OppositePronoun string
}

func inferCharacterGenderHint(char *model.Character) characterGenderHint {
	if char == nil {
		return characterGenderHint{}
	}

	cardText := strings.ToLower(strings.Join([]string{
		char.Description,
		char.Personality,
		char.Scenario,
		char.FirstMsg,
		char.Tags,
	}, "\n"))
	compact := strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "", "　", "").Replace(cardText)

	explicitFemale := containsAnyMarker(compact, []string{
		"性别女", "性别:女", "性别：女", "性别=女", "性别-女",
		"性别为女", "性别是女", "性别女性", "性别:女性", "性别：女性",
	})
	explicitMale := containsAnyMarker(compact, []string{
		"性别男", "性别:男", "性别：男", "性别=男", "性别-男",
		"性别为男", "性别是男", "性别男性", "性别:男性", "性别：男性",
	})

	if explicitFemale != explicitMale {
		if explicitFemale {
			return characterGenderHint{Label: "女性", Pronoun: "她", OppositePronoun: "他"}
		}
		return characterGenderHint{Label: "男性", Pronoun: "他", OppositePronoun: "她"}
	}
	if explicitFemale && explicitMale {
		return characterGenderHint{}
	}

	female := containsAnyMarker(compact, []string{
		"女性", "女生", "女孩", "少女", "女人", "女子", "女主", "女友",
	})
	male := containsAnyMarker(compact, []string{
		"男性", "男生", "男孩", "少年", "男人", "男子", "男主", "男友",
	})
	if female == male {
		return characterGenderHint{}
	}
	if female {
		return characterGenderHint{Label: "女性", Pronoun: "她", OppositePronoun: "他"}
	}
	return characterGenderHint{Label: "男性", Pronoun: "他", OppositePronoun: "她"}
}

func containsAnyMarker(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (s *ChatService) buildRoleIdentityPrompt(char *model.Character, userID string) string {
	if char == nil {
		return ""
	}

	charName := strings.TrimSpace(char.Name)
	if charName == "" {
		charName = "character"
	}

	userName := strings.TrimSpace(s.getUserName(char, userID))
	if userName == "" {
		userName = "user"
	}

	genderHint := inferCharacterGenderHint(char)

	var builder strings.Builder
	builder.WriteString("[Identity Anchor]\n")
	builder.WriteString(fmt.Sprintf("- You are %s. Always roleplay as this character.\n", charName))
	builder.WriteString(fmt.Sprintf("- The chat user is %s. Never roleplay as %s.\n", userName, userName))
	builder.WriteString("- Never swap, merge, or confuse the identities of the character and the user.\n")
	builder.WriteString(fmt.Sprintf("- Any role-card mention of %s refers to you, the character.\n", charName))
	builder.WriteString(fmt.Sprintf("- Any role-card mention of %s refers to the user.\n", userName))
	if genderHint.Label != "" {
		builder.WriteString(fmt.Sprintf("- 角色卡性别锚点：%s 的性别/称谓倾向是%s；旁白里描述 %s 时必须使用“%s”和相符称谓，不要写成“%s”。\n", charName, genderHint.Label, charName, genderHint.Pronoun, genderHint.OppositePronoun))
	} else {
		builder.WriteString(fmt.Sprintf("- 必须服从 %s 角色卡里明确写出的性别、代词、身份称谓；不要根据姓名、语气或用户内容自行改性别。\n", charName))
	}
	if s.getCharacterPOV(char) == "second" {
		builder.WriteString(fmt.Sprintf("- This role card may use second-person wording in the scenario or first message. If it uses \"you\" or \"你\", it refers to %s, the user, not to you.\n", userName))
		builder.WriteString(fmt.Sprintf("- 二人称写作规则：描写用户的动作、感受、处境或对话称呼时用“你”，不要反复用 %s 当主语。\n", userName))
		builder.WriteString(fmt.Sprintf("- 描写 %s 的动作、神态、心理或旁白时，用 %s 或与角色性别一致的第三人称代词；不要用“我”替代 %s。\n", charName, charName, charName))
		builder.WriteString(fmt.Sprintf("- “我”只允许出现在 %s 的直接台词、清晰标注的内心独白或角色自述中，不能用于普通旁白。\n", charName))
		builder.WriteString(fmt.Sprintf("- Only use %s as a literal name when the scene explicitly requires the name itself, such as letters, forms, signatures, quoted text, roll call, or deliberate emphasis.\n", userName))
	} else {
		builder.WriteString(fmt.Sprintf("- This role card uses third-person POV for the user. Mentions of %s still refer to the user, not to you.\n", userName))
	}
	builder.WriteString("- Keep this identity mapping stable for the entire conversation.")
	return builder.String()
}

func (s *ChatService) buildPersistentRoleCardPrompt(char *model.Character, userID string) string {
	if char == nil {
		return ""
	}

	charName := strings.TrimSpace(char.Name)
	if charName == "" {
		charName = "character"
	}

	userName := strings.TrimSpace(s.getUserName(char, userID))
	if userName == "" {
		userName = "user"
	}

	renderRoleFacts := func(text string) string {
		return strings.TrimSpace(replaceRoleRefs(text, charName, userName))
	}

	var builder strings.Builder
	builder.WriteString("[Persistent Role Card]\n")
	builder.WriteString("This role-card snapshot is authoritative and must remain active for the whole conversation. Long-term memory, summaries, and recent chat may add continuity, but they must not override these original character settings.\n")
	builder.WriteString(fmt.Sprintf("Character Name: %s\n", charName))
	builder.WriteString(fmt.Sprintf("User Name: %s\n", userName))

	if userDetail := renderRoleFacts(s.getUserDetail(char, userID)); userDetail != "" {
		builder.WriteString("User Detail:\n")
		builder.WriteString(userDetail)
		builder.WriteString("\n")
	}
	appendRoleCardSection(&builder, "Description", renderRoleFacts(char.Description))
	appendRoleCardSection(&builder, "Personality", renderRoleFacts(char.Personality))
	appendRoleCardSection(&builder, "Scenario", strings.TrimSpace(s.replaceRoleCardText(char.Scenario, char, userID)))
	appendRoleCardSection(&builder, "Opening Message Reference", strings.TrimSpace(s.replaceRoleCardText(char.FirstMsg, char, userID)))
	appendRoleCardSection(&builder, "Tags", renderRoleFacts(char.Tags))
	return strings.TrimSpace(builder.String())
}

func appendRoleCardSection(builder *strings.Builder, title, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	builder.WriteString("\n")
	builder.WriteString(title)
	builder.WriteString(":\n")
	builder.WriteString(content)
	builder.WriteString("\n")
}

func replaceRoleRefs(text, charName, userRef string) string {
	text = strings.ReplaceAll(text, "{{user}}", userRef)
	text = strings.ReplaceAll(text, "{{User}}", userRef)
	text = strings.ReplaceAll(text, "{{char}}", charName)
	text = strings.ReplaceAll(text, "{{Char}}", charName)
	return text
}

func (s *ChatService) getRoleCardUserReference(char *model.Character, userID string) string {
	if s.getCharacterPOV(char) == "second" {
		return "你"
	}

	userName := strings.TrimSpace(s.getUserName(char, userID))
	if userName == "" {
		return "user"
	}
	return userName
}

func (s *ChatService) replaceRoleCardText(text string, char *model.Character, userID string) string {
	if char == nil {
		return text
	}

	charName := strings.TrimSpace(char.Name)
	if charName == "" {
		charName = "character"
	}

	result := replaceRoleRefs(text, charName, s.getRoleCardUserReference(char, userID))
	now := time.Now()
	result = strings.ReplaceAll(result, "{{time}}", now.Format("15:04"))
	result = strings.ReplaceAll(result, "{{date}}", now.Format("2006-01-02"))
	result = strings.ReplaceAll(result, "{{weekday}}", now.Weekday().String())
	result = strings.ReplaceAll(result, "{{isotime}}", now.Format(time.RFC3339))
	result = strings.ReplaceAll(result, "{{time_UTC}}", now.UTC().Format("15:04"))
	return processDynamicMacros(result)
}

func (s *ChatService) replaceVars(template string, char *model.Character, userID string) string {
	result := template

	defaultUserName, defaultUserDetail, hasCustomProfile := s.getDefaultUserProfile(userID)
	userName := defaultUserName
	if char.UseCustomUser && char.UserName != "" {
		userName = char.UserName
	}
	resolveRoleRefs := func(text string) string {
		return replaceRoleRefs(text, char.Name, userName)
	}
	result = resolveRoleRefs(result)

	// {{user}} / {{char}} 等基础引用已替换，下面处理资料与正文变量

	// {{description}}：按需在角色描述前拼接用户资料块（[User Info]）
	userDetail := defaultUserDetail
	if char.UseCustomUser {
		userDetail = char.UserDetail
	}
	descWithUserInfo := resolveRoleRefs(char.Description)
	if char.UseCustomUser && (char.UserName != "" || userDetail != "") {
		var userInfoBlock strings.Builder
		userInfoBlock.WriteString("[User Info]\n")
		userInfoBlock.WriteString("Name: " + userName + "\n")
		if userDetail != "" {
			userInfoBlock.WriteString("Detail: " + userDetail + "\n")
		}
		userInfoBlock.WriteString("\n")
		descWithUserInfo = userInfoBlock.String() + resolveRoleRefs(char.Description)
	} else if hasCustomProfile {
		var userInfoBlock strings.Builder
		userInfoBlock.WriteString("[User Info]\n")
		userInfoBlock.WriteString("Name: " + userName + "\n")
		if userDetail != "" {
			userInfoBlock.WriteString("Detail: " + userDetail + "\n")
		}
		userInfoBlock.WriteString("\n")
		descWithUserInfo = userInfoBlock.String() + resolveRoleRefs(char.Description)
	}
	result = strings.ReplaceAll(result, "{{description}}", descWithUserInfo)
	result = strings.ReplaceAll(result, "{{personality}}", resolveRoleRefs(char.Personality))
	result = strings.ReplaceAll(result, "{{scenario}}", s.replaceRoleCardText(char.Scenario, char, userID))
	result = resolveRoleRefs(result)

	// 处理时间类模板变量：{{time}}/{{date}}/{{weekday}} 等
	now := time.Now()
	result = strings.ReplaceAll(result, "{{time}}", now.Format("15:04"))
	result = strings.ReplaceAll(result, "{{date}}", now.Format("2006-01-02"))
	result = strings.ReplaceAll(result, "{{weekday}}", now.Weekday().String())
	result = strings.ReplaceAll(result, "{{isotime}}", now.Format(time.RFC3339))
	result = strings.ReplaceAll(result, "{{time_UTC}}", now.UTC().Format("15:04"))

	// 处理动态宏：{{roll}}、{{random:a,b,c}}、{{banned:...}} 等
	result = processDynamicMacros(result)

	return result
}

// processDynamicMacros 处理动态宏，支持 {{roll:dN}}、{{roll:N}}、{{random:a,b,c}}、
// {{pick:a,b,c}}、{{random}}、{{// comment}}、{{banned:...}}、{{trim}} 等。
func processDynamicMacros(text string) string {
	// 逐个匹配 {{...}} 宏并替换
	result := macroRegex.ReplaceAllStringFunc(text, func(match string) string {
		// 去掉首尾的 {{ 和 }}
		inner := match[2 : len(match)-2]
		inner = strings.TrimSpace(inner)

		// {{roll:dN}}：掷一个 N 面骰子，返回 1~N
		if strings.HasPrefix(inner, "roll:d") || strings.HasPrefix(inner, "roll:D") {
			nStr := inner[6:]
			n, err := strconv.Atoi(nStr)
			if err == nil && n > 0 {
				return strconv.Itoa(rand.Intn(n) + 1)
			}
			return match // 解析失败则原样返回
		}

		// {{roll:N}}：返回 0~(N-1) 的随机整数
		if strings.HasPrefix(inner, "roll:") {
			nStr := inner[5:]
			n, err := strconv.Atoi(nStr)
			if err == nil && n > 0 {
				return strconv.Itoa(rand.Intn(n))
			}
			return match
		}

		// {{random:a,b,c}} / {{pick:a,b,c}}：从候选项里随机选一个
		if strings.HasPrefix(inner, "random:") || strings.HasPrefix(inner, "pick:") {
			var listStr string
			if strings.HasPrefix(inner, "random:") {
				listStr = inner[7:]
			} else {
				listStr = inner[5:]
			}
			items := strings.Split(listStr, ",")
			if len(items) > 0 {
				chosen := strings.TrimSpace(items[rand.Intn(len(items))])
				return chosen
			}
			return match
		}

		// {{random}}：返回 0.0~1.0 的随机小数
		if inner == "random" {
			return fmt.Sprintf("%.4f", rand.Float64())
		}

		// {{// comment}}：注释宏，直接清空
		if strings.HasPrefix(inner, "//") {
			return ""
		}

		// {{trim}}：清空占位（用于去除多余空白）
		if inner == "trim" {
			return ""
		}

		// {{banned:...}}：屏蔽词宏，这里不展开，保持原样
		if strings.HasPrefix(inner, "banned:") {
			return match // 保持原样，交给上层处理
		}

		// 其它未识别的宏：原样返回
		return match
	})

	return result
}

var macroRegex = regexp.MustCompile(`\{\{[^}]+\}\}`)

// cleanAssistantContent 清洗模型回复：去掉 <think>/<CoT> 等隐藏思考标签与多余空行。
func cleanAssistantContent(text string) string {
	// 去掉 <think>...</think> 思考块
	text = thinkRegex.ReplaceAllString(text, "")
	// 去掉 <CoT>...</CoT> 思考块
	text = cotRegex.ReplaceAllString(text, "")
	// 去掉其它隐藏标签（注释、规则、系统块等）
	for _, re := range hiddenTagRegexes {
		text = re.ReplaceAllString(text, "")
	}
	// 合并连续的多个空行
	text = multiNewlineRegex.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

var thinkRegex = regexp.MustCompile(`(?is)<think>[\s\S]*?</think>`)
var cotRegex = regexp.MustCompile(`(?is)<CoT>[\s\S]*?</CoT>`)
var multiNewlineRegex = regexp.MustCompile(`\n{3,}`)
var hiddenTagRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<!--[\s\S]*?-->`),
	regexp.MustCompile(`(?is)<TBC>[\s\S]*?</TBC>`),
	regexp.MustCompile(`(?is)<rule>[\s\S]*?</rule>`),
	regexp.MustCompile(`(?is)<system>[\s\S]*?</system>`),
	regexp.MustCompile(`(?is)<CONFIG>[\s\S]*?</CONFIG>`),
	regexp.MustCompile(`(?is)<AWC>[\s\S]*?</AWC>`),
	regexp.MustCompile(`(?is)<ASI>[\s\S]*?</ASI>`),
	regexp.MustCompile(`(?is)<STORYTIME>[\s\S]*?</STORYTIME>`),
	regexp.MustCompile(`(?is)<INTERACTION_MOD>[\s\S]*?</INTERACTION_MOD>`),
	regexp.MustCompile(`(?is)<TALKER_MOD>[\s\S]*?</TALKER_MOD>`),
	regexp.MustCompile(`(?is)<novelist_MOD>[\s\S]*?</novelist_MOD>`),
	regexp.MustCompile(`(?is)<WritingStyle>[\s\S]*?</WritingStyle>`),
}

// buildMessages 组装最终发给模型的消息列表：身份锚点、预设多段提示词、摘要记忆、
// 持久角色卡、聊天历史与世界书条目，按 SillyTavern 风格的注入规则拼装，
// 并负责系统消息合并、严格保证首条非系统消息为 user 等收尾处理。
func (s *ChatService) buildMessages(chatID string, preset *model.Preset, char *model.Character, history []*model.Message, userContent string, userID string) []model.ChatCompletionMessage {
	trimmedHistory := history
	summaryContext := ""
	if s.summaryService != nil {
		summaryContext, trimmedHistory = s.summaryService.BuildServiceModeContext(chatID, history)
	}

	// 1. 组装聊天历史：首轮用开场白，否则用裁剪后的历史，并追加本次用户输入
	var recentHistory []model.ChatCompletionMessage
	if char.FirstMsg != "" && len(history) == 0 {
		recentHistory = append(recentHistory, model.ChatCompletionMessage{
			Role: "assistant", Content: s.replaceRoleCardText(char.FirstMsg, char, userID),
		})
	} else {
		for _, msg := range trimmedHistory {
			content := msg.Content
			// assistant 历史内容先清洗掉隐藏思考标签
			if msg.Role == "assistant" {
				content = cleanAssistantContent(content)
			}
			recentHistory = append(recentHistory, model.ChatCompletionMessage{
				Role: msg.Role, Content: content,
			})
		}
	}
	recentHistory = append(recentHistory, model.ChatCompletionMessage{
		Role: "user", Content: userContent,
	})

	var chatHistory []model.ChatCompletionMessage
	chatHistory = append(chatHistory, recentHistory...)

	// 2. 解析预设里的多段提示词（prompts）为 entry 列表
	var entries []model.PromptEntry
	if preset.Prompts != "" {
		if err := json.Unmarshal([]byte(preset.Prompts), &entries); err != nil {
			log.Printf("[preset] 解析多段提示词失败，回退到单条系统提示词: %v", err)
			entries = nil
		}
	}
	if len(entries) == 0 && preset.SystemPrompt != "" {
		entries = []model.PromptEntry{{
			ID: "auto-system", Name: "System Prompt", Content: preset.SystemPrompt,
			Role: "system", Enabled: true, SystemPrompt: true, Order: 0,
		}}
	}

	// 3. 按 SillyTavern 的注入规则处理多段提示词：
	//
	// 大致流程如下：
	//   Step A：system_prompt=true 的条目合并为开头的 system 消息
	//   Step B：插入聊天历史
	//   Step C：system_prompt=false 的条目按各自 role 追加
	//   Step D：把开头之后的 system 消息压成 role=user
	//   Step E：合并相邻的同 role 消息
	//
	log.Printf("[message build] advanced prompt mode with %d entries", len(entries))
	var enabled []model.PromptEntry
	for _, e := range entries {
		if !e.Enabled {
			continue
		}
		e.Content = s.replaceVars(e.Content, char, userID)
		if e.Role == "" {
			e.Role = "system"
		}
		enabled = append(enabled, e)
	}

	// 按 order 升序排序（稳定插入排序）
	sortEntries(enabled)

	// Step A：身份锚点 + system_prompt=true 的条目，拼成开头的 system 消息
	var systemContent strings.Builder
	identityPrompt := s.buildRoleIdentityPrompt(char, userID)
	if identityPrompt != "" {
		systemContent.WriteString(identityPrompt)
	}
	for _, e := range enabled {
		if !e.SystemPrompt {
			continue
		}
		if systemContent.Len() > 0 {
			systemContent.WriteString("\n\n")
		}
		systemContent.WriteString(e.Content)
	}
	if summaryContext != "" {
		if systemContent.Len() > 0 {
			systemContent.WriteString("\n\n")
		}
		systemContent.WriteString("[Summary Memory]\nUse this condensed long-term memory to preserve continuity. It supplements recent chat history and must not be ignored.\n")
		systemContent.WriteString(summaryContext)
	}
	roleCardPrompt := s.buildPersistentRoleCardPrompt(char, userID)
	if roleCardPrompt != "" {
		if systemContent.Len() > 0 {
			systemContent.WriteString("\n\n")
		}
		systemContent.WriteString(roleCardPrompt)
	}

	var result []model.ChatCompletionMessage
	if systemContent.Len() > 0 {
		// 在系统消息末尾追加输入格式说明，并标记新对话开始
		systemContent.WriteString(s.replaceVars(inputFormatHint, char, userID))
		systemContent.WriteString("\n\n[New Chat]")
		result = append(result, model.ChatCompletionMessage{
			Role: "system", Content: systemContent.String(),
		})
	}

	// Step B：追加聊天历史
	result = append(result, chatHistory...)

	// Step C：追加 system_prompt=false 的条目（按各自 role）
	for _, e := range enabled {
		if e.SystemPrompt {
			continue
		}
		result = append(result, model.ChatCompletionMessage{
			Role: e.Role, Content: e.Content,
		})
	}

	// Step D：把开头之后残留的 system 消息改成 user，规避多 system 限制
	for i := 1; i < len(result); i++ {
		if result[i].Role == "system" {
			result[i].Role = "user"
		}
	}

	// Step E：合并相邻的同 role 消息
	var messages []model.ChatCompletionMessage
	for _, msg := range result {
		if len(messages) > 0 && messages[len(messages)-1].Role == msg.Role {
			messages[len(messages)-1].Content += "\n" + msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	// Step F：严格保证首条非系统消息是 user
	// 若历史以 assistant 开头，则在它前面插入一条占位的 user 消息
	for i := 0; i < len(messages); i++ {
		if messages[i].Role == "system" {
			continue
		}
		if messages[i].Role == "assistant" {
			// 首条非系统消息是 assistant，前面补一条占位 user
			userMsg := model.ChatCompletionMessage{Role: "user", Content: "[New Chat]"}
			messages = append(messages[:i], append([]model.ChatCompletionMessage{userMsg}, messages[i:]...)...)
		}
		break
	}

	log.Printf("[message build] final messages=%d system_prompt=%d after_history=%d history=%d",
		len(messages),
		func() int {
			c := 0
			for _, e := range enabled {
				if e.SystemPrompt {
					c++
				}
			}
			return c
		}(),
		func() int {
			c := 0
			for _, e := range enabled {
				if !e.SystemPrompt {
					c++
				}
			}
			return c
		}(),
		len(chatHistory))

	// 4. 注入命中关键词的世界书条目
	messages = s.injectWorldBookEntries(messages, recentHistory, char, userID)

	return messages
}

// sortEntries 按 Order 字段升序排序（稳定插入排序）。
func sortEntries(entries []model.PromptEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Order < entries[j-1].Order; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// injectWorldBookEntries 扫描世界书条目，命中关键词或常驻的条目按注入位置插入消息列表。
// statusBarEntryKey 状态栏特殊条目的标识 key
const statusBarEntryKey = "状态栏"

// statusBarInstruction 是对 AI 的固定约束指令，固化在程序里，用户不可见也不可改。
// 用户在世界书条目里编辑的内容只作为“状态栏样式/包裹方式”，注入时拼在这段约束之后。
const statusBarInstruction = "【系统指令｜状态栏】每次回复的最后，都必须另起一段，严格按下面给出的样式输出当前状态栏。请根据当前剧情如实填写每一项，不要保留方括号占位文字，不要省略，也不要把本条系统指令本身写进回复。状态栏样式如下：\n"

// statusBarFenceRe 匹配用户样式中用三个反引号或三个单引号包裹的代码块围栏。
var statusBarFenceRe = regexp.MustCompile("(?m)^[ \\t]*(?:```|''')[^\\n]*\\n?")

// buildStatusBarContent 把用户编辑的状态栏样式拼成最终注入内容：
// 1. 剥掉用户样式里的三引号/三反引号代码块围栏（仅去掉围栏标记，保留其中文字）；
// 2. 在固定约束指令之后拼上清洗后的用户样式。
func buildStatusBarContent(userStyle string) string {
	cleaned := statusBarFenceRe.ReplaceAllString(userStyle, "")
	cleaned = strings.TrimSpace(cleaned)
	return statusBarInstruction + cleaned
}

func isStatusBarEntry(keys string) bool {
	return strings.TrimSpace(keys) == statusBarEntryKey
}

func (s *ChatService) injectWorldBookEntries(messages []model.ChatCompletionMessage, chatHistory []model.ChatCompletionMessage, char *model.Character, userID string) []model.ChatCompletionMessage {
	// 取出该用户在当前角色下可用的全部世界书条目
	allEntries, err := s.worldBookStore.ListAllEntries(userID, char.ID)
	if err != nil {
		log.Printf("[worldbook] 读取世界书条目失败: %v", err)
		return messages
	}
	if len(allEntries) == 0 {
		return messages
	}

	// 逐条判断：常驻条目直接命中，其余按关键词扫描
	var matched []model.WorldBookEntry
	for _, entry := range allEntries {
		if entry.Constant {
			// 常驻条目：无需关键词，始终注入
			matched = append(matched, entry)
			continue
		}
		// 非常驻条目：按关键词扫描历史是否命中
		if s.matchWorldBookEntry(&entry, chatHistory) {
			matched = append(matched, entry)
		}
	}

	if len(matched) == 0 {
		return messages
	}

	log.Printf("[worldbook] matched %d entries", len(matched))

	// 记录注入前的消息条数，供按深度定位使用
	msgLen := len(messages)

	// 计算每个条目的注入绝对位置
	type wbInject struct {
		pos int
		msg model.ChatCompletionMessage
	}
	var injections []wbInject

	for _, entry := range matched {
		var content string
		if isStatusBarEntry(entry.Keys) {
			// 状态栏特殊条目：用户编辑的内容只作为“样式”，注入时剥掉三引号围栏并前置固定约束指令
			content = s.replaceVars(buildStatusBarContent(entry.Content), char, userID)
		} else {
			content = s.replaceVars(entry.Content, char, userID)
		}
		role := entry.Role
		if role == "" {
			role = "system"
		}

		// 根据注入位置/深度计算绝对插入下标
		var absPos int
		if entry.InjectionDepth == 0 {
			// depth=0：紧跟在开头的 system 消息之后
			// 找到第一条非 system 消息的位置
			absPos = 0
			for i, m := range messages {
				if m.Role != "system" {
					absPos = i
					break
				}
				absPos = i + 1
			}
		} else if entry.InjectionPos == 1 {
			// 从头部往下数 depth 个位置插入
			absPos = entry.InjectionDepth
			if absPos > msgLen {
				absPos = msgLen
			}
		} else {
			// 从尾部往上数 depth 个位置插入
			absPos = msgLen - entry.InjectionDepth
			if absPos < 0 {
				absPos = 0
			}
		}

		injections = append(injections, wbInject{
			pos: absPos,
			msg: model.ChatCompletionMessage{Role: role, Content: content},
		})
	}

	// 按插入位置从大到小排序，保证从后往前插入时下标不串位
	for i := 0; i < len(injections); i++ {
		for j := i + 1; j < len(injections); j++ {
			if injections[j].pos > injections[i].pos {
				injections[i], injections[j] = injections[j], injections[i]
			}
		}
	}

	result := make([]model.ChatCompletionMessage, len(messages))
	copy(result, messages)

	for _, inj := range injections {
		pos := inj.pos
		if pos > len(result) {
			pos = len(result)
		}
		result = append(result[:pos], append([]model.ChatCompletionMessage{inj.msg}, result[pos:]...)...)
	}

	return result
}

// matchWorldBookEntry 判断某条世界书条目是否被聊天历史命中（主关键词命中 + 可选次关键词 AND 条件）。
func (s *ChatService) matchWorldBookEntry(entry *model.WorldBookEntry, chatHistory []model.ChatCompletionMessage) bool {
	if entry.Keys == "" {
		return false
	}

	// 按扫描深度截取要检索的历史片段
	scanMsgs := chatHistory
	if entry.ScanDepth > 0 && entry.ScanDepth < len(chatHistory) {
		scanMsgs = chatHistory[len(chatHistory)-entry.ScanDepth:]
	}

	// 拼接待扫描文本，按需转小写
	var textBuilder strings.Builder
	for _, msg := range scanMsgs {
		textBuilder.WriteString(msg.Content)
		textBuilder.WriteString(" ")
	}
	scanText := textBuilder.String()
	if !entry.CaseSensitive {
		scanText = strings.ToLower(scanText)
	}

	// 主关键词：任意一个命中即视为初步匹配
	keys := strings.Split(entry.Keys, ",")
	primaryMatch := false
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		checkKey := key
		if !entry.CaseSensitive {
			checkKey = strings.ToLower(key)
		}
		if strings.Contains(scanText, checkKey) {
			primaryMatch = true
			break
		}
	}

	if !primaryMatch {
		return false
	}

	// 次关键词：要求全部命中（AND 关系）
	if entry.SecondaryKeys != "" {
		secKeys := strings.Split(entry.SecondaryKeys, ",")
		for _, key := range secKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			checkKey := key
			if !entry.CaseSensitive {
				checkKey = strings.ToLower(key)
			}
			if !strings.Contains(scanText, checkKey) {
				return false // AND 条件：有一个次关键词没命中就不匹配
			}
		}
	}

	return true
}

// callOpenAIStream 单次请求模型并以 SSE 流式读取回复；callback 非空时逐 token 回调。
const formattedReplyMaxAttempts = 3

const formatRetryInstruction = `[Format Retry]
The previous assistant reply did not follow the required output format. Rewrite the assistant reply only.
Keep the same intent and story facts, but output only the final in-character prose.
Use quoted speech for spoken dialogue and unquoted text for narration/action.
Do not use JSON, Markdown, code fences, role labels, analysis, explanations, or out-of-character notes.`

var (
	formattedReplyHeadingRe = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+`)
	formattedReplyListRe    = regexp.MustCompile(`(?m)^\s{0,3}(?:[-*+]|\d+[.)])\s+\S`)
	formattedReplyRoleRe    = regexp.MustCompile(`(?mi)^\s*(?:assistant|ai|bot|system|user|char|character|narrator|ooc|\{\{char\}\}|\{\{user\}\}|助手|用户|角色|旁白|系统)\s*[:：]`)
	formattedReplyMetaRe    = regexp.MustCompile(`(?mi)^\s*(?:analysis|reasoning|explanation|note|说明|解释|分析|备注)\s*[:：]`)
	formattedReplyCodeFence = "```"
)

func (s *ChatService) callFormattedOpenAIStream(settings *model.AppSettings, preset *model.Preset, messages []model.ChatCompletionMessage, callback StreamCallback) (string, error) {
	attemptMessages := append([]model.ChatCompletionMessage(nil), messages...)
	var lastResponse string
	var lastFormatErr error

	for attempt := 1; attempt <= formattedReplyMaxAttempts; attempt++ {
		rawResponse, err := s.callOpenAIStream(settings, preset, attemptMessages, nil)
		response := strings.TrimSpace(cleanAssistantContent(rawResponse))
		if err != nil {
			return response, err
		}

		if formatErr := validateAssistantReplyFormat(response); formatErr == nil {
			if callback != nil && response != "" {
				if err := callback(response); err != nil {
					return response, err
				}
			}
			return response, nil
		} else {
			lastResponse = response
			lastFormatErr = formatErr
			log.Printf("[format-retry] attempt=%d invalid assistant reply: %v", attempt, formatErr)
			if attempt < formattedReplyMaxAttempts {
				attemptMessages = appendFormatRetryMessages(attemptMessages, response, formatErr)
			}
		}
	}

	if lastFormatErr == nil {
		lastFormatErr = fmt.Errorf("unknown format error")
	}
	return lastResponse, fmt.Errorf("AI response format invalid after %d attempts: %w", formattedReplyMaxAttempts, lastFormatErr)
}

func appendFormatRetryMessages(messages []model.ChatCompletionMessage, previous string, formatErr error) []model.ChatCompletionMessage {
	retryMessages := append([]model.ChatCompletionMessage(nil), messages...)
	previous = strings.TrimSpace(previous)
	if previous != "" {
		retryMessages = append(retryMessages, model.ChatCompletionMessage{
			Role:    "assistant",
			Content: previous,
		})
	}
	retryMessages = append(retryMessages, model.ChatCompletionMessage{
		Role:    "user",
		Content: fmt.Sprintf("%s\n\nFormat error: %s", formatRetryInstruction, formatErr),
	})
	return retryMessages
}

func validateAssistantReplyFormat(response string) error {
	cleaned := strings.TrimSpace(cleanAssistantContent(response))
	if cleaned == "" {
		return fmt.Errorf("empty response")
	}
	if strings.Contains(cleaned, formattedReplyCodeFence) {
		return fmt.Errorf("contains code fence")
	}
	if looksLikeJSONValue(cleaned) {
		return fmt.Errorf("looks like JSON")
	}
	if formattedReplyHeadingRe.MatchString(cleaned) {
		return fmt.Errorf("uses Markdown heading")
	}
	if formattedReplyListRe.MatchString(cleaned) {
		return fmt.Errorf("uses Markdown list")
	}
	if formattedReplyRoleRe.MatchString(cleaned) {
		return fmt.Errorf("uses role label")
	}
	if formattedReplyMetaRe.MatchString(cleaned) {
		return fmt.Errorf("contains meta commentary")
	}
	if !hasEvenUnescapedDoubleQuotes(cleaned) {
		return fmt.Errorf("has unbalanced double quotes")
	}
	if !hasBalancedRunePair(cleaned, '“', '”') {
		return fmt.Errorf("has unbalanced Chinese double quotes")
	}
	if !hasBalancedRunePair(cleaned, '「', '」') {
		return fmt.Errorf("has unbalanced corner quotes")
	}
	if !hasBalancedRunePair(cleaned, '『', '』') {
		return fmt.Errorf("has unbalanced corner quotes")
	}
	return nil
}

func looksLikeJSONValue(text string) bool {
	if len(text) < 2 {
		return false
	}
	first := text[0]
	last := text[len(text)-1]
	if !((first == '{' && last == '}') || (first == '[' && last == ']')) {
		return false
	}
	var v interface{}
	return json.Unmarshal([]byte(text), &v) == nil
}

func hasEvenUnescapedDoubleQuotes(text string) bool {
	count := 0
	escaped := false
	for _, r := range text {
		if r == '\\' && !escaped {
			escaped = true
			continue
		}
		if r == '"' && !escaped {
			count++
		}
		escaped = false
	}
	return count%2 == 0
}

func hasBalancedRunePair(text string, open, close rune) bool {
	balance := 0
	for _, r := range text {
		if r == open {
			balance++
		}
		if r == close {
			balance--
			if balance < 0 {
				return false
			}
		}
	}
	return balance == 0
}

func (s *ChatService) callOpenAIStream(settings *model.AppSettings, preset *model.Preset, messages []model.ChatCompletionMessage, callback StreamCallback) (string, error) {
	reqBody := model.ChatCompletionRequest{
		Model:       settings.DefaultModel,
		Messages:    messages,
		Temperature: preset.Temperature,
		MaxTokens:   preset.MaxTokens,
		TopP:        preset.TopP,
		Stream:      true,
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
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用模型 API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("模型 API 返回异常状态码 %d: %s", resp.StatusCode, string(body))
	}

	// 逐行读取 SSE 流
	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// 加大扫描缓冲，避免单条数据过长被截断
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var callbackErr error // 记录回调中出现的错误，出现后停止继续回调

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// 跳过空行和以冒号开头的注释行
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 只处理 data: 开头的数据行
		var data string
		if strings.HasPrefix(line, "data: ") {
			data = line[6:]
		} else if strings.HasPrefix(line, "data:") {
			data = line[5:]
		} else {
			continue
		}

		data = strings.TrimSpace(data)

		// 收到 [DONE] 表示流结束
		if data == "[DONE]" {
			break
		}

		// 跳过空数据
		if data == "" {
			continue
		}

		// 解析增量数据块
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				// 兼容非流式返回：有的实现把内容放在 message 里
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[SSE] JSON 解析失败: %v，原始数据: %s", err, data[:min(len(data), 200)])
			continue
		}

		if len(chunk.Choices) > 0 {
			token := chunk.Choices[0].Delta.Content
			if token == "" {
				token = chunk.Choices[0].Message.Content
			}
			if token != "" {
				fullContent.WriteString(token)
				// 把增量内容写入完整回复，并在有 callback 时实时回调
				if callback != nil && callbackErr == nil {
					if err := callback(token); err != nil {
						callbackErr = err
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[SSE] 读取流失败: %v，已读取 %d 字节", err, fullContent.Len())
	}

	return fullContent.String(), nil
}

// inputFormatHint 注入系统提示的输入格式说明，告诉模型如何区分用户输入与指令。
const inputFormatHint = "\n\n[Format]\nUser input: Text inside quoted speech belongs to {{user}}. Text inside parentheses is {{user}}'s inner thought and cannot be perceived by {{char}}. All remaining text is narration or action.\nAssistant output: Reply only as formatted roleplay prose. Use quoted speech for spoken dialogue and unquoted text for narration/action. Do not output JSON, Markdown, code fences, role labels, analysis, explanations, or out-of-character notes."

// DebugEnabled 调试开关：开启后会把请求消息与响应 dump 到 data 目录的文件中。
const DebugEnabled = false

// debugLogResponse 调试模式下记录一次模型响应（字符数、字节数、错误与内容）。
// 仅在 DebugEnabled 为 true 时写文件，正常运行不产生额外开销。
func (s *ChatService) debugLogResponse(chatID, response string, err error) {
	if !DebugEnabled {
		return
	}

	charCount := len([]rune(response))
	byteCount := len(response)
	log.Printf("[AI响应] chat=%s 字符数=%d 字节数=%d err=%v", chatID, charCount, byteCount, err)

	debugFile := fmt.Sprintf("data/debug_response_%d.txt", time.Now().UnixMilli())
	content := fmt.Sprintf("=== AI 响应 %s chat=%s 字符数=%d 字节数=%d err=%v ===\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), chatID, charCount, byteCount, err, response)
	os.WriteFile(debugFile, []byte(content), 0644)
}
