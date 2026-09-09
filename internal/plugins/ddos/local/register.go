package local

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	localyang "github.com/ze-software/ze/internal/plugins/ddos/local/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages the candidate. The message names the stale-config
// consequence, because that is what the operator actually experiences: a reload
// that reports success and changes nothing.
var errApplyWithoutVerify = errors.New("ddos-local config apply: no verified config staged (config-apply arrived without config-verify); refusing to report success over the previous config")

var eventBusPtr atomic.Pointer[ze.EventBus]

// activeResponder publishes the live responder to the in-process show handler
// (show.go). Nil when the plugin is not configured/running.
var activeResponder atomic.Pointer[responder]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-local: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "DDoS local responder: on-host nft drop on attack detection",
		Features:     "yang",
		YANG:         localyang.ZeDdosLocalConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"firewall"},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			eventBusPtr.Store(&eb)
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ddos-local: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// maxDurationCheckInterval is how often the plugin re-checks whether a live drop
// rule has outlived max-mitigation-duration. One second is finer than any cap
// the YANG admits (the shortest meaningful value is 1) and costs one wakeup per
// second holding no lock unless a rule is installed. It keeps the name its
// flowspec sibling uses, so a reader who greps one finds both.
const maxDurationCheckInterval = time.Second

// startMaxDurationWorker starts the one worker goroutine that removes a drop
// rule which has outlived max-mitigation-duration.
//
// One worker serves the whole plugin, never one per mitigation: it reads
// whichever responder is live through activeResponder, so a config apply that
// replaces the responder needs no restart here
// (ai/rules/goroutine-lifecycle.md). enforceMaxDuration is a no-op unless a
// rule is installed and the operator set a cap, so an unattacked box pays one
// wakeup a second and nothing else.
//
// The caller owns ctx and MUST cancel it to stop the worker. The returned
// channel is closed after the worker has exited, so a caller that must know the
// worker is gone waits on it after the cancel.
func startMaxDurationWorker(ctx context.Context, every time.Duration) (exited <-chan struct{}) {
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if r := activeResponder.Load(); r != nil {
					r.enforceMaxDuration()
				}
			}
		}
	}()

	return done
}

// replaceResponder builds the responder for cfg, carries a live drop rule across
// from the responder it replaces, and publishes it to the cap worker and to the
// show handler.
//
// Carrying is what stops a config apply orphaning a rule. The firewall registry
// is keyed by TABLE NAME rather than by responder, so the nftables table
// applyMitigation registered outlives the responder that registered it. A fresh
// idle responder believes no rule exists, and both removal paths return on
// !active -- enforceMaxDuration and onCleared alike -- so the drop would be
// bounded by neither the operator's cap nor a clear, for the life of the daemon,
// while show ddos local reported no mitigation.
//
// prev MAY be nil, which is the first configure.
func replaceResponder(cfg *Config, bus eventBus, prev *responder) *responder {
	r := newResponder(cfg, bus)
	r.adoptMitigation(prev)
	activeResponder.Store(r)
	return r
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-local plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var resp *responder
	defer activeResponder.Store(nil)

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-local config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-local config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	var (
		pendingCfg         *Config
		unsubDetect        func()
		unsubCharacterized func()
		unsubCleared       func()
	)

	subscribe := func(bus ze.EventBus, r *responder) {
		unsubDetect = ddosevent.Detected.Subscribe(bus, r.onDetected)
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, r.onCharacterized)
		unsubCleared = ddosevent.Cleared.Subscribe(bus, r.onCleared)
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
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, bus, resp)
		subscribe(bus, resp)

		// One empty reconcile while the one-time removal of the tables an older
		// ze build wrote is still pending. This responder's own table is one of
		// them, and it registers nothing until an attack arrives, so a box that
		// is never attacked gets no other reconcile that could reach it
		// (internal/component/firewall/legacy_tables.go).
		//
		// Reported and not returned: a daemon whose firewall backend is unusable
		// must still detect and report.
		if firewall.LegacySweepPending() {
			if err := applyAll(); err != nil {
				log.Warn("ddos-local: the one-time removal of an older ze build's tables did not run", "error", err)
			}
		}
		log.Info("ddos-local: configured", "response-level", cfg.ResponseLevel)
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			// Fail closed. The reload transaction drives verify and apply over the
			// SAME participant set -- runTxCoordinator builds both from `affected`
			// (plugin/server/reload_tx.go:40-45) -- so reaching apply without the
			// config OnConfigVerify stages here is a protocol violation, not a
			// normal state. Returning nil accepted the transaction while silently
			// keeping the PREVIOUS responder, so `ddos { local { forward-mitigation
			// false } }` could be reloaded, the daemon would print "sighup reload
			// complete", and the old value kept mitigating -- with nothing logged at
			// any level to say so. Reject instead, so the reload fails loudly and
			// rolls back rather than reporting success over stale config.
			// See ai/rules/evidence.md.
			return errApplyWithoutVerify
		}
		unsubscribe()
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = replaceResponder(cfg, bus, resp)
		subscribe(bus, resp)
		// Mirrors the "configured" line OnConfigure emits. Without it a reload that
		// reached the responder and one that never did looked identical in the log,
		// which is what made the forward-mitigation reload failure undiagnosable
		// from a QEMU run (ai/rules/cli.md -- one stable phrase per
		// outcome).
		log.Info("ddos-local: reconfigured", "response-level", cfg.ResponseLevel,
			"forward-mitigation", cfg.ForwardMitigation)
		return nil
	})

	p.OnConfigRollback(func(_ string) error { return nil })

	ctx, cancel := sdk.SignalContext()

	// The cap worker outlives every config apply and stops when the signal
	// context is canceled, which is the plugin's own shutdown. The wait after the
	// cancel is owed: a tick in flight sits inside enforceMaxDuration ->
	// removeMitigation -> applyAll, a netlink round trip, so returning on the
	// cancel alone would report the engine done while it was still writing the
	// kernel (ai/rules/goroutine-lifecycle.md).
	workerExited := startMaxDurationWorker(ctx, maxDurationCheckInterval)
	defer func() {
		cancel()
		<-workerExited
	}()

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("ddos-local plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-local plugin stopped")
	return 0
}
