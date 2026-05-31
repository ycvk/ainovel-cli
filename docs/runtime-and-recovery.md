# 运行时与恢复说明

本文档描述当前 `ainovel-cli` 的运行时任务流、事实边界和恢复路径。以 live code 为准，不描述未来计划。

## 1. 边界总览

`Host` 是运行时薄外壳，主要职责是：

- 启动、恢复、继续、暂停和关闭生命周期。
- 组装 Coordinator 与子代理。
- 把用户干预注入 Coordinator。
- 发布运行事件到日志、runtime queue 和 UI 投影。
- 读取 Store 生成 TUI snapshot。

`Host` 不做写作调度决策，不做后台空闲续跑。下一步该调哪个子代理，由 `internal/host/flow.Dispatcher` 在明确时机根据 Store 事实计算。

`Store` 是唯一事实层。恢复、导出、诊断和路由都应优先读取 Store 文件，而不是依赖 UI 文案或模型上下文。

## 2. 启动

新建创作入口是 `Host.StartPrepared`：

1. 重置 checkpoint。
2. 初始化 `meta/progress.json`。
3. 发布持久化 SYSTEM 事件。
4. 重置并启用 Flow Dispatcher 去重。
5. 向 Coordinator 注入启动 prompt。
6. 主动执行一次 `Dispatcher.Dispatch()`。

启动后，规划阶段由 Coordinator 选择 Architect；进入 writing 阶段后，Dispatcher 才会按 Store 事实下达明确子代理指令。

## 3. 恢复

恢复入口是 `Host.Resume()`：

1. `buildResumePrompt` 从 Store 事实生成恢复 prompt。
2. 发布持久化恢复事件。
3. 执行 `Store.CheckConsistency()` 并持久化一致性告警。
4. 刷新 `WriterRestorePack`。
5. 启用 Flow Dispatcher。
6. 向 Coordinator 注入恢复 prompt。
7. 主动执行一次 `Dispatcher.Dispatch()`。

恢复依据是 Store：

- `meta/progress.json`
- `meta/checkpoints.jsonl`
- `meta/run.json`
- `chapters/`
- `drafts/`
- `summaries/`
- `outline.json` / `layered_outline.json`
- `characters.json`
- `world_rules.json`
- `reviews/`

上下文摘要和 UI 事件只是恢复辅助，不是事实来源。

## 4. Flow Dispatcher

`internal/host/flow` 分成两个边界：

- `LoadState`：唯一 IO 边界，从 Store 读取路由所需事实。
- `Route`：纯函数，只根据 `State` 返回下一条 `Instruction`。

`LoadState` 读取失败时必须返回错误。不能把损坏 Store 解释成“缺失事实”或“保守默认”，否则 Dispatcher 可能在错误状态上继续派发。

`Route` 的优先级是：

1. `Phase=Complete` 时不派发。
2. 非 writing 阶段交给 Coordinator / Architect 规划。
3. `PendingRewrites` 优先派 Writer 重写或打磨。
4. reviewing / steering 期间不抢占。
5. 分层长篇的弧末评审、弧摘要、卷摘要、扩弧、新卷决策按顺序派发。
6. 常规写作派 Writer 写下一章。

Dispatcher 在派发 Writer 任务时会预先 `StartChapter`，让 UI 和恢复事实立即看到进行中章节。

## 5. Import

`/import` 不经过 Coordinator。它是确定性回放：

1. 切分源文件。
2. 如果 foundation 缺失，先反推并落盘 foundation。
3. 从 `ResumeFrom` 指定章节开始逐章分析。
4. 通过 `CommitChapterTool` 复用章节提交路径。

`ResumeFrom` 只控制章节循环起点，不绕过 foundation 完整性检查。Fresh store 即使传 `from=N`，也必须先补齐 foundation，避免导入完成后 Store 半成品。

## 6. Writer 恢复包

Writer 压缩后恢复由 `WriterRestorePack` 负责：

- Host 在恢复、停机后继续、初始装配时刷新恢复包。
- 刷新会读取 Store 并缓存 `<post-compact-context>`。
- `PostSummaryHook` 本身不做 IO，只注入已缓存内容。

刷新失败必须返回错误并阻断恢复或装配。Hook 若发现缓存超过预算，也必须返回错误。不能用空恢复包伪造成功。

## 7. Runtime Queue 与 UI 投影

运行事件有三个层次：

- `slog`：排障日志。
- `meta/runtime/queue.jsonl`：可回放的持久化运行事件。
- TUI channel：当前进程的 UI 投影。

UI channel 可以因为背压丢投影；runtime queue 不能无声失败。Host 自发的 SYSTEM / USER / ERROR 生命周期事件必须持久化。Observer 对调用类事件只持久化完成态和错误态，避免 replay 时出现“开始一行、完成又一行”的重复。

Stream delta 仍走 backpressure channel，不逐 token 持久化到 runtime queue，避免长文生成把 runtime queue 放大成正文日志。

## 8. 停机与继续

`Host.waitDone()` 不做自动续跑。Coordinator 停机后：

- 如果 `Progress.Phase=Complete`，Host 标记 completed 并发布完成事件。
- 否则 Host 回到 idle，并发布 Coordinator 停止事件。

继续创作有两条路径：

- 当前进程内调用 `Continue`，停机后会注入用户文本并恢复运行。
- 重启进程后调用 `Resume`，从 Store 事实生成恢复 prompt。

用户干预 `Steer` 在运行中直接注入 Coordinator；停机时写入 `meta/run.json` 的 pending steer，并在后续恢复中生效。

## 9. 维护规则

- Store 读写错误必须显式返回，不能退成默认事实。
- UI 投影失败不能影响事实层，但 runtime queue 持久化失败必须可见。
- 新增路由决策时先扩展 `State` 和 `LoadState`，再改 `Route`，保持纯函数可测。
- 新增恢复依据时，必须写入 Store 或 runtime queue，不能只放在模型上下文或 TUI 内存中。
