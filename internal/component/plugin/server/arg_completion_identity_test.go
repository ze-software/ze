// Design: docs/architecture/cli/command-completion.md -- authenticated live arguments.

package server

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type argumentIdentityAuthorizer struct {
	t       *testing.T
	allowed bool
	checked bool
}

func (a *argumentIdentityAuthorizer) Authorize(username, remote, input string, readOnly bool) bool {
	a.checked = true
	assert.Equal(a.t, "operator", username)
	assert.Equal(a.t, "203.0.113.9:4567", remote)
	assert.Equal(a.t, "request completion probe", input)
	assert.False(a.t, readOnly)
	return a.allowed
}

// TestArgumentCompletionUsesCallerPolicy proves the target command's actual
// caller is authorized before its runtime values are read, including builtins.
func TestArgumentCompletionUsesCallerPolicy(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		t.Run(map[bool]string{true: "allowed", false: "denied"}[allowed], func(t *testing.T) {
			d := NewDispatcher()
			reads := 0
			require.NoError(t, d.RegisterWithOptions("request completion probe", func(*CommandContext, command.ValidatedArgs) (*plugin.Response, error) {
				t.Fatal("completion executed the target")
				return plugin.NewResponse(plugin.StatusDone, plugin.Map{}), nil
			}, "Complete a live value", RegisterOptions{Complete: func(*Server, []string) []Completion {
				reads++
				return []Completion{{Value: "private-value"}}
			}}))
			policy := &argumentIdentityAuthorizer{t: t, allowed: allowed}
			ctx := &CommandContext{Server: &Server{dispatcher: d}, RequestContext: t.Context(), Username: "operator", RemoteAddr: "203.0.113.9:4567", Authorizer: policy}
			resp, err := handleArgComplete(ctx, "request completion probe", nil, "")
			assert.True(t, policy.checked)
			if allowed {
				require.NoError(t, err)
				assert.Equal(t, 1, reads)
				data, ok := resp.Data.(plugin.Map)
				require.True(t, ok)
				assert.Equal(t, []Completion{{Value: "private-value"}}, data[fieldCompletions])
			} else {
				assert.ErrorIs(t, err, ErrUnauthorized)
				assert.Zero(t, reads)
				assert.NotNil(t, resp)
				assert.Equal(t, plugin.StatusError, resp.Status)
			}
		})
	}
}

// TestPluginArgumentCompletionCancelsWithCaller uses a real mux connection and
// an already-canceled caller. A detached Background timeout would ask the plugin
// and disclose its sentinel answer, making the empty-result assertion fail.
func TestPluginArgumentCompletionCancelsWithCaller(t *testing.T) {
	engineEnd, pluginEnd := net.Pipe()
	workerDone := make(chan struct{})
	t.Cleanup(func() {
		require.NoError(t, engineEnd.Close())
		require.NoError(t, pluginEnd.Close())
		<-workerDone
	})
	proc := process.NewProcess(plugin.PluginConfig{Name: "completion-context"})
	proc.SetConn(muxPluginConn(t, engineEnd))
	markProcessRunning(t, proc)
	d := NewDispatcher()
	results := d.Registry().Register(proc, []CommandDef{{Name: "show completion context", Completable: true}})
	require.True(t, results[0].OK, results[0].Error)
	go func() {
		defer close(workerDone)
		conn := rpc.NewConn(pluginEnd, pluginEnd)
		req, err := conn.ReadRequest(t.Context())
		if err != nil {
			return // The canceled caller sends nothing; cleanup closes this reader.
		}
		_ = rpc.WriteDocumentAnswer(conn.AnswerWriter(t.Context()), req.ID, rpc.AnswerTail{}, []byte(`{"completions":[{"value":"detached-request"}]}`))
	}()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	resp, err := handleArgComplete(&CommandContext{Server: &Server{dispatcher: d}, RequestContext: ctx}, "show completion context", nil, "")
	assert.ErrorIs(t, err, context.Canceled)
	assert.NotNil(t, resp)
}

// TestPluginArgumentCompletionCancelsInFlight waits for the actual plugin RPC
// before canceling. A pre-canceled guard alone cannot satisfy this control.
func TestPluginArgumentCompletionCancelsInFlight(t *testing.T) {
	engineEnd, pluginEnd := net.Pipe()
	received := make(chan struct{})
	workerDone := make(chan struct{})
	t.Cleanup(func() {
		require.NoError(t, engineEnd.Close())
		require.NoError(t, pluginEnd.Close())
		<-workerDone
	})
	proc := process.NewProcess(plugin.PluginConfig{Name: "completion-inflight"})
	proc.SetConn(muxPluginConn(t, engineEnd))
	markProcessRunning(t, proc)
	d := NewDispatcher()
	results := d.Registry().Register(proc, []CommandDef{{Name: "show completion inflight", Completable: true}})
	require.True(t, results[0].OK, results[0].Error)
	go func() {
		defer close(workerDone)
		conn := rpc.NewConn(pluginEnd, pluginEnd)
		if _, err := conn.ReadRequest(t.Context()); err != nil {
			return
		}
		close(received)
		// The plugin deliberately leaves this request unanswered. Cleanup ends
		// the worker; cancellation must end the engine's wait independently.
		_, _ = conn.ReadRequest(t.Context()) //nolint:errcheck // teardown closes this lifetime reader
	}()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := handleArgComplete(&CommandContext{Server: &Server{dispatcher: d}, RequestContext: ctx}, "show completion inflight", nil, "")
		finished <- err
	}()
	select {
	case <-received:
		cancel()
	case <-t.Context().Done():
		t.Fatal("plugin never received its completion request")
	}
	select {
	case err := <-finished:
		assert.ErrorIs(t, err, context.Canceled)
	case <-t.Context().Done():
		t.Fatal("completion ignored caller cancellation")
	}
}
