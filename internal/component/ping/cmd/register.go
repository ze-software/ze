// Overview: doc.go -- package doc + schema import
//
// register.go wires the ping module into ze's two registries from init():
//   - the plugin server RPC registry, for the daemon-side show/monitor/resolve
//     ping handlers, and
//   - the local command registry, for offline `show ping` and `monitor ping`.
//
// The module is reached by the daemon through scripts/codegen/plugin_imports.go
// rpcDirs (internal/component/ping/cmd) and by the `ze` binary through plugin/all.

package cmd

import (
	"context"
	"os"
	"os/signal"
	"strconv"

	"github.com/ze-software/ze/internal/component/command/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-show:ping", Handler: handleShowPing},
		pluginserver.RPCRegistration{WireMethod: "ze-monitor:ping", Handler: handleMonitorPing},
		pluginserver.RPCRegistration{WireMethod: "ze-resolve:ping", Handler: handleResolvePing},
	)

	registry.MustRegisterLocalMeta("show ping", showPingLocal, registry.Meta{
		Description: "Ping a target using the internal ICMP engine (works without the daemon)",
		Mode:        "offline",
	})

	registry.MustRegisterLocalMeta("monitor ping", monitorPingLocal, registry.Meta{
		Description: "Continuous ping with live statistics (works without the daemon)",
		Mode:        "offline",
	})
}

func showPingLocal(args []string) int {
	dest, count, timeout, opts, err := parsePingArgs(args)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str("show ping: ").Err(err).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}
	results, pingErr := doPing(dest, count, timeout, opts)
	if pingErr != nil {
		var tb textbuf.Buffer
		tb.Str("show ping: ").Err(pingErr).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}
	printPingResults(os.Stdout, results)
	return 0
}

func monitorPingLocal(args []string) int {
	mp, err := parseMonitorPingArgs(args)
	if err != nil {
		var tb textbuf.Buffer
		tb.Err(err).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}
	dest := mp.Dest

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ch, cancel, sessionErr := NewPingSession(ctx, dest.String(), mp.Interval, mp.Timeout, mp.Count, mp.Size)
	if sessionErr != nil {
		var tb textbuf.Buffer
		tb.Str("monitor ping: ").Err(sessionErr).Byte('\n')
		os.Stderr.WriteString(tb.Slice()) //nolint:errcheck // stderr
		return 1
	}
	defer cancel()

	var tb textbuf.Buffer
	tb.Str("PING ").Str(dest.String()).Str(" (Ctrl-C to stop)\n")
	os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // stdout

	for result := range ch {
		tb.Reset(0)
		seq, _ := result["seq"].(int)
		status, _ := result["status"].(string)
		tb.Str("  seq=").Int(int64(seq))
		if status == "ok" {
			rtt, _ := result["rtt-ms"].(float64)
			tb.Str("  rtt=").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str("ms\n")
		} else {
			tb.Str("  ").Str(status).Byte('\n')
		}
		os.Stdout.WriteString(tb.Slice()) //nolint:errcheck // stdout
	}
	return 0
}
