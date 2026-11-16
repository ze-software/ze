// Design: docs/architecture/api/commands.md -- daemon restart acceptance.
package server

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestDaemonRestartDefersActionUntilTransportCompletes proves that accepted
// restart answers before touching the daemon, and completion releases its wait.
// Method: dispatch the real command with a recording callback and shutdown
// notification channel; repeat completion without invoking a real reactor.
func TestDaemonRestartDefersActionUntilTransportCompletes(t *testing.T) {
	d := restartLineDispatcher(t)
	s := &Server{dispatcher: d, shutdownRequested: make(chan struct{})}
	calls := 0
	s.SetRestartFunc(func() { calls++ })
	resp, err := d.Dispatch(&CommandContext{Server: s}, "request restart")
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)
	assert.Equal(t, plugin.Map{fieldMessage: "restart initiated"}, resp.Data)
	assert.Zero(t, calls)
	assert.False(t, s.stopUnderWay())
	resp.TransportComplete()
	assert.Equal(t, 1, calls)
	assert.True(t, s.stopUnderWay())
	resp.TransportComplete()
	assert.Equal(t, 1, calls)
}

// TestDaemonRestartConcurrentAcceptedResponsesShareOneAction proves that
// separate accepted requests cannot run the GR stop more than once.
// Method: accept a bounded pair before teardown and complete them concurrently.
func TestDaemonRestartConcurrentAcceptedResponsesShareOneAction(t *testing.T) {
	d := restartLineDispatcher(t)
	s := &Server{dispatcher: d, shutdownRequested: make(chan struct{})}
	var calls atomic.Int32
	s.SetRestartFunc(func() { calls.Add(1) })
	first, err := d.Dispatch(&CommandContext{Server: s}, "request restart")
	require.NoError(t, err)
	second, err := d.Dispatch(&CommandContext{Server: s}, "request restart")
	require.NoError(t, err)
	var completed sync.WaitGroup
	completed.Go(first.TransportComplete)
	completed.Go(second.TransportComplete)
	completed.Wait()
	assert.Equal(t, int32(1), calls.Load())
	assert.True(t, s.stopUnderWay())
}

// TestDaemonRestartCanceledRequestCannotAccept proves that request cancellation
// before dispatch refuses restart without installing an accepted action.
// Method: use a configured recording callback and a canceled caller context.
func TestDaemonRestartCanceledRequestCannotAccept(t *testing.T) {
	d := restartLineDispatcher(t)
	s := &Server{dispatcher: d}
	calls := 0
	s.SetRestartFunc(func() { calls++ })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp, err := d.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "request restart")
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)
	resp.TransportComplete()
	assert.Zero(t, calls)
}

// TestDaemonRestartAcceptedActionSurvivesCallerCancellation proves that a lost
// receipt cannot retract a restart already accepted by the command handler.
// Method: cancel after dispatch, then complete the response as a failed writer
// must do, and observe the one accepted callback.
func TestDaemonRestartAcceptedActionSurvivesCallerCancellation(t *testing.T) {
	d := restartLineDispatcher(t)
	s := &Server{dispatcher: d}
	calls := 0
	s.SetRestartFunc(func() { calls++ })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := d.Dispatch(&CommandContext{Server: s, RequestContext: ctx}, "request restart")
	require.NoError(t, err)
	cancel()
	resp.TransportComplete()
	assert.Equal(t, 1, calls)
}

// TestDaemonRestartDeniedReaderCannotRunAction proves that a real policy denial
// cannot acquire the configured lifecycle callback, including on completion.
// Method: dispatch as the reader through the real builtin and authz.Store.
func TestDaemonRestartDeniedReaderCannotRunAction(t *testing.T) {
	d := restartLineDispatcher(t)
	d.SetAuthorizer(authz.StoreAuthorizer{Store: daemonRestartPolicy()})
	s := &Server{dispatcher: d}
	calls := 0
	s.SetRestartFunc(func() { calls++ })
	resp, err := d.Dispatch(&CommandContext{Server: s, Username: "reader"}, "request restart")
	require.ErrorIs(t, err, ErrUnauthorized)
	require.NotNil(t, resp)
	resp.TransportComplete()
	assert.Zero(t, calls)
}

// TestDaemonRestartRefusesArguments proves that an accidental extra word cannot
// become an ignored argument to a destructive lifecycle command.
// Method: dispatch a registered restart with one unexpected positional value.
func TestDaemonRestartRefusesArguments(t *testing.T) {
	d := restartLineDispatcher(t)
	s := &Server{dispatcher: d}
	calls := 0
	s.SetRestartFunc(func() { calls++ })
	resp, err := d.Dispatch(&CommandContext{Server: s}, "request restart accidental")
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, resp.Error, "does not take arguments")
	resp.TransportComplete()
	assert.Zero(t, calls)
}
