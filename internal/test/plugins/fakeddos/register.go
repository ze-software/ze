// Package fakeddos is a test-only plugin that drives the ddos-local responder
// end-to-end against a REAL nft backend. On configure it emits a synthetic
// ddosevent.AttackDetected (which makes ddos-local install the ze_ddos-local drop
// table) and, once the test driver creates a trigger file, emits AttackCleared
// (which makes ddos-local withdraw it).
//
// It exists to prove the "ze_" ownership prefix on ddos-local's table lets a
// cleared mitigation be swept from the kernel -- the withdraw leak fixed in
// plan/learned/1116-copp-firewall-shutdown-flush.md. The responder's own unit
// tests mock the firewall backend, so only a functional test through the real
// backend can show the table actually disappears.
//
// A trigger FILE, not a signal, is the "clear now" channel: ze has no spare
// unhandled signal (SIGUSR1 is the daemon status dump; an unhandled SIGUSR2 hits
// Go's default disposition and terminates the daemon). The driver writes the file
// in the daemon's working directory -- the same directory it reads daemon.pid /
// daemon.ready from -- so no path coordination is needed.
package fakeddos

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	fakeddosyang "github.com/ze-software/ze/internal/test/plugins/fakeddos/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	// Name is the plugin/registry name.
	Name = "ddos-fake"
	// configRoot is the nested YANG path (ddos/fake): the plugin augments the
	// shared `ddos` container with a `fake` subtree, mirroring ddos-local's
	// `ddos/local`. The injector starts only when `enabled true` is set there.
	configRoot = "ddos/fake"
)

func init() {
	reg := registry.Registration{
		Name:         Name,
		Description:  "Test-only synthetic DDoS attack injector for the ddos-local withdraw test (harmless unless `ddos { fake { enabled true; } }` is configured)",
		Features:     "yang",
		YANG:         fakeddosyang.ZeFakeddosConfYANG,
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
		fmt.Fprintf(os.Stderr, "fakeddos: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := logger()
	log.Debug("fakeddos plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	started := false
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		if started {
			return nil
		}
		active, err := activationFromSections(sections)
		if err != nil {
			return err
		}
		if !active {
			log.Info("fakeddos: configured but not enabled, injector inert")
			return nil
		}
		bus, err := loadBus()
		if err != nil {
			return err
		}
		started = true
		// The driver's "clear now" channel: it creates clearTriggerFile in the
		// daemon's working directory once it has observed the mitigation. Poll for
		// it and, when it appears, unblock runScenario's withdraw.
		clear := make(chan struct{}, 1)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(50 * time.Millisecond):
				}
				if _, err := os.Stat(clearTriggerFile); err == nil {
					select {
					case clear <- struct{}{}:
					default:
					}
					return
				}
			}
		}()
		go runScenario(ctx, bus, clear)
		return nil
	})

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 2,
		ApplyBudget:  10,
	}); err != nil {
		log.Error("fakeddos plugin failed", "error", err)
		return 1
	}
	log.Info("fakeddos plugin stopped")
	return 0
}
