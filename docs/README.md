# 文档索引

本目录用于存放项目内部设计、运行机制与排障文档。

## 当前文档

- [上下文管理说明](./context-management.md)
  说明项目当前的上下文管理体系，包括：
  - `agentcore` 上下文策略管线
  - Writer 的 `store_summary` 压缩
  - `writerRestorePack`
  - `novel_context`
  - `ContextProfile` / `MemoryPolicy`
  - handoff / recovery
  - 可观测性与排障入口

- [观测手册](./observability.md)
  跑长篇时的实操排障手册：每个事实工件该看什么、运行日志如何读、上下文压缩和恢复状态如何定位。
  适合"跑到第 N 章发现奇怪现象"时打开。

- [运行时与恢复说明](./runtime-and-recovery.md)
  说明 Host 生命周期、Flow Dispatcher、Import 回放、WriterRestorePack、runtime queue 与 UI 投影的当前边界。

## 建议后续补充

- `writing-pipeline.md`
  聚焦 Architect / Writer / Editor 三个角色的协作流程。

- `diagnostics.md`
  聚焦 `diag` 规则体系、证据来源和常见问题定位方式。
