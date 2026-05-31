package headless

import (
	"strings"
	"testing"
	"time"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/host"
)

func TestWriteEventSanitizesTerminalControls(t *testing.T) {
	var out strings.Builder
	writeEvent(&out, host.Event{
		Time:     time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
		Category: "SYSTEM",
		Summary:  "开始\x1b[2J\r覆盖\f尾",
	})

	got := out.String()
	for _, forbidden := range []string{"\x1b", "\r", "\f"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("writeEvent contains raw terminal control %q: %q", forbidden, got)
		}
	}
	for _, want := range []string{"开始", "覆盖", `\f`, "尾"} {
		if !strings.Contains(got, want) {
			t.Fatalf("writeEvent missing %q: %q", want, got)
		}
	}
}

func TestReplayQueueSanitizesStreamAndEvents(t *testing.T) {
	items := []domain.RuntimeQueueItem{
		{Kind: domain.RuntimeQueueUIEvent, Time: time.Now(), Category: "USER", Summary: "用户\x1b[2J\r覆盖"},
		{Kind: domain.RuntimeQueueStreamDelta, Payload: map[string]any{"delta": "正文\x1b[31m红\x1b[0m\r覆盖\f尾"}},
	}
	var stdout, stderr strings.Builder

	if _, err := replayQueue(items, &stdout, &stderr); err != nil {
		t.Fatalf("replayQueue: %v", err)
	}
	got := stdout.String() + stderr.String()
	for _, forbidden := range []string{"\x1b", "\r", "\f"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("replay output contains raw terminal control %q: %q", forbidden, got)
		}
	}
}
