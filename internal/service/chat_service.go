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

// ChatService 聊天业务逻辑
type ChatService struct {
	chatStore      *store.ChatStore
	messageStore   *store.MessageStore
	characterStore *store.CharacterStore
	presetStore    *store.PresetStore
	worldBookStore *store.WorldBookStore
	configStore    *store.ConfigStore
	userStore      *store.UserStore
}

func NewChatService(
	chatStore *store.ChatStore,
	messageStore *store.MessageStore,
	characterStore *store.CharacterStore,
	presetStore *store.PresetStore,
	worldBookStore *store.WorldBookStore,
	configStore *store.ConfigStore,
	userStore *store.UserStore,
) *ChatService {
	return &ChatService{
		chatStore:      chatStore,
		messageStore:   messageStore,
		characterStore: characterStore,
		presetStore:    presetStore,
		worldBookStore: worldBookStore,
		configStore:    configStore,
		userStore:      userStore,
	}
}

// StreamResponse SSE 流式响应的回调函数类型
type StreamCallback func(token string) error

// SendMessage 发送消息并流式返回 AI 响应
func (s *ChatService) SendMessage(chatID, content, presetID, userID string, callback StreamCallback) (string, error) {
	// 获取对话信息
	chat, err := s.chatStore.GetByID(chatID, userID)
	if err != nil {
		return "", fmt.Errorf("对话不存在: %w", err)
	}

	// 获取角色信息
	character, err := s.characterStore.GetByID(chat.CharacterID, userID)
	if err != nil {
		return "", fmt.Errorf("角色不存在: %w", err)
	}

	// 获取预设
	presetIDToUse := presetID
	if presetIDToUse == "" {
		presetIDToUse = chat.PresetID
	}

	// 判断是否为服务模式
	isServiceMode := s.userStore.GetCurrentMode() == "service"

	var preset *model.Preset
	if presetIDToUse != "" {
		preset, err = s.presetStore.GetByID(presetIDToUse, userID)
		if err != nil {
			log.Printf("[预设] 按ID=%s查找失败: %v", presetIDToUse, err)
			preset = nil
		}
	}
	if preset == nil {
		if isServiceMode {
			// 服务模式：加载 admin 的默认预设（普通用户无自有预设）
			preset, err = s.presetStore.GetDefaultAdmin()
			if err != nil {
				log.Printf("[预设] 服务模式查找admin预设失败: %v，使用内置预设", err)
			}
		} else {
			preset, err = s.presetStore.GetDefault(userID)
			if err != nil {
				log.Printf("[预设] 查找默认预设失败: %v，使用内置预设", err)
			}
		}
		if preset == nil {
			preset = &model.Preset{
				SystemPrompt: "你是{{char}}。请根据角色设定进行扮演。",
				Temperature:  0.8,
				MaxTokens:    2048,
				TopP:         0.9,
			}
		}
	}
	// 调试：记录实际使用的预设
	hasPrompts := preset.Prompts != ""
	log.Printf("[预设] 使用预设: name=%s id=%s user=%s 多段=%v prompts长度=%d",
		preset.Name, preset.ID, preset.UserID, hasPrompts, len(preset.Prompts))

	// 获取历史消息
	history, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("获取消息历史失败: %w", err)
	}

	// 如果是第一条消息且角色有开场白，先持久化开场白
	if len(history) == 0 && character.FirstMsg != "" {
		firstMsg := &model.Message{
			ChatID:  chatID,
			Role:    "assistant",
			Content: s.replaceVars(character.FirstMsg, character),
		}
		if err := s.messageStore.Create(firstMsg); err != nil {
			log.Printf("[开场白] 保存失败: %v", err)
		} else {
			// 将开场白加入历史，以便后续 buildMessages 正确处理
			history = append(history, firstMsg)
		}
	}

	// 保存用户消息
	userMsg := &model.Message{
		ChatID:  chatID,
		Role:    "user",
		Content: content,
	}
	if err := s.messageStore.Create(userMsg); err != nil {
		return "", fmt.Errorf("保存用户消息失败: %w", err)
	}

	// 构建消息列表（支持多段提示词注入）
	messages := s.buildMessages(preset, character, history, content, userID)

	// 调试：将发送给 API 的完整消息列表写入文件
	if DebugEnabled {
		var msgDebug strings.Builder
		msgDebug.WriteString(fmt.Sprintf("=== 发送消息调试 %s ===\n预设: %s (ID: %s)\n消息数: %d\n\n",
			time.Now().Format("15:04:05"), preset.Name, preset.ID, len(messages)))
		for i, m := range messages {
			msgDebug.WriteString(fmt.Sprintf("[%d] role=%s\n%s\n\n", i, m.Role, m.Content))
		}
		debugFile := fmt.Sprintf("data/debug_messages_%d.txt", time.Now().UnixMilli())
		os.WriteFile(debugFile, []byte(msgDebug.String()), 0644)
		log.Printf("[调试] 消息列表已写入 %s (%d 条消息)", debugFile, len(messages))
	}

	// 获取 API 配置
	settings, err := s.configStore.GetSettings()
	if err != nil {
		return "", fmt.Errorf("获取配置失败: %w", err)
	}

	// 调用 OpenAI 兼容 API（流式）
	fullResponse, err := s.callOpenAIStream(settings, preset, messages, callback)

	// 调试日志（始终记录摘要，文件保存可按需开启）
	s.debugLogResponse(chatID, fullResponse, err)

	if err != nil {
		return "", err
	}

	// 保存 AI 响应消息
	aiMsg := &model.Message{
		ChatID:  chatID,
		Role:    "assistant",
		Content: fullResponse,
	}
	if err := s.messageStore.Create(aiMsg); err != nil {
		return "", fmt.Errorf("保存 AI 消息失败: %w", err)
	}

	// 更新对话的 updated_at
	_ = s.chatStore.Touch(chatID, userID)

	return fullResponse, nil
}

// Regenerate 重新生成最后一条 AI 回复（删除旧回复，用已有历史重新请求）
func (s *ChatService) Regenerate(chatID, userID string, callback StreamCallback) (string, error) {
	// 获取对话的所有消息
	allMessages, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("获取消息失败: %w", err)
	}
	if len(allMessages) == 0 {
		return "", fmt.Errorf("没有消息可以重新生成")
	}

	// 找到最后一条 assistant 消息并删除
	var lastAiIdx = -1
	for i := len(allMessages) - 1; i >= 0; i-- {
		if allMessages[i].Role == "assistant" {
			lastAiIdx = i
			break
		}
	}
	if lastAiIdx < 0 {
		return "", fmt.Errorf("没有 AI 回复可以重新生成")
	}

	// 删除最后一条 AI 回复
	s.messageStore.DeleteByID(allMessages[lastAiIdx].ID)

	// 找到最后一条用户消息的内容
	var lastUserContent string
	for i := lastAiIdx - 1; i >= 0; i-- {
		if allMessages[i].Role == "user" {
			lastUserContent = allMessages[i].Content
			break
		}
	}
	if lastUserContent == "" {
		return "", fmt.Errorf("找不到对应的用户消息")
	}

	// 获取对话信息
	chat, err := s.chatStore.GetByID(chatID, userID)
	if err != nil {
		return "", fmt.Errorf("对话不存在: %w", err)
	}

	// 获取角色信息
	character, err := s.characterStore.GetByID(chat.CharacterID, userID)
	if err != nil {
		return "", fmt.Errorf("角色不存在: %w", err)
	}

	// 获取预设
	preset := s.loadPreset(chat.PresetID, "", userID)

	// 获取更新后的历史消息（不含已删除的 AI 回复，也不含最后的用户消息——因为 buildMessages 会自己加）
	history, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("获取消息历史失败: %w", err)
	}
	// 去掉最后一条用户消息（buildMessages 会重新添加）
	if len(history) > 0 && history[len(history)-1].Role == "user" {
		history = history[:len(history)-1]
	}

	// 构建消息列表（不保存用户消息，直接用历史 + lastUserContent）
	messages := s.buildMessages(preset, character, history, lastUserContent, userID)

	// 获取 API 配置
	settings, err := s.configStore.GetSettings()
	if err != nil {
		return "", fmt.Errorf("获取配置失败: %w", err)
	}

	// 调用 API
	fullResponse, err := s.callOpenAIStream(settings, preset, messages, callback)
	s.debugLogResponse(chatID, fullResponse, err)
	if err != nil {
		return "", err
	}

	// 保存新的 AI 响应
	aiMsg := &model.Message{
		ChatID:  chatID,
		Role:    "assistant",
		Content: fullResponse,
	}
	if err := s.messageStore.Create(aiMsg); err != nil {
		return "", fmt.Errorf("保存 AI 消息失败: %w", err)
	}

	_ = s.chatStore.Touch(chatID, userID)
	return fullResponse, nil
}

// loadPreset 加载预设（提取公共逻辑）
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
		preset, err = s.presetStore.GetDefault(userID)
		if err != nil {
			preset = &model.Preset{
				SystemPrompt: "你是{{char}}。请根据角色设定进行扮演。",
				Temperature:  0.8,
				MaxTokens:    2048,
				TopP:         0.9,
			}
		}
	}
	return preset
}

// replaceVars 替换提示词中的模板变量和 SillyTavern 宏
// getUserName 获取用户名称（角色卡自定义 > 全局设置 > "User"）
func (s *ChatService) getUserName(char *model.Character) string {
	if char.UseCustomUser && char.UserName != "" {
		return char.UserName
	}
	settings, err := s.configStore.GetSettings()
	if err == nil && settings.DefaultUserName != "" {
		return settings.DefaultUserName
	}
	return "User"
}

// getUserDetail 获取用户详情（角色卡自定义 > 全局设置）
func (s *ChatService) getUserDetail(char *model.Character) string {
	if char.UseCustomUser {
		return char.UserDetail
	}
	settings, err := s.configStore.GetSettings()
	if err == nil {
		return settings.DefaultUserDetail
	}
	return ""
}

func (s *ChatService) replaceVars(template string, char *model.Character) string {
	result := template

	// 用户变量
	userName := s.getUserName(char)
	result = strings.ReplaceAll(result, "{{user}}", userName)
	result = strings.ReplaceAll(result, "{{User}}", userName)

	// 角色变量
	result = strings.ReplaceAll(result, "{{char}}", char.Name)

	// {{description}} 前面拼接用户信息（仅在用户实际配置了信息时）
	userDetail := s.getUserDetail(char)
	descWithUserInfo := char.Description
	// 只有当用户主动设置了名称（非默认 "User"）或有详情时才拼接
	hasCustomName := (char.UseCustomUser && char.UserName != "") || func() bool {
		settings, err := s.configStore.GetSettings()
		return err == nil && settings.DefaultUserName != ""
	}()
	if hasCustomName || userDetail != "" {
		var userInfoBlock strings.Builder
		userInfoBlock.WriteString("[用户信息]\n")
		userInfoBlock.WriteString("用户名: " + userName + "\n")
		if userDetail != "" {
			userInfoBlock.WriteString("用户详情: " + userDetail + "\n")
		}
		userInfoBlock.WriteString("\n")
		descWithUserInfo = userInfoBlock.String() + char.Description
	}
	result = strings.ReplaceAll(result, "{{description}}", descWithUserInfo)
	result = strings.ReplaceAll(result, "{{personality}}", char.Personality)
	result = strings.ReplaceAll(result, "{{scenario}}", char.Scenario)

	// 时间日期变量
	now := time.Now()
	result = strings.ReplaceAll(result, "{{time}}", now.Format("15:04"))
	result = strings.ReplaceAll(result, "{{date}}", now.Format("2006-01-02"))
	result = strings.ReplaceAll(result, "{{weekday}}", now.Weekday().String())
	result = strings.ReplaceAll(result, "{{isotime}}", now.Format(time.RFC3339))
	result = strings.ReplaceAll(result, "{{time_UTC}}", now.UTC().Format("15:04"))

	// 处理动态宏：{{roll:dN}}, {{random:a,b,c}}, {{banned:...}} 等
	result = processDynamicMacros(result)

	return result
}

// processDynamicMacros 处理 SillyTavern 的动态宏
// 支持: {{roll:dN}}, {{random:a,b,c}}, {{pick:a,b,c}}, {{// comment}}, {{banned:...}}, {{trim}}
func processDynamicMacros(text string) string {
	// 用正则找到所有 {{...}} 模式并逐个处理
	result := macroRegex.ReplaceAllStringFunc(text, func(match string) string {
		// 去掉 {{ 和 }}
		inner := match[2 : len(match)-2]
		inner = strings.TrimSpace(inner)

		// {{roll:dN}} — 骰子，1~N 的随机数
		if strings.HasPrefix(inner, "roll:d") || strings.HasPrefix(inner, "roll:D") {
			nStr := inner[6:]
			n, err := strconv.Atoi(nStr)
			if err == nil && n > 0 {
				return strconv.Itoa(rand.Intn(n) + 1)
			}
			return match // 无法解析则保留原文
		}

		// {{roll:N}} — 0~(N-1) 的随机数
		if strings.HasPrefix(inner, "roll:") {
			nStr := inner[5:]
			n, err := strconv.Atoi(nStr)
			if err == nil && n > 0 {
				return strconv.Itoa(rand.Intn(n))
			}
			return match
		}

		// {{random:a,b,c}} 或 {{pick:a,b,c}} — 随机选择
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

		// {{random}} — 0.0~1.0 随机浮点数
		if inner == "random" {
			return fmt.Sprintf("%.4f", rand.Float64())
		}

		// {{// comment}} — 注释，直接移除
		if strings.HasPrefix(inner, "//") {
			return ""
		}

		// {{trim}} — 移除标记（SillyTavern 用来去除周围换行）
		if inner == "trim" {
			return ""
		}

		// {{banned:...}} — 禁词标记，保留原文让模型看到
		if strings.HasPrefix(inner, "banned:") {
			return match // 保留原样
		}

		// 未知宏，保留原文
		return match
	})

	return result
}

var macroRegex = regexp.MustCompile(`\{\{[^}]+\}\}`)

// cleanAssistantContent 清理 AI 回复中的思考块和隐藏标签，避免污染上下文
func cleanAssistantContent(text string) string {
	// 移除 <think>...</think>
	text = thinkRegex.ReplaceAllString(text, "")
	// 移除 <CoT>...</CoT>
	text = cotRegex.ReplaceAllString(text, "")
	// 移除隐藏的自定义标签
	for _, re := range hiddenTagRegexes {
		text = re.ReplaceAllString(text, "")
	}
	// 清理多余空行
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
	regexp.MustCompile(`(?is)<语言风格>[\s\S]*?</语言风格>`),
}

// buildMessages 构建完整的消息列表
// 如果预设有多段 Prompts（高级模式），按 SillyTavern 格式注入
// 否则回退到简单模式（单段 SystemPrompt）
func (s *ChatService) buildMessages(preset *model.Preset, char *model.Character, history []*model.Message, userContent string, userID string) []model.ChatCompletionMessage {

	// 1. 组装聊天历史（含开场白 + 历史 + 当前用户消息）
	var chatHistory []model.ChatCompletionMessage
	if char.FirstMsg != "" && len(history) == 0 {
		chatHistory = append(chatHistory, model.ChatCompletionMessage{
			Role: "assistant", Content: s.replaceVars(char.FirstMsg, char),
		})
	} else {
		for _, msg := range history {
			content := msg.Content
			// 清理 AI 回复中的思考块和隐藏标签
			if msg.Role == "assistant" {
				content = cleanAssistantContent(content)
			}
			chatHistory = append(chatHistory, model.ChatCompletionMessage{
				Role: msg.Role, Content: content,
			})
		}
	}
	chatHistory = append(chatHistory, model.ChatCompletionMessage{
		Role: "user", Content: userContent,
	})

	// 2. 解析多段提示词；如果没有则将 SystemPrompt 转为单条 entry
	var entries []model.PromptEntry
	if preset.Prompts != "" {
		if err := json.Unmarshal([]byte(preset.Prompts), &entries); err != nil {
			log.Printf("[预设] 解析多段提示词失败: %v", err)
			entries = nil
		}
	}
	if len(entries) == 0 && preset.SystemPrompt != "" {
		entries = []model.PromptEntry{{
			ID: "auto-system", Name: "系统提示词", Content: preset.SystemPrompt,
			Role: "system", Enabled: true, SystemPrompt: true, Order: 0,
		}}
	}

	// 3. 消息组装（SillyTavern 兼容）
	//
	// ST 的实际行为（从日志逆向分析）：
	//   Step A: system_prompt=true 的条目 → 按顺序合并为一条 system 消息（[0]）
	//   Step B: 聊天历史（开场白 + 历史 + 用户消息）
	//   Step C: system_prompt=false 的条目 → 按顺序追加到聊天历史之后
	//   Step D: squash_system_messages — 非首条的 role=system 转为 role=user
	//   Step E: 合并相邻同 role 消息
	//
	log.Printf("[消息组装] 高级模式，共 %d 段提示词", len(entries))
	var enabled []model.PromptEntry
	for _, e := range entries {
		if !e.Enabled {
			continue
		}
		e.Content = s.replaceVars(e.Content, char)
		if e.Role == "" {
			e.Role = "system"
		}
		enabled = append(enabled, e)
	}

	// 按 order 排序（保证 prompt_order 顺序）
	sortEntries(enabled)

	// Step A: system_prompt=true → 合并为系统消息块
	var systemContent strings.Builder
	for _, e := range enabled {
		if !e.SystemPrompt {
			continue
		}
		if systemContent.Len() > 0 {
			systemContent.WriteString("\n")
		}
		systemContent.WriteString(e.Content)
	}

	var result []model.ChatCompletionMessage
	if systemContent.Len() > 0 {
		// 追加格式说明 + [开始新对话] 分隔符
		systemContent.WriteString(s.replaceVars(inputFormatHint, char))
		systemContent.WriteString("\n\n[开始新对话]")
		result = append(result, model.ChatCompletionMessage{
			Role: "system", Content: systemContent.String(),
		})
	}

	// Step B: 聊天历史
	result = append(result, chatHistory...)

	// Step C: system_prompt=false → 按顺序追加到聊天历史之后
	for _, e := range enabled {
		if e.SystemPrompt {
			continue
		}
		result = append(result, model.ChatCompletionMessage{
			Role: e.Role, Content: e.Content,
		})
	}

	// Step D: squash_system_messages — 第一条之后的 role=system 转为 role=user
	for i := 1; i < len(result); i++ {
		if result[i].Role == "system" {
			result[i].Role = "user"
		}
	}

	// Step E: 合并相邻同 role 消息
	var messages []model.ChatCompletionMessage
	for _, msg := range result {
		if len(messages) > 0 && messages[len(messages)-1].Role == msg.Role {
			messages[len(messages)-1].Content += "\n" + msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	// Step F: Strict 模式兼容 — 确保第一条非 system 消息是 user
	// 如果第一条非 system 是 assistant（开场白），在它前面插入一条 user 消息
	for i := 0; i < len(messages); i++ {
		if messages[i].Role == "system" {
			continue
		}
		if messages[i].Role == "assistant" {
			// 在 assistant 前插入 user 消息
			userMsg := model.ChatCompletionMessage{Role: "user", Content: "[开始新对话]"}
			messages = append(messages[:i], append([]model.ChatCompletionMessage{userMsg}, messages[i:]...)...)
		}
		break
	}

	log.Printf("[消息组装] 最终 %d 条消息（system_prompt=%d, after_history=%d, 历史=%d）",
		len(messages),
		func() int { c := 0; for _, e := range enabled { if e.SystemPrompt { c++ } }; return c }(),
		func() int { c := 0; for _, e := range enabled { if !e.SystemPrompt { c++ } }; return c }(),
		len(chatHistory))

	// 6. 世界书注入：扫描聊天历史中的关键词，将匹配的条目注入
	messages = s.injectWorldBookEntries(messages, chatHistory, char, userID)

	return messages
}

// sortEntries 按 Order 字段稳定排序
func sortEntries(entries []model.PromptEntry) {
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Order < entries[j-1].Order; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}

// injectWorldBookEntries 扫描聊天历史，将全局+角色绑定的世界书条目注入到消息列表中
func (s *ChatService) injectWorldBookEntries(messages []model.ChatCompletionMessage, chatHistory []model.ChatCompletionMessage, char *model.Character, userID string) []model.ChatCompletionMessage {
	// 获取全局 + 当前角色绑定的世界书条目
	allEntries, err := s.worldBookStore.ListAllEntries(userID, char.ID)
	if err != nil {
		log.Printf("[世界书] 加载条目失败: %v", err)
		return messages
	}
	if len(allEntries) == 0 {
		return messages
	}

	// 筛选：常驻条目 + 关键词匹配的条目
	var matched []model.WorldBookEntry
	for _, entry := range allEntries {
		if entry.Constant {
			// 常驻条目，直接加入
			matched = append(matched, entry)
			continue
		}
		// 关键词扫描
		if s.matchWorldBookEntry(&entry, chatHistory) {
			matched = append(matched, entry)
		}
	}

	if len(matched) == 0 {
		return messages
	}

	log.Printf("[世界书] 匹配到 %d 个条目", len(matched))

	// 按注入深度分组处理
	msgLen := len(messages)

	// 收集注入点（从后往前插入）
	type wbInject struct {
		pos int
		msg model.ChatCompletionMessage
	}
	var injections []wbInject

	for _, entry := range matched {
		content := s.replaceVars(entry.Content, char)
		role := entry.Role
		if role == "" {
			role = "system"
		}

		// 计算注入位置
		var absPos int
		if entry.InjectionDepth == 0 {
			// depth=0: 在消息列表最前面（紧跟 system prompt 之后）
			// 找到第一个非 system 消息的位置
			absPos = 0
			for i, m := range messages {
				if m.Role != "system" {
					absPos = i
					break
				}
				absPos = i + 1
			}
		} else if entry.InjectionPos == 1 {
			// 绝对位置
			absPos = entry.InjectionDepth
			if absPos > msgLen {
				absPos = msgLen
			}
		} else {
			// 相对末尾
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

	// 按位置从大到小排序（从后往前插入不影响前面的索引）
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

// matchWorldBookEntry 检查聊天历史是否匹配世界书条目的关键词
func (s *ChatService) matchWorldBookEntry(entry *model.WorldBookEntry, chatHistory []model.ChatCompletionMessage) bool {
	if entry.Keys == "" {
		return false
	}

	// 确定扫描范围
	scanMsgs := chatHistory
	if entry.ScanDepth > 0 && entry.ScanDepth < len(chatHistory) {
		scanMsgs = chatHistory[len(chatHistory)-entry.ScanDepth:]
	}

	// 拼接扫描范围内的所有消息文本
	var textBuilder strings.Builder
	for _, msg := range scanMsgs {
		textBuilder.WriteString(msg.Content)
		textBuilder.WriteString(" ")
	}
	scanText := textBuilder.String()
	if !entry.CaseSensitive {
		scanText = strings.ToLower(scanText)
	}

	// 主关键词：逗号分隔，OR 逻辑（任一命中即可）
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

	// 次关键词：逗号分隔，AND 逻辑（全部都要命中）
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
				return false // AND 逻辑，有一个没命中就不匹配
			}
		}
	}

	return true
}

// callOpenAIStream 调用 OpenAI 兼容 API 并流式返回
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
		return "", fmt.Errorf("API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(body))
	}

	// 解析 SSE 流（兼容各种第三方 API 格式）
	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// 增大缓冲区到 1MB，防止长行被截断
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	var callbackErr error // 记录回调错误，但不中断上游读��

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// 提取 data 字段（兼容 "data: " 和 "data:" 两种格式）
		var data string
		if strings.HasPrefix(line, "data: ") {
			data = line[6:]
		} else if strings.HasPrefix(line, "data:") {
			data = line[5:]
		} else {
			continue
		}

		data = strings.TrimSpace(data)

		// 检查流结束标记
		if data == "[DONE]" {
			break
		}

		// 跳过空数据
		if data == "" {
			continue
		}

		// 解析 JSON
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				// 兼容非流式格式的 message 字段
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[SSE] JSON 解析失败: %v, 原始数据: %s", err, data[:min(len(data), 200)])
			continue
		}

		if len(chunk.Choices) > 0 {
			token := chunk.Choices[0].Delta.Content
			if token == "" {
				token = chunk.Choices[0].Message.Content
			}
			if token != "" {
				fullContent.WriteString(token)
				// 回调失败不中断上游读取，只是不再发送给前端
				if callback != nil && callbackErr == nil {
					if err := callback(token); err != nil {
						callbackErr = err
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[SSE] 读取流错误: %v (已收集 %d 字节)", err, fullContent.Len())
	}

	return fullContent.String(), nil
}

// inputFormatHint 追加到 system prompt 末尾的格式提示
const inputFormatHint = "\n\n[格式说明] 用户消息中，\u201C\u201D包裹的内容是{{user}}说出口的话；\uFF08\uFF09包裹的内容是{{user}}的内心想法，{{char}}无法感知到；其余内容是动作描写或旁白。"

// DebugEnabled 控制是否将 AI 响应写入文件，方便调试
var DebugEnabled = true

// debugLogResponse 记录 AI 响应调试信息
// 始终在终端打印摘要；DebugEnabled=true 时同时写入 data/debug_response_*.txt
func (s *ChatService) debugLogResponse(chatID, response string, err error) {
	charCount := len([]rune(response))
	byteCount := len(response)
	log.Printf("[AI响应] chat=%s 字符=%d 字节=%d 错误=%v", chatID, charCount, byteCount, err)

	if !DebugEnabled {
		return
	}

	debugFile := fmt.Sprintf("data/debug_response_%d.txt", time.Now().UnixMilli())
	content := fmt.Sprintf("=== 时间: %s ===\nchat_id: %s\n字符数: %d\n字节数: %d\n错误: %v\n\n--- 完整内容 ---\n%s\n",
		time.Now().Format("2006-01-02 15:04:05"), chatID, charCount, byteCount, err, response)
	os.WriteFile(debugFile, []byte(content), 0644)
}
