package fibvpp

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/statestore"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
	fibvppyang "github.com/ze-software/ze/internal/plugins/fib/vpp/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
	"github.com/ze-software/ze/pkg/zefs"
)

// configRoot is the YANG container this plugin reads. The plugin registers as
// "fib-vpp"; the config path is "fib/vpp".
const configRoot = "fib/vpp"

var srv6OwnershipKey = zefs.MustRegister(zefs.KeyEntry{
	Pattern:     srv6StatePrefix + "{kind}/{identity}",
	Description: "VPP SRv6 policy, steering and ordinary fallback ownership (JSON)",
})

func init() {
	statestore.RegisterPluginKeys("fib-vpp", srv6OwnershipKey)
	reg := registry.Registration{
		Name:         "fib-vpp",
		Description:  "FIB VPP: programs VPP FIB entries from system RIB via GoVPP binary API",
		Features:     "yang",
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"rib", "vpp"},
		// This plugin is the writer for the vpp data plane, the name
		// `interface { backend }` gives VPP. A route producer that declares
		// NeedsDataPlane is answered with this plugin on a VPP deployment,
		// which is why such a deployment needs no `fib { vpp { } }` block to
		// have its static routes programmed.
		ProgramsFIB: true,
		DataPlane:   "vpp",
		YANG:        fibvppyang.ZeFibVPPConfYANG,
		RunEngine:   runFibVPPPlugin,
		Commands:    commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setFibVPPLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setFibVPPEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setFibVPPLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "fib-vpp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runFibVPPPlugin(conn net.Conn) int {
	lg := logger()
	lg.Debug("fib-vpp plugin starting")

	p := sdk.NewWithConn("fib-vpp", conn)
	defer func() { _ = p.Close() }()

	// The event bus delivers Connected and Reconnected on its own goroutines,
	// and the "show fib vpp" handler answers on another, so the state a
	// reconnect rewrites is guarded. Safe for concurrent use.
	var fibMu sync.Mutex
	var tableID uint32
	var fib *fibVPP
	var runCancel context.CancelFunc

	var vppUnsub func() // VPP reconnect subscription cleanup
	var bestChangeUnsub func()
	defer func() {
		fibMu.Lock()
		defer fibMu.Unlock()
		if fib != nil {
			fib.mu.Lock()
			fib.retired = true
			fib.mu.Unlock()
		}
		if bestChangeUnsub != nil {
			bestChangeUnsub()
		}
	}()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRoot {
				continue
			}
			parsed, err := parseFibVPPConfigSection(s.Data)
			if err != nil {
				return fmt.Errorf("fib-vpp: parse config: %w", err)
			}
			fibMu.Lock()
			tableID = parsed.tableID
			fibMu.Unlock()
		}
		return nil
	})

	var activeJournal *sdk.Journal

	p.OnConfigVerify(func(_ []sdk.ConfigSection) error {
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		j := sdk.NewJournal()
		activeJournal = j
		lg.Info("fib-vpp config applied via transaction")
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("fib-vpp rollback: %d errors", len(errs))
		}
		lg.Info("fib-vpp config rolled back")
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		// newBackend returns a COMPLETE fibVPP. The MPLS and SRv6 backends are
		// attached before the value is published, so a reader can never see a
		// fibVPP whose backends are still nil.
		newBackend := func(table uint32) *fibVPP {
			connector := vppcomp.GetActiveConnector()
			if connector == nil {
				lg.Warn("fib-vpp: VPP connector not available, using noop backend")
				return newFibVPP(&mockBackend{})
			}
			ch, err := connector.NewChannel()
			if err != nil {
				lg.Warn("fib-vpp: GoVPP channel failed, using noop backend", "error", err)
				return newFibVPP(&mockBackend{})
			}
			f := newFibVPP(newGovppBackend(ch, table))
			f.mplsBackend = newGovppMPLSBackend(ch, table)
			f.srv6TableID = table
			f.srv6Backend = newGovppSRv6Backend(ch, table, p.StateKeys(ctx))
			if err := f.restoreSRv6(); err != nil {
				lg.Error("fib-vpp: SRv6 ownership restore failed", "error", err)
			}
			return f
		}

		// The plugin-lifetime subscription MUST precede restore and survive
		// replacement. Its callback shares fibMu so an event arriving during
		// restore reaches the replacement, never a retired writer.
		restart := func() {
			fibMu.Lock()
			defer fibMu.Unlock()
			if bestChangeUnsub == nil {
				if eb := getEventBus(); eb != nil {
					bestChangeUnsub = subscribeFibVPP(eb, &fibMu, &fib)
				}
			}

			if fib != nil {
				fib.mu.Lock()
				fib.retired = true
				if err := fib.backend.close(); err != nil {
					lg.Error("fib-vpp: retire backend failed", "error", err)
				}
				fib.mu.Unlock()
			}
			if runCancel != nil {
				runCancel()
			}
			next := newBackend(tableID)
			var runCtx context.Context
			runCtx, runCancel = context.WithCancel(ctx)
			fib = next
			go next.run(runCtx, false)
		}

		eb := getEventBus()
		if eb != nil {
			onVPPReady := events.AsString(func(event string) {
				lg.Info("fib-vpp: VPP ready, reinitializing backend", "event", event)
				restart()
			})
			unsub1 := eb.Subscribe(vppevents.Namespace, vppevents.EventConnected, onVPPReady)
			unsub2 := eb.Subscribe(vppevents.Namespace, vppevents.EventReconnected, onVPPReady)
			vppUnsub = func() { unsub1(); unsub2() }
		}

		restart()
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show fib vpp" {
			fibMu.Lock()
			f := fib
			fibMu.Unlock()
			if f == nil {
				return "done", "[]", nil
			}
			return "done", f.showInstalled(), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands:     commandDecls(),
	})
	if err != nil {
		lg.Error("fib-vpp plugin failed", "error", err)
		return 1
	}

	if vppUnsub != nil {
		vppUnsub()
	}

	return 0
}

// subscribeFibVPP routes each change to the current writer under the lifecycle
// lock. The caller MUST hold mu while subscribing and replacing current, and
// MUST unsubscribe when the plugin stops.
func subscribeFibVPP(eb ze.EventBus, mu *sync.Mutex, current **fibVPP) func() {
	return sysribevents.BestChange.Subscribe(eb, func(batch *incomingBatch) {
		mu.Lock()
		defer mu.Unlock()
		if *current != nil {
			(*current).processEvent(batch)
		}
	})
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      "show fib vpp",
			ShortHelp: "Show the routes this backend programmed into the VPP forwarding table.",
		},
	}
}
