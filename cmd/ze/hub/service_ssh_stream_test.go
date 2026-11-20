//go:build ze_web && ze_bgp && ze_ssh

// Design: docs/architecture/api/commands.md -- the SSH exec stream accounts and authorizes the typed line.

package hub

import (
	"bytes"
	"context"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
)

// recordingStreamPolicy allows every command and records what it judged.
type recordingStreamPolicy struct{ judged []string }

func (p *recordingStreamPolicy) Authorize(_, _, command string, _ bool) bool {
	p.judged = append(p.judged, command)
	return true
}

// TestSSHExecStreamAccountsAndAuthorizesTypedLine asks whether the daemon's
// SSH exec stream executor accounts and authorizes the line as typed, its
// chain included, and hands the monitor its declared operations while the
// residual chain runs over the events (AC-35).
//
// Method: the real executor (sshStreamingExecutorFactory) over a monitor that
// declares `only`, with an accounting hook and a recording authorizer.
// Red before: execMiddleware sent the command part, so neither record held
// the chain, and the lookup handed no declared operation.
func TestSSHExecStreamAccountsAndAuthorizesTypedLine(t *testing.T) {
	const prefix = "monitor test ssh-exec-typed-line"
	const line = prefix + " alpha | only up | json"
	var handed command.ValidatedArgs
	pluginserver.RegisterStreamingHandler(prefix, pluginserver.OneShotStreamingOnly,
		func(_ context.Context, _ *pluginserver.Server, output io.Writer, _ string, args command.ValidatedArgs) error {
			handed = args
			_, err := io.WriteString(output, "{\"state\":\"up\"}\n")
			return err
		})
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(prefix) })
	t.Cleanup(command.ResetPipeFiltersForTest)
	command.RegisterPipeFilters([]string{prefix},
		command.PipeFilter{Name: "only", Description: "Keep the events of one state", Argument: command.ArgText})

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	require.NoError(t, server.StartWithContext(t.Context()))
	accounting := &webStreamAccounting{}
	server.Dispatcher().SetAccountingHook(accounting)
	policy := &recordingStreamPolicy{}

	executor := sshStreamingExecutorFactory(server, server.Dispatcher())("alice", "192.0.2.25:1234", policy)
	var output bytes.Buffer
	require.NoError(t, executor(t.Context(), &output, zessh.StreamRequest{Line: line, Rendering: command.StreamRenderingTerminal}))

	assert.Equal(t, []string{"alice|192.0.2.25:1234|" + line}, accounting.starts, "the accounting record lost the chain")
	require.NotEmpty(t, policy.judged, "the authorizer was never asked")
	assert.True(t, strings.Contains(policy.judged[0], "only"), "the authorizer judged %q, without the chain", policy.judged[0])
	assert.Equal(t, []string{"alpha"}, handed.Tokens())
	want := []command.PipeOperation{{Name: "only", Args: []string{"up"}}}
	assert.True(t, slices.EqualFunc(want, handed.PipeOperations(), sameStreamPipeOperation),
		"declared list = %+v, want %+v", handed.PipeOperations(), want)
	// The declared `only up` shaped the event, so the `| json` answer names it
	// under "pipe", as the one-shot route answers the same line.
	assert.Equal(t,
		"{\n  \"pipe\": [\n    {\n      \"arg\": \"up\",\n      \"op\": \"only\"\n    }\n  ],\n  \"state\": \"up\"\n}\n",
		output.String(), "the residual `| json` was not applied with the chain's metadata")
	_, oneShot, msg := command.ProcessPipesDefaultFormatChecked(line, "")
	require.Empty(t, msg, "the one-shot route refused the line")
	assert.Equal(t, oneShot("{\"state\":\"up\"}")+"\n", output.String(), "the stream and one-shot routes disagree")
}
