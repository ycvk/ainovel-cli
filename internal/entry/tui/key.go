package tui

import tea "charm.land/bubbletea/v2"

const (
	keyBackspace = "backspace"
	keyCtrlC     = "ctrl+c"
	keyCtrlH     = "ctrl+h"
	keyCtrlL     = "ctrl+l"
	keyCtrlR     = "ctrl+r"
	keyCtrlS     = "ctrl+s"
	keyCtrlU     = "ctrl+u"
	keyDown      = "down"
	keyEnd       = "end"
	keyEnter     = "enter"
	keyEsc       = "esc"
	keyHome      = "home"
	keyLeft      = "left"
	keyPgDown    = "pgdown"
	keyPgUp      = "pgup"
	keyRight     = "right"
	keyShiftTab  = "shift+tab"
	keySpace     = "space"
	keyTab       = "tab"
	keyUp        = "up"
)

func keyString(msg tea.KeyPressMsg) string {
	return msg.Keystroke()
}

func keyText(msg tea.KeyPressMsg) string {
	return msg.Key().Text
}

func keyRunes(msg tea.KeyPressMsg) []rune {
	return []rune(keyText(msg))
}

func keyAlt(msg tea.KeyPressMsg) bool {
	return msg.Key().Mod.Contains(tea.ModAlt)
}

func keyWithText(msg tea.KeyPressMsg, text string) tea.KeyPressMsg {
	key := msg.Key()
	key.Text = text
	key.Code = 0
	if runes := []rune(text); len(runes) == 1 {
		key.Code = runes[0]
	}
	return tea.KeyPressMsg(key)
}

func keyPressText(text string) tea.KeyPressMsg {
	runes := []rune(text)
	code := rune(0)
	if len(runes) == 1 {
		code = runes[0]
	}
	return tea.KeyPressMsg(tea.Key{Text: text, Code: code})
}
