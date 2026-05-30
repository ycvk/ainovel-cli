package flow

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/domain"
	storepkg "github.com/voocel/ainovel-cli/internal/store"
)

type recordingModel struct {
	mu       sync.Mutex
	calls    int
	requests [][]agentcore.Message
	second   chan struct{}
}

func newRecordingModel() *recordingModel {
	return &recordingModel{second: make(chan struct{})}
}

func (m *recordingModel) Generate(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	return nil, nil
}

func (m *recordingModel) GenerateStream(_ context.Context, messages []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	copied := append([]agentcore.Message(nil), messages...)
	m.requests = append(m.requests, copied)
	m.mu.Unlock()

	ch := make(chan agentcore.StreamEvent, 1)
	go func() {
		defer close(ch)
		switch call {
		case 1:
			ch <- agentcore.StreamEvent{
				Type: agentcore.StreamEventDone,
				Message: agentcore.Message{
					Role: agentcore.RoleAssistant,
					Content: []agentcore.ContentBlock{agentcore.ToolCallBlock(agentcore.ToolCall{
						ID:   "call-subagent",
						Name: "subagent",
						Args: json.RawMessage(`{"agent":"writer","task":"写第 1 章"}`),
					})},
					StopReason: agentcore.StopReasonToolUse,
				},
				StopReason: agentcore.StopReasonToolUse,
			}
		default:
			if call == 2 {
				close(m.second)
			}
			ch <- agentcore.StreamEvent{
				Type: agentcore.StreamEventDone,
				Message: agentcore.Message{
					Role:       agentcore.RoleAssistant,
					Content:    []agentcore.ContentBlock{agentcore.TextBlock("done")},
					StopReason: agentcore.StopReasonStop,
				},
				StopReason: agentcore.StopReasonStop,
			}
		}
	}()
	return ch, nil
}

func (m *recordingModel) SupportsTools() bool { return true }

func (m *recordingModel) request(i int) []agentcore.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i < 0 || i >= len(m.requests) {
		return nil
	}
	return append([]agentcore.Message(nil), m.requests[i]...)
}

type subagentToolStub struct{}

func (subagentToolStub) Name() string        { return "subagent" }
func (subagentToolStub) Description() string { return "dispatch subagent" }
func (subagentToolStub) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent": map[string]any{"type": "string"},
			"task":  map[string]any{"type": "string"},
		},
		"required": []string{"agent", "task"},
	}
}
func (subagentToolStub) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"committed":true,"next_chapter":2,"flow":"writing"}`), nil
}

func TestDispatcherSteersInstructionBeforeNextLLMCall(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	if err := st.Progress.Save(&domain.Progress{
		Phase: domain.PhaseWriting,
		Flow:  domain.FlowWriting,
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}

	model := newRecordingModel()
	dispatcher := NewDispatcher(nil, st)

	var committedMu sync.Mutex
	var committed []agentcore.AgentMessage
	coordinator := agentcore.NewAgent(
		agentcore.WithModel(model),
		agentcore.WithTools(subagentToolStub{}),
		agentcore.WithMaxTurns(3),
		agentcore.WithMiddlewares(dispatcher.Middleware()),
		agentcore.WithOnMessage(func(msg agentcore.AgentMessage) {
			committedMu.Lock()
			defer committedMu.Unlock()
			committed = append(committed, msg)
		}),
	)
	dispatcher.Bind(coordinator)
	dispatcher.Enable()

	if err := coordinator.Prompt("start"); err != nil {
		t.Fatalf("prompt: %v", err)
	}

	select {
	case <-model.second:
	case <-time.After(time.Second):
		t.Fatal("second LLM request was not observed")
	}

	second := model.request(1)
	if len(second) == 0 {
		t.Fatal("missing second LLM request")
	}
	last := second[len(second)-1]
	if last.Role != agentcore.RoleUser {
		t.Fatalf("second request last role = %s, want user Host instruction", last.Role)
	}
	if !strings.Contains(last.TextContent(), "[Host 下达指令]") {
		t.Fatalf("second request did not receive Host instruction before model call: %q", last.TextContent())
	}
	if !strings.Contains(last.TextContent(), `subagent(writer, "写第 1 章")`) {
		t.Fatalf("unexpected Host instruction: %q", last.TextContent())
	}

	coordinator.WaitForIdle()

	committedMu.Lock()
	ordered := append([]agentcore.AgentMessage(nil), committed...)
	committedMu.Unlock()

	toolResultIndex := -1
	hostIndex := -1
	secondAssistantIndex := -1
	for i, msg := range ordered {
		switch msg.GetRole() {
		case agentcore.RoleTool:
			toolResultIndex = i
		case agentcore.RoleUser:
			if strings.Contains(msg.TextContent(), "[Host 下达指令]") {
				hostIndex = i
			}
		case agentcore.RoleAssistant:
			if strings.Contains(msg.TextContent(), "done") {
				secondAssistantIndex = i
			}
		}
	}
	if toolResultIndex < 0 {
		t.Fatal("session order missing subagent tool result")
	}
	if hostIndex < 0 {
		t.Fatal("session order missing Host instruction")
	}
	if secondAssistantIndex < 0 {
		t.Fatal("session order missing second assistant message")
	}
	if !(toolResultIndex < hostIndex && hostIndex < secondAssistantIndex) {
		t.Fatalf("session order = tool_result:%d host:%d assistant:%d, want tool_result < host < assistant", toolResultIndex, hostIndex, secondAssistantIndex)
	}
}
