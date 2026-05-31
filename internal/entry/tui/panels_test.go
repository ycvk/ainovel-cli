package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/entry/termtext"
	"github.com/voocel/ainovel-cli/internal/host"
)

func assertNoRawUserControls(t *testing.T, label, rendered string, forbidden []string) {
	t.Helper()
	plain := termtext.StripSequences(rendered)
	for _, seq := range forbidden {
		if strings.Contains(plain, seq) {
			t.Fatalf("%s contains raw user terminal control %q: %q", label, seq, rendered)
		}
	}
}

func assertRenderedContains(t *testing.T, label, rendered string, wants []string) {
	t.Helper()
	plain := termtext.StripSequences(rendered)
	for _, want := range wants {
		if !strings.Contains(plain, want) {
			t.Fatalf("%s missing %q: %q", label, want, rendered)
		}
	}
}

func TestWrapStreamTextSanitizesTerminalControls(t *testing.T) {
	lines := wrapStreamText("正常\x1b[31m红色\x1b[0m\r覆盖\f结束\t列\x1b[2J尾", 80)
	got := strings.Join(lines, "\n")

	for _, forbidden := range []string{"\x1b", "\r", "\f", "\t"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("wrapped stream text contains raw terminal control %q: %q", forbidden, got)
		}
	}
	for _, want := range []string{"正常", "红色", "覆盖", "结束", "列", "尾"} {
		if !strings.Contains(got, want) {
			t.Fatalf("wrapped stream text missing %q: %q", want, got)
		}
	}
}

func TestRenderStreamContentSanitizesAgentHeaderControls(t *testing.T) {
	got := renderStreamContent([]string{"\x1b[33m✻ agent\x1b[0m\r\n任务\f说明\t尾"}, 80, "")

	assertNoRawUserControls(t, "rendered stream content", got, []string{"\x1b", "\r", "\f", "\t"})
	assertRenderedContains(t, "rendered stream content", got, []string{"✻", "agent", "任务", "说明", "尾"})
}

func TestRenderEventContentSanitizesSummaries(t *testing.T) {
	events := []host.Event{
		{Time: time.Now(), Category: "DISPATCH", Agent: "coordinator", Summary: "派发\x1b[2J\r覆盖\f尾"},
		{Time: time.Now(), Category: "TOOL", Agent: "writer", Summary: "工具\x1b[31m红\x1b[0m\t尾"},
		{Time: time.Now(), Category: "SYSTEM", Summary: "系统\x1b]8;;https://example.test\a链接\x1b]8;;\a"},
		{Time: time.Now(), Category: "USER", Summary: "用户\r下一行"},
		{Time: time.Now(), Category: "ERROR", Summary: "错误\f尾"},
	}

	got := renderEventContent(events, 100, 0)
	assertNoRawUserControls(t, "event content", got, []string{"\x1b", "\r", "\f", "\t", "\a"})
	assertRenderedContains(t, "event content", got, []string{"派发", "覆盖", `\f`, "工具", "系统", "链接", "用户", "下一行", "错误"})
}

func TestRenderDetailContentSanitizesSnapshotText(t *testing.T) {
	got := renderDetailContent(host.UISnapshot{
		Outline: []host.OutlineSnapshot{{Chapter: 1, Title: "标题\x1b[2J\r覆盖"}},
		Characters: []string{
			"角色\f尾",
		},
		Premise:           "# 前提\x1b[31m红\x1b[0m\r覆盖",
		LastCommitSummary: "提交\x1b[2J尾",
		LastReviewSummary: "审阅\r尾",
		RecentSummaries:   []string{"摘要\f尾"},
		SupportingCount:   1,
		RecentSupporting:  []string{"配角\x1b[2J尾"},
		Layered:           true,
		CurrentVolumeArc:  "第1卷\x1b[2J",
		NextVolumeTitle:   "下一卷\r尾",
		CompassDirection:  "终局\f尾",
		CompassScale:      "长篇\x1b[2J",
	}, 100)

	assertNoRawUserControls(t, "detail content", got, []string{"\x1b", "\r", "\f"})
	assertRenderedContains(t, "detail content", got, []string{"标题", "覆盖", "角色", `\f`, "前提", "提交", "审阅", "摘要", "配角", "下一卷", "终局"})
}
