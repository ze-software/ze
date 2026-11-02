// Design: docs/architecture/api/commands.md -- BGP monitor command handler
// Overview: monitor.go -- StreamMonitor, the producer this route runs

package monitor

import (
	"context"
	"io"

	"github.com/ze-software/ze/internal/component/command"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func init() {
	// The streaming route of `monitor bgp` (`ze monitor bgp`, `ze cli -c`, SSH
	// exec). A bare `monitor bgp` typed in the interactive CLI opens the
	// dashboard view instead: the model matches its live views before its
	// streaming commands. The stream is a sequence of events, so no one-shot
	// answer holds its data.
	pluginserver.RegisterStreamingHandler("monitor bgp", pluginserver.OneShotStreamingOnly, streamBGPMonitor)
}

// streamBGPMonitor runs StreamMonitor over the daemon's monitor set on the
// arguments the model judged, until the caller disconnects.
func streamBGPMonitor(ctx context.Context, s *pluginserver.Server, w io.Writer, _ string, validated command.ValidatedArgs) error {
	return StreamMonitor(ctx, s.Monitors(), w, validated.Tokens())
}
