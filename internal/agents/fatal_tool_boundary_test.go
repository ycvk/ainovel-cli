package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/voocel/agentcore"
	"github.com/voocel/agentcore/subagent"
	"github.com/voocel/ainovel-cli/internal/errs"
)

type scriptedModel struct {
	responses []agentcore.Message
	requests  [][]agentcore.Message
	calls     atomic.Int64
}

func newScriptedModel(responses ...agentcore.Message) *scriptedModel {
	return &scriptedModel{responses: responses}
}

func (m *scriptedModel) Generate(context.Context, []agentcore.Message, []agentcore.ToolSpec, ...agentcore.CallOption) (*agentcore.LLMResponse, error) {
	return nil, errors.New("Generate should not be used")
}

func (m *scriptedModel) GenerateStream(_ context.Context, messages []agentcore.Message, _ []agentcore.ToolSpec, _ ...agentcore.CallOption) (<-chan agentcore.StreamEvent, error) {
	i := int(m.calls.Add(1) - 1)
	m.requests = append(m.requests, append([]agentcore.Message(nil), messages...))
	if i >= len(m.responses) {
		return nil, fmt.Errorf("scripted model: no response %d", i)
	}
	ch := make(chan agentcore.StreamEvent, 1)
	msg := m.responses[i]
	ch <- agentcore.StreamEvent{Type: agentcore.StreamEventDone, Message: msg, StopReason: msg.StopReason}
	close(ch)
	return ch, nil
}

func (m *scriptedModel) SupportsTools() bool { return true }

type errorTool struct {
	name string
	err  error
}

func (t errorTool) Name() string        { return t.name }
func (t errorTool) Description() string { return t.name + " tool" }
func (t errorTool) Schema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t errorTool) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	return nil, t.err
}

func toolCallMessage(name string) agentcore.Message {
	return agentcore.Message{
		Role: agentcore.RoleAssistant,
		Content: []agentcore.ContentBlock{agentcore.ToolCallBlock(agentcore.ToolCall{
			ID:   "call-" + name,
			Name: name,
			Args: json.RawMessage(`{}`),
		})},
		StopReason: agentcore.StopReasonToolUse,
	}
}

func textMessage(text string) agentcore.Message {
	return agentcore.Message{
		Role:       agentcore.RoleAssistant,
		Content:    []agentcore.ContentBlock{agentcore.TextBlock(text)},
		StopReason: agentcore.StopReasonStop,
	}
}

func TestFatalSaveFoundationErrorBubblesThroughSubagent(t *testing.T) {
	model := newScriptedModel(
		toolCallMessage("save_foundation"),
		textMessage("should not continue after fatal tool error"),
	)
	tool := failOnFatalSubagentResult(subagent.New(subagent.Config{
		Name:        "architect_long",
		Description: "architect",
		Model:       model,
		Tools: []agentcore.Tool{
			markFatalToolErrors(errorTool{
				name: "save_foundation",
				err:  fmt.Errorf("append_volume blocked: %w", errs.ErrToolPrecondition),
			}, isFatalSaveFoundationError),
		},
		MaxTurns:            3,
		StopAfterToolResult: stopAfterFatalToolResult(nil),
	}))

	_, err := tool.Execute(context.Background(), json.RawMessage(`{"agent":"architect_long","task":"append volume"}`))
	if err == nil {
		t.Fatal("expected fatal save_foundation error to bubble through subagent")
	}
	if !strings.Contains(err.Error(), "architect_long") || !strings.Contains(err.Error(), "save_foundation") {
		t.Fatalf("error should identify subagent and tool, got: %v", err)
	}
	if !strings.Contains(err.Error(), errs.ErrToolPrecondition.Error()) {
		t.Fatalf("error should preserve original category text, got: %v", err)
	}
	if got := model.calls.Load(); got != 1 {
		t.Fatalf("fatal tool error should stop child loop after first turn, model calls=%d", got)
	}
}

func TestNonFatalToolArgsRemainModelVisible(t *testing.T) {
	model := newScriptedModel(
		toolCallMessage("save_foundation"),
		textMessage("continued after model-visible args error"),
	)
	tool := failOnFatalSubagentResult(subagent.New(subagent.Config{
		Name:        "architect_long",
		Description: "architect",
		Model:       model,
		Tools: []agentcore.Tool{
			markFatalToolErrors(errorTool{
				name: "save_foundation",
				err:  fmt.Errorf("bad args: %w", errs.ErrToolArgs),
			}, isFatalSaveFoundationError),
		},
		MaxTurns:            3,
		StopAfterToolResult: stopAfterFatalToolResult(nil),
	}))

	raw, err := tool.Execute(context.Background(), json.RawMessage(`{"agent":"architect_long","task":"retry args"}`))
	if err != nil {
		t.Fatalf("nonfatal tool args error should remain recoverable by the model: %v", err)
	}
	if !strings.Contains(string(raw), "continued after model-visible args error") {
		t.Fatalf("subagent did not continue after nonfatal args error: %s", string(raw))
	}
	if got := model.calls.Load(); got != 2 {
		t.Fatalf("nonfatal tool error should allow a second model turn, model calls=%d", got)
	}
	secondReq := model.requests[1]
	last := secondReq[len(secondReq)-1]
	if last.GetRole() != agentcore.RoleTool {
		t.Fatalf("second request should expose tool error as tool result, got role=%s", last.GetRole())
	}
	if !strings.Contains(last.TextContent(), errs.ErrToolArgs.Error()) {
		t.Fatalf("tool result should preserve args error for model correction, got: %q", last.TextContent())
	}
}
