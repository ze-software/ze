package fibkernel

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	fibschema "codeberg.org/thomas-mangin/ze/internal/plugins/fibkernel/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:         "fib-kernel",
		Description:  "FIB kernel: programs OS routes from system RIB via netlink/route socket",
		Features:     "yang",
		YANG:         fibschema.ZeFibConfYANG,
		ConfigRoots:  []string{"fib.kernel"},
		Dependencies: []string{"sysrib"},
		RunEngine:    runFIBKernelPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureBus: func(bus any) {
			if b, ok := bus.(ze.Bus); ok {
				setBus(b)
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

	p.OnConfigVerify(func(_ []sdk.ConfigSection) error {
		// fib-kernel has no config fields to validate yet;
		// accept any config change that reaches us.
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		// fib-kernel config is minimal (route table settings).
		// Journal records the apply for potential rollback.
		j := sdk.NewJournal()
		// No-op apply: fib-kernel reacts to sysrib bus events, not config directly.
		// The journal is kept for protocol compliance (rollback = no-op).
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
		// Startup sweep for crash recovery.
		stale := f.startupSweep()

		go f.run(ctx, false)

		// Sweep stale routes after a short delay to allow reconvergence.
		if len(stale) > 0 {
			go func() {
				select {
				case <-ctx.Done():
					return
				case <-sweepTimer():
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

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"fib.kernel"},
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
