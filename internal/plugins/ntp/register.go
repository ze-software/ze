package ntp

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	ntpevents "codeberg.org/thomas-mangin/ze/internal/plugins/ntp/events"
	ntpschema "codeberg.org/thomas-mangin/ze/internal/plugins/ntp/schema"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// eventBusMu guards eventBusRef.
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

func setEventBus(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBus() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() {
	_ = events.RegisterNamespace(ntpevents.Namespace, ntpevents.EventClockSynced)

	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:        "ntp",
		Description: "NTP client: system clock synchronization",
		Features:    "yang",
		YANG:        ntpschema.ZeNTPConfYANG,
		ConfigRoots: []string{"environment"},
		RunEngine:   runNTPPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureEventBus = func(eb any) {
		if e, ok := eb.(ze.EventBus); ok {
			setEventBus(e)
		}
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ntp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runNTPPlugin is the engine-mode entry point for the NTP plugin.
func runNTPPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("ntp plugin starting")

	p := sdk.NewWithConn("ntp", conn)
	defer func() { _ = p.Close() }()

	var worker *syncWorker
	var unsubscribe func()

	// startWorker stops any existing worker, then starts a new one
	// with the given config. Safe to call multiple times (reload).
	startWorker := func(cfg ntpConfig) {
		if worker != nil {
			worker.stopAndWait()
			worker = nil
		}
		if unsubscribe != nil {
			unsubscribe()
			unsubscribe = nil
		}
		if !cfg.Enabled {
			log.Debug("ntp: disabled in config")
			return
		}
		worker = newSyncWorker(cfg, getEventBus())
		worker.start()
		log.Info("ntp: sync worker started",
			"servers", cfg.Servers, "interval", cfg.IntervalSec)

		eb := getEventBus()
		if eb != nil {
			unsubscribe = subscribeDHCP(eb, worker)
		}
	}

	// pendingCfg holds config between verify and apply phases.
	var pendingCfg *ntpConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != "environment" {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			startWorker(cfg)
			return nil
		}
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != "environment" {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			pendingCfg = &cfg
			return nil
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			return nil
		}
		startWorker(*cfg)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"environment"},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("ntp plugin failed", "error", err)
		return 1
	}

	// Shutdown: save time and stop worker.
	if worker != nil {
		if worker.cfg.PersistPath != "" {
			if err := saveTime(worker.cfg.PersistPath, currentTime()); err != nil {
				log.Debug("ntp: final time save failed", "err", err)
			}
		}
		worker.stopAndWait()
	}
	if unsubscribe != nil {
		unsubscribe()
	}

	log.Info("ntp plugin stopped")
	return 0
}

// currentTime returns the current system time.
func currentTime() time.Time {
	return time.Now()
}
