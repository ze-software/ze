package fibkernel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	fibevents "codeberg.org/thomas-mangin/ze/internal/plugins/fib/kernel/events"
	fibschema "codeberg.org/thomas-mangin/ze/internal/plugins/fib/kernel/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

type fibConfig struct {
	FlushOnStop bool
	SweepDelay  time.Duration
}

func parseFIBConfig(sections []sdk.ConfigSection) (fibConfig, error) {
	cfg := fibConfig{SweepDelay: sweepDelay}
	for _, sec := range sections {
		if sec.Root != "fib/kernel" || sec.Data == "" {
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
	_ = events.RegisterNamespace(fibevents.Namespace, fibevents.EventExternalChange)

	reg := registry.Registration{
		Name:         "fib-kernel",
		Description:  "FIB kernel: programs OS routes from system RIB via netlink/route socket",
		Features:     "yang",
		YANG:         fibschema.ZeFibConfYANG,
		ConfigRoots:  []string{"fib/kernel"},
		Dependencies: []string{"rib", "sysctl"},
		RunEngine:    runFIBKernelPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				SetMetricsRegistry(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
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

func runFIBKernelPlugin(conn net.Conn) int {
	logger().Debug("fib-kernel plugin starting (RPC)")

	p := sdk.NewWithConn("fib-kernel", conn)
	defer func() { _ = p.Close() }()

	backend := newBackend()
	f := newFIBKernel(backend)

	var activeJournal *sdk.Journal
	var pendingCfg fibConfig

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		_, err := parseFIBConfig(sections)
		return err
	})

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

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "fib-kernel show" {
			data := f.showInstalled()
			return "done", data, nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"fib/kernel"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "fib-kernel show"},
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
