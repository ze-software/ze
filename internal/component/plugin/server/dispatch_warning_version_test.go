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

// TestDispatchWarningLinesReachEveryPlugin checks that the socket path writes
// a dispatch-command answer's warning lines to the plugin that dispatched it.
// The method: one handler answers status warning with two lines; a process
// dispatches it through the runtime handler (handleSingleProcessCommandsRPC)
// over a pipe, and the plugin end reads the answer to its terminator. The
// process holds no protocol version state, so this one process stands for
// every plugin, whichever version it declared at Stage 1.
//
// VALIDATES: the engine sends the warning lines to every plugin over the
// socket, and the plugin reads status warning with no failure text.
// PREVENTS: the socket path dropping the lines the in-process answer carries.
func TestDispatchWarningLinesReachEveryPlugin(t *testing.T) {
	t.Parallel()

	lines := []string{`line 3: unknown leaf "x"`, "line 9: deprecated"}
	answer := dispatchWarningOverSocket(t, lines)
	assert.Equal(t, rpc.VerdictWarning, answer.Verdict(), "the command completed with a warning")
	assert.Empty(t, answer.Message(), "no failure text reaches the plugin")
	assert.Equal(t, lines, answer.Warnings(), "the warning lines reach the plugin")
}

// dispatchWarningOverSocket runs one dispatch-command through
// handleSingleProcessCommandsRPC for a process whose handler answers status
// warning with lines, and returns the answer the plugin end read to its
// terminator.
func dispatchWarningOverSocket(t *testing.T, lines []string) *rpc.Answer {
	t.Helper()

	d := NewDispatcher()
	require.NoError(t, d.Register("warning lines test", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusWarning, Data: plugin.Map{"ok": true}, Warnings: lines}, nil
	}, "warning lines test"))

	pluginSide, engineSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "warning-lines"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))

	s := &Server{subscriptions: newSubscriptionManager(), dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	t.Cleanup(s.cancel)

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		s.handleSingleProcessCommandsRPC(proc)
	}()

	mux := rpc.NewMuxConn(rpc.NewConn(pluginSide, pluginSide))
	t.Cleanup(func() {
		if err := mux.Close(); err != nil {
			t.Logf("mux close: %v", err)
		}
		s.cancel()
		<-handlerDone
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answer, err := mux.CallAnswer(ctx, rpc.MethodDispatchCommand, &rpc.DispatchCommandInput{Command: "warning lines test"})
	require.NoError(t, err)
	for range answer.Records {
	}
	require.NoError(t, answer.Err(), "the answer reached its terminator")
	return answer
}
