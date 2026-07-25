// Design: docs/architecture/api/commands.md -- monitor traceroute command handler
// Related: traceroute.go -- sequential show traceroute; this is the live monitor variant

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// handleMonitorTraceroute is the RPC handler for `monitor traceroute`
// (ze-monitor:traceroute). The live, continuously-refreshing mtr-style view is
// driven client-side by the CLI model through NewTracerouteSession (see
// stream.go); this RPC runs a single parallel probe round for non-streaming
// callers, identical to `show probe-round`.
func handleMonitorTraceroute(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	return HandleProbeRound(ctx, args)
}
