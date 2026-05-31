package tui

import (
	"regexp"
	"testing"
)

var ansiBackgroundPattern = regexp.MustCompile(`\x1b\[[0-9;:]*(?:4[0-7]|10[0-7]|48)(?:[;:0-9]*)m`)

func TestTextareaViewDoesNotPaintInputLineBackground(t *testing.T) {
	m := NewModel(nil, nil)

	got := m.textarea.View()
	if ansiBackgroundPattern.MatchString(got) {
		t.Fatalf("textarea view contains background color escape: %q", got)
	}
}
