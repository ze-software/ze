package mcp

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestToolCallCarriesTheWarning runs a command through the path every
// generated tool takes (server.run) against a dispatcher whose answer warns,
// and reads the tool result an AI receives.
//
// VALIDATES: the warning `ze <command>` prints on stderr reaches the MCP
// caller, as a second text item, while the first item stays the bare answer.
// PREVENTS: an AI reading an answer the local route qualifies with a warning
// as if nothing had been said about it.
func TestToolCallCarriesTheWarning(t *testing.T) {
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return &plugin.Response{
				Status:   plugin.StatusWarning,
				Data:     plugin.Map{"answer": "kept"},
				Warnings: []string{"warning: first line", "warning: second line"},
			}, nil
		},
	}
	result := runner.run("show config dump")
	if _, isErr := result["isError"]; isErr {
		t.Fatalf("a warning is not a tool error: %#v", result)
	}
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) != 2 {
		t.Fatalf("content = %#v, want the answer and the warning", result["content"])
	}
	if got := content[0][contentTypeText]; got != `{"answer":"kept"}` {
		t.Errorf("first item %q, want the bare answer", got)
	}
	if got := content[1][contentTypeText]; got != "Warning: warning: first line\nwarning: second line" {
		t.Errorf("second item %q, want every warning line", got)
	}

	runner.dispatch = func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"answer": "kept"}}, nil
	}
	quiet, ok := runner.run("show config dump")["content"].([]map[string]any)
	if !ok || len(quiet) != 1 {
		t.Errorf("an answer that warns of nothing has one item, got %#v", quiet)
	}
}
