package termtext

import (
	"strings"
	"testing"
)

func TestPlainNeutralizesTerminalControls(t *testing.T) {
	input := "正常\x1b[31m红色\x1b[0m\r覆盖\f结束\t列\x1b[2J尾\x07"
	got := Plain(input)

	for _, forbidden := range []string{"\x1b", "\r", "\f", "\t", "\x07"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("Plain contains raw terminal control %q: %q", forbidden, got)
		}
	}
	for _, want := range []string{"正常", "红色", "覆盖", `\f`, "结束", "    ", "列", "尾", `\x07`} {
		if !strings.Contains(got, want) {
			t.Fatalf("Plain missing %q: %q", want, got)
		}
	}
}

func TestLineCollapsesSanitizedNewlines(t *testing.T) {
	got := Line("标题\r下一行\n第三行\x1b]8;;https://example.test\a链接\x1b]8;;\a")

	for _, forbidden := range []string{"\x1b", "\r", "\n", "\a"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("Line contains raw terminal control %q: %q", forbidden, got)
		}
	}
	for _, want := range []string{"标题", `\n`, "下一行", "第三行", "链接"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Line missing %q: %q", want, got)
		}
	}
}

func TestWrapSanitizesBeforeWrapping(t *testing.T) {
	lines := Wrap("一二三\x1b[2J四五六\r七八九", 6)
	got := strings.Join(lines, "\n")

	for _, forbidden := range []string{"\x1b", "\r"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("Wrap contains raw terminal control %q: %q", forbidden, got)
		}
	}
	for _, want := range []string{"一二三", "四五六", "七八九"} {
		if !strings.Contains(got, want) {
			t.Fatalf("Wrap missing %q: %q", want, got)
		}
	}
}

func TestTruncateLineSanitizesBeforeTruncating(t *testing.T) {
	got := TruncateLine("abcdef\x1b[2Jghi", 8)

	if strings.Contains(got, "\x1b") {
		t.Fatalf("TruncateLine contains raw escape: %q", got)
	}
	if got != "abcde..." {
		t.Fatalf("TruncateLine = %q, want %q", got, "abcde...")
	}
}
