package flowtriq

import (
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

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
		cl                 *client
		activeUUID         string
		activeFamily       ddosevent.AttackFamily
		activePeakPPS      float64
		activePeakBPS      float64
		activeConfidence   int
		attackStart        time.Time
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

	subscribe := func(bus ze.EventBus) {
		unsubDetected = ddosevent.Detected.Subscribe(bus, func(e *ddosevent.AttackDetected) {
			if cl == nil {
				return
			}
			uuid, err := cl.openIncident(e)
			if err != nil {
				log.Warn("ddos-flowtriq: open incident failed", "error", err)
				return
			}
			activeUUID = uuid
			activeFamily = e.Family
			activePeakPPS = e.PeakRxPps
			activePeakBPS = e.PeakRxBps
			activeConfidence = 0
			attackStart = time.Now()
			log.Info("ddos-flowtriq: incident opened", "uuid", uuid)
		})
		// Characterized carries the confidence score (unavailable at Detected time);
		// capture it so the next update and the resolve report it to the dashboard.
		unsubCharacterized = ddosevent.Characterized.Subscribe(bus, func(e *ddosevent.AttackCharacterized) {
			if activeUUID != "" {
				activeConfidence = e.Confidence
			}
		})
		unsubOngoing = ddosevent.Ongoing.Subscribe(bus, func(e *ddosevent.AttackOngoing) {
			if cl == nil || activeUUID == "" {
				return
			}
			if e.CurrentPps > activePeakPPS {
				activePeakPPS = e.CurrentPps
			}
			if e.CurrentBps > activePeakBPS {
				activePeakBPS = e.CurrentBps
			}
			if err := cl.updateIncident(activeUUID, e.CurrentPps, e.CurrentBps, activeFamily, activeConfidence); err != nil {
				log.Warn("ddos-flowtriq: update incident failed", "error", err)
			}
		})
		unsubCleared = ddosevent.Cleared.Subscribe(bus, func(_ *ddosevent.AttackCleared) {
			if cl == nil || activeUUID == "" {
				return
			}
			duration := time.Since(attackStart).Seconds()
			if err := cl.resolveIncident(activeUUID, duration, activePeakPPS, activePeakBPS, activeConfidence); err != nil {
				log.Warn("ddos-flowtriq: resolve incident failed", "error", err)
			} else {
				log.Info("ddos-flowtriq: incident resolved", "uuid", activeUUID, "duration", duration)
			}
			activeUUID = ""
			activeConfidence = 0
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

	var pendingCfg *Config

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseSections(sections)
		if err != nil {
			return err
		}
		if cfg.Enabled {
			cl = newClient(cfg.APIBase, cfg.APIKey, cfg.NodeUUID)
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
		if cl != nil && activeUUID != "" {
			duration := time.Since(attackStart).Seconds()
			if err := cl.resolveIncident(activeUUID, duration, activePeakPPS, activePeakBPS, activeConfidence); err != nil {
				log.Warn("ddos-flowtriq: resolve on config reload failed", "error", err)
			}
			activeUUID = ""
			activeConfidence = 0
		}
		unsubscribe()
		if cfg.Enabled {
			cl = newClient(cfg.APIBase, cfg.APIKey, cfg.NodeUUID)
			bus, busErr := loadBus()
			if busErr != nil {
				return busErr
			}
			subscribe(bus)
		} else {
			cl = nil
		}
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
		log.Error("ddos-flowtriq plugin failed", "error", err)
		unsubscribe()
		return 1
	}

	unsubscribe()
	log.Info("ddos-flowtriq plugin stopped")
	return 0
}
