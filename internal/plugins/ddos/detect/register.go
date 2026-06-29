package detect

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/trafficstat"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	detectyang "codeberg.org/thomas-mangin/ze/internal/plugins/ddos/detect/yang"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-detect: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "Automatic DDoS attack detector with two-stage detection",
		Features:     "yang",
		YANG:         detectyang.ZeDdosDetectConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"interface"},
		RunEngine:    runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				eventBusPtr.Store(&e)
			}
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ddos-detect: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-detect plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var det *detector
	var collectSubID int
	var rateSubID int

	// subscribe wires the detector to the trafficstat service (preferred)
	// or falls back to the raw iface tick.
	//
	// EnsureGlobal (not Global): the trafficstat service is created lazily and
	// nothing else guarantees it exists at detector-config time, so Global()
	// would almost always be nil here and we would silently fall back to the raw
	// tick -- defeating the layering. EnsureGlobal makes the detector a real
	// consumer of the shared usage service.
	subscribe := func(d *detector) {
		if svc := trafficstat.EnsureGlobal(); svc != nil {
			rateSubID = svc.SubscribeRates(d.onRates)
			log.Info("ddos-detect: enabled, subscribing to trafficstat")
		} else {
			collectSubID = iface.SubscribeCollectNotify(d.onRate)
			log.Info("ddos-detect: enabled, subscribing to iface rate (trafficstat unavailable)")
		}
	}

	unsubscribe := func() {
		if rateSubID != 0 {
			if svc := trafficstat.Global(); svc != nil {
				svc.UnsubscribeRates(rateSubID)
			}
			rateSubID = 0
		}
		if collectSubID != 0 {
			iface.UnsubscribeCollectNotify(collectSubID)
			collectSubID = 0
		}
	}

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-detect config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-detect config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	var pendingCfg *Config

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		det = newDetector(cfg, bus, p.DispatchCommand)
		if cfg.Enabled {
			unsubscribe()
			subscribe(det)
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
		bus, err := loadBus()
		if err != nil {
			return err
		}
		det = newDetector(cfg, bus, p.DispatchCommand)
		unsubscribe()
		if cfg.Enabled {
			subscribe(det)
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("ddos-detect plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-detect plugin stopped")
	return 0
}
