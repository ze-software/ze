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
	"strconv"
	"time"

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
	args := validated.Tokens()
	req, err := parseTracerouteArgs(args)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("monitor traceroute: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // stderr
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var hdr textbuf.Buffer
	hdr.Str("traceroute to ").Str(req.target.String()).Str(" (Ctrl-C to stop)\n")
	hdr.StdOut() //nolint:errcheck // stdout

	for round := 1; ; round++ {
		ch, cancel, sessionErr := NewTracerouteSession(ctx, req.target.String(), req.maxHops)
		if sessionErr != nil {
			if round == 1 {
				var tb textbuf.Buffer
				tb.Str("monitor traceroute: ").Err(sessionErr).Byte('\n')
				tb.StdErr() //nolint:errcheck // stderr
				return 1
			}
			break
		}

		var hops []map[string]any
		for hop := range ch {
			hops = append(hops, hop)
		}
		cancel()

		if ctx.Err() != nil {
			break
		}

		var tb textbuf.Buffer
		tb.Str("--- round ").Int(int64(round)).Str(" ---\n")
		for _, hop := range hops {
			ttl, _ := hop[fieldTTL].(int)
			addr, _ := hop["addr"].(string)
			tb.Str("  ").Int(int64(ttl)).Str("  ").Str(addr)
			if rtt, ok := hop["rtt-ms"].(float64); ok {
				tb.Str("  ").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str(" ms")
			}
			tb.Byte('\n')
		}
		tb.StdOut() //nolint:errcheck // stdout

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return 0
		}
	}
	return 0
}
