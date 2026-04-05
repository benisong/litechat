# LiteChat 构建脚本

.PHONY: all dev build-web build-go run install-deps clean

# 默认目标：完整构建
all: install-deps build-web build-go

# 安装依赖
install-deps:
	@echo ">>> 安装 Go 依赖..."
	cd . && go mod tidy
	@echo ">>> 安装前端依赖..."
	cd web && npm install

# 构建前端
build-web:
	@echo ">>> 构建前端..."
	cd web && npm run build

# 构建后端（嵌入前端）
build-go:
	@echo ">>> 构建 Go 后端..."
	go build -o litechat .

# 开发模式（前后端分别运行）
dev-backend:
	@echo ">>> 启动后端 (port 8080)..."
	go run .

dev-frontend:
	@echo ">>> 启动前端开发服务器 (port 5173)..."
	cd web && npm run dev

# 生产运行
run: all
	./litechat

clean:
	rm -rf web/dist web/node_modules litechat data/litechat.db
