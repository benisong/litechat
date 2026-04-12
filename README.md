# LiteChat

LiteChat 是一个面向角色扮演聊天场景的 Web 应用，围绕“角色卡 + 流式聊天 + 预设 + 世界书”构建，支持多用户登录、管理员配置模型、服务模式部署，以及基于模板由 AI 生成角色卡草稿。

当前版本已经不再只是一个轻量聊天壳，而是一套更完整的角色聊天系统：
- 支持 `admin / user` 双角色与 JWT 登录
- 支持 `self` / `service` 两种运行模式
- 支持角色卡创建、编辑、删除、模板生成
- 支持 SSE 流式聊天、重新生成、消息级删除
- 支持预设、世界书、角色卡生成独立 Prompt
- 支持 service 模式下的自动记忆摘要与上下文裁剪
- 支持 PWA 安装与移动端使用
- 前端会主动检查新版本并自动接管更新，减少手动强刷
- 登录态为浏览器会话级存储，关闭窗口后不会长期保留

## 核心能力

### 1. 多用户与管理员能力
- 管理员可以创建、编辑、删除普通用户
- 管理员可以配置 API Endpoint、API Key、默认模型
- 管理员可以切换系统运行模式：`self` / `service`
- 普通用户拥有自己的资料：`user_name / user_detail`
- 普通用户首次登录且名字仍是默认值 `user` 时，会弹窗提示修改资料

### 2. 角色卡系统
- 支持手动创建和编辑角色卡
- 支持标签、描述、性格、场景、开场白等完整字段
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
- “重新生成”按钮只出现在最新用户消息和最新 AI 消息下方
- 当模型没有返回内容时，用户可以直接重试，不需要重新编辑上一条输入
- 新会话首屏会显示角色场景设定与开场白
- 首轮对话期间会保持角色默认开场白可见，避免第一条发送后界面闪空
- 针对浏览器切后台再切回的场景做了底部操作区可见性修复

### 5. 预设、世界书与记忆存储
- 支持聊天预设管理
- 支持世界书与关键词触发注入
- 聊天生成可结合预设和世界书
- 角色卡生成链路与聊天预设完全独立，避免生成草稿时被聊天 Prompt 污染
- 管理员预设页新增“记忆存储”选项卡
- “记忆存储”用于配置 service 模式自动摘要的补充提示词
- 摘要协议、标签结构和持久化规则由系统固定，管理员只编辑不会破坏解析的补充提示词

### 6. Service 模式自动记忆摘要
这套能力仅在 `service` 模式生效；`self` 模式仍保持你自己维护预设与世界书的工作流。

自动摘要的工作方式：
- 后台统计“未摘要有效文本”字数
- 达到 `3000` 字后，异步生成 1 个小摘要
- 小摘要成功后，会推进上下文边界；该边界之前的原始消息不再继续进入聊天上下文
- 当小摘要累计达到 `5` 个时，后台生成 1 个大摘要
- 聊天时上下文会优先使用：大摘要 + 未合并的小摘要 + 最新原始消息尾部
- 摘要失败不会影响正常聊天，只会延迟记忆更新
- 删除消息、级联删除、重新生成等会触发摘要失效与重建

摘要结构固定为：
- `plot`
- `relationship`
- `user_facts`
- `world_state`
- `open_loops`

## 运行模式

### `self`
适合自用：
- 不启用自动记忆摘要
- 聊天上下文主要依赖你自己编写的预设与世界书
- 适合强调完全可控的个人使用场景

### `service`
适合给普通用户提供服务：
- 使用管理员预设作为服务默认预设
- 启用自动记忆摘要系统
- 启用摘要边界裁剪，降低长对话的上下文膨胀

## 技术栈
- 后端：Go、Gin、SQLite
- 前端：React 18、Vite、Zustand、Tailwind CSS
- 鉴权：JWT
- 流式响应：SSE
- 部署形态：
  - 前端构建后嵌入 Go 二进制
  - 支持 `Docker Compose`
  - 支持 `deploy.sh` 一键部署

## 项目结构

```text
litechat/
├─ internal/
│  ├─ api/                   # 路由、handler、鉴权中间件
│  ├─ service/               # 聊天、角色卡生成、摘要逻辑
│  ├─ store/                 # SQLite 数据访问层
│  └─ model/                 # 数据模型
├─ web/
│  ├─ src/pages/             # 页面
│  ├─ src/components/        # 组件
│  └─ src/store/             # Zustand 状态管理
├─ data/                     # 本地数据目录
├─ deploy.sh                 # 一键部署脚本
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

## 部署

### 1. 一键部署
项目根目录提供了 `deploy.sh`：

```bash
./deploy.sh
```

脚本会：
- 检查 Docker 是否已安装
- 未安装时自动安装 Docker
- 自动选择 `docker compose` 或 `docker-compose`
- 拉取最新代码并重建服务

### 2. 手动 Docker 部署
```bash
docker compose down
docker compose build
docker compose up -d
```

如果服务器拉取代码后前端页面没有变化，常见原因是构建缓存或浏览器 PWA 缓存。当前版本会主动检查并接管新 Service Worker，但首次升级时如果浏览器还残留旧缓存，仍可使用：

```bash
docker compose build --no-cache
docker compose up -d --force-recreate
```

然后清理浏览器站点缓存或 Service Worker 后再验证页面。

## 默认账号
首次启动且数据库为空时，系统会自动创建默认用户：
- 管理员：`admin / admin`
- 普通用户：`user1 / user`

说明：
- 管理员用户名后续可以在系统内修改
- 系统会为普通用户自动创建默认角色卡
- 普通用户默认资料名为 `user`

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
- `PUT /api/auth/me/profile`
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
- `GET /api/models`

## 适用场景
LiteChat 适合这些需求：
- 想快速搭一套多用户 AI 角色聊天系统
- 想要比纯聊天壳更完整的“角色卡 + 设定 + 世界书”体验
- 想用 OpenAI 兼容接口接入不同模型提供商
- 想在移动端获得接近 App 的聊天体验
- 想在 service 模式下改善长对话记忆问题，同时保留 self 模式的手工控制感

## 当前版本重点
和项目初期相比，现在这版 LiteChat 更偏向“可部署、可运营、可扩展”的角色聊天应用：
- 增加了多用户与管理端能力
- 增加了 `self / service` 模式切换
- 增加了角色卡模板 AI 生成
- 增加了角色卡生成专用模型配置
- 增加了普通用户独立资料与首次登录提醒
- 增加了 service 模式自动记忆摘要系统
- 增加了管理员可编辑的“记忆存储”补充提示词
- 增加了更完整的预设、世界书和角色工作流
