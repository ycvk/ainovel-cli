package host

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/voocel/agentcore"
	storepkg "github.com/voocel/ainovel-cli/internal/store"
	"github.com/voocel/ainovel-cli/internal/utils"
)

func TestObserverSuppressesThinkingStreamByDefault(t *testing.T) {
	var deltas []string
	o := &observer{
		emitD: func(delta string) {
			deltas = append(deltas, delta)
		},
		lastThinkingByAgent: make(map[string]string),
	}

	o.handleMessageUpdate(agentcore.Event{
		Delta:     "private chain",
		DeltaKind: agentcore.DeltaThinking,
	})
	o.handleThinkingProgress(agentcore.Event{
		Progress: &agentcore.ProgressPayload{
			Agent:    "coordinator",
			Thinking: "private progress",
		},
	})
	o.handleSubagentDelta(&agentcore.ProgressPayload{
		Delta:     "private subagent",
		DeltaKind: agentcore.DeltaThinking,
	})
	o.handleMessageUpdate(agentcore.Event{
		Delta:     "visible text",
		DeltaKind: agentcore.DeltaText,
	})

	got := strings.Join(deltas, "")
	if strings.Contains(got, "private") {
		t.Fatalf("thinking leaked into stream: %q", got)
	}
	if got != "visible text" {
		t.Fatalf("stream = %q, want visible text only", got)
	}
}

func TestObserverCanStreamThinkingInDiagnosticMode(t *testing.T) {
	var deltas []string
	o := &observer{
		emitD: func(delta string) {
			deltas = append(deltas, delta)
		},
		lastThinkingByAgent:   make(map[string]string),
		streamThinkingEnabled: true,
	}

	o.handleMessageUpdate(agentcore.Event{
		Delta:     "diagnostic chain",
		DeltaKind: agentcore.DeltaThinking,
	})
	o.handleSubagentDelta(&agentcore.ProgressPayload{
		Delta:     " + subagent",
		DeltaKind: agentcore.DeltaThinking,
	})
	o.handleThinkingProgress(agentcore.Event{
		Progress: &agentcore.ProgressPayload{
			Agent:    "coordinator",
			Thinking: " + progress",
		},
	})

	got := strings.Join(deltas, "")
	want := utils.ThinkingSep + "diagnostic chain + subagent + progress"
	if got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}

func TestHostEmitDeltaBackpressuresInsteadOfDropping(t *testing.T) {
	h := &Host{streamCh: make(chan string, 2)}
	h.emitDelta("first")
	h.emitDelta("second")

	done := make(chan struct{})
	go func() {
		h.emitDelta("third")
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("emitDelta returned while stream channel was full; expected backpressure")
	case <-time.After(30 * time.Millisecond):
	}

	if got := <-h.streamCh; got != "first" {
		t.Fatalf("oldest delta was dropped: got %q, want first", got)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("emitDelta did not unblock after stream channel had capacity")
	}

	if got := <-h.streamCh; got != "second" {
		t.Fatalf("second delta = %q, want second", got)
	}
	if got := <-h.streamCh; got != "third" {
		t.Fatalf("third delta = %q, want third", got)
	}
}

func TestHostEmitDurableEventPersistsWhenProjectionFull(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	h := &Host{store: st, events: make(chan Event, 1)}
	h.events <- Event{Summary: "old projection"}

	if err := h.emitDurableEvent(Event{
		Time:     time.Now(),
		Category: "SYSTEM",
		Summary:  "durable event",
		Level:    "info",
	}); err != nil {
		t.Fatalf("emitDurableEvent: %v", err)
	}

	items, err := st.Runtime.LoadQueue()
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}
	if len(items) != 1 || items[0].Summary != "durable event" {
		t.Fatalf("durable event not persisted: %+v", items)
	}
}

func TestObserverPersistEventReturnsAppendQueueError(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir(), "meta", "runtime", "queue.jsonl"), []byte("{\n"), 0o644); err != nil {
		t.Fatalf("corrupt runtime queue: %v", err)
	}

	o := &observer{store: st}
	err := o.persistEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "will fail"})
	if err == nil {
		t.Fatal("persistEvent should return runtime queue append error")
	}
}

func TestParseSubagentResultError(t *testing.T) {
	cases := []struct {
		name   string
		result string
		want   string
	}{
		{"empty", ``, ""},
		{"object form", `{"error":"unknown agent \"writer2\""}`, `unknown agent "writer2"`},
		{"object empty error field", `{"error":""}`, ""},
		{"bare string - invalid params", `"Invalid parameters: provide exactly one mode (agent+task, tasks, or chain)"`, "Invalid parameters: provide exactly one mode (agent+task, tasks, or chain)"},
		{"bare string - background", `"background mode requires agent + task"`, "background mode requires agent + task"},
		{"bare string - parallel cap", `"Too many parallel tasks (5). Max is 3."`, "Too many parallel tasks (5). Max is 3."},
		{"bare string - normal result not flagged", `"Chapter committed"`, ""},
		{"success object not flagged", `{"chapter":1,"status":"ok"}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseSubagentResultError(json.RawMessage(c.result))
			if got != c.want {
				t.Fatalf("parseSubagentResultError(%q) = %q, want %q", c.result, got, c.want)
			}
		})
	}
}
