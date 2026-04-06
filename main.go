package main

import (
	"embed"
	"io/fs"
	"litechat/internal/api"
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

	// 初始化各层
	characterStore := store.NewCharacterStore(db)
	chatStore := store.NewChatStore(db)
	messageStore := store.NewMessageStore(db)
	presetStore := store.NewPresetStore(db)
	worldBookStore := store.NewWorldBookStore(db)
	configStore := store.NewConfigStore(db)
	userStore := store.NewUserStore(db)

	// 确保初始用户存在
	if err := userStore.EnsureInitialUsers(); err != nil {
		log.Fatalf("创建初始用户失败: %v", err)
	}

	chatService := service.NewChatService(chatStore, messageStore, characterStore, presetStore, worldBookStore, configStore, userStore)

	handlers := api.NewHandlers(
		characterStore, chatStore, messageStore,
		presetStore, worldBookStore, configStore,
		userStore,
		chatService,
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
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}
				// SPA 路由：返回 index.html
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
