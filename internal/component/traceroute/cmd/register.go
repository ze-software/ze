// Overview: doc.go -- package doc + schema import
//
// register.go wires the traceroute module into ze's two registries from init():
//   - the plugin server RPC registry, for the daemon-side show/probe-round/
//     monitor/resolve traceroute handlers, and
//   - the local command registry, for offline `show traceroute` and
//     `monitor traceroute`.
//
// The module is reached by the daemon through scripts/codegen/plugin_imports.go
// rpcDirs (internal/component/traceroute/cmd) and by the `ze` binary through
// plugin/all.

package cmd

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:traceroute", Handler: handleTraceroute},
		pluginserver.RPCRegistration{WireMethod: "ze-show:probe-round", Handler: HandleProbeRound},
		pluginserver.RPCRegistration{WireMethod: "ze-monitor:traceroute", Handler: handleMonitorTraceroute},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:traceroute", Handler: handleResolveTraceroute},
	)

	registry.MustRegisterLocalMeta("show traceroute", showTracerouteLocal, registry.Meta{
		Description: "Trace the network path to a target using the internal ICMP engine (works without the daemon)",
		Mode:        "offline",
	})

	registry.MustRegisterLocalMeta("monitor traceroute", monitorTracerouteLocal, registry.Meta{
		Description: "Live streaming traceroute (works without the daemon)",
		Mode:        "offline",
	})
}

func showTracerouteLocal(args []string) int {
	target, maxHops, timeout, probes, err := parseTracerouteArgs(args)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("show traceroute: ").Err(err).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}
	hops, trErr := doTraceroute(target, maxHops, timeout, probes, tracerouteOpts{})
	if trErr != nil {
		var tb textbuf.Buffer
		tb.Str("show traceroute: ").Err(trErr).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}
	printTracerouteResults(os.Stdout, target.String(), hops)
	return 0
}

func monitorTracerouteLocal(args []string) int {
	target, maxHops, _, _, err := parseTracerouteArgs(args)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("monitor traceroute: ").Err(err).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var hdr textbuf.Buffer
	hdr.Str("traceroute to ").Str(target.String()).Str(" (Ctrl-C to stop)\n")
	os.Stdout.WriteString(hdr.Slice()) //nolint:errcheck // stdout

	for round := 1; ; round++ {
		ch, cancel, sessionErr := NewTracerouteSession(ctx, target.String(), maxHops)
		if sessionErr != nil {
			if round == 1 {
				var tb textbuf.Buffer
				tb.Str("monitor traceroute: ").Err(sessionErr).Byte('\n')
				os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
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
			ttl, _ := hop["ttl"].(int)
			addr, _ := hop["addr"].(string)
			tb.Str("  ").Int(int64(ttl)).Str("  ").Str(addr)
			if rtt, ok := hop["rtt-ms"].(float64); ok {
				tb.Str("  ").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str(" ms")
			}
			tb.Byte('\n')
		}
		os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // stdout

		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return 0
		}
	}
	return 0
}
