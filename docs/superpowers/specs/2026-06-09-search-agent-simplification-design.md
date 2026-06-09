# 搜索 Agent 简化设计

## 背景

当前搜索 Agent 有 4 个工具：`web_search`、`analyze_results`、`refine_query`、`import_urls`。其中 `analyze_results` 和 `refine_query` 在工具内部调用 LLM，导致：

1. **死锁**：工具内部调 `chatModel.Generate()`，与 Agent 框架的模型调用冲突
2. **延迟叠加**：每次工具内 LLM 调用额外 30s+ 网络往返
3. **上下文丢失**：工具内的独立 LLM 调用看不到完整对话历史
4. **过度工程**：Agent 本身有完整上下文，完全能自行分析和改良

## 设计决策

- **方案**：纯 Prompt 驱动 — 去掉 `analyze_results` 和 `refine_query`，分析和改良逻辑由 Agent 自行完成
- **搜索轮数**：固定 2 轮
- **导入方式**：保留 `import_urls` 工具

## 工具清单

| 工具 | 职责 | 状态 |
|------|------|------|
| `web_search` | 调用搜索引擎 API，返回搜索结果列表 | 保留 |
| `analyze_results` | 内部调 LLM 对结果评分排序 | 删除 |
| `refine_query` | 内部调 LLM 优化关键词 | 删除 |
| `import_urls` | 批量导入 URL 到资料库 | 保留 |

## 执行流程

```
用户输入
  ↓
第 1 轮：web_search（用原始关键词）
  ↓
Agent 自行分析结果、优化关键词（prompt 驱动，无工具调用）
  ↓
第 2 轮：web_search（用改良后的关键词）
  ↓
Agent 输出带评分的 JSON 结果
  ↓（可选）
用户确认 → import_urls 导入
```

## 文件变更

### 1. `internal/agent/search/tools.go`

- 删除 `AnalyzeResultsInput`、`AnalyzeRankedItem`、`AnalyzeResultsOutput` 结构体
- 删除 `NewAnalyzeResultsTool` 函数
- 删除 `RefineQueryInput`、`RefineQueryOutput` 结构体
- 删除 `NewRefineQueryTool` 函数
- 保留 `NewWebSearchTool` 和 `NewImportURLsTool`
- 移除 `external` 包导入（不再需要独立 LLMClient）

### 2. `internal/agent/search/prompts.go`

重写 `SearchSystemPrompt`，核心要点：

- 明确固定 2 轮搜索流程
- 第 1 轮搜索后，Agent **自己**分析结果质量（不调用工具）
- Agent **自己**想出 1-2 个改良关键词
- 第 2 轮搜索后，立即输出最终 JSON
- 如果用户指定 URL，直接调 `import_urls`

### 3. `internal/agent/search/agent.go`

- `createTools` 方法：只创建 `web_search` 和 `import_urls` 两个工具，移除 `chatModel` 参数
- `maxAgentRounds`：调整为 2
- `MaxIterations`：调整为 4（2 轮搜索 + 最终回复 + 余量）
- `Execute` 和 `ExecuteStream`：移除 `chatModel` 传递给 `createTools` 的逻辑

## 不变的部分

- `web_search` 工具实现不变
- `import_urls` 工具实现不变
- `context.go`（UserID/NotebookID 注入）不变
- Agent 的 Eino 框架集成方式不变
- 流式/非流式执行模式不变

## 预期效果

- 消除死锁风险（工具内不再调 LLM）
- 延迟降低 ~60s（去掉 2 次工具内 LLM 调用）
- 代码量减少 ~100 行
- Agent 行为更可预测（固定 2 轮）
