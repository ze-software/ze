package fibp4

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	fibp4schema "codeberg.org/thomas-mangin/ze/internal/plugins/fibp4/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:         "fib-p4",
		Description:  "FIB P4: programs P4 switch forwarding entries from system RIB via gRPC/P4Runtime",
		Features:     "yang",
		ConfigRoots:  []string{"fib/p4"},
		Dependencies: []string{"rib"},
		YANG:         fibp4schema.ZeFibP4ConfYANG,
		RunEngine:    runFIBP4Plugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		fmt.Fprintf(os.Stderr, "fib-p4: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runFIBP4Plugin(conn net.Conn) int {
	logger().Debug("fib-p4 plugin starting (RPC)")

	p := sdk.NewWithConn("fib-p4", conn)
	defer func() { _ = p.Close() }()

	backend := newBackend("", 0)
	f := newFIBP4(backend)

	var activeJournal *sdk.Journal

	p.OnConfigVerify(func(_ []sdk.ConfigSection) error {
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		j := sdk.NewJournal()
		activeJournal = j
		logger().Info("fib-p4 config applied via transaction")
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("fib-p4 rollback: %d errors", len(errs))
		}
		logger().Info("fib-p4 config rolled back")
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		go f.run(ctx, false)
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "fib-p4 show" {
			data := f.showInstalled()
			return "done", data, nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"fib/p4"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "fib-p4 show"},
		},
	})
	if err != nil {
		logger().Error("fib-p4 plugin failed", "error", err)
		return 1
	}

	if err := backend.close(); err != nil {
		logger().Warn("fib-p4: backend close failed", "error", err)
	}

	return 0
}
