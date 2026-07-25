package local

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"

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
		resp = newResponder(cfg, bus)
		activeResponder.Store(resp)
		subscribe(bus, resp)
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
			// See ai/rules/fail-closed-guards.md.
			return errApplyWithoutVerify
		}
		unsubscribe()
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = newResponder(cfg, bus)
		activeResponder.Store(resp)
		subscribe(bus, resp)
		// Mirrors the "configured" line OnConfigure emits. Without it a reload that
		// reached the responder and one that never did looked identical in the log,
		// which is what made the forward-mitigation reload failure undiagnosable
		// from a QEMU run (ai/rules/error-messages.md -- one stable phrase per
		// outcome).
		log.Info("ddos-local: reconfigured", "response-level", cfg.ResponseLevel,
			"forward-mitigation", cfg.ForwardMitigation)
		return nil
	})

	p.OnConfigRollback(func(_ string) error { return nil })

	ctx, cancel := sdk.SignalContext()
	defer cancel()
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
