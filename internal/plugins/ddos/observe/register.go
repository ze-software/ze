package observe

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/internal/core/slogutil"
	observeyang "github.com/ze-software/ze/internal/plugins/ddos/observe/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var eventBusPtr atomic.Pointer[ze.EventBus]

// activeStore publishes the live incident ring to the in-process show handlers
// (show.go). The plugin runs as a goroutine, so the handler reads it directly.
// Nil when the plugin is not configured/running.
var activeStore atomic.Pointer[store]

func loadBus() (ze.EventBus, error) {
	p := eventBusPtr.Load()
	if p == nil {
		return nil, fmt.Errorf("ddos-observe: event bus not configured")
	}
	return *p, nil
}

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "DDoS observability: incident store and show ddos status/incidents CLI",
		Features:    "yang",
		YANG:        observeyang.ZeDdosObserveConfYANG,
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
		fmt.Fprintf(os.Stderr, "ddos-observe: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("ddos-observe plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	var incidentStore *store
	defer activeStore.Store(nil)

	parseSections := func(sections []sdk.ConfigSection) (*Config, error) {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			cfg, err := ParseConfig(s.Data)
			if err != nil {
				return nil, fmt.Errorf("ddos-observe config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return nil, fmt.Errorf("ddos-observe config: %w", err)
			}
			return cfg, nil
		}
		return DefaultConfig(), nil
	}

	var (
		pendingCfg         *Config
		unsubDetected      func()
		unsubCharacterized func()
		unsubOngoing       func()
		unsubCleared       func()
	)

	subscribe := func(bus ze.EventBus, s *store) {
		unsubDetected = ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) {
			s.open(e)
		})
		// Characterized carries the confidence score and refined signals; record the
		// confidence onto the incident the matching Detected already opened.
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) {
			s.characterize(e)
		})
		unsubOngoing = ddosevent.Ongoing.Subscribe(bus, func(_ *ddosevent.AttackOngoing) {})
		unsubCleared = ddosevent.Cleared.Subscribe(bus, func(e *ddosevent.AttackCleared) {
			s.finalize(e.Target)
		})
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

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		staleTimeout := time.Duration(cfg.StaleIncidentTimeout) * time.Second
		incidentStore = newStore(cfg.IncidentRingSize, staleTimeout)
		activeStore.Store(incidentStore)
		subscribe(bus, incidentStore)
		log.Info("ddos-observe: configured", "ring-size", cfg.IncidentRingSize)
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
		staleTimeout := time.Duration(cfg.StaleIncidentTimeout) * time.Second
		incidentStore = newStore(cfg.IncidentRingSize, staleTimeout)
		activeStore.Store(incidentStore)
		subscribe(bus, incidentStore)
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
		log.Error("ddos-observe plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-observe plugin stopped")
	return 0
}
