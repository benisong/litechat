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
	configStore    *store.ConfigStore
}

func NewChatService(
	chatStore *store.ChatStore,
	messageStore *store.MessageStore,
	characterStore *store.CharacterStore,
	presetStore *store.PresetStore,
	configStore *store.ConfigStore,
) *ChatService {
	return &ChatService{
		chatStore:      chatStore,
		messageStore:   messageStore,
		characterStore: characterStore,
		presetStore:    presetStore,
		configStore:    configStore,
	}
}

// StreamResponse SSE 流式响应的回调函数类型
type StreamCallback func(token string) error

// SendMessage 发送消息并流式返回 AI 响应
func (s *ChatService) SendMessage(chatID, content, presetID string, callback StreamCallback) (string, error) {
	// 获取对话信息
	chat, err := s.chatStore.GetByID(chatID)
	if err != nil {
		return "", fmt.Errorf("对话不存在: %w", err)
	}

	// 获取角色信息
	character, err := s.characterStore.GetByID(chat.CharacterID)
	if err != nil {
		return "", fmt.Errorf("角色不存在: %w", err)
	}

	// 获取预设
	presetIDToUse := presetID
	if presetIDToUse == "" {
		presetIDToUse = chat.PresetID
	}

	var preset *model.Preset
	if presetIDToUse != "" {
		preset, err = s.presetStore.GetByID(presetIDToUse)
		if err != nil {
			preset = nil
		}
	}
	if preset == nil {
		preset, err = s.presetStore.GetDefault()
		if err != nil {
			// 使用内置默认预设
			preset = &model.Preset{
				SystemPrompt: "你是{{char}}。请根据角色设定进行扮演。",
				Temperature:  0.8,
				MaxTokens:    2048,
				TopP:         0.9,
			}
		}
	}

	// 获取历史消息
	history, err := s.messageStore.ListByChatID(chatID)
	if err != nil {
		return "", fmt.Errorf("获取消息历史失败: %w", err)
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
	messages := s.buildMessages(preset, character, history, content)

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
	_ = s.chatStore.Touch(chatID)

	return fullResponse, nil
}

// replaceVars 替换提示词中的模板变量和 SillyTavern 宏
func (s *ChatService) replaceVars(template string, char *model.Character) string {
	result := template

	// 基础角色变量
	result = strings.ReplaceAll(result, "{{char}}", char.Name)
	result = strings.ReplaceAll(result, "{{description}}", char.Description)
	result = strings.ReplaceAll(result, "{{personality}}", char.Personality)
	result = strings.ReplaceAll(result, "{{scenario}}", char.Scenario)
	result = strings.ReplaceAll(result, "{{user}}", "User")
	result = strings.ReplaceAll(result, "{{User}}", "User")

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

// buildMessages 构建完整的消息列表
// 如果预设有多段 Prompts（高级模式），按 SillyTavern 格式注入
// 否则回退到简单模式（单段 SystemPrompt）
func (s *ChatService) buildMessages(preset *model.Preset, char *model.Character, history []*model.Message, userContent string) []model.ChatCompletionMessage {

	// 1. 组装聊天历史（含开场白 + 历史 + 当前用户消息）
	var chatHistory []model.ChatCompletionMessage
	if char.FirstMsg != "" && len(history) == 0 {
		chatHistory = append(chatHistory, model.ChatCompletionMessage{
			Role: "assistant", Content: char.FirstMsg,
		})
	} else {
		for _, msg := range history {
			chatHistory = append(chatHistory, model.ChatCompletionMessage{
				Role: msg.Role, Content: msg.Content,
			})
		}
	}
	chatHistory = append(chatHistory, model.ChatCompletionMessage{
		Role: "user", Content: userContent,
	})

	// 2. 尝试解析多段提示词
	var entries []model.PromptEntry
	if preset.Prompts != "" {
		if err := json.Unmarshal([]byte(preset.Prompts), &entries); err != nil {
			log.Printf("[预设] 解析多段提示词失败: %v，回退到简单模式", err)
			entries = nil
		}
	}

	// 3. 简单模式：单段 SystemPrompt
	if len(entries) == 0 {
		systemPrompt := s.replaceVars(preset.SystemPrompt, char)
		messages := []model.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
		}
		return append(messages, chatHistory...)
	}

	// 4. 高级模式：多段提示词注入
	// 过滤启用的条目并做变量替换
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

	// 按 order 排序（稳定排序）
	sortEntries(enabled)

	// 分为两组：depth=0 的放在聊天历史前面，depth>0 的插入到历史中
	var headEntries []model.PromptEntry  // 在聊天历史之前
	var injectEntries []model.PromptEntry // 插入到聊天历史中

	for _, e := range enabled {
		if e.InjectionDepth == 0 {
			headEntries = append(headEntries, e)
		} else {
			injectEntries = append(injectEntries, e)
		}
	}

	// 5. 组装最终消息列表
	// 先放 depth=0 的提示词
	var messages []model.ChatCompletionMessage
	for _, e := range headEntries {
		messages = append(messages, model.ChatCompletionMessage{
			Role: e.Role, Content: e.Content,
		})
	}

	// 在聊天历史中注入 depth>0 的提示词
	// injection_position=0（相对末尾）: depth=N 表示从末尾倒数第 N 条消息处插入
	// injection_position=1（绝对位置）: depth=N 表示在第 N 条消息后插入
	histLen := len(chatHistory)
	// 为每个注入点计算绝对位置
	type injection struct {
		pos int
		msg model.ChatCompletionMessage
	}
	var injections []injection

	for _, e := range injectEntries {
		var absPos int
		if e.InjectionPos == 1 {
			// 绝对位置
			absPos = e.InjectionDepth
		} else {
			// 相对末尾：depth=2 → 在倒数第 2 条消息前插入
			absPos = histLen - e.InjectionDepth
		}
		if absPos < 0 {
			absPos = 0
		}
		if absPos > histLen {
			absPos = histLen
		}
		injections = append(injections, injection{
			pos: absPos,
			msg: model.ChatCompletionMessage{Role: e.Role, Content: e.Content},
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

	// 复制聊天历史并插��
	result := make([]model.ChatCompletionMessage, len(chatHistory))
	copy(result, chatHistory)

	for _, inj := range injections {
		pos := inj.pos
		// 在 pos 位置插入
		result = append(result[:pos], append([]model.ChatCompletionMessage{inj.msg}, result[pos:]...)...)
	}

	messages = append(messages, result...)
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
