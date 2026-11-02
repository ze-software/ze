// Overview: doc.go -- package doc + schema import
//
// register.go wires the ping module into ze's two registries from init():
//   - the plugin server RPC registry, for the daemon-side show/monitor/resolve
//     ping handlers, and
//   - the local command registry, for offline `show ping` and `monitor ping`.
//
// The module is reached by the daemon through internal/le/plugin/imports/pluginimports.go
// rpcDirs (internal/component/ping/cmd) and by the `ze` binary through plugin/all.

package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/ze-software/ze/internal/component/command"

	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-ping:show-ping", Handler: handleShowPing},
		pluginserver.RPCRegistration{WireMethod: "ze-ping:monitor-ping", Handler: handleMonitorPing},
		pluginserver.RPCRegistration{WireMethod: "ze-ping:resolve-ping", Handler: handleResolvePing},
	)
	// The streaming route of `monitor ping` (`ze cli -c`, SSH exec). The TUI
	// opens the ping view first, and `ze monitor ping` runs monitorPingLocal.
	pluginserver.RegisterStreamingHandler("monitor ping", pluginserver.OneShotSample, streamMonitorPing)

	command.MustRegisterLocalMeta("show ping", showPingLocal, registry.Meta{
		ShortHelp: "Ping a target using the internal ICMP engine (works without the daemon)",
		Mode:      "offline",
	})

	command.MustRegisterLocalMeta("monitor ping", monitorPingLocal, registry.Meta{
		ShortHelp: "Continuous ping with live statistics (works without the daemon)",
		Mode:      "offline",
	})
}

// showPingLocal is the no-daemon route of `show ping`: the producer the daemon
// RPC runs (pingAnswer), printed by the local route. It stays a plain local
// command, not a local-data one, so `ze cli` asks the daemon, which pings from
// the router rather than from the operator's host.
func showPingLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswerWith(showPingPath, pingAnswer(validated), renderPing)
}

func monitorPingLocal(validated command.ValidatedArgs) int {
	mp, err := monitorPingArgsOf(validated)
	if err != nil {
		var tb textbuf.Buffer
		tb.Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // stderr
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ch, cancel, sessionErr := NewPingSession(ctx, mp.Dest.String(), mp.Interval, mp.Timeout, mp.Count, mp.Size)
	if sessionErr != nil {
		var tb textbuf.Buffer
		tb.Str("monitor ping: ").Err(sessionErr).Byte('\n')
		tb.StdErr() //nolint:errcheck // stderr
		return 1
	}
	defer cancel()

	if err := writePingRows(os.Stdout, mp, ch); err != nil {
		var tb textbuf.Buffer
		tb.Str("monitor ping: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // stderr
		return 1
	}
	return 0
}
