# 上下文管理说明

本文档描述当前 `ainovel-cli` 的上下文管理实现。它以 live code 为准，不保留旧编排包路径。

## 1. 目标

小说创作上下文管理要解决四类问题：

1. 长篇写作会超过模型上下文窗口。
2. 继续创作需要的是结构化叙事记忆，而不是完整聊天历史。
3. Writer 压缩后不能丢掉章节计划、角色状态、伏笔、时间线、审稿待修项和风格约束。
4. 恢复写作时必须优先依赖持久化 Store，不能假设模型仍记得上次对话。

因此项目采用四层记忆：

- 最近消息：ContextEngine 保留的尾部对话。
- 压缩摘要：`ContextSummary` 和 FullSummary。
- Store 事实：章节、大纲、摘要、角色、世界状态、进度、checkpoint。
- 恢复包：Writer 压缩后注入的 post-compact context。

## 2. 当前代码入口

### 2.1 通用上下文引擎

来自依赖 `github.com/voocel/agentcore/context`，项目侧通过 `internal/agents/context_manager.go` 组装：

- `ToolResultMicrocompact`
- `LightTrim`
- Writer 专用 `StoreSummaryCompact`
- `FullSummary`

`newContextManager` 负责把策略链、窗口、reserve、日志 hook 接到 `corecontext.ContextEngine`。

### 2.2 Agent 装配

入口是 `internal/agents/build.go`。

Coordinator 使用常驻 `ContextEngine`，运行中切换 coordinator/default 模型时由 Host 联动窗口和 reserve。

Writer 的 `ContextManagerFactory` 每次 subagent 调用都会重新创建，窗口随当前 writer 模型变化。Writer 策略链额外挂载 `ctxpack.NewStoreSummaryCompact`，并配置小说定制的 FullSummary prompt 与 `WriterRestorePack` hook。

Architect 和 Editor 使用各自的 stop guard，不挂 Writer 专用 store summary。

### 2.3 Writer Store 压缩

实现位于：

- `internal/agents/ctxpack/strategy.go`
- `internal/agents/ctxpack/builder.go`

`StoreSummaryCompact` 在 token 压力超过阈值时，把旧消息前缀替换为从 Store 重建的结构化摘要。它不调用 LLM，失败时返回显式错误，不能静默伪造成功。

摘要优先包含：

- 当前进度
- 最近章节摘要
- 当前章节计划
- 当前章节大纲
- 弧/卷摘要
- 角色快照
- 活跃伏笔
- 近期时间线
- 风格规则
- 待处理评审问题

### 2.4 Writer 压缩后恢复包

实现位于：

- `internal/agents/ctxpack/restore.go`

`WriterRestorePack` 在 Host 生命周期关键点刷新当前章节所需上下文，并在 FullSummary 后通过 `PostSummaryHook` 注入 `<post-compact-context>`。刷新失败会显式返回错误并阻断恢复；Hook 本身不做 IO，只读内存缓存，避免压缩路径里再引入 Store 读写不确定性。

### 2.5 结构化上下文工具

实现位于：

- `internal/tools/novel_context.go`
- `internal/tools/novel_context_builders.go`
- `internal/domain/runtime.go`

`novel_context` 是正常写作路径的上下文入口。它根据当前 `Progress` 和 `ContextProfile` 读取 Store 中的计划、摘要、角色、伏笔、时间线、风格规则、相关章节推荐等信息。

## 3. 策略顺序

Writer 策略链顺序：

1. `ToolResultMicrocompact`
2. `LightTrim`
3. `StoreSummaryCompact`
4. `FullSummary`

这个顺序有明确代价层级：

- 先清理旧工具结果。
- 再截断超长文本块。
- Store 数据足够时，用结构化事实零 LLM 压缩。
- 最后才调用 LLM 做 FullSummary。

Coordinator 策略链没有 StoreSummaryCompact，只使用通用策略和 FullSummary。

## 4. Token 预算

窗口来源由 `bootstrap.Config.ResolveContextWindow` 决定：

1. 模型 registry 中的真实窗口。
2. `compact_window` 上限，取 `min(模型真窗口, compact_window)`。
3. 未识别模型使用默认窗口。

reserve 由 `bootstrap.CompactReserveTokens(window)` 计算：

- 默认压缩阈值是窗口的 85%。
- reserve 为窗口的 15%，但最小 8000 token。
- 小窗口模型不会因为 reserve 过小而出现“刚压完又超窗”。

当前 Writer 保留最近 20000 token，Coordinator 保留最近 30000 token。

## 5. 恢复关系

恢复时 Host 读取 Store 事实生成 resume prompt，并刷新 WriterRestorePack。真正的恢复依据是：

- `meta/progress.json`
- `meta/checkpoints.jsonl`
- `meta/run.json`
- drafts / chapters / summaries
- outline / layered outline / compass
- characters / world state / cast ledger

上下文摘要只是运行时优化，不是唯一真相。任何恢复判断都应优先看 Store。

## 6. 可观测性

上下文相关日志通过 `internal/agents/context_manager.go` 的 project/recover hook 输出，关键字段包括：

- `module=context`
- `agent`
- `reason`
- `strategy`
- `tokens_before`
- `tokens_after`
- `msgs_before`
- `msgs_after`
- `compacted`
- `kept`

更完整的排障路径见 `docs/observability.md`。
运行时生命周期与恢复边界见 `docs/runtime-and-recovery.md`。

## 7. 维护规则

- 不再引用旧编排包路径；当前实现归属 `internal/agents`、`internal/host`、`internal/tools` 和 `internal/store`。
- 文档必须描述已经存在的实现，未来计划放在独立 proposal 文档中。
- 新增上下文策略必须有明确代价顺序、触发条件、失败行为和测试。
- 不允许为了“继续跑”而吞掉压缩错误或伪造摘要；错误必须暴露给调用方。
