package rs

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rs/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	_ = events.RegisterNamespace(ribevents.Namespace, ribevents.EventValidationChange)
	// Activate the reactor's route-server fast-path forwarding capability
	// (BGP-owned filterapi seam, not the generic registry). This is the sole
	// caller of EnableRSForwarding: deleting this plugin package removes the
	// activation, leaving the reactor RS fast path inert -- the "delete the
	// folder and the feature vanishes" invariant. The reactor reads the flag
	// once at construction; it never imports this package.
	filterapi.EnableRSForwarding()

	reg := registry.Registration{
		Name:         "bgp-rs",
		Description:  "Route Server",
		RFCs:         []string{"7947"},
		ConfigRoots:  []string{"bgp"},
		Features:     "yang",
		YANG:         yang.ZeRsConfYANG,
		Dependencies: []string{"bgp-rib"},
		// bgp-adj-rib-in is optional. Without it, peer-up replay uses the
		// mandatory RIB for authorized FlowSpec paths; other families require
		// the receive store for replay.
		OptionalDependencies: []string{"bgp-adj-rib-in"},
		// This plugin drives peer-up replay explicitly (replayForPeer), so
		// bgp-adj-rib-in must not also self-replay -- with both firing, a route
		// learned just before a peer established went out twice. Declaring the
		// role here rather than dispatching a command at OnAllPluginsReady is
		// what makes the stand-down deterministic: the engine delivers the claim
		// on bgp-adj-rib-in's Stage-2 configure, which completes before peers
		// start. See ClaimPeerUpReplay in server_handlers.go.
		Claims: []string{ClaimPeerUpReplay},
		// handleState makes a peer a live forward target (Up + ForwardFrom, one
		// critical section, server_handlers.go). An UPDATE taken delivery of
		// before that lands at or below the peer's cut, so it belongs to the
		// announce-only Adj-RIB-In replay and its withdrawals are lost. Declaring
		// the barrier makes the engine hold this peer's initial-sync End-of-RIB
		// until this plugin has taken delivery of the peer-up event, so a peer
		// that waits for the End-of-RIB before sending cannot land in that window.
		PeerUpBarrier: true,
		// The route-server replay to a client that establishes is that client's
		// initial routing update, and sendEOR reports when it is out
		// (server_handlers.go). Distinct from PeerUpBarrier above: that one says
		// this plugin has REGISTERED the peer, this one says its ROUTES are out.
		SignalsSessionReady: true,
		RunEngine:           RunRouteServer,
		Commands:            commandDecls(),
		ConfigureEventBus: func(bus ze.EventBus) {
			validationBus.Store(&bus)
		},
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rs: registration failed: %v\n", err)
		os.Exit(1)
	}
}
