// Design: docs/features/interfaces.md — Interface plugin registration
// Overview: iface.go — shared types and topic constants

package iface

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"

	ifaceschema "codeberg.org/thomas-mangin/ze/internal/component/iface/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
// Stored as atomic.Pointer to avoid data races when tests start
// multiple in-process plugin instances concurrently.
var loggerPtr atomic.Pointer[slog.Logger]

// busMu guards busRef. An interface cannot be stored in atomic.Pointer
// directly, so a mutex is used instead.
var (
	busMu  sync.Mutex
	busRef ze.Bus
)

// SetBus sets the package-level Bus reference used by the monitor.
// MUST be called before RunEngine starts the monitor. The engine calls
// this during plugin startup to inject the Bus dependency.
func SetBus(b ze.Bus) {
	busMu.Lock()
	defer busMu.Unlock()
	busRef = b
}

// GetBus returns the package-level Bus reference, or nil if not set.
func GetBus() ze.Bus {
	busMu.Lock()
	defer busMu.Unlock()
	return busRef
}

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:        "interface",
		Description: "OS network interface monitoring and management",
		Features:    "yang",
		YANG:        ifaceschema.ZeIfaceConfYANG,
		ConfigRoots: []string{"interface"},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureBus: func(bus any) {
			if b, ok := bus.(ze.Bus); ok {
				SetBus(b)
			}
		},
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "interface: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// setLogger sets the package-level logger.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// runEngine is the engine-mode entry point for the interface plugin.
// It uses the SDK 5-stage protocol to receive configuration, starts
// the netlink interface monitor, and blocks until shutdown.
func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("interface plugin starting")

	p := sdk.NewWithConn("interface", conn)
	defer func() { _ = p.Close() }()

	// pendingCfg holds the validated config between verify and apply phases.
	var pendingCfg *ifaceConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg := parseIfaceSections(sections)

		if cfg.Backend == "" {
			return fmt.Errorf("interface: no backend configured and no OS default available")
		}

		if err := LoadBackend(cfg.Backend); err != nil {
			return fmt.Errorf("interface backend %q: %w", cfg.Backend, err)
		}
		log.Info("interface backend loaded", "backend", cfg.Backend)

		b := GetBackend()

		if errs := applyConfig(cfg, b); len(errs) > 0 {
			return joinApplyErrors("interface config", errs)
		}
		log.Info("interface config applied")

		bus := GetBus()
		if bus == nil {
			log.Warn("interface plugin: no bus configured, monitor will not start")
			return nil
		}

		if err := b.StartMonitor(bus); err != nil {
			return fmt.Errorf("interface monitor start: %w", err)
		}
		log.Info("interface monitor started")
		return nil
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg := parseIfaceSections(sections)
		if cfg.Backend == "" {
			return fmt.Errorf("interface: no backend configured and no OS default available")
		}
		pendingCfg = cfg
		log.Debug("interface config verified", "backend", cfg.Backend)
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			log.Warn("interface config apply: no pending config (verify not called?)")
			return nil
		}

		b := GetBackend()
		if b == nil {
			return fmt.Errorf("interface config apply: no backend loaded")
		}

		if errs := applyConfig(cfg, b); len(errs) > 0 {
			return joinApplyErrors("interface reload", errs)
		}
		log.Info("interface config reloaded")
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("interface plugin failed", "error", err)
		return 1
	}

	if err := CloseBackend(); err != nil {
		log.Warn("interface backend close failed", "error", err)
	}
	log.Info("interface backend closed")

	return 0
}

// joinApplyErrors logs each error at Warn level and returns a short summary
// for the status line. Detailed errors are visible via log output.
func joinApplyErrors(prefix string, errs []error) error {
	log := loggerPtr.Load()
	for _, e := range errs {
		log.Warn(prefix, "err", e)
	}
	if len(errs) == 1 {
		return fmt.Errorf("%s: %w", prefix, errs[0])
	}
	return fmt.Errorf("%s: %d errors (see log for details)", prefix, len(errs))
}
