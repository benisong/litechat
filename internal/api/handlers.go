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
	summaryService *service.SummaryService
}

const (
	statusBarEntryKey             = "状态栏"
	defaultStatusBarTemplate      = "'''\n【状态栏】\n时间：{{time}}\n地点：[当前所在地点]\n我方状态：[用户角色当前的身体/心情/处境]\n对方状态：[角色当前的身体/心情/处境]\n关系：[双方当前关系]\n当前事件：[此刻正在发生的事]\n'''"
	statusBarInjectionPosition    = 1
	statusBarInjectionDepth       = 0
	statusBarOrder                = 0
	statusBarRole                 = "system"
)

func isStatusBarEntry(entry *model.WorldBookEntry) bool {
	return entry != nil && strings.TrimSpace(entry.Keys) == statusBarEntryKey
}

// ===== AI 文字问题修正（默认全局纠错条目）=====

const textFixEntryKey = "文字问题修正"

// defaultTextFixInstruction：指令逻辑用英文（约束力更强），仅点名的中文套路样本保留中文。
const defaultTextFixInstruction = `[Writing Style Correction · Constraints]

The following rules apply to all of your narration and description from now on. The goal is to keep the prose specific, restrained, and grounded in this particular character and this particular moment — avoiding clichéd, cheap, or exaggerated expression. Always write in Chinese; only the reasoning of these rules is in English.

1. Avoid clichéd body-language and micro-expression descriptions.
Stock phrases such as "指节发白", "喉结滚动", "瞳孔骤缩", "心头一颤", "呼吸一滞", "血液凝固", "眼底闪过一丝××", "嘴角勾起一抹弧度", "眸光微动" have been so overused that they are now cheap, predictable, and applicable to any character in any scene — they carry almost no information. Avoid them. When you do describe a physical reaction, write something that genuinely belongs to THIS character in THIS situation: a small gesture with personal character, a specific action interrupted mid-way, a sentence left unfinished — not a generic template.

2. Avoid worn-out metaphors and atmospheric clichés.
Comparisons and mood-setting phrases like "眼泪像断了线的珠子", "像石头掉进湖里", "心如刀绞", "如坠冰窟", "空气仿佛凝固", "时间静止", "世界只剩两人" are equally stale. Do not lean on them to convey emotion. Let emotion emerge from concrete detail, action, dialogue, and circumstance — never paper over it with a tired metaphor.

3. Match the intensity of the description to the real intensity of the moment.
Strong physiological reactions such as "指甲掐进掌心渗血", "咬破嘴唇", "掐进肉里" are not forbidden, but they belong only to genuinely extreme moments (great pain, the verge of collapse, life-or-death stakes). Do NOT use such extreme reactions to render what is essentially a minor feeling (mild nervousness, displeasure, concern, hesitation) — that is melodramatic over-exaggeration, both unrealistic and tiresome. Keep the intensity of your description proportional to the true intensity of the situation: render subtle feelings in restrained, specific, character-appropriate ways, and reserve strong physiological description for moments that have actually reached a critical point.

Overall: use fewer big words and ready-made clichés; write more specific, restrained detail that fits the character and the present moment. Prefer plain and true over ornate and hollow.`

func isTextFixEntry(entry *model.WorldBookEntry) bool {
	return entry != nil && strings.TrimSpace(entry.Keys) == textFixEntryKey
}

// buildTextFixEntry：与状态栏对称的标准条目，constant=true 恒定注入，不依赖关键词。
func buildTextFixEntry(worldBookID string) model.WorldBookEntry {
	return model.WorldBookEntry{
		WorldBookID:    worldBookID,
		Keys:           textFixEntryKey,
		SecondaryKeys:  "",
		Content:        defaultTextFixInstruction,
		Enabled:        true,
		Constant:       true,
		Priority:       0,
		InjectionPos:   1,
		InjectionDepth: 0,
		ScanDepth:      0,
		CaseSensitive:  false,
		Order:          0,
		Role:           "system",
	}
}

// ===== 条目模板库（出厂预设，写死进程序；用户可在生成的条目上自行修改）=====

// EntryTemplate 一个可供用户在“新建条目”时选择的标准模板。
type entryTemplate struct {
	Key            string `json:"key"`
	Label          string `json:"label"`
	Desc           string `json:"desc"`
	Keys           string `json:"keys"`
	Constant       bool   `json:"constant"`
	InjectionPos   int    `json:"injection_position"`
	InjectionDepth int    `json:"injection_depth"`
	Content        string `json:"content"`
}

// 文风模板共用：作为软引导注入到对话流偏上的位置（d4），不抢系统层注意力。
// 文字问题修正则注入 d0（紧贴系统提示词，强约束）。
const styleInjectionDepth = 4

const styleMurakami = `[Writing Style · Haruki Murakami]
From now on, write the narration and prose in the style of Haruki Murakami (村上春树). Keep this as a soft stylistic tendency, not a hard rule. Always write in Chinese.
风格要点：
- 疏离、克制、淡淡的孤独感，叙述者像隔着一层玻璃观察世界。
- 大量平实的日常细节（音乐、食物、天气、做饭、跑步），在琐碎里渗出情绪。
- 比喻奇特而精确，常把抽象情绪具象成不相干的事物。
- 句子节奏舒缓，留白多，不急于解释，对话简短而有余味。
- 不煽情、不堆砌华丽辞藻；情绪藏在事实底下，而不是直接喊出来。`

const styleGuLong = `[Writing Style · Gu Long]
From now on, write the narration and prose in the style of Gu Long (古龙) wuxia. Keep this as a soft stylistic tendency, not a hard rule. Always write in Chinese.
风格要点：
- 短句、断句，一句一行的节奏感，像刀光一闪。
- 大量留白与悬念，重意境与气氛，轻具体招式描写。
- 对白机锋，冷峻、简练、话里有话，常以反问和顿挫制造张力。
- 善用意象（月、剑、酒、风、寂寞）渲染情绪与宿命感。
- 不写流水账，重在“顿”和“势”，关键处戛然而止。`

const styleFemaleWeb = `[Writing Style · 网文女频]
From now on, write in the style of popular Chinese web fiction for female readers (女频). Keep this as a soft stylistic tendency, not a hard rule. Always write in Chinese.
风格要点：情绪细腻、注重内心戏与感官细节，节奏明快带钩子，强调氛围感、心动瞬间与情感拉扯；对白有张力，常用细节体现人物地位与魅力。
示例语句（学其腔调，不要照抄）：
- 他骨节分明的手指扣住她的腕，力道不重，却让她半步也退不开。
- “怕我？”他低低地笑，气息扫过她耳廓，“可你刚才，分明是想靠近我的。”
- 满室宾客喧哗，她却只听得见自己心跳，一下，又一下，吵得厉害。
- 他替她拢了拢披散的发，动作熟稔得像做过千百遍，眼底却没什么温度。
- 她以为自己藏得很好，直到对上那双看透一切的眼睛。
- 那一瞬间，所有的伪装都碎了，她在他面前，无所遁形。
- 他向来不近人情，偏偏在她面前，破了一次又一次的例。`

const styleMaleWeb = `[Writing Style · 网文男频]
From now on, write in the style of popular Chinese web fiction for male readers (男频). Keep this as a soft stylistic tendency, not a hard rule. Always write in Chinese.
风格要点：节奏爽利、信息密度高，重事件推进与实力/格局展现，对白干脆有气场，擅长制造反转、压迫感与“装逼打脸”的爽点；环境与气氛服务于冲突。
示例语句（学其腔调，不要照抄）：
- 他抬起头，淡淡地看了对方一眼：“就凭你们，也配？”
- 全场死一般地寂静，没人敢相信，刚才那一掌，竟是他随手挥出的。
- 三年前他被踩进泥里，三年后他归来，要让所有人仰望。
- “记住我的名字。”他转身离去，留下满地不敢吭声的人。
- 实力，从来不需要解释；拳头落下时，质疑自然就闭嘴了。
- 对方脸色骤变——他这才意识到，眼前这个年轻人，深不可测。
- 规矩是强者定的，而从今天起，这里我说了算。`

// entryTemplates：库里有几个就加载几个（前端动态渲染）。第一项是文字问题修正（d0 强约束），
// 其余为文风模板（d4 软引导）。这些只是出厂起点，用户可在生成的条目上自行修改。
var entryTemplates = []entryTemplate{
	{
		Key:            "text_fix",
		Label:          "AI 文字问题修正",
		Desc:           "纠正套路化描写、烂俗比喻与生理夸张，全局恒定生效（强约束）。",
		Keys:           textFixEntryKey,
		Constant:       true,
		InjectionPos:   1,
		InjectionDepth: 0,
		Content:        defaultTextFixInstruction,
	},
	{
		Key:            "style_murakami",
		Label:          "文风 · 村上春树",
		Desc:           "疏离克制、日常细节里渗出孤独感的叙事腔调。",
		Keys:           "文风·村上春树",
		Constant:       true,
		InjectionPos:   1,
		InjectionDepth: styleInjectionDepth,
		Content:        styleMurakami,
	},
	{
		Key:            "style_gulong",
		Label:          "文风 · 古龙",
		Desc:           "短句断句、留白机锋、意境与宿命感的武侠腔调。",
		Keys:           "文风·古龙",
		Constant:       true,
		InjectionPos:   1,
		InjectionDepth: styleInjectionDepth,
		Content:        styleGuLong,
	},
	{
		Key:            "style_female_web",
		Label:          "文风 · 网文女频",
		Desc:           "细腻、心动、情感拉扯，带示例语句锚定腔调。",
		Keys:           "文风·网文女频",
		Constant:       true,
		InjectionPos:   1,
		InjectionDepth: styleInjectionDepth,
		Content:        styleFemaleWeb,
	},
	{
		Key:            "style_male_web",
		Label:          "文风 · 网文男频",
		Desc:           "爽利、格局、装逼打脸，带示例语句锚定腔调。",
		Keys:           "文风·网文男频",
		Constant:       true,
		InjectionPos:   1,
		InjectionDepth: styleInjectionDepth,
		Content:        styleMaleWeb,
	},
}

func buildStatusBarEntry(worldBookID string) model.WorldBookEntry {
	return model.WorldBookEntry{
		WorldBookID:    worldBookID,
		Keys:           statusBarEntryKey,
		SecondaryKeys:  "",
		Content:        defaultStatusBarTemplate,
		Enabled:        true,
		Constant:       true,
		Priority:       0,
		InjectionPos:   statusBarInjectionPosition,
		InjectionDepth: statusBarInjectionDepth,
		ScanDepth:      0,
		CaseSensitive:  false,
		Order:          statusBarOrder,
		Role:           statusBarRole,
	}
}

func sanitizeStatusBarEntryForUpdate(existing *model.WorldBookEntry, incoming *model.WorldBookEntry) {
	if existing == nil || incoming == nil {
		return
	}
	incoming.WorldBookID = existing.WorldBookID
	incoming.Keys = statusBarEntryKey
	incoming.SecondaryKeys = ""
	incoming.Constant = true
	incoming.Priority = existing.Priority
	incoming.InjectionPos = statusBarInjectionPosition
	incoming.InjectionDepth = statusBarInjectionDepth
	incoming.ScanDepth = 0
	incoming.CaseSensitive = false
	incoming.Order = statusBarOrder
	incoming.Role = statusBarRole
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
	summaryService *service.SummaryService,
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
		summaryService: summaryService,
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

// UpdateCurrentUserProfile PUT /api/auth/me/profile 更新当前用户资料
func (h *Handlers) UpdateCurrentUserProfile(c *gin.Context) {
	userID := GetUserID(c)
	role, _ := c.Get("role")
	if role == "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "管理员不需要用户信息"})
		return
	}

	var req model.UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.UserName = strings.TrimSpace(req.UserName)
	if req.UserName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名称不能为空"})
		return
	}

	if err := h.userStore.UpdateProfile(userID, req.UserName, req.UserDetail); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userStore.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "用户信息已保存"})
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
	// 附带状态栏本地渲染配色（来自角色绑定世界书的状态栏条目），供前端渲染
	if entry, e := h.worldBookStore.GetStatusBarEntry(userID, chat.CharacterID); e == nil && entry != nil {
		chat.StatusBarBg = entry.BgColor
		chat.StatusBarFg = entry.FontColor
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
	userID := GetUserID(c)
	msg, err := h.messageStore.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "消息不存在"})
		return
	}
	if _, err := h.chatStore.GetByID(msg.ChatID, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对话不存在"})
		return
	}

	if err := h.messageStore.DeleteByID(msg.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.summaryService != nil {
		h.summaryService.InvalidateFromSeq(msg.ChatID, msg.Seq)
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

	msg, err := h.messageStore.GetByID(msgID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "消息不存在"})
		return
	}

	count, err := h.messageStore.DeleteFromID(msgID, chatID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.summaryService != nil {
		h.summaryService.InvalidateFromSeq(chatID, msg.Seq)
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
	if strings.TrimSpace(wb.CharacterID) != "" {
		statusEntry := buildStatusBarEntry(wb.ID)
		if err := h.worldBookStore.CreateEntry(&statusEntry, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	// 仅【全局世界书】（未绑角色）勾选「AI 文字问题修正」时，自动插入纠错条目。
	if wb.EnableTextFix && strings.TrimSpace(wb.CharacterID) == "" {
		fixEntry := buildTextFixEntry(wb.ID)
		if err := h.worldBookStore.CreateEntry(&fixEntry, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusCreated, wb)
}

// ListEntryTemplates GET /api/worldbooks/entry-templates
// 返回写死的出厂条目模板库（前端动态渲染，库里有几个画几个）。
func (h *Handlers) ListEntryTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"templates": entryTemplates})
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
	if isStatusBarEntry(&entry) {
		special := buildStatusBarEntry(entry.WorldBookID)
		special.Content = entry.Content
		if strings.TrimSpace(special.Content) == "" {
			special.Content = defaultStatusBarTemplate
		}
		special.Enabled = entry.Enabled
		special.BgColor = entry.BgColor
		special.FontColor = entry.FontColor
		entry = special
	}
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

	existing, err := h.worldBookStore.GetEntryByID(entry.ID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	if isStatusBarEntry(existing) {
		sanitizeStatusBarEntryForUpdate(existing, &entry)
	}

	if err := h.worldBookStore.UpdateEntry(&entry, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

// DeleteWorldBookEntry DELETE /api/worldbooks/entries/:entryId
func (h *Handlers) DeleteWorldBookEntry(c *gin.Context) {
	userID := GetUserID(c)
	existing, err := h.worldBookStore.GetEntryByID(c.Param("entryId"), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "条目不存在"})
		return
	}
	if isStatusBarEntry(existing) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态栏特殊条目不允许删除"})
		return
	}
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
		UseDefaultModelForMemory:        true,
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
	h.configStore.Set("use_default_model_for_memory", fmt.Sprintf("%t", settings.UseDefaultModelForMemory))
	h.configStore.Set("memory_model", settings.MemoryModel)
	h.configStore.Set("memory_prompt_suffix", settings.MemoryPromptSuffix)
	if settings.Theme != "" {
		h.configStore.Set("theme", settings.Theme)
	}
	if settings.ServiceMode != "" {
		h.configStore.Set("service_mode", settings.ServiceMode)
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
