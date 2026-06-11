# 对话模块设计文档

> 版本：V2.0
> 日期：2026-06-10
> 状态：草稿
> 变更：V2.0 将 RAG 检索独立为 service 层，包名 ingestion → rag

---

## 1. 概述

### 1.1 范围

本文档定义 YoudaoNoteLM 项目中对话模块的设计，包括：
- 对话管理（CRUD、列表、标题）
- RAG 检索服务（混合检索、融合、重排序）
- RAG 问答（基于资料来源的问答）

不包含：意图识别触发生成（由其他 Agent 处理）

### 1.2 架构定位

对话模块作为 **子 Agent（ChatAgent）**，由主 Agent（Supervisor）在判断意图后分配任务调用。

RAG 检索作为**独立 service 层**，ChatAgent 通过接口调用，其他 Agent 也可复用。

基于 [eino 框架](https://github.com/cloudwego/eino) 实现。

---

## 2. 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                      主 Agent (Supervisor)               │
│  职责：意图识别、任务分派                                   │
└─────────────┬───────────────────────────────────────────┘
              │ 意图为"对话问答"时调用
              ▼
┌─────────────────────────────────────────────────────────┐
│                   对话 Agent (ChatAgent)                  │
│  职责：Query 改写、上下文管理、流式生成回答                    │
│                                                          │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐               │
│  │ Query   │→ │  RAG    │→ │   LLM   │               │
│  │ Rewrite │   │Retriever│   │ Generate│               │
│  └─────────┘   └────┬────┘   └─────────┘               │
└─────────────────────┼───────────────────────────────────┘
                      │ 调用
                      ▼
┌─────────────────────────────────────────────────────────┐
│              RAG Retriever Service (独立 service)         │
│  职责：混合检索、动态权重融合、Rerank 重排序                  │
│                                                          │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐ │
│  │Semantic │   │Keyword  │   │  RRF    │   │ Rerank  │ │
│  │ Search  │+  │ Search  │→ │  Fuse   │→ │ (豆包)  │ │
│  │(Milvus) │   │(BM25)   │   │         │   │         │ │
│  └─────────┘   └─────────┘   └─────────┘   └─────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 2.1 数据流

1. 用户发送消息 → 主 Agent 识别意图 → 调用对话 Agent
2. 对话 Agent 从 Redis 读取最近 10 轮历史 + 摘要
3. Query Rewrite：结合历史改写用户问题（ChatAgent 内部）
4. 调用 RAG Retriever Service：
   - 语义检索 + 关键词检索（并行）
   - RRF 融合
   - Rerank 重排序，取 Top 5
5. LLM Generate：基于检索结果 + 上下文生成回答（SSE 流式）
6. 保存消息到 MySQL，更新 Redis 缓存

---

## 3. 数据模型

### 3.1 已有表结构（无需创建）

- `conversations` - 会话表
- `messages` - 消息表
- `notebooks` - 笔记本表
- `source` - 资料来源表
- `parent_blocks` - 父块表

### 3.2 需要调整的部分

#### messages 表新增字段

当前 `messages` 表只有 `Role` 和 `Content`，需要新增 `Metadata` 字段存储引用来源：

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

#### 新增 conversation_summaries 表

用于存储超出 10 轮后的对话摘要：

```go
// internal/model/entity/conversation_summary.go
type ConversationSummary struct {
    entity.BaseEntity
    ConversationID uint         `gorm:"uniqueIndex;not null"`
    Conversation   Conversation `gorm:"foreignKey:ConversationID;constraint:OnDelete:CASCADE"`
    Summary        string       `gorm:"type:text;not null"`
    MessageCount   int          `gorm:"not null;default:0"` // 摘要覆盖的消息数量
}
```

### 3.3 Redis 缓存结构

```
# 最近 10 轮对话历史
chat:{conversation_id}:recent_messages → List<Message>

# 结构化摘要
chat:{conversation_id}:summary → string (JSON)
```

### 3.4 Metadata 字段结构（JSON）

```json
{
  "references": [
    {
      "source_id": 123,
      "source_name": "资料名称",
      "parent_block_id": 456,
      "chunk_content": "引用的片段内容",
      "score": 0.92
    }
  ],
  "tokens_used": 1500
}
```

---

## 4. RAG Retriever Service 设计

### 4.1 接口定义

```go
// internal/rag/retriever.go

// RAGRetriever RAG 检索接口
type RAGRetriever interface {
    // Retrieve 执行完整的 RAG 检索流程：混合检索 → 融合 → Rerank
    Retrieve(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error)
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
    Query     string   // 改写后的查询文本
    UserID    uint     // 用户 ID（定位 Milvus collection）
    SourceIDs []uint   // 限定的资料来源范围（为空则不限定）
    TopK      int      // 最终返回数量，默认 5（为 0 时使用默认值）
}

// RetrieveResult 检索结果
type RetrieveResult struct {
    Content       string  // chunk 内容
    SourceID      uint    // 资料来源 ID
    ParentBlockID int64   // 父块 ID
    Score         float32 // 最终相关度分数
    ChunkType     string  // chunk 类型
    Metadata      string  // 元数据 JSON
}
```

### 4.2 内部实现

```go
// internal/rag/retriever.go

type ragRetriever struct {
    milvusClient milvusclient.Client
    embedder     embedding.Embedder
    reranker     *DoubaoReranker
    topK         int // 默认 5
}
```

**检索流程**：

```
RetrieveRequest
    │
    ├── 并行执行 ──┬── semanticSearch()
    │              └── keywordSearch()
    │
    ├── fuse()         ← RRF 融合两路结果
    │
    ├── rerank()       ← 豆包 Rerank 重排序
    │
    └── 取 TopK 返回
```

### 4.3 语义检索

```go
func (r *ragRetriever) semanticSearch(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error)
```

- 将 query 通过 embedder 向量化（复用现有 EmbedderProvider）
- 在用户专属 Milvus collection 中执行向量相似度搜索
- 通过 `source_ids` 过滤检索范围
- 返回 Top 20 候选

### 4.4 关键词检索（BM25）

```go
func (r *ragRetriever) keywordSearch(ctx context.Context, req *RetrieveRequest) ([]*RetrieveResult, error)
```

- 使用 Milvus 全文检索功能（sparse vector + BM25 索引）
- 在用户专属 Milvus collection 中执行全文搜索
- 通过 `source_ids` 过滤检索范围
- 返回 Top 20 候选

### 4.5 RRF 融合

```go
func (r *ragRetriever) fuse(semanticResults, keywordResults []*RetrieveResult) []*RetrieveResult
```

- 使用 Reciprocal Rank Fusion 算法
- 公式：`score = Σ 1/(k + rank_i)`，k 默认 60
- 对去重后的结果按融合分数排序

### 4.6 Rerank 重排序

```go
func (r *ragRetriever) rerank(ctx context.Context, query string, results []*RetrieveResult) ([]*RetrieveResult, error)
```

- 调用豆包 Rerank API
- 接收融合后的 Top 20 候选
- 返回按相关度重排序的结果
- Rerank 失败时降级为融合分数排序（不阻断流程）

### 4.7 Milvus Collection 变更

现有 collection 需要新增 sparse vector 字段以支持全文检索：

```go
// 新增字段
{
    Name:     "sparse_vector",
    DataType: entity.FieldTypeSparseVector,  // BM25 sparse vector
}

// 新增 BM25 索引
idxParam := entity.NewIndexBM25Sparse(func() {
    // BM25 参数
})
```

**变更影响**：
- 现有 collection 需要删除重建（Milvus 不支持给已有 collection 新增字段）
- 现有数据需要重新入库
- ingestion 流程中新增 sparse vector 生成步骤

**调整后的入库流程**：

```
现有：  文本 → embedding(dense) → 写入 Milvus
调整后：文本 → embedding(dense) + BM25分析(sparse) → 写入 Milvus
```

---

## 5. 豆包 Rerank 集成

### 5.1 Reranker 封装

```go
// internal/rag/reranker.go

// DoubaoReranker 豆包 Rerank API 封装
type DoubaoReranker struct {
    apiKey  string
    model   string  // Rerank 模型 ID
    baseURL string
}

// Rerank 对候选结果重排序
func (r *DoubaoReranker) Rerank(ctx context.Context, query string, documents []string) ([]RerankScore, error)

type RerankScore struct {
    Index int     // 原始结果中的索引
    Score float32 // 相关度分数
}
```

### 5.2 设计要点

- 复用 `entity.UserConfig` 中的 Provider/APIKey 配置
- Rerank 接收 query + 候选文档列表，返回按分数排序的索引
- 默认取 Top 20 候选进入 Rerank，最终返回 TopK（5）
- Rerank 失败时降级为融合分数排序（不阻断流程）

### 5.3 配置复用

```go
// 通过现有的 EmbedderProvider 获取用户配置
// Rerank 和 Embedding 共用同一套 APIKey/Provider 配置
type RerankProvider func(ctx context.Context, userID uint) (*DoubaoReranker, error)
```

---

## 6. 对话 Agent 核心流程

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        ChatAgent 处理流程                                │
└─────────────────────────────────────────────────────────────────────────┘

用户消息
    │
    ▼
┌─────────────────┐
│ 1. 加载上下文    │ ← Redis 读取最近 10 轮 + 摘要
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 2. Query 改写    │ ← LLM 根据历史改写用户问题
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 3. RAG 检索      │ ← 调用 RAGRetriever.Retrieve()
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 4. 构建 Prompt   │ ← 系统提示 + 上下文 + 检索结果
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 5. LLM 生成回答  │ ← SSE 流式输出
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 6. 保存消息      │ ← MySQL + 更新 Redis
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 7. 检查轮数      │ ← 超过 10 轮则生成摘要
└─────────────────┘
```

### 6.1 各步骤详细说明

**1. 加载上下文**
- 从 Redis 读取最近 10 轮对话历史
- 从 Redis 或 MySQL 读取结构化摘要（如果有）
- 组装成完整的上下文

**2. Query 改写**
- 使用 LLM 根据对话历史改写用户问题
- 目的：解决指代消解（"它"、"这个"）和省略问题
- 示例：
  - 用户问："它的优点是什么？"
  - 改写后："Transformer 架构的优点是什么？"

**3. RAG 检索**
- 调用 `RAGRetriever.Retrieve()`，传入改写后的 query
- RAG Service 内部完成：混合检索 → 融合 → Rerank
- 返回 Top 5 最相关片段

**4. 构建 Prompt**
- 系统提示：角色定义 + 回答规范 + 引用要求
- 上下文：摘要 + 最近对话历史
- 检索结果：Top 5 片段 + 来源信息

**5. LLM 生成回答**
- SSE 流式输出
- 回答中包含引用标注（如 [1][2]）

**6. 保存消息**
- 将用户消息和 AI 回答保存到 MySQL
- AI 回答的 Metadata 中存储引用来源
- 更新 Redis 缓存

**7. 检查轮数**
- 如果对话超过 10 轮，调用 LLM 生成结构化摘要
- 摘要保存到 MySQL 和 Redis
- 从 Redis 中移除超过 10 轮的旧消息

---

## 7. API 接口设计

### 7.1 对话管理接口

```
POST   /api/v1/notebooks/{notebook_id}/conversations          # 创建对话
GET    /api/v1/notebooks/{notebook_id}/conversations          # 获取对话列表
GET    /api/v1/conversations/{conversation_id}                # 获取对话详情
PUT    /api/v1/conversations/{conversation_id}                # 更新对话（标题）
DELETE /api/v1/conversations/{conversation_id}                # 删除对话
```

### 7.2 消息接口

```
POST   /api/v1/conversations/{conversation_id}/messages       # 发送消息（SSE 流式响应）
GET    /api/v1/conversations/{conversation_id}/messages       # 获取消息历史
POST   /api/v1/conversations/{conversation_id}/messages/{message_id}/stop   # 终止回答
```

### 7.3 请求/响应结构

**发送消息请求**
```json
{
  "content": "这个资料的主要观点是什么？",
  "source_ids": [1, 2, 3]  // 选中的资料来源 ID 列表
}
```

**SSE 流式响应**
```
data: {"type": "token", "content": "根据"}
data: {"type": "token", "content": "资料"}
data: {"type": "token", "content": "显示"}
data: {"type": "reference", "data": [{"source_id": 1, "source_name": "xxx", "chunk_content": "..."}]}
data: {"type": "done", "message_id": 123}
```

**终止响应**
```
data: {"type": "stopped", "message_id": 123, "content": "已生成的内容..."}
```

**消息结构响应**
```json
{
  "id": 123,
  "role": "assistant",
  "content": "根据资料显示...",
  "metadata": {
    "references": [
      {
        "source_id": 1,
        "source_name": "资料名称",
        "parent_block_id": 456,
        "chunk_content": "引用的片段内容",
        "score": 0.92
      }
    ]
  },
  "created_at": "2026-06-04T10:00:00Z"
}
```

### 7.4 无资料处理

当用户未选中任何资料来源时：
- **直接阻断**，不调用 LLM
- 返回固定提示："您没有选中资料，我无法为您做出解答，请先选择资料来源。"

```json
{
  "type": "error",
  "code": "NO_SOURCE_SELECTED",
  "message": "您没有选中资料，我无法为您做出解答，请先选择资料来源。"
}
```

---

## 8. 对话 Agent 与主 Agent 的交互

### 8.1 架构方式

对话 Agent 作为 **Supervisor Agent 的 SubAgent**，而非工具：

```
┌─────────────────────────────────────────────────────────┐
│                   Supervisor Agent                       │
│  职责：意图识别、任务分派                                   │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐ │
│  │ SubAgents                                           │ │
│  │  ├── ChatAgent (对话 Agent)                         │ │
│  │  ├── GenerateAgent (生成 Agent)                     │ │
│  │  └── ...其他 Agent                                  │ │
│  └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 8.2 调用流程

```
用户消息 → Supervisor Agent → 意图识别
                                │
                                ├── 意图 = "对话问答"
                                │   └── 调用 ChatAgent.SubAgent()
                                │
                                ├── 意图 = "生成思维导图"
                                │   └── 调用 GenerateAgent.SubAgent()
                                │
                                └── 其他意图
                                    └── 其他处理
```

### 8.3 ChatAgent 作为 SubAgent

```go
// ChatAgent 实现 eino 的 SubAgent 接口
type ChatAgent struct {
    llm         model.ChatModel
    retriever   RAGRetriever  // 注入 RAGRetriever 接口
    redis       *redis.Client
    mysql       *gorm.DB
}

// Execute 作为 SubAgent 的执行入口
func (a *ChatAgent) Execute(ctx context.Context, input *AgentInput) (*AgentOutput, error) {
    // 1. 加载上下文
    // 2. Query 改写
    // 3. 调用 retriever.Retrieve()
    // 4. LLM 生成（SSE 流式）
    // 5. 保存消息
    // 6. 检查轮数
}
```

### 8.4 流式响应传递

```
ChatAgent (SSE) → Supervisor Agent → 前端 (SSE)
```

Supervisor Agent 透传 ChatAgent 的流式响应，不做干预。

---

## 9. Prompt 设计

### 9.1 系统提示（System Prompt）

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

### 9.2 上下文构建

```markdown
# 对话摘要
{summary}  // 如果有摘要

# 最近对话历史
用户: {message_1}
助手: {response_1}
用户: {message_2}
助手: {response_2}
...

# 当前问题
{rewritten_query}
```

### 9.3 检索结果注入

```markdown
# 参考资料
[1] 资料名称: {source_name}
内容: {chunk_content_1}
相关度: {score_1}

[2] 资料名称: {source_name}
内容: {chunk_content_2}
相关度: {score_2}

[3] 资料名称: {source_name}
内容: {chunk_content_3}
相关度: {score_3}
```

### 9.4 Query 改写 Prompt

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

### 9.5 摘要生成 Prompt

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

## 10. 错误处理与边界情况

### 10.1 错误场景处理

| 场景 | 处理方式 | 返回给用户 |
|------|----------|------------|
| 用户未选中资料来源 | 阻断，不调用 LLM | "您没有选中资料，我无法为您做出解答，请先选择资料来源。" |
| 资料来源状态异常（未就绪） | 阻断，提示用户 | "选中的资料来源尚未就绪，请稍后再试。" |
| 检索结果为空 | 正常调用 LLM | "根据已有资料，我无法找到相关信息。" |
| LLM 调用失败 | 重试 1 次，失败后报错 | "AI 服务暂时不可用，请稍后再试。" |
| LLM 调用超时 | 重试 1 次，超时后报错 | "AI 服务响应超时，请稍后再试。" |
| Redis 连接失败 | 降级为从 MySQL 读取 | 无感知，可能响应稍慢 |
| MySQL 写入失败 | 记录日志，不影响本次回答 | 无感知，但历史记录可能丢失 |
| Rerank 调用失败 | 降级为融合分数排序 | 无感知，结果质量略降 |

### 10.2 边界情况处理

**1. 对话轮数达到上限**
- V1 不设置对话轮数上限
- 通过摘要压缩控制上下文长度

**2. 单条消息过长**
- 限制单条消息最大 10000 字符
- 超出时提示用户"消息内容过长，请精简后重试"

**3. 并发请求**
- 同一对话同一时间只允许一个请求
- 使用 Redis 分布式锁实现
- 并发时返回"对话正在处理中，请稍后再试"

**4. 资料来源被删除**
- 对话历史中保留引用信息，但标记为"已删除"
- 前端展示时显示"该资料已被删除"

---

## 11. 目录结构与文件组织

### 11.1 包重命名

```
internal/ingestion/  →  internal/rag/
```

包名 `package ingestion` → `package rag`

### 11.2 RAG 模块目录结构

```
internal/
├── rag/                              # RAG 模块（原 ingestion）
│   ├── service.go                    # 已有：入库服务
│   ├── embedder_factory.go           # 已有：Embedder 创建
│   ├── milvus_indexer.go             # 已有：Milvus 写入
│   ├── md_parser.go                  # 已有：Markdown 解析
│   ├── parent_transformer.go         # 已有：Parent 块
│   ├── child_transformer.go          # 已有：Child 块
│   ├── semantic_transformer.go       # 已有：语义增强
│   │
│   ├── retriever.go                  # 新增：RAGRetriever 接口 + 实现
│   ├── reranker.go                   # 新增：豆包 Rerank 封装
│   └── retriever_test.go             # 新增：检索测试
│
├── agent/
│   └── chat/                         # 对话 Agent
│       ├── agent.go                  # ChatAgent 主结构
│       ├── query_rewrite.go          # Query 改写
│       ├── prompt.go                 # Prompt 模板
│       └── context_manager.go        # 上下文管理（摘要压缩）
│
├── model/
│   └── entity/
│       ├── conversation.go           # 已有
│       ├── message.go                # 已有，需新增 Metadata 字段
│       └── conversation_summary.go   # 新增：对话摘要表
│
├── repository/
│   ├── conversation_interface.go     # 对话数据访问接口
│   ├── conversation_repository.go    # 对话数据访问实现
│   ├── message_interface.go          # 消息数据访问接口
│   ├── message_repository.go         # 消息数据访问实现
│   ├── conversation_summary_interface.go  # 摘要数据访问接口
│   └── conversation_summary_repository.go # 摘要数据访问实现
│
├── service/
│   ├── chat_interface.go             # 对话业务逻辑接口
│   └── chat_service.go               # 对话业务逻辑实现
│
├── api/
│   └── v1/
│       └── chat/
│           ├── controller.go         # 对话控制器
│           └── routes.go             # 对话路由
│
└── pkg/
    ├── redis/
    │   └── chat_cache.go             # 对话缓存操作
    │
    ├── llm/
    │   └── client.go                 # LLM 客户端（已有）
    │
    └── vector/
        └── milvus.go                 # 向量检索（已有）
```

### 11.3 文件职责说明

| 文件 | 职责 |
|------|------|
| `rag/retriever.go` | RAGRetriever 接口定义、ragRetriever 实现、语义检索、关键词检索、RRF 融合 |
| `rag/reranker.go` | DoubaoReranker 结构体、Rerank API 调用 |
| `agent/chat/agent.go` | ChatAgent 主结构，实现 SubAgent 接口 |
| `agent/chat/query_rewrite.go` | 使用 LLM 改写用户问题 |
| `agent/chat/prompt.go` | Prompt 模板管理 |
| `agent/chat/context_manager.go` | 上下文加载、摘要生成 |
| `model/entity/conversation_summary.go` | 对话摘要表结构 |
| `repository/conversation_interface.go` | 对话数据访问接口定义 |
| `repository/conversation_repository.go` | 对话数据访问实现 |
| `repository/message_interface.go` | 消息数据访问接口定义 |
| `repository/message_repository.go` | 消息数据访问实现 |
| `repository/conversation_summary_interface.go` | 摘要数据访问接口定义 |
| `repository/conversation_summary_repository.go` | 摘要数据访问实现 |
| `service/chat_interface.go` | 对话业务逻辑接口定义 |
| `service/chat_service.go` | 对话业务逻辑实现 |
| `api/v1/chat/controller.go` | HTTP 接口处理 |
| `api/v1/chat/routes.go` | 路由注册 |
| `pkg/redis/chat_cache.go` | Redis 缓存操作 |

---

## 12. 技术选型与依赖

### 12.1 核心依赖

| 组件 | 技术选型 | 用途 |
|------|----------|------|
| Agent 框架 | [eino](https://github.com/cloudwego/eino) | Agent 编排、SubAgent 管理 |
| LLM 调用 | eino 内置 / 自定义封装 | Query 改写、回答生成、摘要生成 |
| 向量数据库 | Milvus | 语义检索 + 全文检索 |
| 关系数据库 | MySQL + GORM | 对话、消息存储 |
| 缓存 | Redis | 最近对话历史缓存 |
| HTTP 框架 | Gin | API 接口 |

### 12.2 检索相关

| 功能 | 技术选型 | 说明 |
|------|----------|------|
| 向量检索 | Milvus | 已有，存储资料来源的向量 |
| 关键词检索 | Milvus 全文检索 | 新增 sparse vector + BM25 索引 |
| 混合检索融合 | RRF (Reciprocal Rank Fusion) | k=60，简单高效的排序融合算法 |
| Rerank | 豆包 Rerank API | 与 Embedding 同平台，统一管理 |

### 12.3 流式输出

| 功能 | 技术选型 | 说明 |
|------|----------|------|
| SSE 实现 | Gin SSE + io.Writer | 流式写入响应 |

---

## 13. 终止回答功能

### 13.1 API 接口

```
POST   /api/v1/conversations/{conversation_id}/messages/{message_id}/stop   # 终止回答
```

### 13.2 实现方式

```go
// 使用 context 控制终止
type ChatAgent struct {
    // ...
    cancelFuncs sync.Map  // conversation_id -> context.CancelFunc
}

// 发送消息时保存 cancelFunc
func (a *ChatAgent) Execute(ctx context.Context, input *AgentInput) (*AgentOutput, error) {
    ctx, cancel := context.WithCancel(ctx)
    a.cancelFuncs.Store(input.ConversationID, cancel)
    defer a.cancelFuncs.Delete(input.ConversationID)

    // ... 处理流程
}

// 终止回答
func (a *ChatAgent) Stop(conversationID uint) error {
    if cancel, ok := a.cancelFuncs.Load(conversationID); ok {
        cancel.(context.CancelFunc)()
        return nil
    }
    return errors.New("no active request found")
}
```

### 13.3 SSE 响应

终止时发送：
```
data: {"type": "stopped", "message_id": 123, "content": "已生成的内容..."}
```

### 13.4 前端处理

1. 用户点击"停止"按钮
2. 调用终止 API
3. 前端收到 `stopped` 事件，显示已生成的部分内容
4. 消息保存到数据库（包含已生成的部分内容）
