# LiteChat

轻量级移动端优先的 AI 角色聊天 Web 应用，SillyTavern 的现代替代品。

## 技术栈

- **后端**：Go + Gin + SQLite
- **前端**：React + Vite + Tailwind CSS
- **部署**：单二进制（前端嵌入 Go）

## 快速开始

### 开发模式

```bash
# 安装依赖
make install-deps

# 终端1：启动后端
make dev-backend

# 终端2：启动前端
make dev-frontend

# 访问 http://localhost:5173
```

### 生产构建

```bash
# 构建前端 + 后端（单二进制）
make all

# 运行
./litechat
# 访问 http://localhost:8080
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `8080` | 监听端口 |
| `DATA_DIR` | `./data` | 数据库文件目录 |

## 功能

- **角色卡管理**：创建、编辑、删除 AI 角色，支持头像、性格、场景设定
- **流式聊天**：SSE 实时流式输出，打字机效果
- **预设管理**：多套系统提示词模板，可调温度/MaxTokens/Top-P
- **世界书**：关键词触发的知识注入
- **OpenAI 兼容 API**：支持 OpenAI、DeepSeek、Groq、本地模型等
- **PWA 支持**：可安装到手机主屏幕
- **深色/浅色主题**

## API 文档

所有 API 以 `/api` 为前缀：

- `GET/POST /api/characters` — 角色卡列表/创建
- `GET/PUT/DELETE /api/characters/:id` — 角色卡详情/更新/删除
- `GET/POST /api/chats` — 对话列表/创建
- `POST /api/chats/:id/messages` — 发送消息（SSE 流式）
- `GET/POST /api/presets` — 预设管理
- `GET/POST /api/worldbooks` — 世界书管理
- `GET/PUT /api/settings` — 全局设置
