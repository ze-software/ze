package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestToolCallCarriesTheRefusalAsWritten runs a tool whose command refuses
// with text it wrote whole.
//
// VALIDATES: the tool result is an error whose only text item is the
// refusal as `ze <command>` prints it, with no "Error: " of MCP's own in
// front; any other failure keeps the "Error: " rendering.
// PREVENTS: an AI reading "Error: usage: ..." where the operator reads
// "usage: ...".
func TestToolCallCarriesTheRefusalAsWritten(t *testing.T) {
	runner := &server{
		dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			refusal := plugin.NewRefusalError("usage: show config list\n")
			return &plugin.Response{Status: plugin.StatusError, Error: refusal.Error()}, refusal
		},
	}
	result := runner.run("show config list")
	if result["isError"] != true {
		t.Fatalf("a refusal is a tool error: %#v", result)
	}
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content = %#v, want one text item", result["content"])
	}
	if got := content[0][contentTypeText]; got != "usage: show config list" {
		t.Errorf("text %q, want the refusal as the local route prints it", got)
	}

	runner.dispatch = func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		return nil, errors.New("plain failure")
	}
	plain, ok := runner.run("show config list")["content"].([]map[string]any)
	if !ok || len(plain) != 1 || plain[0][contentTypeText] != "Error: plain failure" {
		t.Errorf("a plain failure keeps MCP's rendering, got %#v", plain)
	}
}
