package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type helpState struct {
	viewport viewport.Model
}

func newHelpState(width, height int) *helpState {
	boxW, boxH := reportModalSize(width, height)
	contentW := paddedModalContentWidth(boxW)
	text := renderHelpText(contentW)

	vp := newViewport(contentW, boxH-4)
	vp.SetContent(text)
	return &helpState{viewport: vp}
}

func renderHelpText(width int) string {
	titleStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true)
	usageStyle := lipgloss.NewStyle().Foreground(colorMuted)
	descStyle := lipgloss.NewStyle().Foreground(bodyTextColor)
	hintStyle := lipgloss.NewStyle().Foreground(colorDim)

	var b strings.Builder
	b.WriteString(titleStyle.Render("命令帮助"))
	b.WriteString("\n\n")

	for i, spec := range commandSpecs() {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(nameStyle.Render("/" + spec.Name))
		if len(spec.Aliases) > 0 {
			b.WriteString(usageStyle.Render("  alias: /" + strings.Join(spec.Aliases, " /")))
		}
		b.WriteString("\n")
		b.WriteString(usageStyle.Render("Usage: " + spec.Usage))
		b.WriteString("\n")
		b.WriteString(descStyle.Render(wrapDisplayText(spec.Description, width)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("快捷键"))
	b.WriteString("\n\n")
	for _, line := range []string{
		"输入 / 搜索命令",
		"↑↓ 选择命令候选",
		"Tab/Enter 接受补全",
		"Esc 关闭当前命令面板",
		"Ctrl+R 切换选中复制模式（关闭鼠标上报后可拖拽选中复制，再按一次恢复）",
	} {
		b.WriteString(hintStyle.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}

func renderHelpModal(width, height int, state *helpState) string {
	if state == nil {
		return ""
	}

	boxW, boxH := reportModalSize(width, height)
	contentW := paddedModalContentWidth(boxW)

	if state.viewport.Width() != contentW {
		state.viewport.SetWidth(contentW)
	}
	if state.viewport.Height() != boxH-4 {
		state.viewport.SetHeight(boxH - 4)
	}

	modal := renderPaddedModalFrame(
		boxW,
		boxH,
		"命令帮助",
		"  ↑↓ 滚动 · Esc 关闭",
		strings.Split(state.viewport.View(), "\n"),
	)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.help == nil {
		return m, nil
	}
	switch keyString(msg) {
	case keyEsc:
		m.help = nil
		if m.mode != modeDone {
			return m, m.textarea.Focus()
		}
		return m, nil
	case keyUp:
		m.help.viewport.ScrollUp(1)
		return m, nil
	case keyDown:
		m.help.viewport.ScrollDown(1)
		return m, nil
	case keyPgUp:
		m.help.viewport.HalfPageUp()
		return m, nil
	case keyPgDown:
		m.help.viewport.HalfPageDown()
		return m, nil
	default:
		return m, nil
	}
}
