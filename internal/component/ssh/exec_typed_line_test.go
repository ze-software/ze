// Design: docs/architecture/api/commands.md -- a rendering surface sends the typed line and renders the residual chain.
package ssh

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestExecOneShotSendsTheTypedLine asks whether the exec channel hands the
// executor the line as typed, pipe chain included, and renders the residual
// chain over the answer itself (package 3c of
// spec-pipe-chain-honored-on-every-command-surface).
//
// Method: a command path that declares one operation (`scope`), and an
// executor that records its input and answers a compact JSON document. The
// line `<path> | scope | json` must reach the executor whole, and the channel
// must print the document as `| json` renders it (indented), which the
// executor never does.
//
// MUTATION: send the command part the formatter returns instead of the line;
// the executor then records `<path>` and the declared `scope` never reaches
// the dispatcher.
func TestExecOneShotSendsTheTypedLine(t *testing.T) {
	const path = "show test ssh-typed-line"
	command.RegisterPipeFilters([]string{path},
		command.PipeFilter{Name: "scope", Description: "Pick a scope", Argument: command.ArgNone})
	const line = path + " | scope | json"

	var mu sync.Mutex
	var received []string
	srv := answerServer(t, func(input string) (*plugin.Response, error) {
		mu.Lock()
		received = append(received, input)
		mu.Unlock()
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.RawJSON(`{"ok":"yes"}`)}, nil
	})
	creds := answerCredentials(t, srv)

	output, err := sshclient.ExecCommand(creds, line)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{line}, received, "the executor did not receive the typed line")
	assert.Contains(t, output, `"ok": "yes"`, "the channel did not render | json over the answer")
	assert.True(t, strings.Contains(output, "\n"), "the answer reached the client unrendered: %q", output)
}
