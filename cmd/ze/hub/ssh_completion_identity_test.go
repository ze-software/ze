//go:build ze_ssh

// Design: docs/architecture/cli/command-completion.md -- session-bound SSH arguments.

package hub

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/config/infra"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zessh "github.com/ze-software/ze/internal/component/ssh"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type sshCompletionPolicy struct{ t *testing.T }

func (p sshCompletionPolicy) Authorize(username, remote, line string, _ bool) bool {
	assert.Equal(p.t, "203.0.113.9:4567", remote)
	if strings.HasPrefix(line, "system command complete") {
		return true
	}
	assert.Equal(p.t, "show ssh completion", line)
	return username == "operator"
}

// TestSSHCompletionBindsCallerContextAndLiveRegistry enters through the real
// session model factory and dispatcher, then reads a real plugin mux answer.
// Reader denial and session cancellation cannot turn into argument candidates.
func TestSSHCompletionBindsCallerContextAndLiveRegistry(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	engineEnd, pluginEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	workerDone := make(chan error, 1)
	t.Cleanup(func() {
		_ = mux.Close()       //nolint:errcheck // test-owned mux teardown
		_ = pluginEnd.Close() //nolint:errcheck // test-owned peer teardown
		require.NoError(t, <-workerDone)
	})
	proc := process.NewProcess(plugin.PluginConfig{Name: "ssh-completion"})
	proc.SetConn(ipc.NewMuxPluginConn(mux))
	proc.SetRunning(true)
	go func() {
		conn := rpc.NewConn(pluginEnd, pluginEnd)
		req, readErr := conn.ReadRequest(t.Context())
		if readErr != nil {
			workerDone <- readErr
			return
		}
		workerDone <- rpc.WriteDocumentAnswer(conn.AnswerWriter(t.Context()), req.ID, rpc.AnswerTail{}, []byte(`{"completions":[{"value":"alpha","short-help":"Live SSH value"}]}`))
	}()
	sshServer, err := zessh.NewServer(zessh.Config{HostKeyPath: filepath.Join(t.TempDir(), "host-key")})
	require.NoError(t, err)
	sessionCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dispatch := serverDispatcher(server, "ssh")
	sshServer.SetExecutorFactory(func(ctx context.Context, username, remote string, policy plugin.Authorizer) zessh.CommandExecutor {
		assert.Same(t, sessionCtx, ctx)
		return func(line string) (*plugin.RenderedResponse, error) {
			return dispatch.Answer(ctx, plugin.CallerIdentity{Username: username, RemoteAddr: remote, Authorizer: policy}, line)
		}
	})
	factory := buildSessionModelFactory(sshServer, infra.HookParams{APIServer: func() *pluginserver.Server { return server }}, nil, nil)
	makeModel := func(username string) cli.Model {
		created, createErr := factory(sessionCtx, username, "203.0.113.9:4567", sshCompletionPolicy{t}, zessh.SessionRequest{})
		require.NoError(t, createErr)
		model, ok := created.(cli.Model)
		require.True(t, ok)
		t.Cleanup(func() { require.NoError(t, model.Close()) })
		model.SetInput("show ssh completion ")
		return model
	}
	operator := makeModel("operator")
	operator.UpdateCompletions()
	assert.False(t, hasSSHCompletion(operator.Completions()))
	registered := server.Dispatcher().Registry().Register(proc, []pluginserver.CommandDef{{Name: "show ssh completion", Completable: true}})
	require.True(t, registered[0].OK, registered[0].Error)
	operator.UpdateCompletions()
	assert.True(t, hasSSHCompletion(operator.Completions()))
	reader := makeModel("reader")
	reader.UpdateCompletions()
	assert.False(t, hasSSHCompletion(reader.Completions()))
	assert.Contains(t, reader.StatusMessage(), plugin.UnauthorizedMessage)
	cancel()
	operator.UpdateCompletions()
	assert.False(t, hasSSHCompletion(operator.Completions()))
	assert.Contains(t, operator.StatusMessage(), context.Canceled.Error())
	server.Dispatcher().Registry().Unregister(proc, []string{"show ssh completion"})
	operator.UpdateCompletions()
	assert.False(t, hasSSHCompletion(operator.Completions()))
}

func hasSSHCompletion(items []cli.Completion) bool {
	for _, item := range items {
		if item.Text == "alpha" {
			return true
		}
	}
	return false
}
