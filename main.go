package main

import (
	"embed"
	"io/fs"
	"litechat/internal/api"
	"litechat/internal/model"
	"litechat/internal/service"
	"litechat/internal/store"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed web/dist
var webDist embed.FS

func main() {
	// 数据目录（默认 ./data）
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 初始化数据库
	db, err := store.NewDB(dataDir)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()

	if err := db.InitSchema(); err != nil {
		log.Fatalf("数据库 Schema 初始化失败: %v", err)
	}
	summaryDB, err := store.NewSummaryDB(dataDir)
	if err != nil {
		log.Fatalf("摘要数据库初始化失败: %v", err)
	}
	defer summaryDB.Close()
	if err := summaryDB.InitSummarySchema(); err != nil {
		log.Fatalf("摘要数据库 Schema 初始化失败: %v", err)
	}
	if err := store.MigrateLegacySummaries(db, summaryDB); err != nil {
		log.Fatalf("迁移旧摘要数据失败: %v", err)
	}

	// 初始化各层
	characterStore := store.NewCharacterStore(db)
	chatStore := store.NewChatStore(db)
	messageStore := store.NewMessageStore(db)
	presetStore := store.NewPresetStore(db)
	worldBookStore := store.NewWorldBookStore(db)
	configStore := store.NewConfigStore(db)
	userStore := store.NewUserStore(db)
	summaryStore := store.NewSummaryStore(summaryDB, db)
	storyStore := store.NewSchedulerStore(db)
	settings, settingsErr := configStore.GetSettings()
	if settingsErr != nil {
		log.Printf("读取剧情编译模型配置失败: %v", settingsErr)
		settings = &model.AppSettings{}
	}
	storyCompiler := service.NewManifestCompiler(storyStore, service.NewOpenAICompletionClient(settings))
	storySourceProvider := service.NewWorldBookStorySourceProvider(worldBookStore)
	storyInitializer := service.NewStoryChatInitializer(chatStore, storyStore, characterStore, storyCompiler, storySourceProvider)

	// 确保初始用户存在
	if err := userStore.EnsureInitialUsers(); err != nil {
		log.Fatalf("创建初始用户失败: %v", err)
	}

	summaryService := service.NewSummaryService(messageStore, summaryStore, characterStore, configStore, userStore)
	chatService := service.NewChatService(chatStore, messageStore, characterStore, presetStore, worldBookStore, configStore, userStore, summaryService)

	handlers := api.NewHandlers(
		characterStore, chatStore, messageStore,
		presetStore, worldBookStore, configStore,
		userStore,
		chatService,
		summaryService,
		storyInitializer,
	)

	r := api.SetupRouter(handlers)

	// 嵌入前端静态文件
	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Println("前端文件未嵌入，跳过静态文件服务")
	} else {
		fileServer := http.FileServer(http.FS(distFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			// 尝试提供静态文件
			if !strings.HasPrefix(path, "/api") {
				// 检查文件是否存在
				cleanPath := strings.TrimPrefix(path, "/")
				if cleanPath == "" {
					cleanPath = "index.html"
				}
				if _, err := distFS.Open(cleanPath); err == nil {
					setWebCacheHeaders(c, cleanPath)
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}
				// SPA 路由：返回 index.html
				setWebCacheHeaders(c, "index.html")
				c.Request.URL.Path = "/"
				fileServer.ServeHTTP(c.Writer, c.Request)
			}
		})
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("LiteChat 启动于 http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func setWebCacheHeaders(c *gin.Context, path string) {
	if path == "index.html" || path == "sw.js" || path == "manifest.webmanifest" {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		if path == "sw.js" {
			c.Header("Service-Worker-Allowed", "/")
		}
		return
	}
	if strings.HasPrefix(path, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}
}
