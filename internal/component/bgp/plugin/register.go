// Design: docs/architecture/core-design.md -- BGP plugin registration with ConfigRoots
//
// Package plugin provides the BGP plugin registration for config-driven loading.
// This package imports neither bgp/config, bgp/reactor, nor plugin/server,
// avoiding all import cycles. It accesses the reactor and server through
// interfaces and closures stored in the Coordinator.

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	bgpschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/transaction"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	bgpMu       sync.Mutex
	bgpEventBus ze.EventBus
	bgpServer   registry.PluginServerAccessor
)

func init() {
	_ = events.RegisterNamespace(bgpevents.Namespace,
		bgpevents.EventUpdate, bgpevents.EventOpen, bgpevents.EventNotification,
		bgpevents.EventKeepalive, bgpevents.EventRefresh, bgpevents.EventState,
		bgpevents.EventNegotiated, bgpevents.EventEOR, bgpevents.EventCongested,
		bgpevents.EventResumed, bgpevents.EventRPKI, bgpevents.EventListenerReady,
		bgpevents.EventUpdateNotification, events.DirectionSent,
	)

	reg := registry.Registration{
		Name:               "bgp",
		Description:        "BGP routing daemon",
		Features:           "yang",
		YANG:               bgpschema.ZeBGPConfYANG,
		ConfigRoots:        []string{"bgp"},
		FatalOnConfigError: true,
		RunEngine:          runBGPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			_ = loggerName // BGP uses its own lazy loggers
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				bgpMu.Lock()
				bgpEventBus = e
				bgpMu.Unlock()
			}
		},
		ConfigurePluginServer: func(server any) {
			if s, ok := server.(registry.PluginServerAccessor); ok {
				bgpMu.Lock()
				bgpServer = s
				bgpMu.Unlock()
				s.SetCommitManager(transaction.NewCommitManager())
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

		// Create reactor using the factory registered by bgp/config.
		factoryFn := registry.GetReactorFactory()
		if factoryFn == nil {
			return fmt.Errorf("bgp: no reactor factory registered")
		}

		var err error
		bgpReactor, err = factoryFn(coord)
		if err != nil {
			return fmt.Errorf("bgp: create reactor: %w", err)
		}

		// Wire reactor to hub-owned infrastructure.
		bgpMu.Lock()
		eb := bgpEventBus
		bgpMu.Unlock()

		if eb != nil {
			bgpReactor.SetEventBusAny(eb)
		}

		// Pass plugin server to reactor for EventDispatcher wiring.
		if serverAny := registry.GetPluginServer(); serverAny != nil {
			bgpReactor.SetPluginServerAny(serverAny)
		}

		// Update server with BGP-specific auto-load config.
		families, events, sendTypes := bgpReactor.ConfiguredAutoLoad()
		server.UpdateProtocolConfig(families, events, sendTypes)

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

	// Transaction protocol: verify, apply with journal, rollback.
	var pendingTree map[string]any
	var activeJournal *sdk.Journal

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != "bgp" {
				continue
			}
			// s.Data is the bgp subtree (contents of "bgp { ... }" as
			// produced by ExtractConfigSubtree on the server side) --
			// NOT wrapped in another "bgp" key. Unmarshal directly.
			var bgpTree map[string]any
			if err := json.Unmarshal([]byte(s.Data), &bgpTree); err != nil {
				return fmt.Errorf("bgp verify: unmarshal: %w", err)
			}
			if bgpTree == nil {
				bgpTree = map[string]any{}
			}
			// Validate via reactor (checks peer field constraints).
			if bgpReactor != nil {
				if _, err := bgpReactor.PeerDiffCount(bgpTree); err != nil {
					return fmt.Errorf("bgp verify: %w", err)
				}
			}
			pendingTree = bgpTree
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		tree := pendingTree
		pendingTree = nil
		if tree == nil {
			return nil
		}
		if bgpReactor == nil {
			return fmt.Errorf("bgp apply: no reactor available")
		}
		j := sdk.NewJournal()
		if err := bgpReactor.ReconcilePeersWithJournal(tree, j); err != nil {
			if rollbackErrs := j.Rollback(); len(rollbackErrs) > 0 {
				log.Error("bgp apply: rollback errors", "count", len(rollbackErrs))
			}
			return fmt.Errorf("bgp apply: %w", err)
		}
		activeJournal = j
		log.Info("bgp config applied via transaction")
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("bgp rollback: %d errors", len(errs))
		}
		log.Info("bgp config rolled back")
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"bgp"},
		VerifyBudget: 5,
		ApplyBudget:  30,
	}); err != nil {
		log.Error("bgp plugin failed", "error", err)
		return 1
	}

	// Clean up reactor on exit (AC-7: BGP removed at reload, or daemon shutdown).
	if bgpReactor != nil {
		bgpReactor.Stop()
		_ = bgpReactor.Wait(context.Background())
	}

	// Clear coordinator state so BGP can be re-loaded at a future reload.
	bgpMu.Lock()
	server := bgpServer
	bgpMu.Unlock()
	if server != nil {
		if coord, ok := server.ReactorAny().(registry.CoordinatorAccessor); ok {
			_ = coord.SetReactor(nil)
		}
	}

	return 0
}
