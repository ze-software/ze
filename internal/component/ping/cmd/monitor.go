// Design: docs/architecture/api/commands.md -- monitor ping command handler
// Related: ping.go -- batch show ping; this is the live monitor variant

package cmd

import (
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// handleMonitorPing is the RPC handler for `monitor ping` (ze-monitor:ping).
// The live, continuously-refreshing ping view is driven client-side by the CLI
// model through NewPingSession (see stream.go); this RPC only acknowledges the
// non-streaming dispatch path.
func handleMonitorPing(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"status": "monitor-ping-configured"},
	}, nil
}
