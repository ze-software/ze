// Design: docs/architecture/core-design.md -- BGP plugin registration with ConfigRoots
//
// Package plugin provides the BGP plugin registration for config-driven loading.
// This package imports neither bgp/config, bgp/reactor, nor plugin/server,
// avoiding all import cycles. It accesses the reactor and server through
// interfaces and closures stored in the Coordinator.

package plugin

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	bgpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Coordinator extra keys used by the hub to pass state to RunEngine.
const (
	// KeyCreateReactor holds a func() (BGPReactorHandle, error) closure.
	KeyCreateReactor = "bgp.createReactor"
)

var (
	bgpMu     sync.Mutex
	bgpBus    ze.Bus
	bgpServer registry.PluginServerAccessor
)

func init() {
	reg := registry.Registration{
		Name:        "bgp",
		Description: "BGP routing daemon",
		Features:    "yang",
		YANG:        bgpschema.ZeBGPConfYANG,
		ConfigRoots: []string{"bgp"},
		RunEngine:   runBGPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			_ = loggerName // BGP uses its own lazy loggers
		},
		ConfigureBus: func(bus any) {
			if b, ok := bus.(ze.Bus); ok {
				bgpMu.Lock()
				bgpBus = b
				bgpMu.Unlock()
			}
		},
		ConfigurePluginServer: func(server any) {
			if s, ok := server.(registry.PluginServerAccessor); ok {
				bgpMu.Lock()
				bgpServer = s
				bgpMu.Unlock()
			}
		},
		CLIHandler: func(_ []string) int {
			return 1
		},
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bgp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runBGPEngine is the engine-mode entry point for the BGP plugin.
func runBGPEngine(conn net.Conn) int {
	log := slogutil.Logger("bgp.plugin")
	log.Debug("bgp plugin starting")

	p := sdk.NewWithConn("bgp", conn)
	defer func() { _ = p.Close() }()

	var bgpReactor registry.BGPReactorHandle

	p.OnConfigure(func(_ []sdk.ConfigSection) error {
		bgpMu.Lock()
		server := bgpServer
		bgpMu.Unlock()

		if server == nil {
			return fmt.Errorf("bgp: no plugin server available")
		}

		coord, ok := server.ReactorAny().(registry.CoordinatorAccessor)
		if !ok {
			return fmt.Errorf("bgp: server reactor is not a Coordinator")
		}

		// Retrieve the reactor factory stored by the hub.
		createFn, ok := coord.GetExtra(KeyCreateReactor).(func() (registry.BGPReactorHandle, error))
		if !ok || createFn == nil {
			return fmt.Errorf("bgp: no reactor factory in coordinator")
		}

		var err error
		bgpReactor, err = createFn()
		if err != nil {
			return fmt.Errorf("bgp: create reactor: %w", err)
		}

		// Wire reactor to hub-owned infrastructure.
		bgpMu.Lock()
		bus := bgpBus
		bgpMu.Unlock()

		if bus != nil {
			bgpReactor.SetBusAny(bus)
		}

		// Pass plugin server to reactor for EventDispatcher wiring.
		if serverAny := registry.GetPluginServer(); serverAny != nil {
			bgpReactor.SetPluginServerAny(serverAny)
		}

		// Update server with BGP-specific auto-load config.
		families, events, sendTypes := bgpReactor.ConfiguredAutoLoad()
		server.UpdateBGPConfig(families, events, sendTypes)

		// Register reactor with coordinator for ReactorLifecycle delegation.
		if err := coord.SetReactor(bgpReactor.ReactorLifecycleAdapter()); err != nil {
			return fmt.Errorf("bgp: register reactor: %w", err)
		}

		// Start reactor (listeners, wiring). Peers are deferred: the externalServer
		// flag skips peer startup in StartWithContext to avoid validate-open
		// callbacks arriving before tier 1+ plugins complete their handshake.
		if err := bgpReactor.StartWithContext(context.Background()); err != nil {
			return fmt.Errorf("bgp: start reactor: %w", err)
		}
		log.Info("bgp reactor started (peers deferred)")

		// Register peer startup as post-startup callback. The coordinator
		// calls this when SignalPluginStartupComplete fires (after all tiers
		// and explicit plugins finish their 5-stage protocol).
		coord.OnPostStartup(func() {
			if peerErr := bgpReactor.StartPeers(); peerErr != nil {
				log.Error("bgp: start peers failed", "error", peerErr)
				return
			}
			log.Info("bgp peers started")
		})

		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("bgp plugin failed", "error", err)
		return 1
	}

	if bgpReactor != nil {
		_ = bgpReactor.Wait(context.Background())
	}

	return 0
}
