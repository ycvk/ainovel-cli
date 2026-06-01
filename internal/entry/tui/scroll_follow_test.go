package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMouseWheelUpInStreamPanePausesFollow(t *testing.T) {
	m := newScrollTestModel(t)
	m.focusPane = focusStream
	m.streamVP.GotoBottom()

	bottom := m.streamVP.YOffset()
	next := updateModel(t, m, mouseWheelMsg(m, focusStream, tea.MouseWheelUp))

	if next.streamScroll {
		t.Fatalf("streamScroll = true after wheel up, want false")
	}
	if got := next.streamVP.YOffset(); got >= bottom {
		t.Fatalf("stream viewport offset = %d, want less than previous bottom %d", got, bottom)
	}
}

func TestStreamFlushDoesNotForceBottomAfterMouseWheelUp(t *testing.T) {
	m := newScrollTestModel(t)
	m.focusPane = focusStream
	m.streamVP.GotoBottom()

	next := updateModel(t, m, mouseWheelMsg(m, focusStream, tea.MouseWheelUp))
	scrolledOffset := next.streamVP.YOffset()

	nextModel, _, handled := next.handleRuntimeMsg(streamDeltaMsg("\nnew streamed line"))
	if !handled {
		t.Fatalf("stream delta was not handled")
	}
	next = nextModel.(*Model)

	nextModel, _, handled = next.handleRuntimeMsg(streamFlushTickMsg{})
	if !handled {
		t.Fatalf("stream flush tick was not handled")
	}
	next = nextModel.(*Model)

	if next.streamScroll {
		t.Fatalf("streamScroll = true after flush, want false")
	}
	if got := next.streamVP.YOffset(); got != scrolledOffset {
		t.Fatalf("stream viewport offset = %d after flush, want preserved offset %d", got, scrolledOffset)
	}
	if next.streamVP.AtBottom() {
		t.Fatalf("stream viewport returned to bottom after flush")
	}
}

func TestMouseWheelDownToStreamBottomResumesFollow(t *testing.T) {
	m := newScrollTestModel(t)
	m.focusPane = focusStream
	m.streamVP.GotoBottom()
	bottom := m.streamVP.YOffset()
	m.streamVP.SetYOffset(bottom - 1)
	m.streamScroll = false

	next := updateModel(t, m, mouseWheelMsg(m, focusStream, tea.MouseWheelDown))

	if !next.streamScroll {
		t.Fatalf("streamScroll = false after wheel down to bottom, want true")
	}
	if !next.streamVP.AtBottom() {
		t.Fatalf("stream viewport is not at bottom after wheel down")
	}
}

func TestMouseWheelUpInEventsPanePausesAutoScroll(t *testing.T) {
	m := newScrollTestModel(t)
	m.focusPane = focusEvents
	m.viewport.GotoBottom()

	bottom := m.viewport.YOffset()
	next := updateModel(t, m, mouseWheelMsg(m, focusEvents, tea.MouseWheelUp))

	if next.autoScroll {
		t.Fatalf("autoScroll = true after wheel up, want false")
	}
	if got := next.viewport.YOffset(); got >= bottom {
		t.Fatalf("events viewport offset = %d, want less than previous bottom %d", got, bottom)
	}
}

func TestMouseWheelUsesPaneUnderPointerInsteadOfFocusedPane(t *testing.T) {
	m := newScrollTestModel(t)
	m.focusPane = focusEvents
	m.viewport.GotoBottom()
	m.streamVP.GotoBottom()

	eventOffset := m.viewport.YOffset()
	streamBottom := m.streamVP.YOffset()
	next := updateModel(t, m, mouseWheelMsg(m, focusStream, tea.MouseWheelUp))

	if got := next.viewport.YOffset(); got != eventOffset {
		t.Fatalf("events viewport offset = %d, want unchanged %d", got, eventOffset)
	}
	if got := next.streamVP.YOffset(); got >= streamBottom {
		t.Fatalf("stream viewport offset = %d, want less than previous bottom %d", got, streamBottom)
	}
	if next.streamScroll {
		t.Fatalf("streamScroll = true after stream wheel up with events focused, want false")
	}
}

func newScrollTestModel(t *testing.T) *Model {
	t.Helper()

	m := NewModel(nil, nil)
	m.mode = modeRunning
	m.width = 120
	m.height = 40
	m.updateViewportSize()

	m.autoScroll = true
	m.streamScroll = true
	m.viewport.SetContent(repeatedLines("event", 80))
	m.stream.Append(repeatedLines("stream", 80))
	m.refreshStreamViewport()
	m.detailVP.SetContent(repeatedLines("detail", 40))
	m.stateVP.SetContent(repeatedLines("state", 40))

	m.viewport.GotoBottom()
	m.streamVP.GotoBottom()
	m.detailVP.GotoBottom()
	m.stateVP.GotoBottom()
	if !m.viewport.AtBottom() || !m.streamVP.AtBottom() {
		t.Fatalf("test setup did not put scrollable panes at bottom")
	}
	return m
}

func updateModel(t *testing.T, m *Model, msg tea.Msg) *Model {
	t.Helper()

	nextModel, _ := m.Update(msg)
	next, ok := nextModel.(*Model)
	if !ok {
		t.Fatalf("updated model has type %T, want *tui.Model", nextModel)
	}
	return next
}

func mouseWheelMsg(m *Model, pane focusPane, button tea.MouseButton) tea.MouseWheelMsg {
	x, y := panePoint(m, pane)
	return tea.MouseWheelMsg(tea.Mouse{
		X:      x,
		Y:      y,
		Button: button,
	})
}

func panePoint(m *Model, pane focusPane) (int, int) {
	topH, _, bodyH := m.layoutHeights()
	leftW := m.sidebarWidth()
	rightW := m.detailWidth()
	centerStartX := leftW
	centerEndX := m.width - rightW
	eventH, _ := m.splitHeights(bodyH)

	switch pane {
	case focusStream:
		return centerStartX + (centerEndX-centerStartX)/2, topH + eventH + 1
	case focusDetail:
		return centerEndX + rightW/2, topH + bodyH/2
	case focusState:
		return leftW / 2, topH + bodyH/2
	default:
		return centerStartX + (centerEndX-centerStartX)/2, topH + eventH/2
	}
}

func repeatedLines(prefix string, count int) string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = prefix + " line"
	}
	return strings.Join(lines, "\n")
}
