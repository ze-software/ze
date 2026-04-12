package ntp

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
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

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != "environment" {
				continue
			}
			cfg, err := parseNTPConfig(s.Data)
			if err != nil {
				return fmt.Errorf("ntp: %w", err)
			}
			if !cfg.Enabled {
				log.Debug("ntp: disabled in config")
				return nil
			}

			worker = newSyncWorker(cfg)
			worker.start()
			log.Info("ntp: sync worker started",
				"servers", cfg.Servers, "interval", cfg.IntervalSec)

			// Subscribe to DHCP events for option 42 NTP servers.
			eb := getEventBus()
			if eb != nil {
				unsubscribe = subscribeDHCP(eb, worker)
			}
			return nil
		}
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"environment"},
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
