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

// TestDispatchWarningLinesFollowNegotiatedVersion checks that the socket path
// writes a dispatch-command answer for the protocol version the process
// negotiated at Stage 1. The method: one handler answers status warning with
// two lines; a process that negotiated nothing, one that negotiated 1 and one
// that negotiated rpc.ProtocolVersionWarningLines each dispatch it through the
// runtime handler over a pipe, and the plugin end reads the answer.
//
// VALIDATES: the engine sends the warning lines only to a plugin that declared
// the version carrying them, and every reader still reads status warning.
// PREVENTS: an older plugin receiving text in the wrn message field, which its
// SDK reads as the command's failure.
func TestDispatchWarningLinesFollowNegotiatedVersion(t *testing.T) {
	t.Parallel()

	lines := []string{`line 3: unknown leaf "x"`, "line 9: deprecated"}
	cases := []struct {
		name    string
		version int
		want    []string
	}{
		{name: "unnegotiated", version: 0, want: nil},
		{name: "version 1", version: 1, want: nil},
		{name: "warning lines", version: rpc.ProtocolVersionWarningLines, want: lines},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			answer := dispatchWarningOverSocket(t, tc.version, lines)
			assert.Equal(t, rpc.VerdictWarning, answer.Verdict(), "the command completed with a warning")
			assert.Empty(t, answer.Message(), "no failure text reaches the plugin")
			assert.Equal(t, tc.want, answer.Warnings(), "the lines follow the negotiated version")
		})
	}
}

// dispatchWarningOverSocket runs one dispatch-command through
// handleSingleProcessCommandsRPC for a process that negotiated version, whose
// handler answers status warning with lines, and returns the answer the plugin
// end read to its terminator.
func dispatchWarningOverSocket(t *testing.T, version int, lines []string) *rpc.Answer {
	t.Helper()

	d := NewDispatcher()
	require.NoError(t, d.Register("warning lines test", func(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
		return &plugin.Response{Status: plugin.StatusWarning, Data: plugin.Map{"ok": true}, Warnings: lines}, nil
	}, "warning lines test"))

	pluginSide, engineSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "warning-lines"})
	proc.SetConn(ipc.NewPluginConn(engineSide, engineSide))
	if version > 0 {
		proc.SetProtocolVersion(version)
	}

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
