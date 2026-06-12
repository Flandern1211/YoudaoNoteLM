# 对话模块设计文档

> 版本：V4.0
> 日期：2026-06-11
> 状态：设计完成（含 Agent 架构）

---

## 1. 概述

### 1.1 范围

本文档定义 YoudaoNoteLM 项目中对话模块的设计，包括：
- 对话管理（CRUD、列表、标题）
- RAG 检索服务（混合检索、RRF 融合）
- RAG 问答（基于资料来源的流式问答）
- **智能 Agent 对话（基于 eino ADK 的 ReAct Agent）**

### 1.2 架构定位

对话模块提供两种模式：
1. **Pipeline 模式**（原有）：固定流程的 RAG 问答，适合简单场景
2. **Agent 模式**（新增）：基于 LLM 自主决策的 ReAct Agent，支持多步推理、工具调用

两种模式共存，通过不同 API 端点访问。

### 1.3 Agent 模式优势

| 维度 | Pipeline 模式 | Agent 模式 |
|------|--------------|-----------|
| 决策者 | 代码逻辑 | LLM 自主决策 |
| 流程 | 固定顺序 | 动态，LLM 决定下一步 |
| 工具使用 | 无（RAG 是硬编码步骤） | LLM 自主选择调用哪些工具 |
| 多步推理 | 不支持 | 支持（思考→调用→观察→再思考） |
| 适用场景 | 简单问答 | 复杂任务、多工具协作 |

---

## 2. 整体架构

### 2.1 双模式架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Controller (SSE)                            │
│                                                                      │
│   POST /chat/conversations/:convId/messages      → Pipeline 模式     │
│   POST /chat/conversations/:convId/agent-messages → Agent 模式       │
└─────────────────────────┬───────────────────────┬───────────────────┘
                          │                       │
                          ▼                       ▼
┌─────────────────────────────────┐ ┌─────────────────────────────────┐
│     ChatService (Pipeline)       │ │     ChatAgentService (Agent)    │
│                                  │ │                                  │
│  固定流程：                       │ │  ReAct 循环：                    │
│  1. 加载上下文                    │ │  1. LLM 分析问题                 │
│  2. Query 改写                    │ │  2. LLM 决定调用工具             │
│  3. RAG 检索                      │ │  3. 执行工具，获取结果            │
│  4. 构建 Prompt                   │ │  4. LLM 观察结果，决定下一步      │
│  5. LLM 生成                      │ │  5. 重复直到 LLM 生成最终回答     │
│  6. 保存消息                      │ │  6. 保存消息                     │
└─────────────────────────────────┘ └──────────────┬──────────────────┘
                                                   │
                                                   ▼
                                  ┌────────────────────────────────────┐
                                  │        adk.ChatModelAgent          │
                                  │  ┌──────────────────────────────┐  │
                                  │  │ Tools                        │  │
                                  │  │  ├── search_knowledge (RAG)  │  │
                                  │  │  ├── get_chat_history        │  │
                                  │  │  ├── web_search              │  │
                                  │  │  └── ...扩展工具              │  │
                                  │  └──────────────────────────────┘  │
                                  └────────────────────────────────────┘
```

### 2.2 Pipeline 模式数据流

```
用户消息 → ChatService.ProcessMessage
  → 加载上下文 → Query 改写 → RAG 检索 → 构建 Prompt → LLM 生成 → 保存消息
  → 返回 <-chan StreamEvent → Controller SSE 输出
```

### 2.3 Agent 模式数据流

```
用户消息 → ChatAgentService.ProcessMessageWithAgent
  → 构建消息 → 创建 Agent → Runner.Run()
  → ReAct 循环：
      LLM 思考 → 调用工具 → 观察结果 → 继续思考
      (直到 LLM 生成最终回答)
  → 转发 AgentEvent → Controller SSE 输出
```

### 2.4 Agent 内部 ReAct 循环

```
┌──────────────────────────────────────────────────────────────┐
│                    ReAct 循环 (adk.ChatModelAgent)            │
│                                                               │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐   │
│   │  LLM 思考   │ ──→ │  调用工具   │ ──→ │  观察结果   │   │
│   └─────────────┘     └─────────────┘     └─────────────┘   │
│          ↑                                         │          │
│          └─────────────────────────────────────────┘          │
│                                                               │
│   终止条件：                                                   │
│   - LLM 不再调用工具，直接生成回答                              │
│   - 达到最大步数 (MaxStep=10)                                  │
└──────────────────────────────────────────────────────────────┘
```

---

## 3. 数据模型

### 3.1 Entity 变更

#### Conversation 表新增字段

```go
// internal/model/entity/conversation.go
type Conversation struct {
    entity.BaseEntity
    NotebookID      uint         `gorm:"index;not null"`
    Notebook        Notebook     `gorm:"foreignKey:NotebookID;constraint:OnDelete:CASCADE"`
    UserID          uint         `gorm:"index;not null"`
    Title           string       `gorm:"type:varchar(255);not null;default:'新对话'"`
    Summary         string       `gorm:"type:text"`     // 新增：对话摘要
    SummaryMsgCount int          `gorm:"default:0"`     // 新增：摘要覆盖的消息数
}
```

#### Message 表新增字段

```go
// internal/model/entity/message.go
type Message struct {
    entity.BaseEntity
    ConversationID uint          `gorm:"index;not null"`
    Conversation   Conversation  `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
    Role           string        `gorm:"type:varchar(20);not null"`
    Content        string        `gorm:"type:text;not null"`
    Metadata       string        `gorm:"type:json"` // 新增：存储引用来源等元信息
}
```

### 3.2 Metadata JSON 结构

```go
// MessageMetadata 消息元数据
type MessageMetadata struct {
    References  []Reference `json:"references"`
    TokensUsed  int         `json:"tokens_used,omitempty"`
}

// Reference 引用来源
type Reference struct {
    SourceID      uint    `json:"source_id"`
    SourceName    string  `json:"source_name"`
    ParentBlockID int64   `json:"parent_block_id"`
    ChunkContent  string  `json:"chunk_content"`
    Score         float32 `json:"score"`
}
```

### 3.3 Redis 缓存结构

| Key | 类型 | TTL | 用途 |
|-----|------|-----|------|
| `chat:{id}:recent_messages` | List | 7 天 | 最近 10 轮（20 条）消息 |
| `chat:{id}:summary` | String | 7 天 | 对话摘要 |
| `chat:{id}:lock` | String (NX) | 120 秒 | 并发锁 |

---

## 4. ChatTool 设计

### 4.1 接口实现

```go
// internal/tool/chat/tool.go
package chat

import (
    "context"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
)

// ChatTool 实现 eino.StreamableTool 接口
type ChatTool struct {
    chatService ChatService
}

// NewChatTool 创建 ChatTool 实例
func NewChatTool(chatService ChatService) *ChatTool {
    return &ChatTool{chatService: chatService}
}

// ToolInfo 定义工具元信息
func (t *ChatTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "chat_qa",
        Desc: "基于用户提供的资料来源进行知识问答，支持多轮对话",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "conversation_id": {
                Type: schema.Integer,
                Desc: "对话 ID，新建对话时为 0",
            },
            "content": {
                Type: schema.String,
                Desc: "用户的问题或消息内容",
            },
            "source_ids": {
                Type: schema.Array,
                Desc: "选中的资料来源 ID 列表",
                Items: &schema.ParameterInfo{
                    Type: schema.Integer,
                },
            },
        }),
    }, nil
}

// Stream 实现流式工具调用
func (t *ChatTool) Stream(ctx context.Context, input any, opts ...tool.Option) (*schema.StreamReader[any], error) {
    // 1. 解析输入参数
    params, err := parseInput(input)
    if err != nil {
        return nil, err
    }

    // 2. 调用 ChatService 处理消息
    eventCh, err := t.chatService.ProcessMessage(ctx, &ProcessMessageRequest{
        ConversationID: params.ConversationID,
        Content:        params.Content,
        SourceIDs:      params.SourceIDs,
        UserID:         getUserIDFromContext(ctx),
    })
    if err != nil {
        return nil, err
    }

    // 3. 将 StreamEvent channel 包装为 schema.StreamReader
    return schema.ConvertChanToStreamReader(eventCh), nil
}
```

### 4.2 输入参数

```go
// internal/tool/chat/types.go
package chat

// ChatToolInput Tool 输入参数
type ChatToolInput struct {
    ConversationID uint   `json:"conversation_id"`
    Content        string `json:"content"`
    SourceIDs      []uint `json:"source_ids"`
}
```

### 4.3 主 Agent 调用方式

```go
// 主 Agent 中使用
chatTool := chat.NewChatTool(chatService)

// 注册到 eino 工具集
tools := []tool.Tool{chatTool}

// 调用时
reader, err := chatTool.Stream(ctx, map[string]interface{}{
    "conversation_id": 123,
    "content":         "这个资料的主要观点是什么？",
    "source_ids":      []uint{1, 2, 3},
})

// 主 Agent 读取 reader 并 SSE 透传
for {
    event, err := reader.Recv()
    if err != nil {
        break
    }
    // 写入 SSE 响应
    writeSSE(c, event)
}
```

---

## 5. ChatService 设计

### 5.1 接口定义

```go
// internal/service/chat_interface.go
package service

// ChatService 对话业务逻辑接口
type ChatService interface {
    // ProcessMessage 处理用户消息，返回流式事件通道
    ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (<-chan StreamEvent, error)

    // CreateConversation 创建对话
    CreateConversation(ctx context.Context, req *CreateConversationRequest) (*Conversation, error)

    // GetConversation 获取对话详情
    GetConversation(ctx context.Context, conversationID uint) (*Conversation, error)

    // ListConversations 获取对话列表
    ListConversations(ctx context.Context, notebookID uint) ([]*Conversation, error)

    // UpdateConversation 更新对话（标题）
    UpdateConversation(ctx context.Context, conversationID uint, req *UpdateConversationRequest) error

    // DeleteConversation 删除对话
    DeleteConversation(ctx context.Context, conversationID uint) error

    // GetMessages 获取消息历史
    GetMessages(ctx context.Context, conversationID uint) ([]*Message, error)

    // StopGeneration 终止回答生成
    StopGeneration(ctx context.Context, conversationID uint) error
}
```

### 5.2 类型定义

```go
// internal/service/chat_types.go
package service

// ProcessMessageRequest 处理消息请求
type ProcessMessageRequest struct {
    ConversationID uint   // 对话 ID，0 表示新建对话
    Content        string // 用户消息内容
    SourceIDs      []uint // 选中的资料来源 ID
    UserID         uint   // 用户 ID
}

// StreamEvent 流式事件
type StreamEvent struct {
    Type    string      `json:"type"`    // "token", "reference", "done", "error"
    Content string      `json:"content"` // 事件内容
    Data    interface{} `json:"data"`    // 附加数据
}

// ChatContext 对话上下文
type ChatContext struct {
    Summary    string        // 对话摘要
    History    []MessagePair // 最近 10 轮对话历史
    TotalCount int           // 总消息数
}

// MessagePair 消息对
type MessagePair struct {
    User      string
    Assistant string
}
```

### 5.3 实现结构

```go
// internal/service/chat_service.go
package service

type chatService struct {
    llm              model.ChatModel
    retriever        rag.RAGRetriever
    conversationRepo repository.ConversationRepository
    messageRepo      repository.MessageRepository
    cache            *redis.ChatCache
    cancelFuncs      sync.Map // conversationID -> context.CancelFunc
}

func NewChatService(
    llm model.ChatModel,
    retriever rag.RAGRetriever,
    conversationRepo repository.ConversationRepository,
    messageRepo repository.MessageRepository,
    cache *redis.ChatCache,
) ChatService {
    return &chatService{
        llm:              llm,
        retriever:        retriever,
        conversationRepo: conversationRepo,
        messageRepo:      messageRepo,
        cache:            cache,
    }
}
```

### 5.4 ProcessMessage 核心流程

```go
func (s *chatService) ProcessMessage(ctx context.Context, req *ProcessMessageRequest) (<-chan StreamEvent, error) {
    ctx, cancel := context.WithCancel(ctx)
    s.cancelFuncs.Store(req.ConversationID, cancel)

    eventCh := make(chan StreamEvent, 100)

    go func() {
        defer close(eventCh)
        defer s.cancelFuncs.Delete(req.ConversationID)

        // 1. 加载上下文
        context, err := s.loadContext(ctx, req.ConversationID)
        if err != nil {
            sendError(eventCh, "加载上下文失败")
            return
        }

        // 2. Query 改写
        rewrittenQuery, err := s.rewriteQuery(ctx, req.Content, context.History)
        if err != nil {
            rewrittenQuery = req.Content
        }

        // 3. RAG 检索
        results, err := s.retriever.Retrieve(ctx, &rag.RetrieveRequest{
            Query:     rewrittenQuery,
            UserID:    req.UserID,
            SourceIDs: req.SourceIDs,
            TopK:      5,
        })
        if err != nil {
            sendError(eventCh, "检索失败")
            return
        }

        // 4. 发送 reference 事件
        sendReferences(eventCh, results)

        // 5. LLM 流式生成
        prompt := s.buildPrompt(context, rewrittenQuery, results)
        fullContent := s.streamGenerate(ctx, eventCh, prompt)

        // 6. 保存消息
        s.saveMessages(ctx, req, fullContent, results)

        // 7. 检查摘要
        s.checkAndGenerateSummary(ctx, req.ConversationID, context)

        // 8. 发送 done 事件
        sendDone(eventCh)
    }()

    return eventCh, nil
}

// StopGeneration 终止回答生成
func (s *chatService) StopGeneration(ctx context.Context, conversationID uint) error {
    if cancel, ok := s.cancelFuncs.Load(conversationID); ok {
        cancel.(context.CancelFunc)()
        return nil
    }
    return errors.New("no active request found")
}
```

---

## 6. Repository 设计

### 6.1 ConversationRepository

```go
// internal/repository/conversation_interface.go
package repository

type ConversationRepository interface {
    Create(conv *entity.Conversation) error
    FindByID(id uint) (*entity.Conversation, error)
    FindByNotebookID(notebookID uint) ([]*entity.Conversation, error)
    Update(conv *entity.Conversation) error
    Delete(id uint) error
}
```

### 6.2 MessageRepository

```go
// internal/repository/message_interface.go
package repository

type MessageRepository interface {
    Create(msg *entity.Message) error
    CreateBatch(msgs []*entity.Message) error
    FindByConversationID(conversationID uint) ([]*entity.Message, error)
    GetRecent(conversationID uint, limit int) ([]MessagePair, error)
}
```

---

## 7. Redis 缓存设计

```go
// internal/pkg/redis/chat_cache.go
package redis

type ChatCache struct {
    rdb *redis.Client
}

// GetRecentMessages 获取最近 10 轮消息
func (c *ChatCache) GetRecentMessages(ctx context.Context, conversationID uint) ([]MessagePair, error)

// AddMessage 添加消息到缓存
func (c *ChatCache) AddMessage(ctx context.Context, conversationID uint, userMsg, assistantMsg string) error

// GetSummary 获取对话摘要
func (c *ChatCache) GetSummary(ctx context.Context, conversationID uint) (string, error)

// SetSummary 设置对话摘要
func (c *ChatCache) SetSummary(ctx context.Context, conversationID uint, summary string) error

// AcquireLock 获取并发锁
func (c *ChatCache) AcquireLock(ctx context.Context, conversationID uint) (bool, error)

// ReleaseLock 释放并发锁
func (c *ChatCache) ReleaseLock(ctx context.Context, conversationID uint) error
```

---

## 8. Prompt 设计

### 8.1 系统提示

```markdown
# 角色
你是一个知识问答助手，基于用户提供的资料来源回答问题。

# 回答规范
1. 仅基于提供的资料回答，不要编造信息
2. 如果资料中没有相关信息，明确告知用户
3. 回答要准确、简洁、有条理
4. 使用中文回答

# 引用规范
1. 回答中引用资料时，使用 [1][2] 等标注
2. 每个引用对应一个资料片段
3. 不需要在回答末尾列出引用来源，引用信息由系统自动处理
```

### 8.2 上下文构建模板

```markdown
# 对话摘要
{summary}

# 最近对话历史
用户: {message_1}
助手: {response_1}
用户: {message_2}
助手: {response_2}
...

# 当前问题
{rewritten_query}
```

### 8.3 检索结果注入模板

```markdown
# 参考资料
[1] 资料名称: {source_name}
内容: {chunk_content_1}
相关度: {score_1}

[2] 资料名称: {source_name}
内容: {chunk_content_2}
相关度: {score_2}
```

### 8.4 Query 改写 Prompt

```markdown
# 任务
根据对话历史，将用户的当前问题改写为独立、完整的问题。

# 规则
1. 解决指代消解（"它"、"这个"、"那个"等）
2. 解决省略问题（补充省略的主语、宾语）
3. 保持原意不变
4. 如果当前问题已经是完整的，直接返回原问题

# 对话历史
{conversation_history}

# 当前问题
{current_query}

# 改写后的问题
```

### 8.5 摘要生成 Prompt

```markdown
# 任务
将以下对话历史压缩为结构化摘要，保留关键信息。

# 要求
1. 保留用户的主要问题和关注点
2. 保留助手回答的关键结论
3. 保留提到的重要概念、名词
4. 压缩率：约 10 轮对话压缩为 200-300 字

# 对话历史
{conversation_history}

# 结构化摘要
```

---

## 9. 错误处理与边界情况

### 9.1 错误场景处理

| 场景 | 处理方式 | StreamEvent |
|------|----------|-------------|
| 用户未选中资料来源 | 阻断，不调用 LLM | `{"type": "error", "content": "您没有选中资料，我无法为您做出解答，请先选择资料来源。"}` |
| 资料来源状态异常 | 阻断，提示用户 | `{"type": "error", "content": "选中的资料来源尚未就绪，请稍后再试。"}` |
| 检索结果为空 | 正常调用 LLM | LLM 回答"根据已有资料，我无法找到相关信息" |
| LLM 调用失败 | 重试 1 次 | `{"type": "error", "content": "AI 服务暂时不可用，请稍后再试。"}` |
| LLM 调用超时 | 重试 1 次 | `{"type": "error", "content": "AI 服务响应超时，请稍后再试。"}` |
| Redis 连接失败 | 降级为 MySQL | 无感知 |
| MySQL 写入失败 | 记录日志 | 无感知 |

### 9.2 边界情况处理

**对话轮数**：V1 不设置上限，通过摘要压缩控制上下文长度

**单条消息长度**：限制最大 10000 字符

**并发请求**：使用 Redis 分布式锁，同一对话同一时间只允许一个请求

**终止回答**：通过 context.Cancel 取消 LLM 流式生成，返回已生成的部分内容

### 9.3 降级策略

```
Redis 不可用 → 降级：从 MySQL 读取最近 20 条消息
Query 改写失败 → 降级：使用原始 query
```

---

## 10. 目录结构

```
internal/
├── rag/                              # RAG 模块（已有）
│   └── retriever.go                  # RAGRetriever 接口 + 实现
│
├── service/                          # 业务逻辑层
│   ├── chat_interface.go             # 新增：ChatService 接口
│   ├── chat_service.go               # 新增：ChatService 实现
│   └── chat_types.go                 # 新增：对话相关类型定义
│
├── tool/                             # 工具层（新增）
│   └── chat/
│       ├── tool.go                   # 新增：ChatTool 实现 StreamableTool
│       ├── types.go                  # 新增：Tool 输入输出类型
│       └── prompt.go                 # 新增：Prompt 模板管理
│
├── repository/                       # 数据访问层
│   ├── conversation_interface.go     # 新增
│   ├── conversation_repository.go    # 新增
│   ├── message_interface.go          # 新增
│   └── message_repository.go         # 新增
│
├── model/entity/
│   ├── conversation.go               # 已有，需新增字段
│   └── message.go                    # 已有，需新增字段
│
├── api/v1/chat/
│   ├── controller.go                 # 新增
│   └── routes.go                     # 新增
│
└── pkg/redis/
    └── chat_cache.go                 # 新增
```

---

## 11. 实现计划

### Phase 1：数据层（1-2 天）

- [ ] 修改 entity/conversation.go，新增 Summary、SummaryMsgCount 字段
- [ ] 修改 entity/message.go，新增 Metadata 字段
- [ ] 创建 repository/conversation_interface.go + conversation_repository.go
- [ ] 创建 repository/message_interface.go + message_repository.go
- [ ] 编写单元测试

### Phase 2：缓存层（1 天）

- [ ] 创建 pkg/redis/chat_cache.go
- [ ] 实现 GetRecentMessages / AddMessage
- [ ] 实现 GetSummary / SetSummary
- [ ] 实现 AcquireLock / ReleaseLock

### Phase 3：业务层（2-3 天）

- [ ] 创建 service/chat_interface.go + chat_types.go
- [ ] 创建 service/chat_service.go
- [ ] 实现 loadContext、rewriteQuery、buildPrompt
- [ ] 实现 ProcessMessage 核心流程
- [ ] 实现 StopGeneration
- [ ] 集成 RAG Retriever

### Phase 4：工具层（1 天）

- [ ] 创建 tool/chat/tool.go + types.go + prompt.go
- [ ] 实现 Info() + Stream()

### Phase 5：接口层（1-2 天）

- [ ] 创建 api/v1/chat/controller.go + routes.go
- [ ] 实现对话 CRUD 接口
- [ ] 实现发送消息接口（SSE）
- [ ] 实现终止回答接口

### 依赖关系

```
Phase 1（数据层）→ Phase 2（缓存层）→ Phase 3（业务层）→ Phase 4（工具层）
                                                            ↓
                                                    Phase 5（接口层）
```

---

## 12. 技术依赖

| 组件 | 版本 | 用途 |
|------|------|------|
| github.com/cloudwego/eino | v0.9.4 | StreamableTool 接口、ADK Agent 框架 |
| github.com/redis/go-redis | 已有 | Redis 缓存 |
| gorm.io/gorm | 已有 | MySQL ORM |

---

# 第二部分：Agent 架构设计

---

## 13. Agent 架构概述

### 13.1 技术选型

eino 框架提供两套 Agent 实现：

| 框架 | 包路径 | 特点 |
|------|--------|------|
| Legacy ReAct | `flow/agent/react` | 简单，基于 compose.Graph |
| **ADK (推荐)** | `adk/` | 功能丰富，支持 Checkpoint/Resume、中间件、Runner |

**选择 ADK**，原因：
- 官方推荐的 Agent 开发框架
- 支持 Checkpoint/Resume（中断恢复）
- 内置中间件（上下文压缩、摘要、工具搜索）
- Runner 统一入口，简化使用
- 支持多种预构建 Agent 模式（ReAct、Plan-and-Execute、Supervisor）

### 13.2 Agent 类型选择

使用 `adk.NewChatModelAgent`（ReAct 模式），适合：
- 知识问答场景
- LLM 自主决策工具调用
- 多步推理（检索→分析→回答）

---

## 14. Agent 核心组件设计

### 14.1 ChatAgent

```go
// internal/agent/chat_agent.go
package agent

import (
    "context"
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/model"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
)

// ChatAgentConfig Agent 配置
type ChatAgentConfig struct {
    Model        model.ToolCallingChatModel  // 带 tool calling 的 LLM
    Tools        []tool.Tool                  // 可用工具集
    MaxSteps     int                          // 最大推理步数，默认 10
    SystemPrompt string                       // 系统提示词
}

// ChatAgent 基于 eino ADK 的对话 Agent
type ChatAgent struct {
    runner *adk.Runner
    config *ChatAgentConfig
}

// NewChatAgent 创建 Agent 实例
func NewChatAgent(ctx context.Context, cfg *ChatAgentConfig) (*ChatAgent, error) {
    if cfg.MaxSteps == 0 {
        cfg.MaxSteps = 10
    }

    // 1. 创建 ChatModelAgent（ReAct 循环）
    agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        ToolCallingModel: cfg.Model,
        SystemPrompt:     cfg.SystemPrompt,
        MaxStep:          cfg.MaxSteps,
    })
    if err != nil {
        return nil, err
    }

    // 2. 创建 Runner
    runner, err := adk.NewRunner(ctx, adk.RunnerConfig{
        Agent: agent,
        Tools: cfg.Tools,
    })
    if err != nil {
        return nil, err
    }

    return &ChatAgent{
        runner: runner,
        config: cfg,
    }, nil
}

// Run 执行 Agent，返回流式事件迭代器
func (a *ChatAgent) Run(ctx context.Context, messages []*schema.Message) (*adk.AsyncIterator[*adk.AgentEvent], error) {
    return a.runner.Run(ctx, messages)
}
```

### 14.2 Agent 工具集

#### RAGRetrieverTool — 知识库检索工具

```go
// internal/agent/tools/rag_retriever_tool.go
package tools

import (
    "context"
    "encoding/json"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
    "youdaoNoteLM/internal/rag"
)

// RAGRetrieverTool 知识库检索工具
type RAGRetrieverTool struct {
    retriever rag.RAGRetriever
    userID    uint
    sourceIDs []uint
}

// NewRAGRetrieverTool 创建检索工具
func NewRAGRetrieverTool(retriever rag.RAGRetriever, userID uint, sourceIDs []uint) tool.InvokableTool {
    return &RAGRetrieverTool{
        retriever: retriever,
        userID:    userID,
        sourceIDs: sourceIDs,
    }
}

func (t *RAGRetrieverTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "search_knowledge",
        Desc: "从用户的知识库中检索相关资料。当需要查找文档、笔记、资料内容时使用此工具。",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "query": {
                Type:     schema.String,
                Desc:     "搜索查询词，应该是具体、明确的关键词",
                Required: true,
            },
            "top_k": {
                Type: schema.Integer,
                Desc: "返回结果数量，默认 5，最大 10",
            },
        }),
    }, nil
}

func (t *RAGRetrieverTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    var params struct {
        Query string `json:"query"`
        TopK  int    `json:"top_k"`
    }
    if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
        return "", err
    }
    if params.TopK == 0 || params.TopK > 10 {
        params.TopK = 5
    }

    results, err := t.retriever.Retrieve(ctx, &rag.RetrieveRequest{
        Query:     params.Query,
        UserID:    t.userID,
        SourceIDs: t.sourceIDs,
        TopK:      params.TopK,
    })
    if err != nil {
        return "检索失败: " + err.Error(), nil
    }

    // 格式化检索结果
    return formatRetrievalResults(results), nil
}

func formatRetrievalResults(results []*rag.RetrievalResult) string {
    if len(results) == 0 {
        return "未找到相关资料"
    }

    var output string
    for i, r := range results {
        output += fmt.Sprintf("[%d] 来源: %s\n", i+1, r.SourceName)
        if r.Section != "" {
            output += fmt.Sprintf("章节: %s\n", r.Section)
        }
        output += fmt.Sprintf("内容: %s\n", r.Content)
        output += fmt.Sprintf("相关度: %.2f\n\n", r.Score)
    }
    return output
}
```

#### ChatHistoryTool — 对话历史工具

```go
// internal/agent/tools/chat_history_tool.go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
    "youdaoNoteLM/internal/repository"
    "youdaoNoteLM/pkg/cache"
)

// ChatHistoryTool 对话历史工具
type ChatHistoryTool struct {
    messageRepo repository.MessageRepository
    cache       *cache.ChatCache
}

func NewChatHistoryTool(messageRepo repository.MessageRepository, cache *cache.ChatCache) tool.InvokableTool {
    return &ChatHistoryTool{
        messageRepo: messageRepo,
        cache:       cache,
    }
}

func (t *ChatHistoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "get_chat_history",
        Desc: "获取当前对话的历史消息，用于理解上下文、指代消解（如"它"、"这个"指代什么）。",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "conversation_id": {
                Type:     schema.Integer,
                Desc:     "对话 ID",
                Required: true,
            },
            "limit": {
                Type: schema.Integer,
                Desc: "获取最近 N 轮对话，默认 10",
            },
        }),
    }, nil
}

func (t *ChatHistoryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
    var params struct {
        ConversationID uint `json:"conversation_id"`
        Limit          int  `json:"limit"`
    }
    if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
        return "", err
    }
    if params.Limit == 0 {
        params.Limit = 10
    }

    // 优先从缓存获取
    history, err := t.cache.GetRecentMessages(ctx, params.ConversationID)
    if err != nil || len(history) == 0 {
        // 降级到数据库
        history, err = t.messageRepo.GetRecent(params.ConversationID, params.Limit)
        if err != nil {
            return "获取对话历史失败: " + err.Error(), nil
        }
    }

    if len(history) == 0 {
        return "暂无对话历史", nil
    }

    // 格式化输出
    var output string
    for i, pair := range history {
        output += fmt.Sprintf("第 %d 轮:\n", i+1)
        output += fmt.Sprintf("用户: %s\n", pair.User)
        output += fmt.Sprintf("助手: %s\n\n", pair.Assistant)
    }
    return output, nil
}
```

### 14.3 Agent System Prompt

```go
// internal/agent/prompts/system.go
package prompts

const ChatAgentSystemPrompt = `# 角色
你是一个智能知识问答助手，基于用户提供的资料来源回答问题。

# 能力
你可以使用以下工具：
1. search_knowledge - 从知识库中检索相关资料
2. get_chat_history - 获取对话历史，理解上下文

# 工作流程
1. 分析用户问题，判断是否需要检索资料
2. 如果需要，调用 search_knowledge 检索相关资料
3. 如果问题涉及上下文指代（如"它"、"这个"），调用 get_chat_history 获取历史
4. 基于检索结果和对话历史，生成准确、有条理的回答

# 回答规范
1. 仅基于检索到的资料回答，不要编造信息
2. 如果资料中没有相关信息，明确告知用户
3. 回答要准确、简洁、有条理
4. 使用中文回答
5. 引用资料时使用 [1][2] 等标注

# 注意
- 不要一次调用太多工具，优先用最相关的查询
- 如果第一次检索结果不理想，可以换一个查询词重试
- 对于简单的问候或闲聊，可以直接回答，不需要调用工具
`
```

---

## 15. Agent Service 设计

### 15.1 接口定义

```go
// internal/service/chat_agent_service.go
package service

import (
    "context"
)

// ChatAgentService Agent 对话服务接口
type ChatAgentService interface {
    // ProcessMessageWithAgent 使用 Agent 处理消息
    ProcessMessageWithAgent(ctx context.Context, req *ProcessMessageRequest) (<-chan AgentStreamEvent, error)

    // StopGeneration 终止 Agent 生成
    StopGeneration(ctx context.Context, conversationID uint) error
}

// AgentStreamEvent Agent 流式事件
type AgentStreamEvent struct {
    Type    string      `json:"type"`    // 事件类型
    Content string      `json:"content"` // 事件内容
    Data    interface{} `json:"data,omitempty"` // 附加数据
}

// Agent 事件类型常量
const (
    AgentEventToken       = "token"        // LLM 生成的 token
    AgentEventToolCall    = "tool_call"    // 工具调用开始
    AgentEventToolResult  = "tool_result"  // 工具调用结果
    AgentEventReference   = "reference"    // 检索引用
    AgentEventDone        = "done"         // 生成完成
    AgentEventError       = "error"        // 错误
)
```

### 15.2 实现结构

```go
// internal/service/chat_agent_service.go

type chatAgentService struct {
    // 复用原有依赖
    llmConfigRepo    repository.UserLLMConfigRepository
    retriever        rag.RAGRetriever
    conversationRepo repository.ConversationRepository
    messageRepo      repository.MessageRepository
    cache            *cache.ChatCache
    cancelFuncs      sync.Map
}

func NewChatAgentService(
    llmConfigRepo repository.UserLLMConfigRepository,
    retriever rag.RAGRetriever,
    conversationRepo repository.ConversationRepository,
    messageRepo repository.MessageRepository,
    cache *cache.ChatCache,
) ChatAgentService {
    return &chatAgentService{
        llmConfigRepo:    llmConfigRepo,
        retriever:        retriever,
        conversationRepo: conversationRepo,
        messageRepo:      messageRepo,
        cache:            cache,
    }
}
```

### 15.3 ProcessMessageWithAgent 核心流程

```go
func (s *chatAgentService) ProcessMessageWithAgent(ctx context.Context, req *ProcessMessageRequest) (<-chan AgentStreamEvent, error) {
    ctx, cancel := context.WithCancel(ctx)
    s.cancelFuncs.Store(req.ConversationID, cancel)

    eventCh := make(chan AgentStreamEvent, 64)

    go func() {
        defer close(eventCh)
        defer s.cancelFuncs.Delete(req.ConversationID)

        // 1. 获取用户的 LLM 配置
        llmConfig, err := s.llmConfigRepo.FindDefaultByUserID(req.UserID)
        if err != nil {
            sendAgentError(eventCh, "获取 AI 配置失败")
            return
        }

        // 2. 创建 ToolCallingChatModel
        chatModel, err := llm.NewToolCallingChatModel(ctx, llmConfig)
        if err != nil {
            sendAgentError(eventCh, "创建 AI 模型失败")
            return
        }

        // 3. 准备工具集
        tools := s.buildTools(req.UserID, req.SourceIDs)

        // 4. 创建 Agent
        chatAgent, err := agent.NewChatAgent(ctx, &agent.ChatAgentConfig{
            Model:        chatModel,
            Tools:        tools,
            MaxSteps:     10,
            SystemPrompt: prompts.ChatAgentSystemPrompt,
        })
        if err != nil {
            sendAgentError(eventCh, "创建 Agent 失败")
            return
        }

        // 5. 构建消息（包含历史）
        messages, err := s.buildAgentMessages(ctx, req)
        if err != nil {
            sendAgentError(eventCh, "加载对话历史失败")
            return
        }

        // 6. 运行 Agent
        iter, err := chatAgent.Run(ctx, messages)
        if err != nil {
            sendAgentError(eventCh, "Agent 执行失败")
            return
        }

        // 7. 转发流式事件
        s.forwardAgentEvents(ctx, eventCh, iter, req)

        // 8. 保存消息
        s.saveAgentMessages(ctx, req, eventCh)
    }()

    return eventCh, nil
}

func (s *chatAgentService) buildTools(userID uint, sourceIDs []uint) []tool.Tool {
    return []tool.Tool{
        tools.NewRAGRetrieverTool(s.retriever, userID, sourceIDs),
        tools.NewChatHistoryTool(s.messageRepo, s.cache),
    }
}

func (s *chatAgentService) buildAgentMessages(ctx context.Context, req *ProcessMessageRequest) ([]*schema.Message, error) {
    var messages []*schema.Message

    // 加载历史消息
    history, err := s.cache.GetRecentMessages(ctx, req.ConversationID)
    if err != nil || len(history) == 0 {
        history, err = s.messageRepo.GetRecent(req.ConversationID, 10)
        if err != nil {
            return nil, err
        }
    }

    // 转换为 schema.Message
    for _, pair := range history {
        messages = append(messages, schema.UserMessage(pair.User))
        messages = append(messages, schema.AssistantMessage(pair.Assistant, nil))
    }

    // 添加当前用户消息
    messages = append(messages, schema.UserMessage(req.Content))

    return messages, nil
}
```

---

## 16. LLM 工厂升级

### 16.1 支持 ToolCallingChatModel

```go
// internal/llm/factory.go 新增

// NewToolCallingChatModel 创建支持 Tool Calling 的 ChatModel
// 推荐使用此方法，返回的 ToolCallingChatModel 并发安全
func NewToolCallingChatModel(ctx context.Context, cfg *UserLLMConfig) (model.ToolCallingChatModel, error) {
    // 先创建基础 ChatModel
    chatModel, err := NewChatModel(ctx, cfg)
    if err != nil {
        return nil, err
    }

    // 断言为 ToolCallingChatModel
    tccm, ok := chatModel.(model.ToolCallingChatModel)
    if !ok {
        return nil, fmt.Errorf("model provider '%s' does not support ToolCallingChatModel interface", cfg.Provider)
    }
    return tccm, nil
}
```

### 16.2 eino-ext 模型兼容性

| Provider | 实现包 | ToolCallingChatModel 支持 |
|----------|--------|--------------------------|
| ark/doubao | `eino-ext/components/model/ark` | ✅ 支持 |
| deepseek | `eino-ext/components/model/deepseek` | ✅ 支持 |
| openai | `eino-ext/components/model/openai` | ✅ 支持 |
| qianwen/qwen | 通过 openai 包 | ✅ 支持 |
| anthropic | 自定义适配器 | ⚠️ 需要实现 `WithTools()` 方法 |

---

## 17. Controller 适配

### 17.1 新增 Agent 端点

```go
// internal/api/v1/chat/controller.go 新增

// SendMessageWithAgent 使用 Agent 模式处理消息
func (ctrl *ChatController) SendMessageWithAgent(c *gin.Context) {
    // 1. 参数解析（同 SendMessage）
    convID, err := strconv.ParseUint(c.Param("convId"), 10, 32)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
        return
    }

    var req request.SendMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    userID := middleware.GetUserIDFromCtx(c)

    // 2. 设置 SSE 头
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    // 3. 调用 Agent 服务
    eventCh, err := ctrl.chatAgentService.ProcessMessageWithAgent(c.Request.Context(), &service.ProcessMessageRequest{
        ConversationID: uint(convID),
        Content:        req.Content,
        SourceIDs:      req.SourceIDs,
        UserID:         userID,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // 4. SSE 流式输出
    c.Stream(func(w io.Writer) bool {
        if event, ok := <-eventCh; ok {
            data, _ := json.Marshal(event)
            c.SSEvent("message", string(data))
            return true
        }
        return false
    })
}
```

### 17.2 路由注册

```go
// internal/api/v1/chat/routers.go 新增

func RegisterChatRoutes(r *gin.RouterGroup, ctrl *ChatController, tokenBlacklist service.TokenBlacklist) {
    chat := r.Group("/chat", middleware.Auth(tokenBlacklist))
    {
        // 原有 Pipeline 模式
        chat.POST("/conversations/:convId/messages", ctrl.SendMessage)

        // 新增 Agent 模式
        chat.POST("/conversations/:convId/agent-messages", ctrl.SendMessageWithAgent)

        // 其他路由不变...
    }
}
```

---

## 18. Agent 模式目录结构

```
internal/
├── agent/                              # 新增：Agent 模块
│   ├── chat_agent.go                   # ChatAgent 核心实现
│   ├── prompts/
│   │   └── system.go                   # Agent 系统提示词
│   └── tools/
│       ├── rag_retriever_tool.go       # 知识库检索工具
│       ├── chat_history_tool.go        # 对话历史工具
│       └── format.go                   # 工具输出格式化
│
├── llm/
│   └── factory.go                      # 新增 NewToolCallingChatModel
│
├── service/
│   ├── chat_agent_service.go           # 新增：Agent 服务接口+实现
│   └── chat_service.go                 # 保留：Pipeline 服务
│
├── api/v1/chat/
│   └── controller.go                   # 新增 SendMessageWithAgent
│
└── tool/chat/
    └── tool.go                         # 保留：可被 Agent 作为工具调用
```

---

## 19. Agent 模式实现计划

### Phase A：LLM 工厂升级（0.5 天）

- [ ] 实现 `NewToolCallingChatModel` 方法
- [ ] 验证各 Provider 的 ToolCallingChatModel 支持
- [ ] 处理 Anthropic 适配器的 `WithTools()` 实现

### Phase B：Agent 工具实现（1 天）

- [ ] 创建 `internal/agent/tools/rag_retriever_tool.go`
- [ ] 创建 `internal/agent/tools/chat_history_tool.go`
- [ ] 创建 `internal/agent/tools/format.go`
- [ ] 编写工具单元测试

### Phase C：ChatAgent 核心（1-2 天）

- [ ] 创建 `internal/agent/chat_agent.go`
- [ ] 创建 `internal/agent/prompts/system.go`
- [ ] 实现 `NewChatAgent` 和 `Run` 方法
- [ ] 测试 ReAct 循环

### Phase D：Agent Service（1-2 天）

- [ ] 创建 `internal/service/chat_agent_service.go`
- [ ] 实现 `ProcessMessageWithAgent`
- [ ] 实现 `buildTools` 和 `buildAgentMessages`
- [ ] 实现流式事件转发 `forwardAgentEvents`
- [ ] 实现消息保存 `saveAgentMessages`

### Phase E：接口层（0.5 天）

- [ ] Controller 新增 `SendMessageWithAgent`
- [ ] 注册 Agent 路由
- [ ] 前端适配 Agent 事件格式

### Phase F：测试与调优（1-2 天）

- [ ] 端到端测试
- [ ] System Prompt 调优
- [ ] 工具描述调优
- [ ] 性能测试

### 依赖关系

```
Phase A (LLM 工厂) → Phase B (工具) → Phase C (Agent) → Phase D (Service) → Phase E (接口)
                                                                                  ↓
                                                                          Phase F (测试)
```

---

## 20. 与原有架构的关系

### 20.1 共存策略

```
原有路径（Pipeline 模式）：
  POST /chat/conversations/:convId/messages
  → ChatService.ProcessMessage
  → 固定 pipeline
  → SSE 输出

新增路径（Agent 模式）：
  POST /chat/conversations/:convId/agent-messages
  → ChatAgentService.ProcessMessageWithAgent
  → Agent ReAct 循环
  → SSE 输出
```

### 20.2 迁移路径

1. **Phase 1**：两种模式共存，用户可选择
2. **Phase 2**：收集使用数据，对比效果
3. **Phase 3**：根据效果决定是否将 Agent 作为默认模式
4. **Phase 4**：可选废弃 Pipeline 模式

### 20.3 共享组件

两种模式共享以下组件：
- Repository 层（Conversation、Message）
- Cache 层（Redis 缓存）
- RAG Retriever
- LLM 工厂
- Controller CRUD 接口

---

## 21. 扩展性设计

### 21.1 添加新工具

```go
// 示例：添加网络搜索工具
func (s *chatAgentService) buildTools(userID uint, sourceIDs []uint) []tool.Tool {
    return []tool.Tool{
        tools.NewRAGRetrieverTool(s.retriever, userID, sourceIDs),
        tools.NewChatHistoryTool(s.messageRepo, s.cache),
        eino.NewWebSearchTool(),  // 已有实现
        // tools.NewSummaryTool(),  // 未来扩展
        // tools.NewCodeTool(),     // 未来扩展
    }
}
```

### 21.2 Agent 模式扩展

未来可根据需求扩展更多 Agent 模式：

| 模式 | 适用场景 | eino 支持 |
|------|----------|-----------|
| ReAct (当前) | 简单问答、工具调用 | `adk.NewChatModelAgent` |
| Plan-and-Execute | 复杂任务分解 | `adk/prebuilt/planexecute` |
| Supervisor | 多 Agent 协作 | `adk/prebuilt/supervisor` |
| Deep | 深度思考 | `adk/prebuilt/deep` |

### 21.3 中间件扩展

ADK 支持中间件，可扩展：
- `reduction` - 上下文压缩（处理长对话）
- `summarization` - 自动摘要
- `patchtoolcalls` - 修复格式错误的工具调用
