// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-shaper plugin lifecycle

package l2tpshaper

import (
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	schema "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpshaper/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:        Name,
		Description: "Traffic shaping for L2TP subscriber sessions",
		Features:    "yang",
		YANG:        schema.ZeL2TPShaperConfYANG,
		ConfigRoots: []string{"l2tp"},
		RunEngine:   runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				shaperInstance.setEventBus(e)
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
		fmt.Fprintf(os.Stderr, "%s: registration failed: %v\n", Name, err)
		os.Exit(1)
	}
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "l2tp" {
				continue
			}
			if _, _, err := parseShaperConfig(sec.Data); err != nil {
				return err
			}
		}
		return nil
	})

	var pending *shaperConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "l2tp" {
				continue
			}
			cfg, found, err := parseShaperConfig(sec.Data)
			if err != nil {
				return err
			}
			if found {
				pending = cfg
			}
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pending != nil {
			shaperInstance.cfgPtr.Store(pending)
			logger().Info("l2tp-shaper: configured",
				"qdisc", pending.QdiscType, "rate", pending.DefaultRate)
			pending = nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		pending = nil
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "l2tp shaper show" {
			return "done", shaperInstance.showSessions(), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"l2tp"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "l2tp shaper show"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}
