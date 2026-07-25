package redistributeegress

import (
	"context"
	"net"
	"strings"

	bgpredist "github.com/ze-software/ze/internal/component/bgp/redistribute"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// Compile-time contract: the BGP redistribute consumer satisfies the generic
// RedistConsumer seam it is registered under (below). Stated here so the
// cross-package BGPConsumer type is an explicit dependency of this plugin.
var _ configredist.RedistConsumer = (*bgpredist.BGPConsumer)(nil)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Redistribute orchestrator: dispatches protocol route events to registered consumers",
		ConfigRoots: []string{"redistribute"},
		// BGP dependency: the orchestrator registers a BGP consumer whose
		// UpdateRoute dispatches via this plugin's SDK connection to the reactor.
		// When future protocols add consumers, each will register in its own plugin.
		Dependencies: []string{"bgp"},
		RunEngine:    runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(r metrics.Registry) {
			setMetricsRegistry(r)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: " + Name + " registration failed: " + err.Error())
	}
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	// Peer-up trigger: fire a targeted redistribute replay to a peer that
	// establishes after an injection (spec-redistribute-late-join-replay).
	coord := newReplayCoordinator()
	setReplayCoordinator(coord)

	p.OnStarted(func(ctx context.Context) error {
		// p provides UpdateRoute, satisfying the RouteDispatcher seam the BGP
		// consumer wraps to dispatch redistributed routes to the reactor.
		var _ bgpredist.RouteDispatcher = p
		consumer := bgpredist.NewBGPConsumer(p)
		if err := configredist.RegisterConsumer(consumer); err != nil {
			logger().Warn("failed to register BGP consumer", "error", err)
		}
		go run(ctx)
		return nil
	})

	// Subscribe to peer state; on a down->up edge fire a targeted replay so a
	// newly-established peer receives the current redistribute route set. Mirrors
	// the watchdog's state-subscription pattern (in-process DirectBridge via
	// OnStructuredEvent, text fallback via OnEvent).
	p.SetStartupSubscriptions([]string{"state"}, nil, "")
	p.SetEncoding("text")
	p.OnStructuredEvent(func(events []any) error {
		bus := getEventBus()
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			if se.State == rpc.SessionStateUp {
				coord.onPeerUp(bus, se.PeerAddress)
			} else if se.State != rpc.SessionStateUnspecified {
				coord.onPeerDown(se.PeerAddress)
			}
		}
		return nil
	})
	p.OnEvent(func(eventStr string) error {
		peerAddr, state := parseStateEvent(eventStr)
		if peerAddr == "" {
			return nil
		}
		bus := getEventBus()
		if state == "up" {
			coord.onPeerUp(bus, peerAddr)
		} else {
			coord.onPeerDown(peerAddr)
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

// parseStateEvent extracts the peer address and state from a text state event
// (the non-DirectBridge / external-plugin delivery path). Format:
// "peer 10.0.0.1 remote as 65001 state up\n". Returns ("", "") when the line is
// not a recognized state event. Mirrors the watchdog plugin's parser.
func parseStateEvent(text string) (peerAddr, state string) {
	fields := strings.Fields(strings.TrimRight(text, "\n"))
	if len(fields) < 4 || fields[0] != "peer" {
		return "", ""
	}
	addr := fields[1]
	for i := 2; i < len(fields)-1; i++ {
		if fields[i] == "state" {
			return addr, fields[i+1]
		}
	}
	return "", ""
}
