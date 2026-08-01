# Story Runtime V1 Completion Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** 将当前已完成的剧情运行时基础设施接成真正可用的复杂聊天闭环，同时保持旧普通聊天链路冻结、不受新功能污染。

**Architecture:** 继续采用“旧流程冻结、新流程平行实现”。`POST /api/chats/:id/messages` 和旧 `ChatService` 不改业务行为；新增复杂剧情 API 使用独立的 `StoryChatRuntime`、生产级 PromptBuilder、主模型流式客户端和调度模型 PromptBuilder。所有状态变更仍经过规则引擎、版本检查和数据库事务。

**Tech Stack:** Go 1.21、Gin、SQLite、现有 Store、OpenAI Chat Completions 兼容接口、SSE、现有 React/Vite 前端。

---

## 当前基线与已知事实

- 工作目录：`/home/ubuntu/workspace/litechat`
- 最新提交：`9458477 feat: add story runtime status and retry flow`
- 当前工作区干净。
- 已完成：剧情表、调度解析/校验、规则/效果引擎、原子事务、Manifest 编译/缓存、compile_only Provider、初始化/状态/retry API、`StoryChatRuntime` 测试骨架和 Runtime 内部状态事件。
- 当前尚未真实接通：复杂消息 HTTP/SSE、生产主模型客户端、生产 StoryPromptBuilder、独立 SchedulerPromptBuilder、前端剧情流程。
- 当前重要逻辑缺口：`ManifestStoryTurnProcessor` 把主模型消息直接传给 `SchedulerService`，尚未构造调度模型专用输入。
- 当前全量测试环境存在远端已有摘要异步测试的 SQLite 锁竞争；每次改动仍需运行编译、Store、剧情相关定向测试，并单独记录全量测试结果，不得将该已知基线问题误归因于剧情代码。

---

## 阶段一：先接通生产复杂聊天后端

### Task 1: 固化复杂消息 API 契约测试

**Objective:** 先用 API 测试固定新复杂消息接口和 SSE 事件格式，防止后续实现偏离协议。

**Files:**
- Create: `internal/api/story_handlers_test.go`
- Modify: `internal/api/router.go`（仅新增路由，不修改旧消息路由）
- Reference: `internal/api/handlers.go`

**Required endpoint:**

```http
POST /api/story/chats/:id/messages
```

**Expected SSE events:**

```text
data: {"token":"..."}

data: {"scheduler_status":"processing","record_id":"..."}

data: {"scheduler_status":"success","record_id":"..."}

data: {"done":true}
```

失败事件必须包含：

```json
{"scheduler_status":"failed","record_id":"...","message":"..."}
```

**TDD:**
1. 使用可注入 fake StoryRuntime 写失败测试。
2. 验证未配置 Runtime 返回 `503`。
3. 验证成功路径输出 SSE headers、token、processing、success、done。
4. 验证 Runtime 错误输出结构化 SSE error。
5. 运行：

```bash
export PATH=$HOME/go-sdk/go/bin:$PATH
go test ./internal/api -run TestStory -count=1
```

---

### Task 2: 为 Handler 注入独立 Story Runtime

**Objective:** API Handler 能调用具体的 `StoryChatRuntime`，不在旧 `SendMessage` 中添加 `scheduler_enabled` 分支。

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/router.go`
- Modify: `main.go`
- Test: `internal/api/story_handlers_test.go`

**Implementation:**

- 在 Handler 中增加独立 `storyRuntime *service.StoryChatRuntime` 依赖。
- 保留 `NewHandlers` 旧调用兼容性，使用可选参数或新增独立构造器。
- 新增 `StorySendMessage` Handler。
- 旧 `SendMessage` 完全保持原实现。
- 新 Handler 使用 `SendMessageWithEvents`。
- SSE callback 只负责把 token/status 转换为 JSON，不负责业务逻辑。

**Acceptance:**

```text
scheduler_enabled=false 的聊天不能进入 StoryRuntime
scheduler_enabled=true 的聊天不调用旧 ChatService.SendMessage
```

**Verification:**

```bash
go test ./internal/api ./internal/service -run 'TestStory' -count=1
```

---

### Task 3: 实现独立主模型流式客户端

**Objective:** 提供生产版 `StoryPrimaryClient`，只实现主模型流式调用，不复用旧 ChatService 的编排逻辑。

**Files:**
- Create: `internal/service/openai_story_primary_client.go`
- Create: `internal/service/openai_story_primary_client_test.go`
- Reference: `internal/service/openai_completion_client.go`
- Reference: `internal/service/chat_service.go` 中现有 SSE 解析代码，但不要修改旧方法。

**Implementation requirements:**

- 依赖 `model.AppSettings`。
- 接受 `modelName`、消息列表和 `StreamCallback`。
- 请求 OpenAI Chat Completions 兼容接口。
- 支持 `context.Context` 取消。
- 逐行解析 SSE `data:`。
- 支持 `[DONE]`。
- 把 delta content 传给 callback，同时累积完整回复。
- 非 2xx、空响应、JSON 错误、Scanner 错误都返回错误。
- 不记录 API key 或 Authorization 到日志。

**TDD cases:**

- 多个 SSE chunk 拼成完整回复。
- callback 收到每个 token。
- 服务器返回 500。
- context 取消。
- `[DONE]` 正常结束。
- 超长 SSE 行不被默认 Scanner 限制截断。

**Verification:**

```bash
go test ./internal/service -run TestOpenAIStoryPrimaryClient -count=1
```

---

### Task 4: 实现生产 StoryPromptBuilder

**Objective:** 让复杂主模型获得正确的角色卡、static 世界书、动态剧情上下文和历史，同时绝不注入 compile_only 原文或完整 Manifest。

**Files:**
- Create: `internal/service/story_prompt_builder.go`
- Create: `internal/service/story_prompt_builder_test.go`
- Modify if necessary: `internal/service/story_context_builder.go`
- Reference: `internal/store/character_store.go`, `internal/store/preset_store.go`, `internal/store/message_store.go`

**Dependencies:**

```go
CharacterStore
WorldBookStore
PresetStore（若确实需要预设）
StoryContextBuilder
```

**Implementation:**

1. 根据 `chat.CharacterID` 和 `chat.UserID` 读取角色卡。
2. 读取普通聊天历史。
3. 读取当前角色可见的 static 世界书及启用条目。
4. 不读取 compile_only 条目进入主模型上下文。
5. 从 `ChatStoryState` 生成最小 dynamic context。
6. 使用 `StoryContextBuilder` 合并 system 消息。
7. 保证最终只有一个首条 system 消息。
8. 不放入完整 Manifest、未触发事件、未来结局和 raw 调度输出。

**Tests:**

- static 进入，compile_only 不进入。
- state context 进入。
- 角色卡信息进入。
- 多 system 合并成首条 system。
- 历史和最新用户消息角色顺序正确。
- Manifest 原文不出现在消息中。

**Verification:**

```bash
go test ./internal/service -run TestStoryPromptBuilder -count=1
```

---

### Task 5: 实现 SchedulerPromptBuilder 和正确调度输入

**Objective:** 调度模型只看到本轮用户消息、主模型回复、上一成功状态和相关 Manifest 定义，而不是直接复用主模型消息。

**Files:**
- Create: `internal/service/scheduler_prompt_builder.go`
- Create: `internal/service/scheduler_prompt_builder_test.go`
- Modify: `internal/service/manifest_story_processor.go`
- Modify: `internal/service/scheduler_service.go`（如需传入独立 prompt）

**Required input:**

```text
本轮 user message
本轮 assistant message
最近成功状态
active_event/current_scene/route
Manifest 中相关字段
Manifest 中相关 observation/event rules
固定 scheduler system prompt
```

**Required exclusions:**

```text
完整 compile_only 世界书
完整 Manifest 未相关部分
API key/token
未授权未来剧情
```

**Implementation:**

- 扩展 `StoryTurnProcessor` 输入，或让 processor 从 `record`/message Store 读取本轮 user/assistant 内容。
- 不再把主模型的 system prompt 原样传给调度模型。
- 调度 prompt 必须固定声明“输出候选观察，不输出 SQL/最终数值”。
- 将 `SchedulerValidationSpec` 与 prompt 中允许的 key 对齐。

**Tests:**

- 调度输入包含 user/assistant。
- 不包含 compile_only 原文。
- 不包含完整 Manifest。
- 只包含相关字段/事件。
- 调度模型输出继续经过现有 parser/validator。

**Verification:**

```bash
go test ./internal/service -run 'TestSchedulerPromptBuilder|TestManifestStoryTurnProcessor' -count=1
```

---

### Task 6: 在 main.go 组装完整 StoryRuntime

**Objective:** 让生产程序真正拥有可调用的复杂 Runtime。

**Files:**
- Modify: `main.go`
- Create if needed: `internal/service/story_runtime_factory.go`
- Test: `internal/service/story_runtime_wiring_test.go` 或 API 集成测试

**Assembly:**

```text
WorldBookStorySourceProvider
StoryPromptBuilder
OpenAIStoryPrimaryClient
OpenAICompletionClient
SchedulerService
ManifestStoryTurnProcessor
StoryChatRuntime
```

**Configuration:**

- 主模型从当前应用配置读取。
- 调度模型从独立配置读取。
- 配置缺失时复杂 Runtime 不可用，返回明确 `503`，不能影响普通聊天启动。
- 不在 startup 阶段调用模型。
- 不输出凭据。

**Verification:**

```bash
go test ./... -run '^$'
go test ./internal/api ./internal/service -run 'TestStory' -count=1
```

---

## 阶段二：保护复杂剧情数据一致性

### Task 7: 禁止复杂剧情普通删除和重生成

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/service/chat_service.go`（只增加明确的复杂聊天拒绝边界；不修改普通流程）
- Create: `internal/api/story_restrictions_test.go`

**Rules:**

- 复杂剧情消息删除返回 `409` 或 `422`。
- 复杂剧情级联删除返回 `409` 或 `422`。
- 复杂剧情聊天删除返回 `409` 或 `422`，除非专门实现完整状态清理。
- 复杂剧情 regenerate 返回明确“V1 不支持”。
- 普通聊天原行为回归测试必须通过。

**Verification:**

```bash
go test ./internal/api -run 'TestStory.*Delete|TestStory.*Regenerate' -count=1
```

---

### Task 8: 增加 StoryRuntime per-chat 并发锁

**Files:**
- Modify: `internal/service/story_chat_runtime.go`
- Create: `internal/service/story_chat_runtime_concurrency_test.go`

**Rules:**

- 同一 chat 同时只能有一轮。
- 第二个请求返回 `ErrChatBusy` 或明确冲突错误。
- 锁必须在模型错误、调度错误、callback 错误时释放。
- 不与旧 ChatService 的锁共享，避免跨流程耦合。

**Verification:**

```bash
go test ./internal/service -run TestStoryChatRuntimeConcurrent -count=1
```

---

### Task 9: 完善失败计数、暂停和调度重试

**Files:**
- Modify: `internal/store/scheduler_store.go`
- Modify: `internal/service/manifest_story_processor.go`
- Modify: `internal/service/story_chat_runtime.go`
- Create: `internal/service/story_scheduler_retry_test.go`
- Create: `internal/api/story_scheduler_retry_test.go`

**Implementation:**

- 调度失败递增 `chat_story_states.failure_count`。
- 成功提交后清零。
- 连续失败达到阈值时设为 paused。
- paused 状态禁止新调度，但保留主模型是否继续生成的明确策略；V1 建议直接拒绝并提示用户。
- 实现针对某个 `scheduler_record` 的 retry，而不是只支持 Manifest 编译 retry。
- retry 前重新读取最新 State，使用新 `state_version`。
- 版本冲突标记为 `conflict`，不能覆盖新状态。

**Verification:**

```bash
go test ./internal/store ./internal/service -run 'Test.*Failure|Test.*Retry|Test.*Conflict' -count=1
```

---

## 阶段三：补齐 Manifest V1 Schema

### Task 10: 扩展 Manifest 结构和缓存键

**Files:**
- Modify: `internal/model/models.go`
- Modify: `internal/service/manifest_story_processor.go`
- Modify: `internal/service/manifest_compiler.go`
- Modify: `internal/store/scheduler_store.go`
- Modify: `internal/store/db.go`
- Create: `internal/service/manifest_schema_test.go`

**Add:**

```text
source_version
initial_state
events
visibility_rules
warnings
compiler_prompt_version
compiler_model
manifest_schema_version
```

**Cache key must include:**

```text
character_id
character_version
worldbook_hash
prompt_version
compiler_model
manifest_schema_version
```

**Migration:**

- 为旧数据库添加必要列，保留旧数据可读。
- 不删除旧 Manifest。
- 新旧 Schema 不兼容时标记 stale/failed，而不是静默使用。

**Verification:**

```bash
go test ./internal/store ./internal/service -run 'TestManifest|Test.*Cache' -count=1
```

---

### Task 11: 实现 Manifest stale 和主动重新编译

**Files:**
- Modify: `internal/store/scheduler_store.go`
- Modify: `internal/service/manifest_compiler.go`
- Modify: `internal/service/story_chat_initializer.go`
- Modify: `internal/api/handlers.go`
- Create: `internal/service/manifest_stale_test.go`

**Rules:**

- source key 变化时旧 ready Manifest 标记 stale。
- 编译 prompt/model/schema 变化时旧 ready Manifest 不可命中。
- 作者主动重新编译时保留旧 Manifest 审计记录。
- 新 Manifest 未 ready 前，不能创建复杂聊天。

---

### Task 12: 完善 Manifest 初始化校验

**Files:**
- Modify: `internal/service/manifest_compiler.go`
- Create: `internal/service/manifest_validator.go`
- Create: `internal/service/manifest_validator_test.go`

**Validation:**

- 顶层未知字段拒绝。
- 字段类型和初始值类型匹配。
- 初始值满足 min/max/allowed。
- 事件引用字段存在。
- 条件操作符合法。
- 事件依赖无循环。
- 字段/事件/JSON 大小有限制。
- 敏感字段权限组合合法。
- `counter` 等设计允许类型明确处理。
- 任何非法 Manifest 都是 failed，不创建聊天。

---

### Task 13: 实现 once、cooldown、依赖和高风险状态规则

**Files:**
- Modify: `internal/service/story_rules.go`
- Modify: `internal/service/story_effects.go`
- Modify: `internal/service/manifest_story_processor.go`
- Modify: `internal/store/scheduler_commit.go`
- Create: `internal/service/story_advanced_rules_test.go`

**Rules:**

- `chat_id + event_key` 幂等。
- once 事件重复报告记录 duplicate，不重复效果。
- cooldown 在窗口内拒绝重复触发。
- 事件依赖和优先级由后端判断。
- 死亡、夺舍、身份揭示、路线切换、核心资源转移等高风险变化整轮拒绝，除非 Manifest 明确且所有条件满足。
- 普通非法字段可按策略部分接受；高风险非法变化整轮拒绝。

---

## 阶段四：前端联调

### Task 14: 增加复杂剧情 API 客户端

**Files:**
- Modify: `web/src/store/index.js`
- Create or modify: `web/src/api/storyRuntime.js`（按现有前端结构决定）

**Functions:**

```text
createStoryChat
getStoryChatStatus
retryStoryManifest
sendStoryMessageSSE
retryStoryScheduler
```

**Rules:**

- 根据 `chat.scheduler_enabled` 选择旧 endpoint 或 story endpoint。
- 不修改普通聊天的请求协议。
- SSE parser 支持 token、scheduler_status、done、error。

---

### Task 15: 增加复杂剧情初始化和状态面板

**Files:**
- Modify: `web/src/pages/ChatPage.jsx`
- Modify: `web/src/store/index.js`
- Create if useful: `web/src/components/StoryStatusPanel.jsx`

**UI:**

- Manifest 初始化中。
- 初始化失败 + retry。
- 当前场景/路线/事件。
- scheduler processing。
- 调度失败警告。
- 调度 retry。
- paused 状态。
- 复杂聊天隐藏删除和 regenerate 操作。

**Acceptance:**

- 普通聊天界面和行为不变。
- 复杂聊天收到 assistant 内容后继续等待 scheduler 状态。
- 调度失败不清除 assistant 回复。
- 状态失败时显示上一份有效状态提示。

---

## 阶段五：完整验证与文档更新

### Task 16: 增加端到端 API/SSE 测试

**Files:**
- Create: `internal/api/story_runtime_integration_test.go`
- Create: `internal/service/story_runtime_e2e_test.go`

**Coverage:**

```text
初始化成功
初始化失败
Manifest retry
创建复杂聊天
发送复杂消息
主模型流式输出
调度 processing
调度 success
调度 failed
调度 retry
版本冲突
并发请求
复杂删除拒绝
普通聊天回归
```

使用 fake 主模型和 fake 调度模型，不使用真实凭据。

---

### Task 17: 运行完整质量门禁

**Commands:**

```bash
export PATH=$HOME/go-sdk/go/bin:$PATH
gofmt -w $(git diff --name-only -- '*.go')
git diff --check
go test ./internal/store -count=1
go test ./internal/service -count=1
go test ./internal/api -count=1
go test ./... -run '^$'
git status --short
```

如果 `go test ./...` 仍被远端摘要异步测试的 SQLite 锁竞争阻塞：

1. 单独运行失败测试并记录精确错误。
2. 单独运行剧情包测试证明本次改动通过。
3. 不把基线问题伪报为通过。
4. 决定是否另开独立修复提交处理摘要锁问题。

---

### Task 18: 更新 V1 设计和操作文档

**Files:**
- Modify: `docs/design/story-runtime-v1.md`
- Modify: `docs/character-design/liyu-complex-card-draft.md`（仅补充已验证接口，不提前硬编码剧情）
- Create if useful: `docs/operations/story-runtime-v1.md`

**Document:**

- 实际 API 路径。
- Manifest Schema 实际版本。
- 失败/重试/暂停行为。
- 普通和复杂 endpoint 的区别。
- 删除和 regenerate 限制。
- 配置方式和无凭据示例。
- 已实现/未实现边界。

---

## 推荐提交节奏

不要把全部 V1 合并成一个大提交。建议：

```text
1. feat: connect story runtime message api
2. feat: add production story primary client
3. feat: add story prompt and scheduler prompt builders
4. feat: enforce story chat mutation restrictions
5. feat: add story scheduler retry and pause policy
6. feat: complete manifest schema and stale handling
7. feat: add story runtime frontend flow
8. test: add story runtime integration coverage
```

每个提交前执行：

```bash
git diff --check
go test <受影响包>
git status --short
```

只有阶段性测试通过且工作区范围确认后才 commit；用户明确要求时再 push。

---

## 最终 V1 验收标准

只有全部满足以下条件，才能称为“V1 完成”：

- 普通聊天仍完整走旧链路。
- 复杂聊天真实走 `StoryChatRuntime`，不是测试 fake。
- 复杂消息 API/SSE 可用。
- 主模型和调度模型分别调用。
- 调度模型输入不复用主模型 system 上下文。
- compile_only 原文不进入主模型运行时。
- Manifest 按完整版本键缓存并支持 stale/retry。
- 初始状态来自已校验 Manifest。
- 规则、效果、事件、版本和事务均由后端决定。
- 调度失败可见、可重试，上一成功状态保留。
- 连续失败可暂停复杂剧情。
- 复杂消息删除和 regenerate 有 V1 明确限制。
- 前端能显示 scheduler 状态并执行 retry。
- API、Service、Store、并发和失败路径有自动化测试。
- 不提交、不日志输出任何凭据。
