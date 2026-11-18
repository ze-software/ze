// Design: docs/architecture/mcp/overview.md -- command results.
package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Both generated tools and ze_execute must keep completed records beside an
// incomplete evaluation. The MCP failure flag remains set for the caller.
func TestCommandProgressKeepsFailedData(t *testing.T) {
	for _, route := range []string{"generated", "execute"} {
		t.Run(route, func(t *testing.T) {
			const document = `{"cleanup":[{"id":1},{"id":2}]}`
			const failure = "evaluation stopped before the final acknowledgement"
			response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(document))
			require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{Count: 2, Message: failure}))
			runner := progressToolRunner(response)
			result := progressToolCall(runner, route)
			assert.Equal(t, true, result["isError"])
			content, ok := result["content"].([]map[string]any)
			require.True(t, ok)
			require.Len(t, content, 2, "completed data and failure must both reach the tool caller")
			assert.JSONEq(t, document, content[0][contentTypeText].(string))
			assert.Equal(t, "Error: "+failure, content[1][contentTypeText])
		})
	}
}

// A partial command retains its independent row causes and warning lines.
// Warning text does not turn a partial answer into a successful tool result.
func TestCommandProgressKeepsPartialDataAndWarnings(t *testing.T) {
	for _, route := range []string{"generated", "execute"} {
		t.Run(route, func(t *testing.T) {
			const document = `{"cleanup":[{"id":1}],"errors":[{"id":2,"cause":"acknowledgement missing"}]}`
			const warning = "retry the cleanup retained by its original owner"
			response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(document))
			require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
				Count: 1, Faults: 1, Warning: true, Warnings: []string{warning},
			}))
			result := progressToolCall(progressToolRunner(response), route)
			assert.Equal(t, true, result["isError"])
			content, ok := result["content"].([]map[string]any)
			require.True(t, ok)
			require.Len(t, content, 3)
			assert.JSONEq(t, document, content[0][contentTypeText].(string))
			assert.Equal(t, "Warning: "+warning, content[1][contentTypeText])
			assert.Contains(t, content[2][contentTypeText], "partial")
		})
	}
}

// A refusal that produced no record progress keeps its ordinary error-only
// result; an arbitrary payload beside a handler failure is not completed data.
func TestCommandProgressKeepsOrdinaryFailureErrorOnly(t *testing.T) {
	for _, route := range []string{"generated", "execute"} {
		t.Run(route, func(t *testing.T) {
			response := &plugin.Response{Status: plugin.StatusError, Error: "denied", Data: plugin.Map{"private": "not produced"}}
			result := progressToolCall(progressToolRunner(response), route)
			assert.Equal(t, true, result["isError"])
			content, ok := result["content"].([]map[string]any)
			require.True(t, ok)
			require.Len(t, content, 1)
			assert.Equal(t, "Error: denied", content[0][contentTypeText])
		})
	}
}

// A healthy command warning also reaches ze_execute, which shares the same
// answer contract as a generated tool. Its warning never sets the error flag.
func TestCommandProgressKeepsHealthyWarning(t *testing.T) {
	for _, route := range []string{"generated", "execute"} {
		t.Run(route, func(t *testing.T) {
			response := &plugin.Response{Status: plugin.StatusWarning, Data: plugin.Map{"answer": "kept"}, Warnings: []string{"the original warning"}}
			result := progressToolCall(progressToolRunner(response), route)
			assert.NotEqual(t, true, result["isError"])
			content, ok := result["content"].([]map[string]any)
			require.True(t, ok)
			require.Len(t, content, 2)
			assert.JSONEq(t, `{"answer":"kept"}`, content[0][contentTypeText].(string))
			assert.Equal(t, "Warning: the original warning", content[1][contentTypeText])
		})
	}
}

// Both command routes must retain a warning verdict without inventing warning
// diagnostics. Direct responses and completed records share the same label;
// healthy and explicit-warning controls retain their original content shape.
func TestCommandProgressKeepsStatusOnlyWarning(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   string
		metadata *rpc.ExecuteCommandRecordAnswer
		warnings []string
		label    string
	}{
		{name: "healthy status", status: plugin.StatusDone},
		{name: "explicit status warning", status: plugin.StatusWarning, warnings: []string{"the original warning"}, label: "Warning: the original warning"},
		{name: "status only warning", status: plugin.StatusWarning, label: "Warning"},
		{name: "healthy completed records", status: plugin.StatusDone, metadata: &rpc.ExecuteCommandRecordAnswer{Count: 1}},
		{name: "explicit completed warning", status: plugin.StatusDone, metadata: &rpc.ExecuteCommandRecordAnswer{Count: 1, Warning: true, Warnings: []string{"the original warning"}}, warnings: []string{"the original warning"}, label: "Warning: the original warning"},
		{name: "status only completed warning", status: plugin.StatusDone, metadata: &rpc.ExecuteCommandRecordAnswer{Count: 1, Warning: true}, label: "Warning"},
	} {
		for _, route := range []string{"generated", "execute"} {
			t.Run(test.name+"/"+route, func(t *testing.T) {
				const document = `{"answer":"kept"}`
				response := plugin.NewResponse(test.status, plugin.RawJSON(document))
				response.Warnings = test.warnings
				if test.metadata != nil {
					require.NoError(t, response.CompleteRecordAnswer(test.metadata))
				}
				result := progressToolCall(progressToolRunner(response), route)
				assert.NotEqual(t, true, result["isError"])
				content, ok := result[toolResultKeyContent].([]map[string]any)
				require.True(t, ok)
				wantContent := 1
				if test.label != "" {
					wantContent++
				}
				require.Len(t, content, wantContent)
				assert.JSONEq(t, document, content[0][contentTypeText].(string))
				if test.label != "" {
					assert.Equal(t, test.label, content[1][contentTypeText])
				}
				assert.Equal(t, test.warnings, response.Warnings, "presentation must preserve the original warning diagnostics")
				if test.metadata != nil {
					metadata, completed := response.RecordAnswerMetadata()
					require.True(t, completed)
					assert.Equal(t, test.metadata, metadata, "presentation must preserve the completed outcome")
				}
			})
		}
	}
}

// A completed answer may fail and warn without producing a document. Both
// MCP command routes must retain its warning lines and error without data.
func TestCommandProgressAbsentDocumentKeepsOutcome(t *testing.T) {
	const failure = "evaluation stopped before the final acknowledgement"
	warnings := []string{"retained cleanup warning", "the original owner must retry cleanup"}
	for _, test := range []struct {
		name     string
		message  string
		warnings []string
		failed   bool
		content  []string
	}{
		{name: "failed with warnings", message: failure, warnings: warnings, failed: true, content: []string{"Warning: " + warnings[0] + "\n" + warnings[1], "Error: " + failure}},
		{name: "failed without warnings", message: failure, failed: true, content: []string{"Error: " + failure}},
		{name: "warning only", warnings: warnings, content: []string{"", "Warning: " + warnings[0] + "\n" + warnings[1]}},
		{name: "healthy absent", content: []string{""}},
	} {
		for _, route := range []string{"generated", "execute"} {
			t.Run(test.name+"/"+route, func(t *testing.T) {
				response := plugin.NewResponse(plugin.StatusDone, nil)
				require.NoError(t, response.CompleteRecordAnswer(&rpc.ExecuteCommandRecordAnswer{
					DocumentAbsent: true, Message: test.message, Warning: len(test.warnings) != 0, Warnings: test.warnings,
				}))
				result := progressToolCall(progressToolRunner(response), route)
				assert.Equal(t, test.failed, result["isError"] == true)
				content, ok := result[toolResultKeyContent].([]map[string]any)
				require.True(t, ok)
				require.Len(t, content, len(test.content), "an absent document must not gain a data item on failure")
				for index, expected := range test.content {
					assert.Equal(t, expected, content[index][contentTypeText])
				}
				assert.Nil(t, response.Data, "rendering manufactured a document")
			})
		}
	}
}

// This dispatcher keeps the production ResponseJSON outcome projection while
// isolating the MCP adapter's two registered command entry points.
func progressToolRunner(response *plugin.Response) *server {
	return &server{dispatch: func(context.Context, plugin.CallerIdentity, string) (*plugin.Response, error) {
		return response, nil
	}}
}

func progressToolCall(runner *server, route string) map[string]any {
	if route == "execute" {
		return toolHandlers[toolNameExecute](runner, json.RawMessage(`{"command":"show cleanup"}`))
	}
	return runner.run("show cleanup")
}
