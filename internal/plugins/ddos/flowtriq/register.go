package flowtriq

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	flowtriqyang "github.com/ze-software/ze/internal/plugins/ddos/flowtriq/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-flowtriq: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "DDoS incident reporter for Flowtriq cloud API",
		Features:    "yang",
		YANG:        flowtriqyang.ZeDdosFlowtriqConfYANG,
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
		fmt.Fprintf(os.Stderr, "ddos-flowtriq: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-flowtriq plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var (
		rep                reporter
		unsubDetected      func()
		unsubCharacterized func()
		unsubOngoing       func()
		unsubCleared       func()
	)

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-flowtriq config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-flowtriq config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	// The four handles are written here and read by unsubscribe, and both run
	// on the SDK's reader goroutine (OnConfigure, OnConfigApply, and the tail of
	// this function once Run has returned). The incident state the callbacks
	// share with the detector's goroutines lives in reporter, which guards it.
	subscribe := func(bus ze.EventBus) {
		unsubDetected = ddosevent.Detected.Subscribe(bus, rep.onDetected)
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, rep.onCharacterized)
		unsubOngoing = ddosevent.Ongoing.Subscribe(bus, rep.onOngoing)
		unsubCleared = ddosevent.Cleared.Subscribe(bus, rep.onCleared)
	}

	unsubscribe := func() {
		if unsubDetected != nil {
			unsubDetected()
		}
		if unsubCharacterized != nil {
			unsubCharacterized()
		}
		if unsubOngoing != nil {
			unsubOngoing()
		}
		if unsubCleared != nil {
			unsubCleared()
		}
	}

	var pendingCfg *Config

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		if cfg.Enabled {
			rep.swapClient(newClient(cfg.APIBase, cfg.APIKey, cfg.NodeUUID))
			bus, busErr := loadBus()
			if busErr != nil {
				return busErr
			}
			subscribe(bus)
			log.Info("ddos-flowtriq: enabled, reporting to Flowtriq API")
		}
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
		// pendingCfg needs no lock: OnConfigVerify writes it and this reads it,
		// and both are SDK callbacks the one reader goroutine runs in sequence.
		// The incident state is the part a detector goroutine also touches, and
		// swapClient is what orders this apply against a delivery in flight.
		unsubscribe()
		if !cfg.Enabled {
			rep.swapClient(nil)
			return nil
		}
		rep.swapClient(newClient(cfg.APIBase, cfg.APIKey, cfg.NodeUUID))
		bus, busErr := loadBus()
		if busErr != nil {
			return busErr
		}
		subscribe(bus)
		return nil
	})

	p.OnConfigRollback(func(_ string) error { return nil })

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		// 20 rather than 10 because an apply now waits for a delivery already
		// inside a callback before it takes the lock, and a callback holds it
		// across one API post, which the HTTP client caps at 10 seconds
		// (newClient, client.go). The apply's own resolveIncident post is a
		// second one, so the budget covers two.
		ApplyBudget: 20,
	}); err != nil {
		log.Error("ddos-flowtriq plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-flowtriq plugin stopped")
	return 0
}
