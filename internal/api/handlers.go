package api

import (
	"encoding/json"
	"fmt"
	"io"
	"litechat/internal/auth"
	"litechat/internal/model"
	"litechat/internal/service"
	"litechat/internal/store"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handlers 所有 API 处理器的集合
type Handlers struct {
	characterStore *store.CharacterStore
	chatStore      *store.ChatStore
	messageStore   *store.MessageStore
	presetStore    *store.PresetStore
	worldBookStore *store.WorldBookStore
	configStore    *store.ConfigStore
	userStore      *store.UserStore
	chatService    *service.ChatService
}

func NewHandlers(
	characterStore *store.CharacterStore,
	chatStore *store.ChatStore,
	messageStore *store.MessageStore,
	presetStore *store.PresetStore,
	worldBookStore *store.WorldBookStore,
	configStore *store.ConfigStore,
	userStore *store.UserStore,
	chatService *service.ChatService,
) *Handlers {
	return &Handlers{
		characterStore: characterStore,
		chatStore:      chatStore,
		messageStore:   messageStore,
		presetStore:    presetStore,
		worldBookStore: worldBookStore,
		configStore:    configStore,
		userStore:      userStore,
		chatService:    chatService,
	}
}

// ========== 认证 API ==========

// Login POST /api/auth/login 用户登录
func (h *Handlers) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找用户（按当前运行模式）
	currentMode := h.userStore.GetCurrentMode()
	user, err := h.userStore.GetByUsernameAndMode(req.Username, currentMode)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 验证密码
	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 生成 token
	token, err := auth.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, model.LoginResponse{
		Token: token,
		User:  *user,
	})
}

// GetCurrentUser GET /api/auth/me 获取当前用户信息
func (h *Handlers) GetCurrentUser(c *gin.Context) {
	userID := GetUserID(c)
	user, err := h.userStore.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// CreateUser POST /api/auth/users 创建用户（管理员）
func (h *Handlers) CreateUser(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// admin 用户唯一，不允许创建
	role := req.Role
	if role == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "管理员账户唯一，不能创建新管理员"})
		return
	}
	if role == "" {
		role = "user"
	}

	// 获取当前运行模式
	currentMode := h.userStore.GetCurrentMode()

	// 检查同模式下用户名是否已存在
	if existing, err := h.userStore.GetByUsernameAndMode(req.Username, currentMode); err == nil && existing.Role != "admin" {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名在当前模式下已存在"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	user := &model.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         role,
		Mode:         currentMode,
	}

	if err := h.userStore.Create(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.userStore.CreateDefaultCharacter(user.ID)
	c.JSON(http.StatusCreated, user)
}

// ListUsers GET /api/auth/users 列出当前模式下的用户（管理员）
func (h *Handlers) ListUsers(c *gin.Context) {
	currentMode := h.userStore.GetCurrentMode()
	users, err := h.userStore.List(currentMode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if users == nil {
		users = []*model.User{}
	}
	c.JSON(http.StatusOK, users)
}

// DeleteUser DELETE /api/auth/users/:id 删除用户（管理员）
func (h *Handlers) DeleteUser(c *gin.Context) {
	targetID := c.Param("id")
	currentUserID := GetUserID(c)

	// 不允许删除自己
	if targetID == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前登录用户"})
		return
	}

	if err := h.userStore.Delete(targetID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ChangePassword PUT /api/auth/password 修改密码
func (h *Handlers) ChangePassword(c *gin.Context) {
	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := GetUserID(c)
	user, err := h.userStore.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证旧密码
	if !auth.VerifyPassword(user.PasswordHash, req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码错误"})
		return
	}

	// 哈希新密码
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	if err := h.userStore.UpdatePassword(userID, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// UpdateUser PUT /api/auth/users/:id 管理员编辑用户（用户名/密码/角色）
func (h *Handlers) UpdateUser(c *gin.Context) {
	targetID := c.Param("id")

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 禁止将角色改为 admin
	if req.Role == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能将用户提升为管理员"})
		return
	}

	// 获取目标用户
	target, err := h.userStore.GetByID(targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 用户名
	username := target.Username
	if req.Username != "" {
		username = req.Username
	}

	// 角色
	role := target.Role
	if req.Role != "" {
		role = req.Role
	}

	// 密码（如果提供则更新）
	var passwordHash string
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
			return
		}
		passwordHash = hash
	}

	if err := h.userStore.UpdateUser(targetID, username, role, passwordHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的用户
	updated, _ := h.userStore.GetByID(targetID)
	if updated != nil {
		c.JSON(http.StatusOK, updated)
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
	}
}

// ========== 角色卡 API ==========

// ListCharacters GET /api/characters
func (h *Handlers) ListCharacters(c *gin.Context) {
	userID := GetUserID(c)
	list, err := h.characterStore.List(userID)
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
	userID := GetUserID(c)
	char, err := h.characterStore.GetByID(c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	c.JSON(http.StatusOK, char)
}

// CreateCharacter POST /api/characters
func (h *Handlers) CreateCharacter(c *gin.Context) {
	userID := GetUserID(c)
	var char model.Character
	if err := c.ShouldBindJSON(&char); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.characterStore.Create(&char, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, char)
}

// GenerateCharacterCard POST /api/characters/generate
func (h *Handlers) GenerateCharacterCard(c *gin.Context) {
	var req model.GenerateCharacterCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	draft, err := h.chatService.GenerateCharacterCardDraft(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, model.GenerateCharacterCardResponse{Draft: *draft})
}

// UpdateCharacter PUT /api/characters/:id
func (h *Handlers) UpdateCharacter(c *gin.Context) {
	userID := GetUserID(c)
	var char model.Character
	if err := c.ShouldBindJSON(&char); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	char.ID = c.Param("id")
	if err := h.characterStore.Update(&char, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 重新查询以返回完整的数据库数据（包括正确的时间字段）
	updated, err := h.characterStore.GetByID(char.ID, userID)
	if err != nil {
		c.JSON(http.StatusOK, char)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteCharacter DELETE /api/characters/:id
func (h *Handlers) DeleteCharacter(c *gin.Context) {
	userID := GetUserID(c)
	if err := h.characterStore.Delete(c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 对话 API ==========

// ListChats GET /api/chats
func (h *Handlers) ListChats(c *gin.Context) {
	userID := GetUserID(c)
	characterID := c.Query("character_id")
	var err error
	var list []*model.Chat

	if characterID != "" {
		list, err = h.chatStore.ListByCharacter(characterID, userID)
	} else {
		list, err = h.chatStore.ListAll(userID)
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
	userID := GetUserID(c)
	var chat model.Chat
	if err := c.ShouldBindJSON(&chat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果未指定预设，自动分配默认预设
	if chat.PresetID == "" {
		isServiceMode := h.userStore.GetCurrentMode() == "service"
		var preset *model.Preset
		var err error
		if isServiceMode {
			preset, err = h.presetStore.GetDefaultAdmin()
		} else {
			preset, err = h.presetStore.GetDefault(userID)
		}
		if err == nil && preset != nil {
			chat.PresetID = preset.ID
		}
	}

	if err := h.chatStore.Create(&chat, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, chat)
}

// GetChat GET /api/chats/:id
func (h *Handlers) GetChat(c *gin.Context) {
	userID := GetUserID(c)
	chat, err := h.chatStore.GetByID(c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}
	c.JSON(http.StatusOK, chat)
}

// DeleteChat DELETE /api/chats/:id
func (h *Handlers) DeleteChat(c *gin.Context) {
	userID := GetUserID(c)
	if err := h.chatStore.Delete(c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetMessages GET /api/chats/:id/messages
func (h *Handlers) GetMessages(c *gin.Context) {
	userID := GetUserID(c)
	// 先验证对话属于当前用户
	_, err := h.chatStore.GetByID(c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

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
	userID := GetUserID(c)
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

	_, err := h.chatService.SendMessage(chatID, req.Content, req.PresetID, userID, callback)
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

// DeleteMessageCascade DELETE /api/chats/:id/messages/:msgId 级联删除（该消息及之后的所有消息）
func (h *Handlers) DeleteMessageCascade(c *gin.Context) {
	chatID := c.Param("id")
	msgID := c.Param("msgId")
	userID := GetUserID(c)

	// 验证对话属于当前用户
	if _, err := h.chatStore.GetByID(chatID, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	count, err := h.messageStore.DeleteFromID(msgID, chatID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("已删除 %d 条消息", count), "deleted": count})
}

// RegenerateMessage POST /api/chats/:id/regenerate 重新生成最后一条 AI 回复
func (h *Handlers) RegenerateMessage(c *gin.Context) {
	chatID := c.Param("id")
	userID := GetUserID(c)

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

	// 流式回调
	callback := func(token string) error {
		tokenBytes, _ := json.Marshal(map[string]string{"token": token})
		_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", string(tokenBytes))
		if err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	_, err := h.chatService.RetryLastOrRegenerate(chatID, userID, callback)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}

	fmt.Fprintf(c.Writer, "data: {\"done\":true}\n\n")
	flusher.Flush()
}

// UpdateUserInfo PUT /api/settings/user-info 保存用户信息（所有用户可用）
func (h *Handlers) UpdateUserInfo(c *gin.Context) {
	var req struct {
		DefaultUserName   string `json:"default_user_name"`
		DefaultUserDetail string `json:"default_user_detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.configStore.Set("default_user_name", req.DefaultUserName)
	h.configStore.Set("default_user_detail", req.DefaultUserDetail)
	c.JSON(http.StatusOK, gin.H{"message": "用户信息已保存"})
}

// ========== 预设 API ==========

// ListPresets GET /api/presets
func (h *Handlers) ListPresets(c *gin.Context) {
	userID := GetUserID(c)
	list, err := h.presetStore.List(userID)
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
	userID := GetUserID(c)
	preset, err := h.presetStore.GetByID(c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "预设不存在"})
		return
	}
	c.JSON(http.StatusOK, preset)
}

// CreatePreset POST /api/presets
func (h *Handlers) CreatePreset(c *gin.Context) {
	userID := GetUserID(c)
	var preset model.Preset
	if err := c.ShouldBindJSON(&preset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.presetStore.Create(&preset, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, preset)
}

// UpdatePreset PUT /api/presets/:id
func (h *Handlers) UpdatePreset(c *gin.Context) {
	userID := GetUserID(c)
	var preset model.Preset
	if err := c.ShouldBindJSON(&preset); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preset.ID = c.Param("id")
	if err := h.presetStore.Update(&preset, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preset)
}

// DeletePreset DELETE /api/presets/:id
func (h *Handlers) DeletePreset(c *gin.Context) {
	userID := GetUserID(c)
	if err := h.presetStore.Delete(c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ========== 世界书 API ==========

// ListWorldBooks GET /api/worldbooks
func (h *Handlers) ListWorldBooks(c *gin.Context) {
	userID := GetUserID(c)
	list, err := h.worldBookStore.List(userID)
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
	userID := GetUserID(c)
	wb, err := h.worldBookStore.GetByID(c.Param("id"), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "世界书不存在"})
		return
	}
	c.JSON(http.StatusOK, wb)
}

// CreateWorldBook POST /api/worldbooks
func (h *Handlers) CreateWorldBook(c *gin.Context) {
	userID := GetUserID(c)
	var wb model.WorldBook
	if err := c.ShouldBindJSON(&wb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.worldBookStore.Create(&wb, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, wb)
}

// UpdateWorldBook PUT /api/worldbooks/:id
func (h *Handlers) UpdateWorldBook(c *gin.Context) {
	userID := GetUserID(c)
	var wb model.WorldBook
	if err := c.ShouldBindJSON(&wb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	wb.ID = c.Param("id")
	if err := h.worldBookStore.Update(&wb, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wb)
}

// DeleteWorldBook DELETE /api/worldbooks/:id
func (h *Handlers) DeleteWorldBook(c *gin.Context) {
	userID := GetUserID(c)
	if err := h.worldBookStore.Delete(c.Param("id"), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// CreateWorldBookEntry POST /api/worldbooks/:id/entries
func (h *Handlers) CreateWorldBookEntry(c *gin.Context) {
	userID := GetUserID(c)
	var entry model.WorldBookEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry.WorldBookID = c.Param("id")
	if err := h.worldBookStore.CreateEntry(&entry, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entry)
}

// UpdateWorldBookEntry PUT /api/worldbooks/entries/:entryId
func (h *Handlers) UpdateWorldBookEntry(c *gin.Context) {
	userID := GetUserID(c)
	var entry model.WorldBookEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entry.ID = c.Param("entryId")
	if err := h.worldBookStore.UpdateEntry(&entry, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// DeleteWorldBookEntry DELETE /api/worldbooks/entries/:entryId
func (h *Handlers) DeleteWorldBookEntry(c *gin.Context) {
	userID := GetUserID(c)
	if err := h.worldBookStore.DeleteEntry(c.Param("entryId"), userID); err != nil {
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
	settings := model.AppSettings{
		UseDefaultModelForCharacterCard: true,
	}
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
	h.configStore.Set("use_default_model_for_character_card", fmt.Sprintf("%t", settings.UseDefaultModelForCharacterCard))
	h.configStore.Set("character_card_model", settings.CharacterCardModel)
	if settings.Theme != "" {
		h.configStore.Set("theme", settings.Theme)
	}
	if settings.ServiceMode != "" {
		h.configStore.Set("service_mode", settings.ServiceMode)
	}
	// 用户信息字段始终保存（允许清空）
	h.configStore.Set("default_user_name", settings.DefaultUserName)
	h.configStore.Set("default_user_detail", settings.DefaultUserDetail)

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
