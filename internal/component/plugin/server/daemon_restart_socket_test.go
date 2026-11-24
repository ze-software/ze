// Design: docs/architecture/api/process-protocol.md -- accepted lifecycle answers.
package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDaemonRestartSocketCompletesAcceptedAction proves the actual shared
// restart command reaches a forked caller as a complete accepted record answer
// before its restart action runs. Method: use the production socket dispatch
// loop over net.Pipe, with a recording action in place of a real daemon stop.
func TestDaemonRestartSocketCompletesAcceptedAction(t *testing.T) {
	pluginSide, engineSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "daemon-restart-socket"})
	proc.SetRegistration(&plugin.PluginRegistration{
		Sends: []rpc.MessageSendDecl{{Message: "cli:request restart"}},
	})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	s := &Server{
		subscriptions: newSubscriptionManager(), dispatcher: restartLineDispatcher(t),
		shutdownRequested: make(chan struct{}),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	completed := make(chan struct{}, 2)
	s.SetRestartFunc(func() { completed <- struct{}{} })
	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()
	t.Cleanup(func() {
		s.cancel()
		require.NoError(t, pluginSide.Close())
		require.NoError(t, engineSide.Close())
		select {
		case <-handlerDone:
		case <-time.After(3 * time.Second):
			t.Error("restart socket handler did not join after its connection closed")
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	status, data, err := dispatchAnswerOverSocket(ctx, pluginSide,
		&rpc.DispatchCommandInput{Command: "request restart"})
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.JSONEq(t, `{"message":"restart initiated"}`, string(data))
	select {
	case <-completed:
	case <-ctx.Done():
		t.Fatal("the restart answer never released its accepted action")
	}
	select {
	case <-completed:
		t.Fatal("the restart action ran more than once")
	default:
	}
}
