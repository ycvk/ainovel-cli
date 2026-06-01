package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/entry/startup"
	"github.com/voocel/ainovel-cli/internal/host"
	"github.com/voocel/ainovel-cli/internal/host/imp"
	"github.com/voocel/ainovel-cli/internal/utils"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTextarea()
		m.updateViewportSize()
		m.refreshDetailViewport()
		m.refreshStateViewport()
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	default:
		if next, cmd, handled := m.handleRuntimeMsg(msg); handled {
			return next, cmd
		}
		return m.handleTextareaMsg(msg)
	}
}

func (m *Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, cmd, handled := m.handleOverlayKeyMsg(msg); handled {
		return next, cmd
	}

	if keyString(msg) == keyCtrlC {
		if m.quitPending {
			return m, tea.Quit
		}
		m.quitPending = true
		return m, tea.Tick(time.Second, func(time.Time) tea.Msg { return quitResetMsg{} })
	}
	m.quitPending = false

	if next, cmd, handled := m.handleCommandPaletteKey(msg); handled {
		return next, cmd
	}

	return m.handleBaseKeyMsg(msg)
}

func (m *Model) handleOverlayKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case m.askState != nil:
		return m.handleBlockingModalKey(msg, m.handleAskUserKey)
	case m.cocreate != nil:
		return m.handleBlockingModalKey(msg, m.handleCoCreateKey)
	case m.help != nil:
		return m.handleBlockingModalKey(msg, m.handleHelpKey)
	case m.modelSwitch != nil:
		return m.handleBlockingModalKey(msg, m.handleModelSwitchKey)
	case m.report != nil:
		return m.handleBlockingModalKey(msg, m.handleReportKey)
	case m.importer != nil:
		return m.handleBlockingModalKey(msg, m.handleImportKey)
	default:
		return m, nil, false
	}
}

func (m *Model) handleBlockingModalKey(msg tea.KeyPressMsg, next func(tea.KeyPressMsg) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd, bool) {
	if keyString(msg) == keyCtrlC {
		if m.quitPending {
			return m, tea.Quit, true
		}
		m.quitPending = true
		return m, tea.Tick(time.Second, func(time.Time) tea.Msg { return quitResetMsg{} }), true
	}
	m.quitPending = false
	// 跨模态全局快捷键：modal 打开期间也要能切鼠标上报，否则共创/help/report 等
	// 锁屏式 modal 下用户无法用原生拖拽选中复制。
	if keyString(msg) == keyCtrlR {
		next, cmd := m.toggleMouseReporting()
		return next, cmd, true
	}
	model, cmd := next(msg)
	return model, cmd, true
}

// toggleMouseReporting 切换鼠标上报开关。开 → 关让用户原生拖拽选中复制；
// 关 → 开恢复点击切焦点 / 滚轮。base 路径与 blocking modal 路径共用。
func (m *Model) toggleMouseReporting() (*Model, tea.Cmd) {
	// 欢迎页(modeNew)本就不开鼠标上报，原生拖拽即可复制；此处忽略 Ctrl+R，
	// 避免误开上报反而破坏原生复制。鼠标上报由 enterRunning 在进入工作台时打开。
	if m.mode == modeNew {
		return m, nil
	}
	m.mouseOff = !m.mouseOff
	return m, nil
}

// enterRunning 进入创作工作台：开启鼠标上报（工作台需要点击切面板 / 滚轮 /
// 拖拽侧边栏）。返回的命令需由调用方 Batch 进最终返回值。
func (m *Model) enterRunning() tea.Cmd {
	m.mode = modeRunning
	m.mouseOff = false
	return nil
}

func (m *Model) handleCommandPaletteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	if !m.compActive {
		return m, nil, false
	}

	switch keyString(msg) {
	case keyEsc:
		m.clearCommandPalette()
		return m, nil, true
	case keyUp:
		if m.compIdx > 0 {
			m.compIdx--
		}
		return m, nil, true
	case keyDown:
		if m.compIdx < len(m.compItems)-1 {
			m.compIdx++
		}
		return m, nil, true
	case keyTab:
		m.acceptCommandCompletion()
		return m, nil, true
	case keyEnter:
		item, ok := m.acceptCommandCompletion()
		if !ok {
			return m, nil, true
		}
		if item.AutoExecute {
			m.textarea.Reset()
			next, cmd := m.handleSlashCommand(slashCommand{name: item.Name})
			return next, cmd, true
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *Model) handleBaseKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// 节流防御：粘贴 \n 在不支持 bracketed paste 的终端会退化成连续 KeyEnter；
	// 真人按 Enter 与前一字符间隔通常 > 100ms，<50ms 极可能是粘贴流残片。
	// 只记 Key.Text（字符流）—— 功能键（↑↓/Tab/Ctrl-x）不应污染节流，
	// 否则用户翻历史选定后立刻按 Enter 会被误吞。
	if keyText(msg) != "" {
		m.lastKeyAt = time.Now()
	}
	switch keyString(msg) {
	case keyEsc:
		if m.mode == modeRunning && m.snapshot.IsRunning {
			return m, abortRuntime(m.runtime)
		}
		m.textarea.Reset()
		m.historyIdx = len(m.inputHistory)
		m.historyDraft = ""
		m.refitTextareaHeight()
		m.clearCommandPalette()
		return m, nil
	case keyCtrlL:
		m.resetOutputPanels()
		return m, nil
	case keyCtrlU:
		// 清空当前输入；同时退出历史浏览态。
		m.textarea.Reset()
		m.historyIdx = len(m.inputHistory)
		m.historyDraft = ""
		m.refitTextareaHeight()
		m.clearCommandPalette()
		return m, nil
	case keyCtrlR:
		return m.toggleMouseReporting()
	case keyTab:
		if m.mode == modeNew {
			if m.cocreate != nil {
				return m, nil
			}
			if m.startupMode == startupModeQuick {
				m.startupMode = startupModeCoCreate
			} else {
				m.startupMode = startupModeQuick
			}
			m.textarea.Placeholder = placeholderForNewMode(m.startupMode)
			return m, nil
		}
		m.focusPane = (m.focusPane + 1) % focusPaneCount
		return m, nil
	case keyEnter:
		// Alt+Enter 是主动换行，让 textarea.Update 接管（KeyMap.InsertNewline 已绑到此键）。
		if keyAlt(msg) {
			break
		}
		// 与上一次非 Enter 按键间隔过短 → 视为粘贴流的 \n 残片：
		// 替换为空格保留视觉间隔，与 cleanHumanKeyText 路径语义一致（"abc\ndef" → "abc def"）。
		// 防御 bracketed paste 失效的终端环境（旧 SSH/某些 tmux 配置）。
		if !m.lastKeyAt.IsZero() && time.Since(m.lastKeyAt) < 50*time.Millisecond {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(keyPressText(" "))
			m.refitTextareaHeight()
			return m, cmd
		}
		return m.handleEnterKey()
	case keyUp:
		// 多行输入：让 textarea 接管光标行内移动（落到 switch 后的 textarea.Update）
		if m.textareaIsMultiline() {
			break
		}
		// 单行：优先翻历史，没有可用历史时回退到事件流滚动
		if m.tryHistoryUp() {
			return m, nil
		}
		return m.handleVerticalScrollKey(msg, true)
	case keyDown:
		if m.textareaIsMultiline() {
			break
		}
		if m.tryHistoryDown() {
			return m, nil
		}
		return m.handleVerticalScrollKey(msg, false)
	case keyPgUp:
		return m.handleVerticalScrollKey(msg, true)
	case keyPgDown:
		return m.handleVerticalScrollKey(msg, false)
	case keyEnd:
		switch m.focusPane {
		case focusStream:
			m.streamScroll = true
			m.streamVP.GotoBottom()
		case focusDetail:
			m.detailVP.GotoBottom()
		case focusState:
			m.stateVP.GotoBottom()
		default:
			m.autoScroll = true
			m.viewport.GotoBottom()
		}
		return m, nil
	}

	if keyText(msg) != "" && (containsSGRFragment(keyText(msg)) || isCSILeak(keyRunes(msg))) {
		return m, nil
	}
	var ok bool
	if msg, ok = cleanHumanKeyText(msg); !ok {
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.refitTextareaHeight()
	m.updateCommandPalette()
	return m, cmd
}

func (m *Model) handleEnterKey() (tea.Model, tea.Cmd) {
	text := utils.CleanInputLine(m.textarea.Value())
	if text == "" {
		return m, nil
	}
	m.clearCommandPalette()
	if cmd, ok := parseSlashCommand(text); ok {
		m.pushInputHistory(text)
		m.textarea.Reset()
		m.refitTextareaHeight()
		return m.handleSlashCommand(cmd)
	}

	m.pushInputHistory(text)
	m.textarea.Reset()
	m.refitTextareaHeight()
	switch m.mode {
	case modeNew:
		m.err = nil
		if m.startupMode == startupModeQuick {
			plan, err := startup.PrepareQuick(startup.Request{
				Mode:        startup.ModeQuick,
				UserPrompt:  text,
				OutputDir:   m.runtime.Dir(),
				Interactive: true,
			})
			if err != nil {
				m.err = err
				return m, nil
			}
			return m, startRuntime(m.runtime, plan)
		}
		m.cocreate = newCoCreateState(text)
		return m, m.sendCoCreate()
	case modeRunning:
		// 不本地回显 USER 事件 —— Host.Continue/Steer 入口已 emit "USER" 事件，
		// 走 events channel 回流到 TUI。架构 §2.3：观察层只观察，不产生事实。
		if !m.snapshot.IsRunning {
			return m, continueRuntime(m.runtime, text)
		}
		return m, steerRuntime(m.runtime, text)
	default:
		return m, nil
	}
}

func (m *Model) handleVerticalScrollKey(msg tea.KeyPressMsg, upward bool) (tea.Model, tea.Cmd) {
	return m.scrollPane(m.focusPane, msg, keyScrollDirection(upward))
}

func (m *Model) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	_, pressed := msg.(tea.MouseClickMsg)
	if m.cocreate != nil {
		// 鼠标按 X 坐标分流：屏幕左半 = conv 面板，右半 = prompt 面板。
		// modal 居中且 conv 占左 ~58%，用屏幕中线判别足够准确。
		// 用户在 conv 区滚轮自动停止 follow（让其能稳定停在某个历史位置）。
		var cmd tea.Cmd
		if mouse.X < m.width/2 {
			m.cocreate.convFollow = false
			m.cocreate.convVP, cmd = m.cocreate.convVP.Update(msg)
			if m.cocreate.convVP.AtBottom() {
				m.cocreate.convFollow = true
			}
		} else {
			m.cocreate.promptVP, cmd = m.cocreate.promptVP.Update(msg)
		}
		return m, cmd
	}
	if m.modelSwitch != nil || m.askState != nil {
		return m, nil
	}
	if pane, ok := m.paneAtMouse(mouse.X, mouse.Y); ok {
		m.hoverPane = pane
		m.hoverActive = true
		if pressed {
			m.focusPane = pane
		}
	} else {
		m.hoverActive = false
	}

	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		return m.scrollPaneAtMouse(wheel)
	}
	return m.scrollPane(m.focusPane, msg, scrollDirectionNone)
}

func (m *Model) handleRuntimeMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case eventMsg:
		ev := host.Event(msg)
		m.applyEvent(ev)
		m.refreshEventViewport()
		return m, listenEvents(m.runtime), true
	case bootstrapMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, fetchSnapshot(m.runtime), true
		}
		m.applyRuntimeReplay(msg.replay)
		if msg.resumed && m.mode == modeNew {
			enableMouse := m.enterRunning()
			m.resizeTextarea()
			m.textarea.Placeholder = defaultSteerPlaceholder()
			return m, tea.Batch(fetchSnapshot(m.runtime), enableMouse), true
		}
		return m, fetchSnapshot(m.runtime), true
	case askUserMsg:
		m.askState = newAskUserState(askUserRequest(msg))
		m.textarea.Blur()
		m.applyEvent(host.Event{
			Time: time.Now(), Category: "SYSTEM", Summary: "等待用户补充关键信息", Level: "info",
		})
		m.refreshEventViewport()
		return m, listenAskUser(m.askBridge), true
	case snapshotMsg:
		m.snapshot = host.UISnapshot(msg)
		m.syncRuntimePlaceholder()
		m.refreshEventViewport()
		m.refreshStreamViewport()
		m.refreshDetailViewport()
		m.refreshStateViewport()
		return m, tickSnapshot(m.runtime), true
	case doneMsg:
		m.snapshot.IsRunning = false
		m.refreshEventViewport()
		m.refreshStreamViewport()
		m.refreshStateViewport()
		if msg.complete {
			m.abortPending = false
			m.mode = modeDone
			m.textarea.Placeholder = "创作已完成"
			m.textarea.Blur()
		} else if m.abortPending {
			m.abortPending = false
			m.snapshot.RuntimeState = "paused"
			m.syncRuntimePlaceholder()
		} else {
			m.textarea.Placeholder = "运行中断，输入任意内容恢复创作"
		}
		return m, tea.Batch(fetchSnapshot(m.runtime), listenDone(m.runtime)), true
	case abortResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "ERROR", Summary: msg.err.Error(), Level: "error",
			})
			m.refreshEventViewport()
			return m, fetchSnapshot(m.runtime), true
		}
		if msg.stopped {
			m.abortPending = true
			m.textarea.Placeholder = "正在暂停创作..."
		}
		return m, nil, true
	case reportLoadedMsg:
		if m.report == nil || msg.reqID != m.report.reqID {
			return m, nil, true
		}
		boxW, _ := reportModalSize(m.width, m.height)
		m.report.load(msg.report, paddedModalContentWidth(boxW), msg.finishedAt)
		return m, nil, true
	case importEventMsg:
		if m.importer == nil || msg.reqID != m.importer.reqID {
			return m, nil, true
		}
		boxW, _ := reportModalSize(m.width, m.height)
		m.importer.appendEvent(msg.ev, paddedModalContentWidth(boxW))
		if msg.ev.Stage == imp.StageDone || msg.ev.Stage == imp.StageError {
			return m, nil, true
		}
		return m, listenImportEvent(msg.reqID, msg.ch), true
	case exportDoneMsg:
		if msg.err != nil {
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "ERROR", Summary: "导出失败：" + msg.err.Error(), Level: "error",
			})
		} else if msg.result != nil {
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "SYSTEM", Summary: formatExportSuccess(msg.result), Level: "success",
			})
		}
		m.refreshEventViewport()
		return m, nil, true
	case startResultMsg:
		next, cmd := m.handleStartResultMsg(msg)
		return next, cmd, true
	case cocreateDeltaMsg:
		if m.cocreate == nil || msg.reqID != m.cocreate.reqID {
			return m, nil, true
		}
		m.cocreate.applyDelta(msg.kind, msg.text)
		return m, listenCoCreateDelta(m.cocreate), true
	case cocreateDoneMsg:
		next, cmd := m.handleCoCreateDoneMsg(msg)
		return next, cmd, true
	case steerResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "ERROR", Summary: msg.err.Error(), Level: "error",
			})
			m.refreshEventViewport()
			return m, fetchSnapshot(m.runtime), true
		}
		return m, tea.Batch(fetchSnapshot(m.runtime), listenDone(m.runtime)), true
	case continueResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "ERROR", Summary: msg.err.Error(), Level: "error",
			})
			m.refreshEventViewport()
			return m, tea.Batch(fetchSnapshot(m.runtime), m.textarea.Focus()), true
		}
		m.err = nil
		m.textarea.Placeholder = defaultSteerPlaceholder()
		return m, tea.Batch(fetchSnapshot(m.runtime), listenDone(m.runtime), m.textarea.Focus()), true
	case spinnerTickMsg:
		m.spinnerIdx = (m.spinnerIdx + 1) % len(spinnerFrames)
		if m.snapshot.IsRunning {
			// 星星 / 顶栏 spinner 的视觉刷新都走这里（350ms）
			m.refreshEventViewport()
		}
		return m, tickSpinner(), true
	case toolSpinnerTickMsg:
		m.toolSpinnerIdx = (m.toolSpinnerIdx + 1) % len(toolSpinnerFrames)
		// 事件流"进行中"行的 spinner 刷新（150ms，独立节奏）。
		// spinner 帧只影响 running 事件行，已完成行的渲染输出 byte-for-byte 相同；
		// 没有 running 事件时整个重渲是无意义的，跳过。
		if m.snapshot.IsRunning && m.hasRunningEvent() {
			m.refreshEventViewport()
		}
		return m, tickToolSpinner(), true
	case cursorTickMsg:
		m.cursorIdx++
		if m.snapshot.IsRunning {
			// cursor 闪烁需要全量重渲流式面板（光标位于 content 末尾）；
			// 顺便把 dirty 一并清掉，flush tick 紧跟着不必重复刷。
			m.refreshStreamViewport()
			m.streamDirty = false
		}
		return m, tickCursor(), true
	case streamDeltaMsg:
		m.stream.Append(string(msg))
		// 不立即 refreshStreamViewport；只在有 delta 后排一次 active-only flush tick。
		// LLM 高速流式期每秒数十 token，逐个刷新等于每秒数十次全量重渲。
		m.streamDirty = true
		cmd := listenStream(m.runtime)
		if !m.streamFlushDue {
			m.streamFlushDue = true
			cmd = tea.Batch(cmd, tickStreamFlush())
		}
		return m, cmd, true
	case streamClearMsg:
		// round 边界：先把累积 delta 刷出去，新 round 才能视觉对齐
		if m.flushStreamIfDirty() && m.streamScroll {
			m.streamVP.GotoBottom()
		}
		m.stream.StartRound()
		m.refreshStreamViewport()
		if m.streamScroll {
			m.streamVP.GotoBottom()
		}
		return m, listenStream(m.runtime), true
	case streamFlushTickMsg:
		m.streamFlushDue = false
		if m.flushStreamIfDirty() && m.streamScroll {
			m.streamVP.GotoBottom()
		}
		return m, nil, true
	case quitResetMsg:
		m.quitPending = false
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m *Model) handleStartResultMsg(msg startResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		if m.mode != modeNew {
			m.applyEvent(host.Event{
				Time: time.Now(), Category: "ERROR", Summary: msg.err.Error(), Level: "error",
			})
			m.refreshEventViewport()
		}
		if m.cocreate != nil {
			m.cocreate.awaiting = false
			m.textarea.Placeholder = placeholderForCoCreate(m.cocreate)
			return m, tea.Batch(fetchSnapshot(m.runtime), m.textarea.Focus())
		}
		if m.mode == modeNew {
			m.textarea.Placeholder = placeholderForNewMode(m.startupMode)
			return m, tea.Batch(fetchSnapshot(m.runtime), m.textarea.Focus())
		}
		return m, fetchSnapshot(m.runtime)
	}

	if m.mode == modeNew {
		m.cocreate = nil
		enableMouse := m.enterRunning()
		m.resizeTextarea()
		m.textarea.Placeholder = defaultSteerPlaceholder()
		return m, tea.Batch(fetchSnapshot(m.runtime), m.textarea.Focus(), enableMouse)
	}

	return m, fetchSnapshot(m.runtime)
}

func (m *Model) handleCoCreateDoneMsg(msg cocreateDoneMsg) (tea.Model, tea.Cmd) {
	if m.cocreate == nil || msg.reqID != m.cocreate.reqID {
		return m, nil
	}
	if msg.err != nil {
		m.err = msg.err
		m.cocreate.awaiting = false
		m.textarea.Placeholder = placeholderForCoCreate(m.cocreate)
		return m, m.textarea.Focus()
	}
	m.err = nil
	m.cocreate.apply(msg.reply)
	m.textarea.Placeholder = placeholderForCoCreate(m.cocreate)
	return m, m.textarea.Focus()
}

func (m *Model) handleTextareaMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.refitTextareaHeight()
	m.updateCommandPalette()
	return m, cmd
}

// applyEvent 把一条事件应用到 m.events：
// - 带 ID 且已存在 → 原地更新（合并完成态字段，保留首次的 Time / Summary）
// - 新事件 → 追加，必要时记录到 eventIndex
// - 超过 maxEvents 时做滑动截断并重建索引
func (m *Model) applyEvent(ev host.Event) {
	if ev.ID != "" {
		if idx, ok := m.eventIndex[ev.ID]; ok && idx >= 0 && idx < len(m.events) {
			existing := &m.events[idx]
			if !ev.FinishedAt.IsZero() {
				existing.FinishedAt = ev.FinishedAt
			}
			if ev.Duration > 0 {
				existing.Duration = ev.Duration
			}
			if ev.Failed {
				existing.Failed = true
			}
			if ev.Level != "" {
				existing.Level = ev.Level
			}
			// Summary 非空时允许覆盖（结束态可能带补充信息）；否则保留首次
			if ev.Summary != "" {
				existing.Summary = ev.Summary
			}
			return
		}
	}

	m.events = append(m.events, ev)
	if ev.ID != "" {
		m.eventIndex[ev.ID] = len(m.events) - 1
	}
	if len(m.events) > maxEvents {
		drop := len(m.events) - maxEvents
		m.events = m.events[drop:]
		m.rebuildEventIndex()
	}
}

func (m *Model) rebuildEventIndex() {
	m.eventIndex = make(map[string]int, len(m.events))
	for i, e := range m.events {
		if e.ID != "" {
			m.eventIndex[e.ID] = i
		}
	}
}

func (m *Model) resetOutputPanels() {
	m.events = nil
	m.eventIndex = make(map[string]int)
	m.viewport.SetContent("")
	m.viewport.GotoTop()
	m.stream.Reset()
	m.streamDirty = false
	m.streamFlushDue = false
	m.streamVP.SetContent("")
	m.streamVP.GotoTop()
}

func (m *Model) applyRuntimeReplay(items []domain.RuntimeQueueItem) {
	for _, item := range items {
		switch item.Kind {
		case domain.RuntimeQueueUIEvent:
			// 事件流不做回放：队列里只有完成态事件，且 Agent/Depth/Duration/Level
			// 等渲染所需字段未随 replay 还原，出来的行残缺不齐。宁可空面板也不要半截数据。
			continue
		case domain.RuntimeQueueStreamClear:
			m.stream.StartRound()
		case domain.RuntimeQueueStreamDelta:
			text := host.ReplayDeltaText(item)
			if text == "" {
				continue
			}
			m.stream.Append(text)
		}
	}
	m.stream.Trim(maxStreamRounds)
	m.refreshEventViewport()
	m.refreshStreamViewport()
}
