# 观测手册

本文档描述当前 `ainovel-cli` 的运行时观测入口。目标是排障时先读事实层，再读日志，最后才推测模型行为。

## 1. 先看进度事实

默认小说目录是 `output/novel/`。当前用户配置不暴露 `output_dir` 字段；排障时优先打开：

- `meta/progress.json`
  当前阶段、流程、当前章节、已完成章节、总字数、正在写的章节、章节字数。
- `meta/checkpoints.jsonl`
  每个成功工具步骤的顺序记录。`seq` 必须递增，`scope` 表示全局或章节，`step` 常见值为 `premise`、`outline`、`characters`、`world_rules`、`plan`、`draft`、`consistency_check`、`commit`。
- `meta/run.json`
  本次运行的 provider、model、style、planning tier 和 pending steer 信息。
- `meta/usage.json`
  聚合 token 用量、cache read/write、按 agent 和 model 的拆分。

判断当前是否卡住时，以 `progress.json` 和 `checkpoints.jsonl` 为准，不以 TUI 文案为准。

## 2. 看章节产物

章节写作链路正常时，单章会依次产生：

1. `drafts/NN.plan.json`
2. `drafts/NN.draft.md`
3. `summaries/NN.json`
4. `chapters/NN.md`
5. `meta/checkpoints.jsonl` 中对应 `commit`

如果 `draft` 已存在但没有 `commit`，说明 Writer 还没完成提交，或进程在 `check_consistency` / `commit_chapter` 附近中断。再次启动应通过恢复逻辑继续。

## 3. 看运行日志

TUI 日志在：

- `logs/tui.log`

Headless 日志在：

- `logs/headless.log`

关键日志字段：

- `module=event category=TOOL`
  工具开始或完成。
- `module=event category=DISPATCH`
  Coordinator 或 Host 路由派发给子代理的任务。
- `module=host.flow`
  Host 事实路由的决策结果。
- `module=context`
  上下文窗口占用、压缩策略、重写事件。
- `module=usage`
  用量加载、回放、保存。

日志是解释层，事实工件是权威层。二者不一致时，先用 Store 文件判断当前可恢复状态。

## 4. 看会话与回放

会话记录保存在：

- `meta/sessions/`
- `meta/sessions/agents/`

这些文件用于用量回放和恢复排障。每条 assistant 消息会带模型元信息，运行中切换模型后仍能按历史真实模型统计。

## 5. 看运行队列

运行事件和控制指令保存在：

- `meta/runtime/`
- `meta/runtime/tasks/`

TUI 重放、恢复控制和任务日志从这里读取。Host 自发的 SYSTEM / USER / ERROR 生命周期事件会写入 runtime queue；调用类事件由 observer 持久化完成态和错误态。若界面显示和 Store 事实不一致，先检查 runtime queue 是否落后于 `progress.json`。

## 6. 常见定位路径

### 6.1 章节没有继续写

1. 看 `meta/progress.json` 的 `phase`、`flow`、`in_progress_chapter`。
2. 看 `meta/checkpoints.jsonl` 最后一条是否停在 `plan`、`draft`、`consistency_check` 或 `commit`。
3. 看 `logs/tui.log` 最近的 `category=DISPATCH` 和 `module=host.flow`。
4. 如果最后一个完成章节是弧末，检查是否缺少 arc review、arc summary 或 volume summary。

### 6.2 用量显示不对

1. 看 `meta/usage.json` 的 `schema` 和 `updated_at`。
2. 看日志里是否有 `usage replay` 或 `usage 加载失败`。
3. 看 `meta/sessions/` 是否存在历史消息。

### 6.3 上下文压缩异常

1. 看 `logs/tui.log` 中 `module=context` 的窗口占用。
2. 如果出现上下文重写，确认策略名：`tool_result_microcompact`、`light_trim`、`store_summary`、`full_summary`。
3. Writer 压缩后应能从 Store 恢复当前章节计划、大纲、角色状态和最近摘要；缺失时先检查对应 Store 文件是否存在。

### 6.4 文档与代码不一致

以代码和 Store 事实为准。当前主要实现入口：

- `internal/host/host.go`
- `internal/host/flow/router.go`
- `internal/agents/build.go`
- `internal/agents/context_manager.go`
- `internal/agents/ctxpack/`
- `internal/tools/novel_context.go`
- `internal/store/`
