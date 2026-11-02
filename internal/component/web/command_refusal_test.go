package web

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/plugin"
)

// refusalDispatcher answers every command with a usage refusal the command
// wrote whole, as the daemon serves a local-data refusal.
func refusalDispatcher() CommandDispatcher {
	return func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
		refusal := plugin.NewRefusalError("usage: show config list\n")
		return &plugin.Response{Status: plugin.StatusError, Error: refusal.Error()}, refusal
	}
}

// TestTerminalOperationalShowsTheRefusalAsWritten runs a refused command in
// the web terminal.
//
// VALIDATES: the terminal shows the refusal as `ze <command>` prints it,
// "usage: ..." and not "error: usage: ...".
// PREVENTS: the web terminal reading otherwise than the local route.
func TestTerminalOperationalShowsTheRefusalAsWritten(t *testing.T) {
	output, _ := executeTerminalOperational(t.Context(), refusalDispatcher(), "admin", "127.0.0.1:1", "show config list")
	assert.Equal(t, "usage: show config list", strings.TrimRight(output, "\n"))
}
