package monitor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// TestMonitorBGPStreamingRoute proves AC-19 for `monitor bgp` on the
// streaming routes (`ze monitor bgp`, `ze cli -c`, SSH exec): the command is a
// registered streaming command, and the handler the streaming executor runs
// streams the daemon's BGP events. Method: resolve the handler the way the SSH
// executor does (GetStreamingHandlerForCommand), run it against a server,
// deliver one event to the server's monitor set, and read the header and the
// event from the stream.
func TestMonitorBGPStreamingRoute(t *testing.T) {
	require.True(t, pluginserver.IsStreamingCommand("monitor bgp"), "monitor bgp must be a streaming command")
	handler, validated, err := pluginserver.GetStreamingHandlerForCommand("monitor bgp")
	require.NoError(t, err)
	require.NotNil(t, handler, "the streaming executor finds no handler for monitor bgp")

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var buf syncBuffer
	done := make(chan error, 1)
	go func() { done <- handler(ctx, server, &buf, "operator", validated) }()

	require.Eventually(t, func() bool { return server.Monitors().Count() == 1 },
		2*time.Second, 10*time.Millisecond, "the route registers no monitor client")
	eventJSON := `{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1","remote":{"as":65001}},"message":{"type":"update","direction":"received"}}}`
	server.Monitors().Deliver(bgpevents.Namespace, bgpevents.EventUpdate, events.DirectionReceived, "10.0.0.1", "", eventJSON)
	require.Eventually(t, func() bool { return strings.Contains(buf.String(), eventJSON) },
		2*time.Second, 10*time.Millisecond, "the delivered event never reached the stream")

	cancel()
	require.NoError(t, <-done)
	require.True(t, strings.HasPrefix(buf.String(), "monitoring: all events, all peers\n"), "stream header: %q", buf.String())
}
