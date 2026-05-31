package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/voocel/agentcore"
	"github.com/voocel/ainovel-cli/internal/errs"
	"github.com/voocel/ainovel-cli/internal/store"
	"github.com/voocel/ainovel-cli/internal/tools"
)

type fatalToolMarker struct {
	FatalToolError bool   `json:"fatal_tool_error"`
	Tool           string `json:"tool"`
	Message        string `json:"message"`
}

type fatalToolWrapper struct {
	tool    agentcore.Tool
	isFatal func(error) bool
}

func markFatalToolErrors(tool agentcore.Tool, isFatal func(error) bool) agentcore.Tool {
	return fatalToolWrapper{tool: tool, isFatal: isFatal}
}

func (w fatalToolWrapper) Name() string { return w.tool.Name() }
func (w fatalToolWrapper) Description() string {
	return w.tool.Description()
}
func (w fatalToolWrapper) Schema() map[string]any { return w.tool.Schema() }
func (w fatalToolWrapper) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	result, err := w.tool.Execute(ctx, args)
	if err == nil || w.isFatal == nil || !w.isFatal(err) {
		return result, err
	}
	// agentcore turns normal tool errors into model-visible tool results and
	// keeps the child loop alive. This explicit marker lets StopAfterToolResult
	// stop the child loop, then the parent subagent wrapper restores it to a
	// real error at the coordinator boundary.
	marker, marshalErr := json.Marshal(fatalToolMarker{
		FatalToolError: true,
		Tool:           w.tool.Name(),
		Message:        err.Error(),
	})
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal fatal tool marker: %w", marshalErr)
	}
	return marker, nil
}

func (w fatalToolWrapper) Label() string {
	if labeled, ok := w.tool.(agentcore.ToolLabeler); ok {
		return labeled.Label()
	}
	return w.tool.Name()
}

func newArchitectSaveFoundationTool(st *store.Store) agentcore.Tool {
	return markFatalToolErrors(tools.NewSaveFoundationTool(st), isFatalSaveFoundationError)
}

func isFatalSaveFoundationError(err error) bool {
	return errors.Is(err, errs.ErrToolPrecondition) ||
		errors.Is(err, errs.ErrStoreRead) ||
		errors.Is(err, errs.ErrStoreWrite)
}

func stopAfterFatalToolResult(next func(toolName string, result json.RawMessage) bool) func(toolName string, result json.RawMessage) bool {
	return func(toolName string, result json.RawMessage) bool {
		if _, ok := decodeFatalToolMarker(result); ok {
			return true
		}
		return next != nil && next(toolName, result)
	}
}

type fatalSubagentResultWrapper struct {
	inner agentcore.Tool
}

func failOnFatalSubagentResult(inner agentcore.Tool) agentcore.Tool {
	return fatalSubagentResultWrapper{inner: inner}
}

func (w fatalSubagentResultWrapper) Name() string { return w.inner.Name() }
func (w fatalSubagentResultWrapper) Description() string {
	return w.inner.Description()
}
func (w fatalSubagentResultWrapper) Schema() map[string]any { return w.inner.Schema() }
func (w fatalSubagentResultWrapper) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	result, err := w.inner.Execute(ctx, args)
	if err != nil {
		return nil, err
	}
	if marker, ok := decodeFatalSubagentResult(result); ok {
		agentName := decodeSubagentName(args)
		if agentName == "" {
			agentName = "subagent"
		}
		return nil, fmt.Errorf("agent %q failed in %s: %s", agentName, marker.Tool, marker.Message)
	}
	return result, nil
}

func (w fatalSubagentResultWrapper) Label() string {
	if labeled, ok := w.inner.(agentcore.ToolLabeler); ok {
		return labeled.Label()
	}
	return w.inner.Name()
}

func decodeFatalSubagentResult(result json.RawMessage) (fatalToolMarker, bool) {
	var out struct {
		TerminalResult json.RawMessage `json:"terminal_result"`
	}
	if err := json.Unmarshal(result, &out); err != nil || len(out.TerminalResult) == 0 {
		return fatalToolMarker{}, false
	}
	return decodeFatalToolMarker(out.TerminalResult)
}

func decodeFatalToolMarker(result json.RawMessage) (fatalToolMarker, bool) {
	var marker fatalToolMarker
	if err := json.Unmarshal(result, &marker); err != nil {
		return fatalToolMarker{}, false
	}
	if !marker.FatalToolError || marker.Tool == "" || marker.Message == "" {
		return fatalToolMarker{}, false
	}
	return marker, true
}

func decodeSubagentName(args json.RawMessage) string {
	var p struct {
		Agent string `json:"agent"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return ""
	}
	return p.Agent
}
