package tui

import tea "charm.land/bubbletea/v2"

type scrollDirection int

const (
	scrollDirectionNone scrollDirection = iota
	scrollDirectionUp
	scrollDirectionDown
)

func mouseWheelDirection(msg tea.MouseWheelMsg) scrollDirection {
	if msg.Mod.Contains(tea.ModShift) {
		return scrollDirectionNone
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		return scrollDirectionUp
	case tea.MouseWheelDown:
		return scrollDirectionDown
	default:
		return scrollDirectionNone
	}
}

func keyScrollDirection(upward bool) scrollDirection {
	if upward {
		return scrollDirectionUp
	}
	return scrollDirectionDown
}

func (m *Model) scrollPane(pane focusPane, msg tea.Msg, direction scrollDirection) (*Model, tea.Cmd) {
	switch pane {
	case focusStream:
		if direction == scrollDirectionUp {
			m.streamScroll = false
		}
		var cmd tea.Cmd
		m.streamVP, cmd = m.streamVP.Update(msg)
		if direction == scrollDirectionDown && m.streamVP.AtBottom() {
			m.streamScroll = true
		}
		return m, cmd
	case focusDetail:
		var cmd tea.Cmd
		m.detailVP, cmd = m.detailVP.Update(msg)
		return m, cmd
	case focusState:
		var cmd tea.Cmd
		m.stateVP, cmd = m.stateVP.Update(msg)
		return m, cmd
	default:
		if direction == scrollDirectionUp {
			m.autoScroll = false
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		if direction == scrollDirectionDown && m.viewport.AtBottom() {
			m.autoScroll = true
		}
		return m, cmd
	}
}

func (m *Model) scrollPaneAtMouse(msg tea.MouseWheelMsg) (*Model, tea.Cmd) {
	mouse := msg.Mouse()
	pane, ok := m.paneAtMouse(mouse.X, mouse.Y)
	if !ok {
		return m, nil
	}
	return m.scrollPane(pane, msg, mouseWheelDirection(msg))
}
