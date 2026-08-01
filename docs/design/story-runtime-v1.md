# LiteChat 复杂角色卡剧情运行时 V1 详细设计

> 状态：设计阶段，不是实现代码。
>
> 关联角色卡：`docs/character-design/liyu-complex-card-draft.md`
>
> 目标：为复杂角色卡提供独立的剧情初始化、结构化记忆、状态管理和调度能力，同时保证现有普通聊天链路不被污染。

---

## 1. 设计目标

### 1.1 必须实现

- 普通角色卡继续使用现有聊天链路。
- 复杂角色卡可以按会话开启剧情运行时。
- 复杂角色卡第一次使用时，可以将指定剧情世界书交给高能力模型进行一次性编译。
- 编译结果包括动态字段、事件条件、事件效果、信息可见性和初始状态。
- 主模型只负责正常角色扮演，不输出记忆标签，不负责状态计算。
- 便宜调度模型读取本轮用户消息和主模型回复，提取候选事件和状态变化。
- 程序负责 Schema 校验、字段白名单、证据检查、条件判断、数值计算和最终落库。
- 调度结果保存到独立数据结构中。
- 下一轮主模型只读取最近一次确认成功的剧情状态和动态上下文。
- 调度失败必须可见、可重试，不能静默污染剧情。
- 原始 `compile_only` 世界书在调度运行时不再直接注入主模型。

### 1.2 V1 明确不实现

- 不允许调度器直接重写原始世界书文本。
- 不让模型直接生成或执行 SQL。
- 不让模型直接决定最终数值或客观战斗结果。
- 不在 V1 实现完整固定剧情树。
- 不在 V1 自动生成无限新的状态字段。
- 不将完整未来剧情 Manifest 注入主模型。
- 不修改普通角色卡的现有运行逻辑。
- 不把调度器失败变成普通聊天的隐性行为变化。

---

## 2. 总体架构

```text
普通会话：
  前端
    -> 现有 ChatService
    -> 现有 buildMessages
    -> 主模型
    -> SSE

复杂剧情会话：
  前端
    -> StoryRuntime
       -> 读取 Manifest 和当前状态
       -> 读取最近成功的调度上下文
       -> 组装主模型上下文
       -> 主模型 SSE
       -> 保存用户/助手消息
       -> SchedulerService
          -> 调度模型
          -> 容错解析
          -> Schema 校验
          -> 证据校验
          -> RuleEngine
          -> 数据库事务
       -> 返回 scheduler_status
```

职责分离：

| 组件 | 职责 | 不负责什么 |
|---|---|---|
| 初始化模型 | 理解完整剧情世界书并编译 Manifest | 不直接对用户输出剧情 |
| 主模型 | 角色扮演、叙事、NPC回应 | 不写记忆协议、不算状态 |
| 调度模型 | 从本轮对话提取候选事实和事件 | 不直接提交数据库、不决定最终数值 |
| 规则引擎 | 判断条件、计算变化、处理状态迁移 | 不生成自然语言剧情 |
| Store/DB | 保存版本、状态、事件、审计和失败记录 | 不解释模型文本 |
| ContextBuilder | 根据已确认状态生成下一轮上下文 | 不读取未授权未来剧情 |

---

## 3. 世界书运行模式

现有 `world_books` 增加运行模式字段：

```text
runtime_mode:
  static
  compile_only
  dynamic_context
```

### 3.1 `static`

运行时正常注入。

适合：

- 世界认知与客观规律
- 修仙境界和客观力量差距
- 固定角色人格
- 文风和输出格式
- 用户控制权
- 不随剧情变化的设定

默认值必须是 `static`，保证旧数据库和旧世界书不改变行为。

### 3.2 `compile_only`

只作为初始化编译模型的输入，复杂剧情运行时不直接注入原文。

适合：

- 剧情节点
- 隐藏真相
- 事件条件
- 路线结构
- 结局条件
- 角色隐藏动机
- 逐步揭示的秘密

调度运行时读取由 Manifest 和 Runtime State 生成的当前上下文，不读取这类世界书原文。

### 3.3 `dynamic_context`

V1 暂不作为用户手工编辑的核心能力。概念上表示：内容来自当前状态，经后端 ContextBuilder 生成后注入。

---

## 4. Manifest 编译

### 4.1 编译缓存粒度

Manifest 按以下版本组合缓存：

```text
character_id
+ character_version
+ compile_only_worldbook_version_hash
+ compiler_prompt_version
+ compiler_model
+ manifest_schema_version
```

同一张角色卡创建多个聊天时，复用同一个 `ready` Manifest，只复制新的初始 Runtime State。

### 4.2 触发重新编译

以下任一项变化时，旧 Manifest 标记为 `stale`：

- 角色卡固定字段变化
- `compile_only` 世界书变化
- 初始化提示词变化
- Manifest Schema 变化
- 作者主动点击重新编译

### 4.3 初始化输入

高能力初始化模型可以看到：

- 全部 `compile_only` 世界书
- 角色卡固定内容
- 允许读取的静态世界书
- 初始化编译协议
- 输出字段限制和安全约束

它的输出只进入后端，不直接展示给用户，也不直接作为主模型上下文。

### 4.4 Manifest 顶层结构

```json
{
  "manifest_version": 1,
  "source_version": {
    "character_version": "...",
    "worldbook_hash": "..."
  },
  "fields": [],
  "events": [],
  "initial_state": {},
  "visibility_rules": [],
  "warnings": []
}
```

### 4.5 字段定义

允许的字段类型：

```text
boolean
integer
number
enum
string
string_set
event_set
counter
```

字段定义至少包括：

```json
{
  "key": "relationships.liu_ruyan.disappointment",
  "type": "integer",
  "initial": 0,
  "min": 0,
  "max": 100,
  "scheduler_writable": true,
  "main_model_visible": true,
  "user_visible": false,
  "author_only": false
}
```

禁止初始化模型创建：

- SQL
- 可执行脚本
- 任意提示词片段
- 无限嵌套对象
- 未声明的系统规则
- 无边界自由文本状态

### 4.6 事件定义

```json
{
  "id": "resource_dispute_001",
  "name": "第一次资源让渡争议",
  "preconditions": {
    "all": [
      {
        "field": "world.jiang_muchen.injured",
        "operator": "eq",
        "value": true
      },
      {
        "field": "events.resource_dispute_001",
        "operator": "not_exists"
      }
    ]
  },
  "exclusions": [],
  "priority": 80,
  "once": true,
  "effects": [],
  "unlock_clues": [],
  "next_events": [],
  "user_visibility": "indirect"
}
```

Manifest 的事件条件是声明式配置，运行时由后端判断，不由调度模型临时发挥。

### 4.7 初始化校验

初始化模型输出进入运行时前，必须经过：

```text
原始响应保存
-> 容错解析
-> Manifest Schema 校验
-> 字段名和类型校验
-> 初始值范围校验
-> 事件引用完整性校验
-> 条件操作符校验
-> 事件依赖循环检测
-> 数量和大小限制
-> 敏感字段权限校验
-> 保存为 ready
```

失败则：

```text
Manifest = failed
复杂剧情会话不可启动
用户看到“剧情初始化失败，可重试”
```

---

## 5. Runtime State

每个聊天拥有独立状态，不能直接修改 Manifest。

### 5.1 初始状态

新聊天创建时：

```text
Manifest.initial_state
-> 深拷贝
-> 创建 chat_story_state
```

多个聊天共享事件定义，但不共享：

- 当前剧情阶段
- 关系数值
- 已触发事件
- 已解锁线索
- 用户认知
- 资源和伤势

### 5.2 状态结构示例

```json
{
  "state_version": 0,
  "current_scene": "unknown",
  "active_event": "opening_arrival",
  "route": "survival",
  "relationships": {
    "liu_ruyan": {
      "trust": 85,
      "dependence": 90,
      "disappointment": 0
    },
    "jiang_muchen": {
      "gratitude": 20,
      "fear": 50,
      "ambition": 30
    }
  },
  "knowledge": {
    "knows_transmigration": true,
    "knows_original_plot": false,
    "suspects_being_used": false,
    "knows_tool_role": false
  },
  "facts": [],
  "unlocked_clues": [],
  "open_loops": [],
  "resources": {}
}
```

---

## 6. 主模型运行时上下文

调度器成功后，`compile_only` 原始世界书不再插入。

主模型可见内容：

```text
角色卡固定字段
+ static 世界书
+ ContextBuilder 生成的当前剧情上下文
+ 允许主模型读取的状态字段
+ 当前事件允许表现的 NPC 动机
+ 已解锁且允许公开的线索
+ 普通聊天历史
+ 用户最新消息
```

主模型不可见：

- 完整 Manifest
- 未触发事件
- 未解锁线索
- 最终结局
- 用户工具人真相
- 江沐宸夺舍结局
- 其他角色不应知道的未来动机

`context_text` 不直接信任调度模型返回的自然语言，而由后端根据确认后的状态重新生成。

---

## 7. 每轮运行流程

### 7.1 主模型前处理

```text
1. 验证 chat 所属用户
2. 读取 chat scheduler 配置
3. 如果关闭，走旧 ChatService
4. 如果开启，检查 Manifest 状态
5. Manifest 非 ready：拒绝进入复杂剧情模式
6. 读取最近一次 success 的 Runtime State
7. 读取当前允许注入的动态上下文
8. 组装主模型消息
```

### 7.2 主模型生成

主模型保持现有 SSE 行为：

```text
用户输入落库
-> 主模型流式输出
-> 前端实时显示
-> assistant 消息落库
```

主模型不附加记忆标签，不负责调度协议。

### 7.3 调度模型处理

主模型回复保存成功后：

```text
创建 scheduler_record = pending
-> scheduler_record = processing
-> 调度模型读取本轮用户消息、assistant 回复、上一份成功状态和 Manifest 相关定义
-> 调度模型输出候选观察和状态提议
-> 后端解析与校验
-> 规则引擎执行
-> 数据库事务提交
-> scheduler_record = success/failed
```

### 7.4 调度模型输入限制

调度模型看到：

- 本轮用户消息
- 本轮主模型回复
- 最近成功状态
- 当前 active event
- Manifest 中相关字段和可触发事件
- 调度器专用固定提示词
- 用户允许的补充调度提示词

调度模型不需要每轮读取全部原始世界书。

---

## 8. 调度模型输出协议

调度模型输出的是候选观察，不是最终数据库变更。

```json
{
  "schema_version": 1,
  "observations": [
    {
      "key": "liu_ruyan_requested_resource_transfer",
      "value": true,
      "evidence": "柳如烟当众要求李预将资源交给江沐宸",
      "confidence": 0.96
    }
  ],
  "event_candidates": [
    {
      "event_id": "resource_dispute_001",
      "reason": "本轮出现资源让渡请求",
      "evidence": "..."
    }
  ],
  "inferences": [],
  "warnings": []
}
```

模型不直接输出：

- SQL
- 任意状态路径
- 未声明字段
- 最终数值
- 最终死亡/夺舍判定
- 未经条件满足的路线跳转

`observations` 是“模型看到的事实”，`inferences` 是推测。只有确认事实可以直接进入规则处理；推测只能作为低权限候选线索。

---

## 9. 容错解析和校验

### 9.1 解析顺序

```text
保存 raw_output
-> 去除首尾空白
-> 去除 markdown code fence
-> 提取第一个完整 JSON 对象
-> JSON decode
-> Schema 校验
```

不做无限制的自动修复，不随意补引号、不随意删字段。

### 9.2 Schema 校验

检查：

- `schema_version` 是否支持
- 顶层字段类型
- 数组元素类型
- 字符串长度
- 数值范围
- 枚举值
- 字段数量
- evidence 长度
- 未知字段

### 9.3 业务校验

检查：

- key 是否在 Manifest 白名单
- observation 是否有证据
- 事件是否已经触发
- 字段是否允许调度器写入
- 当前状态是否满足操作前提
- 是否产生不允许的逆转
- 是否违反客观世界规则

### 9.4 部分接受策略

普通状态变化可以部分接受：

```text
合法事件 A + 合法关系变化 B + 非法字段 C
-> 接受 A、B
-> 拒绝 C
-> 记录 warning
```

关键变化必须整轮拒绝：

- 死亡
- 夺舍
- 核心身份揭示
- 重大资源转移
- 路线切换
- 剧情阶段跃迁
- 不可逆状态变化

---

## 10. 规则引擎

规则引擎负责：

- `all/any/none` 条件
- 数值加减和边界
- enum 状态转换
- once 事件
- cooldown
- 事件依赖
- 事件去重
- 资源存在性
- 伤势和死亡状态一致性
- Manifest 字段权限

例如：

```text
观察到：public_refusal_request
规则：
  柳如烟失望 +5
  宗门声望 -3
  标记事件 public_refusal_request 已发生
  检查后续事件条件
```

模型可以提出“发生了公开拒绝”，但不能提出“失望从 80 改成 12”。

### 10.1 幂等

事件唯一性建议使用：

```text
chat_id + event_key
```

Manifest 声明：

```text
once = true
```

重复报告只记录 duplicate，不重复执行。

### 10.2 版本控制

每次状态更新必须带：

```text
state_version_before
state_version_after
```

版本不匹配时标记 `conflict`，不能直接覆盖新状态。

---

## 11. 数据库事务

一次成功调度包含多个操作时必须原子提交：

```text
BEGIN
  检查 state_version
  验证所有变更
  写入 chat_story_events
  更新 chat_story_states
  写入 applied_changes
  更新 scheduler_record
COMMIT
```

任何错误：

```text
ROLLBACK
scheduler_record = failed
```

不能出现事件写入成功但关系值更新失败的半状态。

---

## 12. 调度失败策略

### 12.1 模型超时/网络失败

```text
scheduler_record = failed
chat_story_state 保持上一成功版本
前端提示本轮调度失败
支持重试
```

### 12.2 格式错误

严格格式重试一次；仍失败则 `failed`。

### 12.3 状态冲突

重新读取最新状态并重新调度；超过重试次数后标记 `conflict`。

### 12.4 前端显示

主模型回复下显示：

```text
⚠ 本轮回复已生成，但剧情调度失败。
下一轮暂时使用上一份有效剧情状态。
[重试调度]
```

### 12.5 连续失败

建议：

```text
一次失败：允许继续
连续两次：持续警告
超过阈值：暂停复杂剧情模式
```

暂停后用户只能：

```text
重试调度
关闭剧情模式，转为普通聊天
```

---

## 13. SSE 设计

当前主模型输出仍然流式进行。

建议在主模型内容结束后补充调度状态事件：

```text
data: {"done": true}
```

或扩展为：

```text
data: {"scheduler_status": "success"}
```

失败：

```text
data: {
  "scheduler_status": "failed",
  "error_code": "model_timeout",
  "message": "剧情调度失败"
}
```

V1 可以在主模型回复完成后再同步等待调度模型，保证前端当场得到结果；以后再考虑后台异步和轮询。

---

## 14. 重新生成、删除和并发

### 14.1 重新生成

重新生成主模型回复时：

```text
旧 assistant 消息对应 scheduler_record -> invalid
旧事件和状态变更不能继续生效
从重新生成后的 assistant 消息重新调度
```

### 14.2 级联删除

删除某条消息及后续消息时：

```text
对应 scheduler_record -> invalid
删除或重建该消息之后的状态
后续 Runtime State 需要从最近一致版本重放
```

V1 若不实现完整重放，可以先禁止在复杂剧情模式中对已调度历史进行任意删除，或者要求从该节点重新初始化状态。

### 14.3 并发

同一 chat 同时只能有一个调度任务：

```text
chat_id 级别锁 / processing 标识
```

防止两个调度任务互相覆盖状态。

---

## 15. 用户和管理员配置

### 15.1 系统配置

管理员设置：

```text
scheduler_api_endpoint
scheduler_api_key
scheduler_model
scheduler_timeout
scheduler_retry_limit
scheduler_enabled_default
```

### 15.2 会话配置

每个复杂聊天保存：

```text
scheduler_enabled
manifest_id
scheduler_status
```

### 15.3 用户补充提示词

后续支持用户编辑补充提示词，但分三层：

```text
系统核心调度规则：不可编辑
角色卡调度配置：由作者/管理员管理
用户补充提示词：用户可编辑
```

用户补充内容不能覆盖：

- 字段权限
- 客观规则
- 信息隐藏规则
- 用户控制权
- 事务和安全约束

---

## 16. 现有项目接入边界

建议新增：

```text
internal/model/scheduler.go
internal/store/scheduler_store.go
internal/service/scheduler_service.go
internal/service/story_runtime.go
internal/service/scheduler_validator.go
internal/service/story_rules.go
```

API 增加：

```text
POST /api/chats/:id/story/initialize
GET  /api/chats/:id/story/status
POST /api/chats/:id/story/scheduler/:recordId/retry
PUT  /api/chats/:id/story/settings
```

现有文件主要只做接入：

```text
internal/api/router.go
internal/api/handlers.go
internal/service/chat_service.go
internal/store/db.go
web/src/store/index.js
web/src/pages/ChatPage.jsx
web/src/pages/CharactersPage.jsx
```

普通聊天应继续走原来的路径，不要把 SchedulerService 直接嵌入所有 `buildMessages` 逻辑。

更安全的结构：

```text
LegacyChatRuntime
StoryRuntime
```

根据会话配置选择运行时。

---

## 17. 实现阶段

### Phase 1：只编译和记录，不影响主模型

- Manifest 初始化
- 调度模型调用
- 原始输出保存
- 解析和校验
- 调度记录状态
- 旁路观察

### Phase 2：接入 Runtime State

- 状态表
- 事件日志
- 规则引擎
- 下一轮读取最近成功状态
- 调度失败提示

### Phase 3：动态上下文

- ContextBuilder
- `compile_only` 世界书停止运行时注入
- 当前事件和已解锁线索注入
- 信息可见性控制

### Phase 4：复杂事件能力

- 事件依赖
- 路线状态
- 关键状态变更复核
- 重放和分支恢复
- 商店角色卡剧情 Manifest

---

## 18. 测试要求

### 普通聊天回归

- 调度器关闭时请求消息完全保持旧结构
- 普通角色卡不触发初始化
- 普通世界书行为不变
- 现有 SSE 流式聊天不受影响

### Manifest

- 初始化成功
- JSON 格式错误
- 缺少字段
- 未知字段
- 引用不存在的状态字段
- 事件循环依赖
- 初始值越界
- 世界书版本变化导致 stale

### 调度解析

- 纯 JSON
- markdown 包裹 JSON
- JSON 前后带说明文字
- 不完整 JSON
- 未知字段
- 重复事件
- 证据不足
- 非法状态路径

### 状态和事务

- 成功状态更新
- 部分非法变更
- 关键变更整轮拒绝
- 事务回滚
- 状态版本冲突
- 并发调度
- 重试幂等

### 用户流程

- 初始化失败
- 调度失败
- 连续失败暂停
- 手动重试
- 重新生成
- 删除消息
- 删除聊天
- 关闭剧情模式

---

## 19. 可行性结论

方案可行，但不是单纯新增一个模型调用。它实际包含：

```text
剧情 Manifest 编译器
+ 独立 Runtime State
+ 调度记录
+ 事件日志
+ 容错解析器
+ Schema/权限校验
+ 确定性规则引擎
+ 数据库事务
+ SSE 状态反馈
```

为了不污染旧程序，必须坚持：

1. 普通聊天默认走旧链路。
2. 新数据表和新 Service 独立存在。
3. `compile_only` 世界书只在初始化阶段读取。
4. 调度模型输出只是候选，不是最终状态。
5. 后端规则引擎拥有最终写入权。
6. 调度失败保留上一成功状态并明确提示。
7. 所有状态更新可追踪、可拒绝、可回滚。

## 22. 接口优先的运行时设计

LiteChat 使用 Go，不采用 Java 类继承，但采用等价的接口优先和组合设计：

```go
type ChatRuntime interface {
    SendMessage(...)
    Regenerate(...)
    Retry(...)
}

type BaseChatRuntime struct {
    ChatStore      *store.ChatStore
    MessageStore   *store.MessageStore
    CharacterStore *store.CharacterStore
    PresetStore    *store.PresetStore
    WorldBookStore *store.WorldBookStore
    ConfigStore    *store.ConfigStore
}

type LegacyChatRuntime struct {
    BaseChatRuntime
}

type StoryChatRuntime struct {
    BaseChatRuntime
    SchedulerService *SchedulerService
    StoryStore       *store.SchedulerStore
}
```

设计目标是：调用方只依赖 `ChatRuntime` 接口，不需要知道底层具体实现。

```text
RuntimeFactory
    -> ChatRuntime
        -> LegacyChatRuntime
        -> StoryChatRuntime
```

接口只暴露用户动作，不暴露底层实现细节：

- 发送消息
- 重新生成
- 重试

不把所有 Store 方法、提示词拼接方法和数据库细节塞进接口。

Go 中使用 struct embedding 复用稳定依赖和公共辅助能力，但不把两套高层业务流程强行合并。普通流程和复杂剧情流程仍然可以独立演进、独立测试和独立回退。

### 22.1 分阶段迁移顺序

采用“先复杂、后简单”的迁移策略：

```text
阶段 1：旧简单流程完全不动
        独立开发和运行 StoryChatRuntime

阶段 2：复杂流程实际试用和稳定性验证
        验证初始化、主模型、调度、状态、失败恢复和长线运行

阶段 3：复杂流程达到可正常使用标准
        再将 LegacyChatRuntime 接入同一套 ChatRuntime 接口

阶段 4：视收益决定是否进一步合并公共代码
        不以消除重复为目标强行重构旧逻辑
```

在阶段 1 和阶段 2：

- 简单角色卡继续调用旧接口。
- 旧 `ChatService` 作为稳定基线保留。
- 复杂角色卡使用独立 StoryChatRuntime 和剧情接口。
- 新架构的问题不会直接影响简单角色卡。
- 测试和实际试用优先集中在复杂流程。

只有当复杂流程确认能够正常完成以下链路后，才开始迁移简单流程：

```text
创建复杂剧情会话
→ Manifest 初始化
→ 主模型回复
→ 调度模型处理
→ 规则引擎判断
→ 状态和事件事务提交
→ 下一轮读取动态上下文
→ 调度失败可见并可恢复
```

后续迁移简单流程时，先用 `LegacyChatRuntime` 包装现有代码，不立即重写旧业务；待新接口运行稳定后，再逐步提取真正无行为差异的公共能力。


### 22.2 平行实现与旧流程保护

采用“旧流程冻结、新流程平行实现”的策略：

```text
旧聊天流程：保留原代码和原接口
复杂剧情流程：复制必要的聊天编排逻辑，使用独立 StoryChatService
```

允许共享的内容仅限于稳定的底层能力：

- CharacterStore
- PresetStore
- MessageStore
- WorldBookStore 的基础读取能力
- 配置读取
- OpenAI 请求 DTO

不共享会改变行为的高层流程：

- 消息上下文编排
- 世界书注入逻辑
- 调度状态处理
- 失败策略
- 复杂剧情记录策略
- 重新生成和删除行为

推荐增加独立接口：

```text
POST /api/chats/:id/messages
  旧普通聊天接口，保持原行为

POST /api/story/chats/:id/messages
  复杂剧情聊天接口，只接受 scheduler_enabled 的会话
```

前端根据会话模式选择接口。普通角色卡永远调用旧接口，复杂角色卡调用剧情接口。

### 21.1 回退策略

回退必须区分阶段：

1. **剧情初始化前失败**：不自动切换；提示初始化失败。用户主动选择“普通聊天模式”后，才关闭剧情模式并调用旧接口。
2. **主模型调用前失败**：可以提示用户切换普通聊天，但不自动切换。
3. **主模型已经生成并落库、调度模型失败**：不能切回旧流程；保留主模型回复，标记调度失败，使用上一份成功状态并提供重试。
4. **状态事务提交失败**：不能继续推进剧情；保持上一状态，提示用户重试。
5. **新流程出现未捕获异常**：由剧情接口边界恢复并返回错误，不能让整个 HTTP 服务退出。

不允许在复杂剧情已经产生部分状态后静默切回旧流程，因为旧流程会重新注入 `compile_only` 世界书，可能造成剧透和状态不一致。

### 21.2 复制代码的边界

可以复制旧 `SendMessage` 的编排骨架，快速建立 `StoryChatService`，但不复制所有底层 Store。

复制后两条流程分别维护：

```text
LegacyChatService：旧代码保持稳定
StoryChatService：独立演进剧情初始化、上下文、调度和状态事务
```

未来确认两条流程稳定后，再考虑提取无行为差异的公共辅助函数；V1 不为了消除重复而重构旧流程。

该策略优先保证：

- 新功能失败不会阻塞普通项目
- 普通角色卡不经过调度器
- 旧接口可独立回归测试
- 新流程可逐步接入
- 出现问题时可以关闭剧情模式继续使用旧系统
### 21.3 V1 已确认决策

以下决策已冻结，后续实现按此执行：

1. **数据结构**：接受三张核心表加事件日志表：
   - `story_manifests`
   - `chat_story_states`
   - `chat_scheduler_records`
   - `chat_story_events`

2. **初始化失败**：复杂剧情初始化失败时，不自动静默降级为普通剧情模式；必须明确提示用户并支持重试。用户如需普通聊天，必须主动选择关闭剧情模式。

3. **复杂剧情记录**：V1 暂不支持复杂剧情模式下删除已生成的剧情记录，也不支持对中间历史进行任意删除后的自动重放。复杂剧情消息和调度记录先按不可删除设计。

4. **调度时机**：主模型回复结束后，同步调用调度模型处理本轮内容，再结束本轮 SSE，以便前端立即得到明确的调度成功/失败状态。

这些决策的目的，是优先保证复杂剧情状态的连续性和可追踪性，不在 V1 同时引入历史分支、回放和异步任务一致性问题。
