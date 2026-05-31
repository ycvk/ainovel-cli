package termtext

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const tabSpaces = "    "

// StripSequences removes terminal escape sequences while leaving ordinary
// control runes untouched for callers that must preserve in-band separators.
func StripSequences(s string) string {
	return ansi.Strip(strings.ReplaceAll(s, "\r\n", "\n"))
}

// Plain returns text that is safe to write to a terminal while preserving raw
// persistence elsewhere. It strips ANSI escape sequences and makes control
// characters visible instead of executable.
func Plain(s string) string {
	s = StripSequences(s)
	if s == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			b.WriteRune('\n')
		case r == '\r':
			b.WriteRune('\n')
		case r == '\t':
			b.WriteString(tabSpaces)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r >= 0x80 && r <= 0x9f:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Line returns a single terminal-safe display line.
func Line(s string) string {
	return strings.ReplaceAll(Plain(s), "\n", `\n`)
}

// Wrap sanitizes text, then wraps it to maxWidth terminal cells.
func Wrap(s string, maxWidth int) []string {
	s = Plain(s)
	if maxWidth <= 0 {
		return []string{s}
	}

	var out []string
	for _, raw := range strings.Split(s, "\n") {
		if raw == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrapLine(raw, maxWidth)...)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// WrapString is Wrap joined with newlines.
func WrapString(s string, maxWidth int) string {
	return strings.Join(Wrap(s, maxWidth), "\n")
}

func wrapLine(s string, maxWidth int) []string {
	if ansi.StringWidth(s) <= maxWidth {
		return []string{s}
	}

	var lines []string
	var current strings.Builder
	currentWidth := 0
	for _, r := range s {
		rw := ansi.StringWidth(string(r))
		if currentWidth > 0 && currentWidth+rw > maxWidth {
			lines = append(lines, strings.TrimRight(current.String(), " "))
			current.Reset()
			currentWidth = 0
			if r == ' ' {
				continue
			}
		}
		current.WriteRune(r)
		currentWidth += rw
	}
	if current.Len() > 0 {
		lines = append(lines, strings.TrimRight(current.String(), " "))
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func TruncateLine(s string, maxRunes int) string {
	s = Line(s)
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes < 4 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func TruncateWidth(s string, maxWidth int) string {
	s = Line(s)
	if ansi.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 0 {
		return ""
	}

	var b strings.Builder
	currentWidth := 0
	for _, r := range s {
		rw := ansi.StringWidth(string(r))
		if currentWidth+rw > maxWidth {
			break
		}
		b.WriteRune(r)
		currentWidth += rw
	}
	return b.String()
}
