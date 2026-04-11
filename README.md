# LiteChat

LiteChat 是一个移动端优先的 AI 角色聊天 Web 应用，围绕“角色卡 + 流式聊天 + 预设 + 世界书”构建，同时支持多用户登录、管理员配置模型与服务模式，以及通过模板让 AI 自动生成更像角色卡站风格的角色卡草稿。

现在的项目已经不再只是最初的轻量聊天壳，而是一套更完整的角色扮演聊天系统：
- 支持 `admin / user` 双角色与 JWT 登录
- 支持自用模式与服务模式
- 支持角色卡创建、编辑、删除、模板生成
- 支持流式聊天、重生成、消息级删除
- 支持预设提示词与世界书注入
- 支持为“角色卡生成”单独指定模型，不与聊天预设混用
- 支持 PWA 安装与移动端使用

## 功能概览

### 1. 多用户与管理端
- 管理员可以创建、编辑、删除普通用户
- 管理员可以配置 API 端点、API Key、默认模型
- 管理员可以切换系统运行模式：`self` / `service`
- 管理员可以配置“角色卡生成是否跟随当前模型”
- 管理员可以为角色卡生成单独指定模型

### 2. 角色卡系统
- 支持手动创建和编辑角色卡
- 支持角色标签、描述、性格、场景、开场白等完整字段
- 支持模板化创建角色卡
- 模板创建不再在前端本地拼装角色卡，而是由后端调用 AI 生成角色卡草稿
- AI 返回内容会被解析后回填到角色卡编辑页，用户确认后再保存，减少脏数据

### 3. 模板生成角色卡
模板创建流程现在是：

`模板选择 -> 显示“生成角色卡中，请等候” -> 调用角色卡专用 Prompt -> AI 返回标签包裹结果 -> 后端提取字段 -> 回填到角色卡编辑页`

当前模板维度包括：
- 角色性别
- 故事场景：现代都市、校园青春、职场办公室、娱乐圈、西幻异世界、仙侠江湖、末日废土
- 故事基调：白月光、求而不得、治愈陪伴、欢喜冤家、禁忌拉扯、危险关系
- 角色性格：傲娇、温柔、腹黑、天然呆、高冷、强势、会撩
- 性格补充输入框：支持额外自定义人设要求
- 叙事视角：第二人称 / 第三人称

角色卡生成有两个重要特性：
- 使用独立的“角色卡生成提示词”，不走聊天预设提示词
- 支持跟随默认模型，或使用管理员单独指定的角色卡生成模型

### 4. 聊天体验
- SSE 流式输出，适合角色扮演场景
- 支持创建角色专属会话
- 支持重新生成最后一次请求
- 重生成按钮只出现在最新用户消息和最新 AI 消息下方
- 当模型没有返回内容时，用户可以直接重试，不需要重新编辑上一条输入
- 首轮对话期间会保持角色默认开场白可见，避免第一次发送后界面闪空
- 针对浏览器切后台再切回来的场景做了底部操作区域可见性修复

### 5. 预设与世界书
- 支持预设提示词管理
- 支持世界书与关键词触发注入
- 聊天生成可结合预设和世界书
- 角色卡生成链路与预设完全独立，避免生成草稿时被聊天提示词污染

### 6. 前端体验
- React + Vite 构建
- Tailwind CSS 样式体系
- Zustand 状态管理
- PWA 支持，可安装到移动设备主屏幕
- 移动端优先布局，同时兼容桌面浏览器

## 技术栈

- 后端：Go、Gin、SQLite
- 前端：React 18、Vite、Zustand、Tailwind CSS
- 鉴权：JWT
- 流式响应：SSE
- 部署形态：
  - 前端构建后嵌入 Go 二进制
  - 也支持 `Docker Compose` 与 `deploy.sh` 部署

## 项目结构

```text
litechat/
├─ internal/                 # Go 后端
│  ├─ api/                   # 路由、handler、鉴权中间件
│  ├─ service/               # 聊天与角色卡生成逻辑
│  ├─ store/                 # SQLite 数据访问层
│  └─ model/                 # 数据模型
├─ web/                      # React 前端
│  ├─ src/pages/             # 页面
│  ├─ src/components/        # 组件
│  └─ src/store/             # Zustand 状态管理
├─ data/                     # 本地数据目录
├─ deploy.sh                 # 服务器部署脚本
├─ docker-compose.yml        # Docker Compose 配置
└─ README.md
```

## 快速开始

### 开发模式

```bash
# 安装依赖
make install-deps

# 终端 1：启动后端
make dev-backend

# 终端 2：启动前端
make dev-frontend

# 访问 http://localhost:5173
```

### 生产构建

```bash
# 构建前端并嵌入 Go 二进制
make all

# 启动
./litechat

# 访问 http://localhost:8080
```

### Docker / 服务器部署

项目根目录已提供：
- `docker-compose.yml`
- `deploy.sh`

常见部署方式：

```bash
# 直接执行部署脚本
./deploy.sh
```

如果需要手动部署：

```bash
docker compose down
docker compose build
docker compose up -d
```

如果服务器拉取代码后前端页面没有变化，通常是构建缓存或浏览器 PWA 缓存导致，可以使用：

```bash
docker compose build --no-cache
docker compose up -d --force-recreate
```

然后清理浏览器站点缓存或 Service Worker 后再验证页面。

## 默认账号

首次启动、数据库为空时，系统会自动创建默认用户：

- 管理员：`admin / admin`
- 普通用户：`user1 / user`

说明：
- 管理员用户名后续可以在系统内修改
- 系统会为普通用户自动创建默认角色卡

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | 服务监听端口 |
| `DATA_DIR` | `./data` | SQLite 数据目录 |
| `JWT_SECRET` | 内置默认值 | JWT 签名密钥，生产环境务必覆盖 |
| `GIN_MODE` | 空 | 生产环境建议设置为 `release` |

## API 概览

所有接口均以 `/api` 为前缀。

### 认证与用户
- `POST /api/auth/login`
- `GET /api/auth/me`
- `PUT /api/auth/password`
- `GET /api/auth/users`
- `POST /api/auth/users`
- `PUT /api/auth/users/:id`
- `DELETE /api/auth/users/:id`

### 角色卡
- `GET /api/characters`
- `POST /api/characters`
- `POST /api/characters/generate`
- `GET /api/characters/:id`
- `PUT /api/characters/:id`
- `DELETE /api/characters/:id`

### 聊天
- `GET /api/chats`
- `POST /api/chats`
- `GET /api/chats/:id`
- `DELETE /api/chats/:id`
- `GET /api/chats/:id/messages`
- `POST /api/chats/:id/messages`
- `POST /api/chats/:id/regenerate`
- `DELETE /api/chats/:id/messages/:msgId`
- `DELETE /api/messages/:id`

### 预设与世界书
- `GET /api/presets`
- `POST /api/presets`
- `GET /api/presets/:id`
- `PUT /api/presets/:id`
- `DELETE /api/presets/:id`
- `GET /api/worldbooks`
- `POST /api/worldbooks`
- `GET /api/worldbooks/:id`
- `PUT /api/worldbooks/:id`
- `DELETE /api/worldbooks/:id`
- `POST /api/worldbooks/:id/entries`
- `PUT /api/worldbooks/entries/:entryId`
- `DELETE /api/worldbooks/entries/:entryId`

### 设置与模型
- `GET /api/settings`
- `PUT /api/settings`
- `PUT /api/settings/user-info`
- `GET /api/models`

## 适用场景

LiteChat 适合这几类需求：
- 希望快速搭一套多用户 AI 角色聊天系统
- 想要比纯聊天壳更完整的“角色卡 + 设定 + 世界书”体验
- 想用 OpenAI 兼容接口接入不同模型供应商
- 想在移动端获得更接近 App 的聊天体验
- 想让管理员单独控制模型与服务配置，而普通用户专注聊天

## 当前版本重点

和项目初期相比，现在这版 LiteChat 更偏向“可部署、可运营、可扩展”的角色聊天应用，而不只是简单的本地演示：
- 增加了多用户与管理端能力
- 增加了服务模式与自用模式切换
- 增加了角色卡模板 AI 生成
- 增加了角色卡生成专用模型配置
- 增加了消息重生成与聊天体验修复
- 增加了更完整的预设、世界书和角色工作流

## 后续可扩展方向

- 角色卡生成 Prompt 后台可配置化
- 模板选项后台化，不再写死在前端
- 更细的会话管理与消息回溯能力
- 更丰富的移动端交互与安装体验
