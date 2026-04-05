package api

import (
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/model"
	"litechat/internal/service"
	"litechat/internal/store"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handlers 所有 API 处理器的集合
type Handlers struct {
	characterStore  *store.CharacterStore
	chatStore       *store.ChatStore
	messageStore    *store.MessageStore
	presetStore     *store.PresetStore
	worldBookStore  *store.WorldBookStore
	configStore     *store.ConfigStore
	chatService     *service.ChatService
}

func NewHandlers(
	characterStore *store.CharacterStore,
	chatStore *store.ChatStore,
	messageStore *store.MessageStore,
	presetStore *store.PresetStore,
	worldBookStore *store.WorldBookStore,
	configStore *store.ConfigStore,
	chatService *service.ChatService,
) *Handlers {
	return &Handlers{
		characterStore: characterStore,
		chatStore:      chatStore,
		messageStore:   messageStore,
		presetStore:    presetStore,
		worldBookStore: worldBookStore,
		configStore:    configStore,
		chatService:    chatService,
	}
}

// ========== 角色卡 API ==========

// ListCharacters GET /api/characters
func (h *Handlers) ListCharacters(c *gin.Context) {
	list, err := h.characterStore.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*model.Character{}
	}
	c.JSON(http.StatusOK, list)
}

// GetCharacter GET /api/characters/:id
func (h *Handlers) GetCharacter(c *gin.Context) {
	char, err := h.characterStore.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	c.JSON(http.StatusOK, char)
}

// CreateCharacter POST /api/characters
func (h *Handlers) CreateCharacter(c *gin.Context) {
	var char model.Character
	if err := c.ShouldBindJSON(&char); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.characterStore.Create(&char); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, char)
}

// UpdateCharacter PUT /api/characters/:id
func (h *Handlers) UpdateCharacter(c *gin.Context) {
	var char model.Character
	if err := c.ShouldBindJSON(&char); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	char.ID = c.Param("id")
	if err := h.characterStore.Update(&char); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, char)
}

// DeleteCharacter DELETE /api/characters/:id
func (h *Handlers) DeleteCharacter(c *gin.Context) {
	if err := h.characterStore.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 对话 API ==========

// ListChats GET /api/chats
func (h *Handlers) ListChats(c *gin.Context) {
	characterID := c.Query("character_id")
	var err error
	var list []*model.Chat

	if characterID != "" {
		list, err = h.chatStore.ListByCharacter(characterID)
	} else {
		list, err = h.chatStore.ListAll()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*model.Chat{}
	}
	c.JSON(http.StatusOK, list)
}

// CreateChat POST /api/chats
func (h *Handlers) CreateChat(c *gin.Context) {
	var chat model.Chat
	if err := c.ShouldBindJSON(&chat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.chatStore.Create(&chat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, chat)
}

// GetChat GET /api/chats/:id
func (h *Handlers) GetChat(c *gin.Context) {
	chat, err := h.chatStore.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}
	c.JSON(http.StatusOK, chat)
}

// DeleteChat DELETE /api/chats/:id
func (h *Handlers) DeleteChat(c *gin.Context) {
	if err := h.chatStore.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetMessages GET /api/chats/:id/messages
func (h *Handlers) GetMessages(c *gin.Context) {
	messages, err := h.messageStore.ListByChatID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if messages == nil {
		messages = []*model.Message{}
	}
	c.JSON(http.StatusOK, messages)
}

// SendMessage POST /api/chats/:id/messages  (SSE 流式响应)
func (h *Handlers) SendMessage(c *gin.Context) {
	chatID := c.Param("id")

	var req model.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "不支持流式响应"})
		return
	}

	// 流式回调：每次收到 token 就发送给客户端
	callback := func(token string) error {
		// 用 encoding/json 正确编码，避免 %q 对中文的转义问题
		tokenBytes, _ := json.Marshal(map[string]string{"token": token})
		_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", string(tokenBytes))
		if err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_, err := h.chatService.SendMessage(chatID, req.Content, req.PresetID, callback)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}

	// 发送结束标记
	fmt.Fprintf(c.Writer, "data: {\"done\":true}\n\n")
	flusher.Flush()
}

// DeleteMessage DELETE /api/messages/:id
func (h *Handlers) DeleteMessage(c *gin.Context) {
	if err := h.messageStore.DeleteByID(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 预设 API ==========

// ListPresets GET /api/presets
func (h *Handlers) ListPresets(c *gin.Context) {
	list, err := h.presetStore.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*model.Preset{}
	}
	c.JSON(http.StatusOK, list)
}

// GetPreset GET /api/presets/:id
func (h *Handlers) GetPreset(c *gin.Context) {
	preset, err := h.presetStore.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "预设不存在"})
		return
	}
	c.JSON(http.StatusOK, preset)
}

// CreatePreset POST /api/presets
func (h *Handlers) CreatePreset(c *gin.Context) {
	var preset model.Preset
	if err := c.ShouldBindJSON(&preset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.presetStore.Create(&preset); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, preset)
}

// UpdatePreset PUT /api/presets/:id
func (h *Handlers) UpdatePreset(c *gin.Context) {
	var preset model.Preset
	if err := c.ShouldBindJSON(&preset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preset.ID = c.Param("id")
	if err := h.presetStore.Update(&preset); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preset)
}

// DeletePreset DELETE /api/presets/:id
func (h *Handlers) DeletePreset(c *gin.Context) {
	if err := h.presetStore.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 世界书 API ==========

// ListWorldBooks GET /api/worldbooks
func (h *Handlers) ListWorldBooks(c *gin.Context) {
	list, err := h.worldBookStore.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if list == nil {
		list = []*model.WorldBook{}
	}
	c.JSON(http.StatusOK, list)
}

// GetWorldBook GET /api/worldbooks/:id
func (h *Handlers) GetWorldBook(c *gin.Context) {
	wb, err := h.worldBookStore.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "世界书不存在"})
		return
	}
	c.JSON(http.StatusOK, wb)
}

// CreateWorldBook POST /api/worldbooks
func (h *Handlers) CreateWorldBook(c *gin.Context) {
	var wb model.WorldBook
	if err := c.ShouldBindJSON(&wb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.worldBookStore.Create(&wb); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, wb)
}

// UpdateWorldBook PUT /api/worldbooks/:id
func (h *Handlers) UpdateWorldBook(c *gin.Context) {
	var wb model.WorldBook
	if err := c.ShouldBindJSON(&wb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wb.ID = c.Param("id")
	if err := h.worldBookStore.Update(&wb); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wb)
}

// DeleteWorldBook DELETE /api/worldbooks/:id
func (h *Handlers) DeleteWorldBook(c *gin.Context) {
	if err := h.worldBookStore.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// CreateWorldBookEntry POST /api/worldbooks/:id/entries
func (h *Handlers) CreateWorldBookEntry(c *gin.Context) {
	var entry model.WorldBookEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry.WorldBookID = c.Param("id")
	if err := h.worldBookStore.CreateEntry(&entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// UpdateWorldBookEntry PUT /api/worldbooks/entries/:entryId
func (h *Handlers) UpdateWorldBookEntry(c *gin.Context) {
	var entry model.WorldBookEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry.ID = c.Param("entryId")
	if err := h.worldBookStore.UpdateEntry(&entry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// DeleteWorldBookEntry DELETE /api/worldbooks/entries/:entryId
func (h *Handlers) DeleteWorldBookEntry(c *gin.Context) {
	if err := h.worldBookStore.DeleteEntry(c.Param("entryId")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 配置 API ==========

// GetSettings GET /api/settings
func (h *Handlers) GetSettings(c *gin.Context) {
	settings, err := h.configStore.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 不返回完整 API 密钥，只返回是否已设置
	if settings.APIKey != "" {
		settings.APIKey = "***" + settings.APIKey[max(0, len(settings.APIKey)-4):]
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings PUT /api/settings
func (h *Handlers) UpdateSettings(c *gin.Context) {
	var settings model.AppSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 逐个保存配置项
	if settings.APIEndpoint != "" {
		h.configStore.Set("api_endpoint", settings.APIEndpoint)
	}
	// 只有非掩码值才保存 API 密钥
	if settings.APIKey != "" && !isKeyMasked(settings.APIKey) {
		h.configStore.Set("api_key", settings.APIKey)
	}
	if settings.DefaultModel != "" {
		h.configStore.Set("default_model", settings.DefaultModel)
	}
	if settings.Theme != "" {
		h.configStore.Set("theme", settings.Theme)
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置已保存"})
}

// FetchModels GET /api/models — 从配置的 API 端点获取可用模型列表
func (h *Handlers) FetchModels(c *gin.Context) {
	// 支持通过 query 传入临时端点和密钥（设置页保存前试用）
	endpoint := c.Query("endpoint")
	apiKey := c.Query("key")

	if endpoint == "" || apiKey == "" {
		// 从数据库读取已保存的配置
		settings, err := h.configStore.GetSettings()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配置失败"})
			return
		}
		if endpoint == "" {
			endpoint = settings.APIEndpoint
		}
		if apiKey == "" {
			apiKey = settings.APIKey
		}
	}

	if endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 API 端点"})
		return
	}
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 API 密钥"})
		return
	}

	// 请求 /models
	url := strings.TrimRight(endpoint, "/") + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的端点地址"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 15 * 1000000000} // 15s
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("请求失败: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("API 返回 %d: %s", resp.StatusCode, string(body))})
		return
	}

	// 解析模型列表（OpenAI 兼容格式）
	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析模型列表失败"})
		return
	}

	// 提取模型 ID 列表
	models := make([]gin.H, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, gin.H{
			"id":       m.ID,
			"owned_by": m.OwnedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

// isKeyMasked 判断 API 密钥是否是掩码值
func isKeyMasked(key string) bool {
	return len(key) > 3 && key[:3] == "***"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
