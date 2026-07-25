package flowspec

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"

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

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-flowspec config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-flowspec config: %w", err)
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
		resp = newResponder(cfg, dispatcher)
		activeResponder.Store(resp)
		subscribe(bus, resp)
		log.Info("ddos-flowspec: configured", "response-level", cfg.ResponseLevel, "action", cfg.Action)
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
			return nil
		}
		unsubscribe()
		bus, err := loadBus()
		if err != nil {
			return err
		}
		resp = newResponder(cfg, dispatcher)
		activeResponder.Store(resp)
		subscribe(bus, resp)
		return nil
	})

	p.OnConfigRollback(func(_ string) error { return nil })

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
