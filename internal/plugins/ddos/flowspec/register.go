package flowspec

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	flowspecyang "github.com/ze-software/ze/internal/plugins/ddos/flowspec/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

// activeResponder publishes the live responder to the in-process show handler
// (show.go). Nil when the plugin is not configured/running.
var activeResponder atomic.Pointer[responder]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-flowspec: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "DDoS FlowSpec/RTBH responder: upstream mitigation with leak-probe clear",
		Features:    "yang",
		YANG:        flowspecyang.ZeDdosFlowspecConfYANG,
		ConfigRoots: []string{configRoot},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ddos-flowspec: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages its candidate. Returning nil accepted the
// transaction while silently keeping the PREVIOUS responder, so the daemon
// would print "sighup reload complete" over config it never installed.
var errApplyWithoutVerify = errors.New("ddos-flowspec config apply: no verified config staged (config-apply arrived without config-verify); refusing to report success over the previous config")

// pendingConfig is what OnConfigVerify stages for OnConfigApply: the parsed
// config, and whether the delivered section carried a `ddos flowspec` body at
// all.
//
// The two are one type because the apply cannot recover the second from the
// first. A removal delivers an empty body, which parses to the same values as a
// block naming no leaf, so the answer has to travel; and a path that cleared one
// while leaving the other would arm the next apply with a mismatched pair.
//
// NOT safe for concurrent use, and it does not need to be: the plugin SDK drives
// verify, apply and rollback for one transaction on one goroutine.
type pendingConfig struct {
	cfg        *Config
	configured bool
}

// stage records the candidate a config-verify accepted.
func (p *pendingConfig) stage(cfg *Config, configured bool) {
	p.cfg = cfg
	p.configured = configured
}

// take returns the staged candidate and unstages it, so a second apply behind
// one verify gets a nil config and the caller's fail-closed branch.
func (p *pendingConfig) take() (cfg *Config, configured bool) {
	cfg, configured = p.cfg, p.configured
	p.clear()
	return cfg, configured
}

// clear unstages the candidate without applying it. The rollback path calls it:
// a transaction that verified here and then failed elsewhere never reaches this
// plugin's apply, and a candidate left staged is applied by the NEXT transaction
// that reaches an apply without a verify. That is the state the apply's nil
// check exists to refuse, and a stale candidate defeats it -- the pair a
// rolled-back removal leaves behind is (DefaultConfig, false), so the next apply
// would withdraw a live announcement nobody asked it to withdraw.
func (p *pendingConfig) clear() {
	p.cfg = nil
	p.configured = false
}

// rollback is the plugin's config-rollback handler, and unstaging the candidate
// is the whole of its job.
func (p *pendingConfig) rollback(_ string) error {
	p.clear()
	return nil
}

// replaceResponder builds the responder for cfg, decides what becomes of an
// announcement the responder it replaces made, and publishes it to the cap
// worker and to the show handler.
//
// configured says the delivered config still carries a `ddos flowspec` section.
//
// The default is to CARRY the announcement, and carrying is what stops an
// ordinary commit orphaning it (adoptAnnouncement states what crosses and why).
// Two commits say the opposite, that the box must stop announcing, and each
// withdraws instead. Both go through the OUTGOING responder, because the
// incoming one has announced nothing and its own removal paths return on
// !active:
//
//   - The section is gone. The plugin is STOPPED as soon as this reload
//     transaction commits (Server.collectProcessesForRemovedConfigPaths,
//     internal/component/plugin/server/startup_autoload.go), while the FlowSpec
//     NLRI sits in the BGP engine's RIB and in every peer's. A carried
//     announcement would have no responder left to withdraw it and no cap worker
//     left to time it out, so the peer would discard the victim's traffic for the
//     life of the daemon. The withdrawal is on `configured` and not on the parsed
//     response-level, which a removed section leaves at the DefaultConfig value
//     of alert: reading the removal off that default would make the guard vanish
//     the day the default changes, with no line deleted and no test red
//     (docs/contributing/ze-go-style.md -- a zero value is never an answer).
//   - response-level left enforce. `alert` is "detect and report, do not act"
//     (onCharacterized logs "would announce" and returns), and lifting an
//     upstream discard is what an operator commits it for. Neither
//     enforceMaxDuration nor probeTick reads response-level, so a carried
//     announcement would outlive the instruction by up to
//     max-mitigation-duration, 3600 seconds by default, with show ddos flowspec
//     naming a rule the operator has just asked to end.
//
// prev MAY be nil, which is the first configure.
//
// Whatever became of the announcement, prev is RETIRED here, inside the same
// critical section: it is unreachable from this point (not in activeResponder,
// unsubscribed), so anything still holding a pointer to it must not be able to
// announce through it. An event dispatched before the unsubscribe is exactly
// such a holder (responder.retire, responder.go).
func replaceResponder(cfg *Config, dispatcher routeDispatcher, prev *responder, configured bool) *responder {
	r := newResponder(cfg, dispatcher)

	if prev != nil {
		// One critical section covers the whole handover: what becomes of the
		// outgoing responder's rule, the carry into the new one, and the publish.
		// A cap tick or a probe tick that raced the swap then finds prev either
		// before the handover or after it, never halfway through.
		prev.mu.Lock()
		defer prev.mu.Unlock()

		switch {
		case !configured:
			prev.withdrawForConfig(withdrawSectionRemoved)
		case cfg.ResponseLevel != responseEnforce:
			prev.withdrawForConfig(withdrawNotEnforcing)
		}
		prev.retire()
	}

	r.adoptAnnouncement(prev)
	activeResponder.Store(r)
	return r
}

// maxDurationCheckInterval is how often the plugin re-checks whether a live
// announce has outlived max-mitigation-duration. One second is finer than any
// cap the YANG admits (the minimum meaningful value is 1) and costs one wakeup
// per second holding no lock unless an announce is live.
const maxDurationCheckInterval = time.Second

// sdkDispatcher is the production routeDispatcher: it sends rendered update-text
// commands to the BGP engine via the plugin SDK's UpdateRoute over the RPC path.
type sdkDispatcher struct {
	p   *sdk.Plugin
	ctx context.Context
}

func (d sdkDispatcher) Dispatch(command string) error {
	_, _, err := d.p.UpdateRoute(d.ctx, flowspecSelector, command)
	return err
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-flowspec plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	dispatcher := sdkDispatcher{p: p, ctx: ctx}

	var resp *responder
	defer activeResponder.Store(nil)

	parseSections := func(sections []sdk.ConfigSection) (*Config, bool, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, configured, err := ParseConfig(s.Data)
			if err != nil {
				return nil, false, fmt.Errorf("ddos-flowspec config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, false, fmt.Errorf("ddos-flowspec config: %w", err)
			}
			return cfg, configured, nil
		}
		return DefaultConfig(), false, nil
	}

	var (
		pending            pendingConfig
		unsubDetect        func()
		unsubCharacterized func()
		unsubCleared       func()
		unsubOngoing       func()
	)

	subscribe := func(bus ze.EventBus, r *responder) {
		unsubDetect = ddosevent.Detected.Subscribe(bus, r.onDetected)
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, r.onCharacterized)
		unsubCleared = ddosevent.Cleared.Subscribe(bus, r.onCleared)
		// The leak probe's only driver. Without it probe.Tick never runs outside
		// tests, and onCleared ignores the detector's clear while mitigating, so
		// nothing withdraws an announce at all.
		unsubOngoing = ddosevent.Ongoing.Subscribe(bus, r.onOngoing)
	}

	unsubscribe := func() {
		if unsubDetect != nil {
			unsubDetect()
		}
		if unsubCharacterized != nil {
			unsubCharacterized()
		}
		if unsubCleared != nil {
			unsubCleared()
		}
		if unsubOngoing != nil {
			unsubOngoing()
		}
	}

	// One long-lived worker for the whole plugin, not one per announce: it reads
	// whichever responder is live through activeResponder, so a config apply that
	// replaces the responder needs no restart here, and there is no goroutine per
	// event (ai/rules/goroutine-lifecycle.md). It selects on ctx, which
	// sdk.SignalContext cancels at shutdown.
	//
	// enforceMaxDuration is a no-op unless an announce is live and the operator
	// set a cap, so this ticks cheaply through the common idle case.
	go func() {
		t := time.NewTicker(maxDurationCheckInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if r := activeResponder.Load(); r != nil {
					r.enforceMaxDuration()
				}
			}
		}
	}()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, configured, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, dispatcher, resp, configured)
		subscribe(bus, resp)
		log.Info("ddos-flowspec: configured", "response-level", cfg.ResponseLevel, "action", cfg.Action)
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, configured, err := parseSections(sections)
		if err != nil {
			return err
		}
		pending.stage(cfg, configured)
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg, configured := pending.take()
		if cfg == nil {
			// Fail closed. The reload transaction drives verify and apply over the
			// SAME participant set -- runTxCoordinator builds both from `affected`
			// (plugin/server/reload_tx.go) -- so reaching apply without the config
			// OnConfigVerify stages here is a protocol violation, not a normal
			// state. Returning nil accepted the transaction while silently keeping
			// the PREVIOUS responder, with nothing logged at any level to say so.
			return errApplyWithoutVerify
		}
		unsubscribe()
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, dispatcher, resp, configured)
		subscribe(bus, resp)
		// Mirrors the "configured" line OnConfigure emits, so a reload that reached
		// the responder and one that never did do not look identical in the log
		// (ai/rules/cli.md -- one stable phrase per outcome). `announced` is read
		// through status(), which takes no lock, because the responder's mu is held
		// across a dispatcher round trip to the BGP engine.
		announced, _, _ := resp.status()
		log.Info("ddos-flowspec: reconfigured", "response-level", cfg.ResponseLevel,
			"action", cfg.Action, "announced", announced)
		return nil
	})

	// A rollback is the end of the transaction, so the candidate this plugin
	// verified is unstaged rather than left for an apply that will never ask for
	// it (pendingConfig.clear).
	p.OnConfigRollback(pending.rollback)

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("ddos-flowspec plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-flowspec plugin stopped")
	return 0
}
