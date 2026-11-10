package server

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOnMessageBatchReceivedLogsEveryUndeliveredBatch proves that each early
// return of onMessageBatchReceived names its case in the log before it answers
// zero consumers, so a batch dropped for want of a subscriber, for a type with
// no event kind, or at shutdown is not silent. The no-subscriber line used to
// sit after its return, so that case logged nothing.
//
// Method: point the package logger at a debug text handler, drive each case,
// and read the captured text for the reason, the peer and the batch size, and
// the counts for one zero per message. It swaps a package variable, so it MUST
// NOT run in parallel.
func TestOnMessageBatchReceivedLogsEveryUndeliveredBatch(t *testing.T) {
	cases := []struct {
		name   string
		server func(t *testing.T) *pluginserver.Server
		msg    bgptypes.RawMessage
		reason string
	}{
		{
			name:   "no subscriber",
			server: newTestServer,
			msg:    keepaliveMsg(),
			reason: "no subscriber and no monitor",
		},
		{
			name:   "unknown message type",
			server: newTestServer,
			msg:    bgptypes.RawMessage{Type: msgtype.MessageType(99), Direction: rpc.DirectionReceived},
			reason: "unknown message type",
		},
		{
			name:   "server shutting down",
			server: newStoppedBatchLogServer,
			msg:    keepaliveMsg(),
			reason: "server shutting down",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := captureBatchLog(t)
			srv := tc.server(t)
			msgs := []bgptypes.RawMessage{tc.msg, tc.msg}

			counts := onMessageBatchReceived(srv, format.NewJSONEncoder("test"), testPeerInfo(), msgs)

			require.Equal(t, []int{0, 0}, counts, "an undelivered batch has no consumer")
			out := buf.String()
			for _, want := range []string{"level=DEBUG", "batch not delivered", "reason=\"" + tc.reason + "\"", "peer=10.0.0.1", "msgs=2"} {
				require.True(t, strings.Contains(out, want), "log %q lacks %q", out, want)
			}
		})
	}
}

// captureBatchLog points the package logger at a debug text handler and
// answers what it writes, restoring the logger when the test ends.
func captureBatchLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	captured := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	old := logger
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = old })
	return &buf
}

// newStoppedBatchLogServer answers a started server whose context is already
// canceled, the state onMessageBatchReceived meets during shutdown.
func newStoppedBatchLogServer(t *testing.T) *pluginserver.Server {
	t.Helper()
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	require.NoError(t, srv.StartWithContext(ctx))
	t.Cleanup(func() { srv.Stop() })
	cancel()
	return srv
}
