// Design: plan/spec-iface-0-umbrella.md — Interface plugin registration
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

// getBus returns the package-level Bus reference, or nil if not set.
func getBus() ze.Bus {
	busMu.Lock()
	defer busMu.Unlock()
	return busRef
}

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:        "iface",
		Description: "OS network interface monitoring and management",
		Features:    "yang",
		YANG:        ifaceschema.ZeIfaceConfYANG,
		ConfigRoots: []string{"interface"},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(_ []string) int {
		return 1
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "iface: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// setLogger sets the package-level logger.
func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// runEngine is the engine-mode entry point for the iface plugin.
// It uses the SDK 5-stage protocol to receive configuration, starts
// the netlink interface monitor, and blocks until shutdown.
func runEngine(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("iface plugin starting")

	p := sdk.NewWithConn("iface", conn)
	defer func() { _ = p.Close() }()

	var mon *Monitor

	p.OnConfigure(func(_ []sdk.ConfigSection) error {
		bus := getBus()
		if bus == nil {
			log.Warn("iface plugin: no bus configured, monitor will not start")
			return nil
		}

		var err error
		mon, err = NewMonitor(bus)
		if err != nil {
			return fmt.Errorf("iface monitor create: %w", err)
		}
		if err = mon.Start(); err != nil {
			mon = nil
			return fmt.Errorf("iface monitor start: %w", err)
		}
		log.Info("iface monitor started")
		return nil
	})

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("iface plugin failed", "error", err)
		return 1
	}

	if mon != nil {
		mon.Stop()
		log.Info("iface monitor stopped")
	}

	return 0
}
