// Design: docs/architecture/core-design.md -- BGP plugin registration with ConfigRoots
//
// Package plugin provides the BGP plugin registration for config-driven loading.
// This package imports neither bgp/config, bgp/reactor, nor plugin/server,
// avoiding all import cycles. It accesses the reactor and server through
// interfaces and closures stored in the Coordinator.
//
// The reactor factory it calls at OnConfigure is registered by bgp/config's
// init(); that package is linked from the gated CLI composition root
// cmd/ze/dispatch_bgp.go, NOT from here -- bgp/config's own tests import
// plugin/all, so an import edge from this package back into it is an import
// cycle in test (spec-feature-gate-10-bgp).

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp/transaction"
	bgpyang "github.com/ze-software/ze/internal/component/bgp/yang"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errBgpNoPluginServerAvailable    = errors.New("bgp: no plugin server available")
	errBgpServerReactorIsNotA        = errors.New("bgp: server reactor is not a Coordinator")
	errBgpNoReactorFactoryRegistered = errors.New("bgp: no reactor factory registered")
	errBgpApplyNoReactorAvailable    = errors.New("bgp apply: no reactor available")
)

var (
	bgpMu       sync.Mutex
	bgpEventBus ze.EventBus
	bgpServer   registry.PluginServerAccessor
)

func init() {
	// The reactor raises these codes (reactor/session_health.go); the BGP
	// subsystem owns its health row (ai/rules/plugin-self-containment.md).
	health.Register("bgp", report.HealthProbeDegraded(
		"session-stuck", "session-flap", "eor-timeout"))

	_ = events.RegisterNamespace(bgpevents.Namespace,
		bgpevents.EventUpdate, bgpevents.EventOpen, bgpevents.EventNotification,
		bgpevents.EventKeepalive, bgpevents.EventRefresh, bgpevents.EventState,
		bgpevents.EventNegotiated, bgpevents.EventEOR, bgpevents.EventCongested,
		bgpevents.EventResumed, bgpevents.EventRPKI, bgpevents.EventListenerReady,
		bgpevents.EventUpdateNotification,
		events.DirectionSent, // "sent" is a config receive flag, not a true event type
	)

	// BGP owns the namespace for plugin-declared event types and
	// namespace-less plugin RPC subscriptions; the plugin host stays
	// protocol-neutral (ai/rules/plugin-self-containment.md).
	zeplugin.RegisterDefaultEventNamespace(bgpevents.Namespace)

	reg := registry.Registration{
		Name:               "bgp",
		Description:        "BGP routing daemon",
		Features:           "yang",
		YANG:               bgpyang.ZeBGPConfYANG,
		ConfigRoots:        []string{"bgp"},
		FatalOnConfigError: true,
		RunEngine:          runBGPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			_ = loggerName // BGP uses its own lazy loggers
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			bgpMu.Lock()
			bgpEventBus = eb
			bgpMu.Unlock()
		},
		ConfigurePluginServer: func(server registry.PluginServerAccessor) {
			bgpMu.Lock()
			bgpServer = server
			bgpMu.Unlock()
			server.SetCommitManager(transaction.NewCommitManager())
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
			return errBgpNoPluginServerAvailable
		}

		coord, ok := server.ReactorAny().(registry.CoordinatorAccessor)
		if !ok {
			return errBgpServerReactorIsNotA
		}

		// Create reactor using the factory registered by bgp/config.
		factoryFn := registry.GetReactorFactory()
		if factoryFn == nil {
			return errBgpNoReactorFactoryRegistered
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
	operationJournals := make(map[string]*sdk.Journal)
	var operationMu sync.Mutex

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
			return errBgpApplyNoReactorAvailable
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
		pendingTree = nil
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

	p.OnConfigOperationDecompose(func(input sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
		return decomposeBGPOperationInput(context.Background(), input)
	})

	p.OnConfigOperationVerify(func(input sdk.ConfigOperationVerifyInput) error {
		return verifyBGPOperation(&input.Operation, bgpReactor)
	})

	p.OnConfigOperationApply(func(input sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
		j := sdk.NewJournal()
		out, err := applyBGPOperation(&input.Operation, bgpReactor, j)
		if err != nil {
			if rollbackErrs := j.Rollback(); len(rollbackErrs) > 0 {
				log.Error("bgp operation apply: rollback errors", "count", len(rollbackErrs))
			}
			return nil, err
		}
		operationMu.Lock()
		operationJournals[operationJournalKey(input.TransactionID, input.Operation.ID)] = j
		operationMu.Unlock()
		return out, nil
	})

	p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
		operationMu.Lock()
		defer operationMu.Unlock()
		for i := range input.Operations {
			op := &input.Operations[i]
			key := operationJournalKey(input.TransactionID, op.ID)
			j := operationJournals[key]
			delete(operationJournals, key)
			if j == nil {
				continue
			}
			if errs := j.Rollback(); len(errs) > 0 {
				return fmt.Errorf("bgp operation rollback %s: %d errors", op.ID, len(errs))
			}
		}
		return nil
	})

	p.OnConfigOperationCommit(func(input sdk.ConfigOperationCommitInput) error {
		operationMu.Lock()
		for key, j := range operationJournals {
			if strings.HasPrefix(key, input.TransactionID+"\x00") {
				j.Discard()
				delete(operationJournals, key)
			}
		}
		operationMu.Unlock()
		pendingTree = nil
		log.Info("bgp config operation journal committed")
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"bgp"},
		ConfigOperations: []sdk.ConfigOperationDecl{{
			Root:      configRootBGP,
			Decompose: true,
			Operations: []sdk.ConfigOperationType{
				sdk.OperationAddPeer,
				sdk.OperationRemovePeer,
				sdk.OperationModifyPeer,
			},
		}},
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
