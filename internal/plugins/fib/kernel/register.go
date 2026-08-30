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
		var tree map[string]any
		if err := json.Unmarshal([]byte(sec.Data), &tree); err != nil {
			return cfg, fmt.Errorf("fib/kernel: invalid config JSON: %w", err)
		}
		if v, ok := tree["flush-on-stop"].(bool); ok {
			cfg.FlushOnStop = v
		}
		if v, ok := tree["sweep-delay"].(float64); ok && v > 0 {
			cfg.SweepDelay = time.Duration(v) * time.Second
		}
	}
	return cfg, nil
}

func init() {
	// This plugin raises the fib-* warning codes (fibkernel.go); the FIB
	// programming layer owns its health row.
	health.Register("fib", report.HealthProbeDegraded(
		"fib-sync-failure", "fib-orphan", "fib-programming-lag"))

	_ = events.RegisterNamespace(fibevents.Namespace, fibevents.EventExternalChange)

	reg := registry.Registration{
		Name:                    "fib-kernel",
		Description:             "FIB kernel: programs OS routes from system RIB via netlink/route socket",
		Features:                "yang",
		YANG:                    fibyang.ZeFibConfYANG,
		ConfigRoots:             []string{configRoot},
		Dependencies:            []string{"rib", "sysctl"},
		InProcessConfigVerifier: verifyFIBConfig,
		RunEngine:               runFIBKernelPlugin,
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

	p := sdk.NewWithConn("fib-kernel", conn)
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
		Commands: []sdk.CommandDecl{
			{Name: "show fib kernel"},
		},
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
