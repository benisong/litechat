package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter 配置路由
func SetupRouter(h *Handlers) *gin.Engine {
	r := gin.Default()

	// CORS 配置（开发环境允许所有来源）
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// 公开路由（不需要认证）
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/integrations/napcat/callback", h.HandleNapCatCallback)

	// API 路由组（需要 JWT 认证）
	api := r.Group("/api")
	api.Use(JWTAuthMiddleware())
	{
		// 当前用户信息
		api.GET("/auth/me", h.GetCurrentUser)
		api.PUT("/auth/me/profile", h.UpdateCurrentUserProfile)
		api.PUT("/auth/password", h.ChangePassword)

		// 用户管理（仅管理员）
		api.POST("/auth/users", AdminOnly(), h.CreateUser)
		api.GET("/auth/users", AdminOnly(), h.ListUsers)
		api.PUT("/auth/users/:id", AdminOnly(), h.UpdateUser)
		api.DELETE("/auth/users/:id", AdminOnly(), h.DeleteUser)

		// 角色卡
		api.GET("/characters", h.ListCharacters)
		api.POST("/characters", h.CreateCharacter)
		api.POST("/characters/generate", h.GenerateCharacterCard)
		api.GET("/characters/:id", h.GetCharacter)
		api.PUT("/characters/:id", h.UpdateCharacter)
		api.DELETE("/characters/:id", h.DeleteCharacter)

		// 对话
		api.GET("/chats", h.ListChats)
		api.POST("/chats", h.CreateChat)
		api.GET("/chats/:id", h.GetChat)
		api.DELETE("/chats/:id", h.DeleteChat)
		api.GET("/chats/:id/messages", h.GetMessages)
		api.POST("/chats/:id/messages", h.SendMessage)                   // SSE 流式
		api.POST("/chats/:id/regenerate", h.RegenerateMessage)           // 重新生成
		api.DELETE("/chats/:id/messages/:msgId", h.DeleteMessageCascade) // 级联删除

		// 消息
		api.DELETE("/messages/:id", h.DeleteMessage)

		// 预设
		api.GET("/presets", h.ListPresets)
		api.POST("/presets", h.CreatePreset)
		api.GET("/presets/:id", h.GetPreset)
		api.PUT("/presets/:id", h.UpdatePreset)
		api.DELETE("/presets/:id", h.DeletePreset)

		// 世界书
		api.GET("/worldbooks", h.ListWorldBooks)
		api.POST("/worldbooks", h.CreateWorldBook)
		api.GET("/worldbooks/:id", h.GetWorldBook)
		api.PUT("/worldbooks/:id", h.UpdateWorldBook)
		api.DELETE("/worldbooks/:id", h.DeleteWorldBook)
		api.POST("/worldbooks/:id/entries", h.CreateWorldBookEntry)
		api.PUT("/worldbooks/entries/:entryId", h.UpdateWorldBookEntry)
		api.DELETE("/worldbooks/entries/:entryId", h.DeleteWorldBookEntry)

		// 设置（仅管理员）
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", AdminOnly(), h.UpdateSettings)

		// 模型列表（仅管理员）
		api.GET("/models", AdminOnly(), h.FetchModels)

		// NapCat whitelist (admin only)
		api.GET("/integrations/napcat/whitelist", AdminOnly(), h.ListNapCatWhitelist)
		api.POST("/integrations/napcat/whitelist", AdminOnly(), h.CreateNapCatWhitelist)
		api.PUT("/integrations/napcat/whitelist/:id", AdminOnly(), h.UpdateNapCatWhitelist)
		api.DELETE("/integrations/napcat/whitelist/:id", AdminOnly(), h.DeleteNapCatWhitelist)
		api.GET("/integrations/napcat/owner", AdminOnly(), h.GetNapCatOwner)
		api.PUT("/integrations/napcat/owner", AdminOnly(), h.UpdateNapCatOwner)
		api.GET("/integrations/napcat/config", AdminOnly(), h.GetNapCatConfig)
		api.PUT("/integrations/napcat/config", AdminOnly(), h.UpdateNapCatConfig)
		api.GET("/integrations/napcat/friends", AdminOnly(), h.ListNapCatFriends)
		api.PUT("/integrations/napcat/whitelist/bulk", AdminOnly(), h.ReplaceNapCatWhitelist)
	}

	return r
}
