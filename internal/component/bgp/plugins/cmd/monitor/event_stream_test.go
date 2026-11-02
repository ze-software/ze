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

// TestMonitorEventStreamAppliesFilters proves that `monitor event` honors its
// include, exclude and peer arguments on the route that streams it (SSH exec,
// `ze cli -c`). The plugin fixtures event-monitor-include, -exclude and -peer
// proved this parsing through the one-shot echo
// (`{"status":"monitor-configured", "include": ...}`); a streaming-only command
// is refused on a one-shot route, so the proof lives here. Method: resolve the
// handler the way the SSH executor does (GetStreamingHandlerForCommand) from
// the full command line, run it against a server, deliver one event the
// filter keeps and one it drops, and read the header and the stream. The
// dropped event is delivered first: the client queue is FIFO, so once the kept
// event is read, a dropped event the filter let through would already be in
// the stream.
func TestMonitorEventStreamAppliesFilters(t *testing.T) {
	const peerKept, peerDropped = "10.0.0.1", "10.0.0.2"
	tests := []struct {
		name    string
		line    string
		header  string
		kept    [3]string // event type, peer, JSON
		dropped [3]string
	}{
		{
			name:    "include",
			line:    "monitor event include update,state",
			header:  "include=update,state",
			kept:    [3]string{bgpevents.EventUpdate, peerKept, `{"kept":"update"}`},
			dropped: [3]string{bgpevents.EventKeepalive, peerKept, `{"dropped":"keepalive"}`},
		},
		{
			name:    "exclude",
			line:    "monitor event exclude keepalive",
			header:  "exclude=keepalive",
			kept:    [3]string{bgpevents.EventUpdate, peerKept, `{"kept":"update"}`},
			dropped: [3]string{bgpevents.EventKeepalive, peerKept, `{"dropped":"keepalive"}`},
		},
		{
			name:    "peer",
			line:    "monitor event peer " + peerKept,
			header:  "peer=" + peerKept,
			kept:    [3]string{bgpevents.EventUpdate, peerKept, `{"kept":"peer"}`},
			dropped: [3]string{bgpevents.EventUpdate, peerDropped, `{"dropped":"peer"}`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, validated, err := pluginserver.GetStreamingHandlerForCommand(tt.line)
			require.NoError(t, err)
			require.NotNil(t, handler, "the streaming executor finds no handler for %q", tt.line)

			server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var buf syncBuffer
			done := make(chan error, 1)
			go func() { done <- handler(ctx, server, &buf, "operator", validated) }()

			require.Eventually(t, func() bool { return server.Monitors().Count() == 1 },
				2*time.Second, 10*time.Millisecond, "%q registers no monitor client", tt.line)
			server.Monitors().Deliver(bgpevents.Namespace, tt.dropped[0], events.DirectionReceived, tt.dropped[1], "", tt.dropped[2])
			server.Monitors().Deliver(bgpevents.Namespace, tt.kept[0], events.DirectionReceived, tt.kept[1], "", tt.kept[2])
			require.Eventually(t, func() bool { return strings.Contains(buf.String(), tt.kept[2]) },
				2*time.Second, 10*time.Millisecond, "%q: the kept event never reached the stream", tt.line)

			cancel()
			require.NoError(t, <-done)
			output := buf.String()
			header, _, _ := strings.Cut(output, "\n")
			require.Contains(t, header, tt.header, "%q stream header", tt.line)
			require.NotContains(t, output, tt.dropped[2], "%q: the filter let the dropped event through", tt.line)
		})
	}
}
