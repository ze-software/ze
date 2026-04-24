package fibvpp

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	vppcomp "codeberg.org/thomas-mangin/ze/internal/component/vpp"
	vppevents "codeberg.org/thomas-mangin/ze/internal/component/vpp/events"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	fibvppschema "codeberg.org/thomas-mangin/ze/internal/plugins/fib/vpp/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:         "fib-vpp",
		Description:  "FIB VPP: programs VPP FIB entries from system RIB via GoVPP binary API",
		Features:     "yang",
		ConfigRoots:  []string{"fib/vpp"},
		Dependencies: []string{"rib", "vpp"},
		YANG:         fibvppschema.ZeFibVppConfYANG,
		RunEngine:    runFibVPPPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setFibVPPLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				SetMetricsRegistry(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setFibVPPEventBus(e)
			}
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

	var tableID uint32
	var fib *fibVPP     // shared across OnStarted and OnExecuteCommand
	var vppUnsub func() // VPP reconnect subscription cleanup

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != "fib/vpp" {
				continue
			}
			parsed, err := parseFibVPPConfigSection(s.Data)
			if err != nil {
				return fmt.Errorf("fib-vpp: parse config: %w", err)
			}
			tableID = parsed.tableID
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
		var runCancel context.CancelFunc

		initBackend := func() {
			connector := vppcomp.GetActiveConnector()
			if connector == nil {
				lg.Warn("fib-vpp: VPP connector not available, using noop backend")
				fib = newFibVPP(&mockBackend{})
				return
			}
			ch, err := connector.NewChannel()
			if err != nil {
				lg.Warn("fib-vpp: GoVPP channel failed, using noop backend", "error", err)
				fib = newFibVPP(&mockBackend{})
				return
			}
			backend := newGovppBackend(ch, tableID)
			fib = newFibVPP(backend)
		}

		startFib := func() {
			if runCancel != nil {
				runCancel()
			}
			var runCtx context.Context
			runCtx, runCancel = context.WithCancel(ctx)
			go fib.run(runCtx, false)
		}

		initBackend()

		eb := getEventBus()
		if eb != nil {
			onVPPReady := events.AsString(func(event string) {
				lg.Info("fib-vpp: VPP ready, reinitializing backend", "event", event)
				initBackend()
				startFib()
			})
			unsub1 := eb.Subscribe(vppevents.Namespace, vppevents.EventConnected, onVPPReady)
			unsub2 := eb.Subscribe(vppevents.Namespace, vppevents.EventReconnected, onVPPReady)
			vppUnsub = func() { unsub1(); unsub2() }
		}

		startFib()
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "fib-vpp show" {
			if fib == nil {
				return "done", "[]", nil
			}
			return "done", fib.showInstalled(), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"fib/vpp"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "fib-vpp show"},
		},
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
