// Design: docs/architecture/rib-transition.md — watchdog plugin extraction
// Detail: server.go — command dispatch and state management
// Detail: config.go — config tree parser
// Detail: pool.go — route pool management
//
// Package bgp_watchdog implements a watchdog route management plugin for ze.
// It manages per-peer config-based watchdog groups. Routes are injected
// into the engine via "update text" commands.
package watchdog

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// watchdogMetrics holds Prometheus metrics for the watchdog plugin.
type watchdogMetrics struct {
	peersUp            metrics.Gauge   // currently up peers
	routeAnnouncements metrics.Counter // routes announced to peers
	routeWithdrawals   metrics.Counter // routes withdrawn from peers
}

// watchdogMetricsPtr stores watchdog metrics, set by SetMetricsRegistry.
const configRootBGP = "bgp"

var watchdogMetricsPtr atomic.Pointer[watchdogMetrics]

// SetMetricsRegistry creates watchdog metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	m := &watchdogMetrics{
		peersUp:            reg.Gauge("ze_watchdog_peers_up", "Watchdog peers with established sessions."),
		routeAnnouncements: reg.Counter("ze_watchdog_route_announcements_total", "Watchdog routes announced to peers."),
		routeWithdrawals:   reg.Counter("ze_watchdog_route_withdrawals_total", "Watchdog routes withdrawn from peers."),
	}
	watchdogMetricsPtr.Store(m)
}

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setLogger sets the package-level logger.
// Called from register.go ConfigureEngineLogger and ConfigLogger callbacks.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// runWatchdogPlugin runs the watchdog plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
//
// Lifecycle:
//  1. OnConfigure — parses config tree, builds per-peer route pools
//  2. SetStartupSubscriptions — subscribes to state events
//  3. OnEvent — handles peer up/down, resends announced routes
//  4. OnExecuteCommand — handles request bgp watchdog announce/withdraw commands
func runWatchdogPlugin(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-watchdog", conn)
	defer func() { _ = p.Close() }()

	srv := newWatchdogServer(func(peer, cmd string) {
		ctx := context.Background()
		_, _, err := p.UpdateRoute(ctx, peer, cmd)
		if err != nil {
			logger().Warn("update-route failed", "peer", peer, "error", err)
		}
	}, func(peer string, initialReplay uint64) {
		// Report the exact session whose peer-up routes have completed.
		var tb textbuf.Buffer
		command := tb.Str("request peer ").Str(peer).Str(" plugin session ready session ").Uint(initialReplay).String()
		ctx := context.Background()
		if _, _, err := p.DispatchCommand(ctx, command); err != nil {
			logger().Warn("plugin session ready failed", "peer", peer, "error", err)
		}
	})

	// Stage 2: receive config, extract per-peer watchdog route definitions
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			peerPools, err := parseConfig(section.Data)
			if err != nil {
				logger().Error("config parse failed", "error", err)
				return err
			}
			srv.mu.Lock()
			srv.peerPools = peerPools
			srv.mu.Unlock()
			logger().Info("config loaded", "peers", len(peerPools))
		}
		return nil
	})

	// Subscribe to state events for peer up/down lifecycle
	p.SetStartupSubscriptions([]string{"state"}, nil, "")
	p.SetEncoding("text")

	// Runtime: handle structured events (peer state changes via DirectBridge).
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			if se.State == rpc.SessionStateUp {
				srv.handleStateUp(se.PeerAddress, se.InitialReplay)
			} else if se.State != rpc.SessionStateUnspecified {
				srv.handleStateDown(se.PeerAddress)
			}
		}
		return nil
	})

	// Fallback: handle text events for non-DirectBridge delivery (external plugins).
	p.OnEvent(func(eventStr string) error {
		peerAddr, state, initialReplay := parseStateEvent(eventStr)
		if peerAddr == "" {
			return nil // Not a state event we care about
		}
		if state == "up" {
			srv.handleStateUp(peerAddr, initialReplay)
		} else {
			srv.handleStateDown(peerAddr)
		}
		return nil
	})

	// Runtime: handle watchdog commands from CLI
	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return srv.handleCommand(command, args, peer)
	})

	logger().Info("watchdog plugin starting")
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{configRootBGP},
		Commands:    commandDecls(),
	})
	if err != nil {
		logger().Error("watchdog plugin failed", "error", err)
		return 1
	}
	return 0
}

// parseStateEvent extracts peer address, state and replay token from a text state event.
// Format: "peer 10.0.0.1 remote as 65001 state up initial-replay 42\n".
// An unrecognized event or malformed token returns an empty peer address.
func parseStateEvent(text string) (peerAddr, state string, initialReplay uint64) {
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 4 {
		return "", "", 0
	}
	if fields[0] != "peer" {
		return "", "", 0
	}
	for i := 2; i < len(fields); i++ {
		switch fields[i] {
		case "state":
			i++
			if i == len(fields) {
				return "", "", 0
			}
			state = fields[i]
		case "initial-replay":
			i++
			if i == len(fields) {
				return "", "", 0
			}
			token, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				return "", "", 0
			}
			initialReplay = token
		}
	}
	if state == "" {
		return "", "", 0
	}
	return fields[1], state, initialReplay
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: commandRequestWatchdogAnnounce, ShortHelp: "Announce routes in watchdog group"},
		{Name: commandRequestWatchdogWithdraw, ShortHelp: "Withdraw routes in watchdog group"},
	}
}
