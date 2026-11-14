// Design: docs/architecture/api/process-protocol.md -- plugin RPC dispatch.
// Related: dispatch_registry.go -- answerEngineOpJSON, recordAnswer.
// Related: dispatch_test.go -- dispatchAnswerOverSocket, TestDispatchCommandDirectBridge.

package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDispatchCommandSocketCompletesAcceptedAction is the socket half of
// TestDispatchCommandDirectBridge. A command a forked plugin dispatches can
// accept a lifecycle action (`request shutdown`, `daemon reboot`), which runs
// only once the transport has delivered the answer. Over the socket the plugin
// reads the record answer whole, and the action then runs exactly once.
//
// Method: the real dispatch-command engine op served by the runtime socket loop
// (handleSingleProcessCommandsRPC), read by the answer reader a forked plugin
// uses (dispatchAnswerOverSocket).
//
// VALIDATES: answerEngineOpJSON writes a record answer as its record sequence
// and completes the action the answer carries.
// PREVENTS: an accepted shutdown or reboot that a forked plugin requested
// never running, and an answer sent as one `ok` line that the answer reader
// refuses.
func TestDispatchCommandSocketCompletesAcceptedAction(t *testing.T) {
	t.Parallel()

	pluginSide, engineSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "test-socket-completion"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	completed := make(chan struct{}, 2)
	d := NewDispatcher()
	if err := d.Register("completion test", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		resp := &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"result": "accepted"}}
		resp.OnTransportComplete(func() { completed <- struct{}{} })
		return resp, nil
	}, "completion test"); err != nil {
		t.Fatal(err)
	}

	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	status, data, err := dispatchAnswerOverSocket(ctx, pluginSide, &rpc.DispatchCommandInput{Command: "completion test"})
	require.NoError(t, err)
	assert.Equal(t, rpc.StatusDone, status)
	assert.JSONEq(t, `{"result":"accepted"}`, string(data))

	select {
	case <-completed:
	case <-ctx.Done():
		t.Fatal("the socket transport never completed the accepted action")
	}

	if err := pluginSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	if err := engineSide.Close(); err != nil {
		t.Logf("close: %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit")
	}
	select {
	case <-completed:
		t.Fatal("the socket transport completed the accepted action twice")
	default:
	}
}
