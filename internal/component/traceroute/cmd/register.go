// Overview: doc.go -- package doc + schema import
//
// register.go wires the traceroute module into ze's two registries from init():
//   - the plugin server RPC registry, for the daemon-side show/probe-round/
//     monitor/resolve traceroute handlers, and
//   - the local command registry, for offline `show traceroute` and
//     `monitor traceroute`.
//
// The module is reached by the daemon through internal/le/plugin/imports/pluginimports.go
// rpcDirs (internal/component/traceroute/cmd) and by the `ze` binary through
// plugin/all.

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
		pluginserver.RPCRegistration{WireMethod: "ze-traceroute:show-traceroute", Handler: handleTraceroute},
		pluginserver.RPCRegistration{WireMethod: "ze-traceroute:show-probe-round", Handler: HandleProbeRound},
		pluginserver.RPCRegistration{WireMethod: "ze-traceroute:monitor-traceroute", Handler: handleMonitorTraceroute},
		pluginserver.RPCRegistration{WireMethod: "ze-traceroute:resolve-traceroute", Handler: handleResolveTraceroute},
	)
	// The streaming route of `monitor traceroute` (`ze cli -c`, SSH exec). The
	// TUI opens the traceroute view first, and `ze monitor traceroute` runs
	// monitorTracerouteLocal.
	pluginserver.RegisterStreamingHandler("monitor traceroute", pluginserver.OneShotSample, streamMonitorTraceroute)

	command.MustRegisterLocalMeta("show traceroute", showTracerouteLocal, registry.Meta{
		ShortHelp: "Trace the network path to a target using the internal ICMP engine (works without the daemon)",
		Mode:      "offline",
	})

	command.MustRegisterLocalMeta("monitor traceroute", monitorTracerouteLocal, registry.Meta{
		ShortHelp: "Live streaming traceroute (works without the daemon)",
		Mode:      "offline",
	})
}

// showTracerouteLocal is the no-daemon route of `show traceroute`: the
// producer the daemon RPC runs (tracerouteAnswer), printed by the local route.
// It stays a plain local command, not a local-data one, so `ze cli` asks the
// daemon, which traces from the router rather than from the operator's host.
func showTracerouteLocal(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswerWith(showTraceroutePath, tracerouteAnswer(validated), renderTraceroute)
}

func monitorTracerouteLocal(validated command.ValidatedArgs) int {
	req, err := monitorTracerouteArgsOf(validated)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("monitor traceroute: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // stderr
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := writeTracerouteRounds(ctx, os.Stdout, req); err != nil {
		var tb textbuf.Buffer
		tb.Str("monitor traceroute: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // stderr
		return 1
	}
	return 0
}
