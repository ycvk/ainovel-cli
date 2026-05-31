package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/voocel/agentcore"
	storepkg "github.com/voocel/ainovel-cli/internal/store"
)

// Dispatcher 在 coordinator 的 subagent 工具返回后计算路由并下达 Host 指令。
type Dispatcher struct {
	coordinator *agentcore.Agent
	store       *storepkg.Store

	enabled atomic.Bool // 由 Host 控制是否派发（启动完成前应关）

	// 去重：记住最近一次派发的 Agent+Task，完全相同时跳过。
	// 主要挡多次 subagent 调用间状态未推进的场景（例：subagent 报错后 coordinator 重派，
	// Router 也会再次派同一章）。
	// 路由消息语义是 append，若不去重会把同一条指令重复压进队列，污染 Coordinator 上下文。
	lastMu   sync.Mutex
	lastSent *Instruction
}

// NewDispatcher 创建 Dispatcher。若构造 Agent 时需要先拿到 middleware，可传 nil
// coordinator，随后用 Bind 回填。
func NewDispatcher(coordinator *agentcore.Agent, store *storepkg.Store) *Dispatcher {
	d := &Dispatcher{coordinator: coordinator, store: store}
	return d
}

// Bind 绑定 Coordinator。BuildCoordinator 需要先构造 Dispatcher 再把它作为
// middleware 注入 Agent，因此 Agent 实例只能在 NewAgent 后回填。
func (d *Dispatcher) Bind(coordinator *agentcore.Agent) {
	d.coordinator = coordinator
}

// Enable 打开路由派发；关闭时 middleware 不会发路由指令。
// Host 在 Start/Resume 完成首条 prompt 之后启用，避免与启动流程冲突。
func (d *Dispatcher) Enable()  { d.enabled.Store(true) }
func (d *Dispatcher) Disable() { d.enabled.Store(false) }

// Middleware 在 subagent 工具执行返回后同步下达下一条 Host 指令。
//
// 不能靠 Subscribe 的异步 EventToolExecEnd 监听：agentcore 的事件消费与主循环解耦，
// 监听器可能晚于下一次 LLM 调用，导致 coordinator 在没有 Host 指令的 tool_result 后
// 自由生成。middleware 在 Execute 返回、ToolExecEnd 事件发出前运行，能保证 Steer 消息
// 进入本轮 tool executor 的 steering queue，并在下一次 LLM 调用前提交进上下文。
func (d *Dispatcher) Middleware() agentcore.ToolMiddleware {
	return func(ctx context.Context, call agentcore.ToolCall, next agentcore.ToolExecuteFunc) (json.RawMessage, error) {
		result, err := next(ctx, call.Args)
		if call.Name == "subagent" && err == nil && ctx.Err() == nil && d.enabled.Load() {
			if dispatchErr := d.Dispatch(); dispatchErr != nil {
				return nil, dispatchErr
			}
		}
		return result, err
	}
}

// Dispatch 立即计算路由并下达指令；可被 Host 在特殊时机（如 Resume 后）主动调用。
func (d *Dispatcher) Dispatch() error {
	if d.store == nil {
		return fmt.Errorf("flow router dispatch: store is not bound")
	}
	if d.coordinator == nil {
		return fmt.Errorf("flow router dispatch: coordinator is not bound")
	}

	state, err := LoadState(d.store)
	if err != nil {
		return fmt.Errorf("flow router load state: %w", err)
	}
	inst := Route(state)
	if inst == nil {
		return nil
	}
	if d.dedupe(inst) {
		slog.Debug("flow router skip duplicate", "module", "host.flow", "agent", inst.Agent, "task", inst.Task)
		return nil
	}

	// Writer 任务：在派发同一刻把章节标为进行中，UI 右侧大纲立即反映"▸ 进行中"，
	// 不用等 plan_chapter 真正执行（plan_chapter 会再调一次 StartChapter，幂等）。
	if inst.Agent == "writer" && inst.Chapter > 0 {
		if err := d.store.Progress.StartChapter(inst.Chapter); err != nil {
			d.forget(inst)
			return fmt.Errorf("flow router pre-mark chapter %d in progress: %w", inst.Chapter, err)
		}
	}

	msg := FormatMessage(inst)
	slog.Debug("flow router dispatch", "module", "host.flow", "agent", inst.Agent, "reason", inst.Reason)
	d.coordinator.Steer(agentcore.UserMsg(msg))
	return nil
}

// dedupe 返回 true 表示本次指令与上次相同，应跳过。
// 用 Agent+Task 相等性（不比 Reason，因为 Reason 是给人看的辅助文本）。
func (d *Dispatcher) dedupe(next *Instruction) bool {
	d.lastMu.Lock()
	defer d.lastMu.Unlock()
	if d.lastSent != nil && d.lastSent.Agent == next.Agent && d.lastSent.Task == next.Task {
		return true
	}
	sent := *next
	d.lastSent = &sent
	return false
}

func (d *Dispatcher) forget(inst *Instruction) {
	d.lastMu.Lock()
	defer d.lastMu.Unlock()
	if d.lastSent != nil && d.lastSent.Agent == inst.Agent && d.lastSent.Task == inst.Task {
		d.lastSent = nil
	}
}

// ResetDedupe 清空去重缓存。Resume / 新 Start 时 Host 调用，确保恢复或新建后能派发首条指令。
func (d *Dispatcher) ResetDedupe() {
	d.lastMu.Lock()
	defer d.lastMu.Unlock()
	d.lastSent = nil
}
