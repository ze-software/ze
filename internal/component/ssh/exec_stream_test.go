// Design: docs/architecture/api/commands.md -- the stream route over SSH exec and the rendering key.

package ssh

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/internal/component/cli/sshclient"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// lookupStreamExecutor is the daemon's stream executor without its AAA: it
// records each request the exec channel hands it, then runs the line through
// the one lookup (pluginserver.GetStreamingHandlerRendered), which partitions
// the chain and applies the daemon's part of it, as the hub's executor does.
func lookupStreamExecutor(requests *[]StreamRequest) StreamingExecutor {
	return func(ctx context.Context, output io.Writer, request StreamRequest) error {
		if requests != nil {
			*requests = append(*requests, request)
		}
		handler, validated, err := pluginserver.GetStreamingHandlerRendered(request.Line, request.Rendering)
		if err != nil {
			return err
		}
		if handler == nil {
			return io.ErrUnexpectedEOF
		}
		return handler(ctx, nil, output, "operator", validated)
	}
}

// runRawSSHExecEnv is runRawSSHExec with session environment keys set before
// the command runs.
func runRawSSHExecEnv(t *testing.T, server *Server, line string, environ map[string]string) (stdout, stderr string, runErr error) {
	t.Helper()
	client, err := gossh.Dial("tcp", server.Address(), &gossh.ClientConfig{
		User:            "operator",
		Auth:            []gossh.AuthMethod{gossh.Password("read-pass")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test server key is generated per run.
		Timeout:         5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	session, err := client.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	for key, value := range environ {
		require.NoError(t, session.Setenv(key, value))
	}
	var out, errOut bytes.Buffer
	session.Stdout = &out
	session.Stderr = &errOut
	runErr = session.Run(line)
	return out.String(), errOut.String(), runErr
}

// registerTwoStateStream registers a monitor at prefix whose events are one
// row each, one `up` and one `down`, and counts its runs.
func registerTwoStateStream(t *testing.T, prefix string) *atomic.Int32 {
	t.Helper()
	var runs atomic.Int32
	pluginserver.RegisterStreamingHandler(prefix, pluginserver.OneShotStreamingOnly,
		func(_ context.Context, _ *pluginserver.Server, output io.Writer, _ string, _ command.ValidatedArgs) error {
			runs.Add(1)
			_, err := io.WriteString(output, "{\"peers\":[{\"state\":\"up\"}]}\n{\"peers\":[{\"state\":\"down\"}]}\n")
			return err
		})
	t.Cleanup(func() { pluginserver.UnregisterStreamingHandler(prefix) })
	return &runs
}

// TestExecStreamPassesTypedLine asks whether the exec channel hands the
// daemon's stream executor the monitor line as typed, chain included, and
// applies none of the chain itself (AC-35).
//
// Method: a stream executor that records its request and writes one raw
// event; the line carries a format. Red before: the channel split the chain
// and sent the command part, and it formatted the event itself.
func TestExecStreamPassesTypedLine(t *testing.T) {
	const line = "monitor ssh-typed-line-test peer Blue | json | match up"
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	var requests []StreamRequest
	server.SetStreamingExecutorFactory(func(_, _ string, _ plugin.Authorizer) StreamingExecutor {
		return func(_ context.Context, output io.Writer, request StreamRequest) error {
			requests = append(requests, request)
			_, err := io.WriteString(output, "{\"raw\":true}\n")
			return err
		}
	})
	registerTwoStateStream(t, "monitor ssh-typed-line-test")

	stdout, stderr, err := runRawSSHExec(t, server, line)

	require.NoError(t, err, stderr)
	assert.Equal(t, []StreamRequest{{Line: line, Rendering: command.StreamRenderingTerminal}}, requests)
	assert.Equal(t, "{\"raw\":true}\n", stdout, "the channel applied a pipe the daemon owns")
}

// TestExecSessionRendersStatement asks whether the rendering key is read on
// the exec channel: set to its one value, the request states that the client
// presents; absent, the daemon renders (A-10, AC-23).
func TestExecSessionRendersStatement(t *testing.T) {
	const prefix = "monitor ssh-render-statement-test"
	registerTwoStateStream(t, prefix)
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	var requests []StreamRequest
	server.SetStreamingExecutorFactory(func(_, _ string, _ plugin.Authorizer) StreamingExecutor {
		return lookupStreamExecutor(&requests)
	})

	_, stderr, err := runRawSSHExecEnv(t, server, prefix, map[string]string{sshclient.EnvCLIRender: sshclient.CLIRenderClient})
	require.NoError(t, err, stderr)
	_, stderr, err = runRawSSHExecEnv(t, server, prefix, nil)
	require.NoError(t, err, stderr)

	require.Len(t, requests, 2)
	assert.Equal(t, command.StreamRenderingClientPresentation, requests[0].Rendering)
	assert.Equal(t, command.StreamRenderingTerminal, requests[1].Rendering)
}

// TestRenderingKeyNeverSkipsSelection asks the security question of the key:
// a client that sets it and sends `| match up | save f` still receives no
// event the match removes, and the daemon writes no file (AC-36, R-12).
//
// Method: the real lookup over a monitor whose second event has no `up`.
// MUTATION: make ReadStreamLine drop the whole residual chain under
// StreamRenderingClientPresentation; the `down` row reaches stdout.
func TestRenderingKeyNeverSkipsSelection(t *testing.T) {
	const prefix = "monitor ssh-render-selection-test"
	registerTwoStateStream(t, prefix)
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	server.SetStreamingExecutorFactory(func(_, _ string, _ plugin.Authorizer) StreamingExecutor {
		return lookupStreamExecutor(nil)
	})
	path := filepath.Join(t.TempDir(), "daemon-side.out")

	stdout, stderr, err := runRawSSHExecEnv(t, server, prefix+" | match up | save "+path,
		map[string]string{sshclient.EnvCLIRender: sshclient.CLIRenderClient})

	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "up")
	assert.NotContains(t, stdout, "down", "the key let a client skip the daemon's match")
	_, statErr := os.Stat(path)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "the daemon wrote the client's save")
}

// TestRenderingKeyUnknownValueRefused asks whether a value of the key other
// than its one value is refused by name before any command runs, on the exec
// channel and in the request the interactive session reads (AC-37).
func TestRenderingKeyUnknownValueRefused(t *testing.T) {
	const prefix = "monitor ssh-render-unknown-test"
	runs := registerTwoStateStream(t, prefix)
	var dispatched atomic.Bool
	server := answerServer(t, func(string) (*plugin.Response, error) {
		dispatched.Store(true)
		return plugin.NewResponse(plugin.StatusDone, nil), nil
	})
	server.SetStreamingExecutorFactory(func(_, _ string, _ plugin.Authorizer) StreamingExecutor {
		return lookupStreamExecutor(nil)
	})
	wrong := map[string]string{sshclient.EnvCLIRender: "server"}

	for _, line := range []string{prefix + " | match up", "show version"} {
		stdout, stderr, err := runRawSSHExecEnv(t, server, line, wrong)
		require.Error(t, err, line)
		assert.Empty(t, stdout, line)
		assert.Contains(t, stderr, sshclient.EnvCLIRender, line)
		assert.Contains(t, stderr, "SSH exec", "the refusal does not name the exec channel: "+line)
		assert.NotContains(t, stderr, "interactive SSH", "the exec refusal names the interactive session: "+line)
	}
	assert.Zero(t, runs.Load(), "a stream ran under a refused key")
	assert.False(t, dispatched.Load(), "a command ran under a refused key")

	_, err := parseSessionRequest([]string{sshclient.EnvCLIRender + "=server"})
	assert.ErrorContains(t, err, sshclient.EnvCLIRender, "the interactive session accepted the refused value")
}

// TestRenderingKeyIgnoredOnOneShotExec asks whether the key changes nothing
// on a one-shot exec: the channel renders the whole chain with and without
// it (AC-37).
func TestRenderingKeyIgnoredOnOneShotExec(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"version":"test"}`)), nil
	})
	const line = "show version | json"

	plain, plainErr, err := runRawSSHExecEnv(t, server, line, nil)
	require.NoError(t, err, plainErr)
	keyed, keyedErr, err := runRawSSHExecEnv(t, server, line, map[string]string{sshclient.EnvCLIRender: sshclient.CLIRenderClient})
	require.NoError(t, err, keyedErr)

	assert.NotEmpty(t, plain)
	assert.Equal(t, plain, keyed, "the key changed a one-shot answer")
}

// TestExecIgnoresInteractiveKeys asks whether a one-shot exec still ignores
// the keys that select an interactive terminal (ZE_CLI_MODE, ZE_CONFIG_NAME),
// even with a value the interactive session refuses: the exec channel has no
// terminal to select, so it answers exactly as with no environment (AC-10).
//
// Method: the same one-shot line run with no environment, with an unknown
// mode, and with a configuration name that is a path. Red before: the exec
// channel parsed the whole interactive request and refused both.
func TestExecIgnoresInteractiveKeys(t *testing.T) {
	server := answerServer(t, func(string) (*plugin.Response, error) {
		return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(`{"version":"test"}`)), nil
	})
	const line = "show version | json"

	plain, plainErr, err := runRawSSHExecEnv(t, server, line, nil)
	require.NoError(t, err, plainErr)
	require.NotEmpty(t, plain)

	for _, environ := range []map[string]string{
		{sshclient.EnvCLIMode: "bogus"},
		{sshclient.EnvConfigName: "../x"},
	} {
		stdout, stderr, err := runRawSSHExecEnv(t, server, line, environ)
		require.NoError(t, err, "an interactive key refused a one-shot exec: %v: %s", environ, stderr)
		assert.Equal(t, plainErr, stderr, "an interactive key changed the one-shot stderr: %v", environ)
		assert.Equal(t, plain, stdout, "an interactive key changed a one-shot answer: %v", environ)
	}
}
