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

	// API 路由组
	api := r.Group("/api")
	{
		// 角色卡
		api.GET("/characters", h.ListCharacters)
		api.POST("/characters", h.CreateCharacter)
		api.GET("/characters/:id", h.GetCharacter)
		api.PUT("/characters/:id", h.UpdateCharacter)
		api.DELETE("/characters/:id", h.DeleteCharacter)

		// 对话
		api.GET("/chats", h.ListChats)
		api.POST("/chats", h.CreateChat)
		api.GET("/chats/:id", h.GetChat)
		api.DELETE("/chats/:id", h.DeleteChat)
		api.GET("/chats/:id/messages", h.GetMessages)
		api.POST("/chats/:id/messages", h.SendMessage) // SSE 流式

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

		// 设置
		api.GET("/settings", h.GetSettings)
		api.PUT("/settings", h.UpdateSettings)

		// 模型列表（从远端 API 获取）
		api.GET("/models", h.FetchModels)
	}

	return r
}
