package fibkernel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	fibevents "github.com/ze-software/ze/internal/plugins/fib/kernel/events"
	fibyang "github.com/ze-software/ze/internal/plugins/fib/kernel/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// configRoot is the YANG container this plugin reads.
const configRoot = "fib/kernel"

// pluginName is the name this plugin registers under, reports as the source
// of a sysctl default, and enrolls its kernel capability with.
const pluginName = "fib-kernel"

type fibConfig struct {
	FlushOnStop bool
	SweepDelay  time.Duration
}

func parseFIBConfig(sections []sdk.ConfigSection) (fibConfig, error) {
	cfg := fibConfig{SweepDelay: sweepDelay}
	for _, sec := range sections {
		if sec.Root != configRoot || sec.Data == "" {
			continue
		}
		var delivered map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &delivered); err != nil {
			return cfg, fmt.Errorf("fib/kernel: invalid config JSON: %w", err)
		}
		// An EMPTY object is the root being REMOVED, not a malformed section.
		// Every producer spells a deleted root that way
		// (buildPluginConfigSectionsTransition and marshalOperationRoot in
		// internal/component/config, reload.go in the plugin server), so
		// refusing it would stop an operator deleting a `fib { kernel { } }`
		// block they can commit today. The defaults stand, which is what the
		// removal asks for.
		if len(delivered) == 0 {
			continue
		}
		// The section arrives wrapped in its full root path, so configRoot
		// "fib/kernel" delivers {"fib":{"kernel":{...}}}. Indexing the outer
		// map by leaf name found nothing and kept every default, which is why
		// neither setting had ever reached a running daemon.
		tree := configvalue.Section(configRoot, delivered)
		if tree == nil {
			return cfg, fmt.Errorf("fib/kernel: config section carries no %q object", configRoot)
		}
		// The delivered map carries every leaf as the string the operator
		// wrote: Tree.values is a map[string]string and toMap copies it
		// through unchanged (internal/component/config/tree.go). A .(bool) or
		// .(float64) assertion here never succeeds, so both settings were
		// silently discarded and the defaults stood whatever the config said.
		//
		// An ABSENT leaf and an UNREADABLE one are separated here rather than
		// inside the reader: both make configvalue answer false, and keeping
		// the default for the second is the same silence this replaced. The
		// map lookup says which, so a malformed value is refused and named.
		if raw, present := tree["flush-on-stop"]; present {
			v, ok := configvalue.Bool(raw)
			if !ok {
				return cfg, fmt.Errorf("fib/kernel: flush-on-stop is %q, want true or false", raw)
			}
			cfg.FlushOnStop = v
		}
		// 0 is a value, not an absence. ze-fib-conf.yang declares sweep-delay
		// as a uint16 with no range, so `sweep-delay 0` commits, and it asks
		// for the sweep to run at once rather than after a reconvergence
		// window. Guarding on v > 0 would keep the 30-second default over it,
		// which is the operator's setting discarded in silence.
		if raw, present := tree["sweep-delay"]; present {
			v, ok := configvalue.Int(raw)
			if !ok || v < 0 {
				return cfg, fmt.Errorf("fib/kernel: sweep-delay is %q, want a whole number of seconds", raw)
			}
			cfg.SweepDelay = time.Duration(v) * time.Second
		}
	}
	return cfg, nil
}

// preservesForwardingState answers whether the routes this plugin installs are
// still in the kernel after ze stops, which is what RFC 4724's Forwarding
// State bit reports to a peer (registry.ForwardingStatePreserved,
// internal/component/plugin/registry/registry.go).
//
// Two facts decide it and both are configuration. The operator must have
// written a `fib { kernel { } }` block, because a plugin that was never
// configured installs no route to preserve. And flush-on-stop must be off,
// because the plugin removes every route it installed as it stops when that
// leaf is true (f.run, fibkernel.go). The default keeps them, and
// ze-fib-conf.yang says so: "The default false keeps those routes in the
// kernel after shutdown, which supports graceful restart."
//
// What happens on the way back up is the other half, and it is why the
// default is the honest answer rather than an optimistic one: startupSweep
// (fibkernel.go) marks the surviving ze routes rather than deleting them, and
// sweepStale removes only the ones sysrib does not refresh inside the sweep
// window. So the forwarding state is continuous across the restart.
//
// A malformed leaf answers false. parseFIBConfig REFUSES such a section, so
// the daemon never reaches a state where this prediction matters, and false
// is the answer that claims nothing.
func preservesForwardingState(tree map[string]any) bool {
	section := configvalue.Section(configRoot, tree)
	if section == nil {
		return false
	}
	raw, present := section["flush-on-stop"]
	if !present {
		return true
	}
	flush, ok := configvalue.Bool(raw)
	if !ok {
		return false
	}
	return !flush
}

func init() {
	// This plugin raises the fib-* warning codes (fibkernel.go); the FIB
	// programming layer owns its health row.
	health.Register("fib", report.HealthProbeDegraded(
		"fib-sync-failure", "fib-orphan", "fib-programming-lag"))

	_ = events.RegisterNamespace(fibevents.Namespace, fibevents.EventExternalChange)

	reg := registry.Registration{
		Name:         pluginName,
		Description:  "FIB kernel: programs OS routes from system RIB via netlink/route socket",
		Features:     "yang",
		YANG:         fibyang.ZeFibConfYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{"rib", "sysctl"},
		// This plugin is the writer for the netlink data plane, the name
		// `interface { backend }` gives the Linux kernel. A route producer
		// that declares NeedsDataPlane is answered with this plugin when the
		// operator wrote no `fib { ... }` block of their own.
		ProgramsFIB:              true,
		DataPlane:                "netlink",
		PreservesForwardingState: preservesForwardingState,
		InProcessConfigVerifier:  verifyFIBConfig,
		RunEngine:                runFIBKernelPlugin,
		Commands:                 commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "fib-kernel: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyFIBConfig(sections []sdk.ConfigSection) error {
	_, err := parseFIBConfig(sections)
	return err
}

func runFIBKernelPlugin(conn net.Conn) int {
	logger().Debug("fib-kernel plugin starting (RPC)")

	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	backend := newBackend()
	f := newFIBKernel(backend)

	var activeJournal *sdk.Journal
	var pendingCfg fibConfig

	p.OnConfigVerify(verifyFIBConfig)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseFIBConfig(sections)
		if err != nil {
			return err
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		j := sdk.NewJournal()
		activeJournal = j
		logger().Info("fib-kernel config applied via transaction")
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("fib-kernel rollback: %d errors", len(errs))
		}
		logger().Info("fib-kernel config rolled back")
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		cfg := pendingCfg

		emitForwardingDefaults()

		stale := f.startupSweep()

		go f.run(ctx, cfg.FlushOnStop)

		if len(stale) > 0 {
			delay := cfg.SweepDelay
			go func() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
					f.sweepStale(stale)
				}
			}()
		}

		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show fib kernel" {
			data := f.showInstalled()
			return "done", data, nil
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
		logger().Error("fib-kernel plugin failed", "error", err)
		return 1
	}

	if err := backend.close(); err != nil {
		logger().Warn("fib-kernel: backend close failed", "error", err)
	}

	return 0
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
			Name:      "show fib kernel",
			ShortHelp: "Show the routes this backend programmed into the Linux forwarding table.",
		},
	}
}
