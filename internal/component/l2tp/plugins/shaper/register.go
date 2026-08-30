// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-shaper plugin lifecycle

package l2tpshaper

import (
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/l2tp/plugins/shaper/yang"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// configRootL2TP is the YANG container this plugin reads.
const configRootL2TP = "l2tp"

func init() {
	subscriber.RegisterShaperHandler(shaperInstance.handleSubscriberSessionUp)
	reg := registry.Registration{
		Name:                    Name,
		Description:             "Traffic shaping for L2TP subscriber sessions",
		Features:                "yang",
		YANG:                    yang.ZeL2TPShaperConfYANG,
		ConfigRoots:             []string{configRootL2TP},
		InProcessConfigVerifier: verifyShaperConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			shaperInstance.setEventBus(eb)
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

func verifyShaperConfig(sections []sdk.ConfigSection) error {
	for _, sec := range sections {
		if sec.Root != configRootL2TP {
			continue
		}
		if _, _, err := parseShaperConfig(sec.Data); err != nil {
			return err
		}
	}
	return nil
}

func runPlugin(conn net.Conn) int {
	logger().Debug(Name + " plugin starting (RPC)")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnConfigVerify(verifyShaperConfig)

	var pending *shaperConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != configRootL2TP {
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
		if pending != nil {
			shaperInstance.cfgPtr.Store(pending)
			logger().Info("l2tp-shaper: configured",
				"qdisc", pending.QdiscType, "rate", pending.DefaultRate)
			pending = nil
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

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show l2tp shaper" {
			return "done", shaperInstance.showSessions(), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootL2TP},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "show l2tp shaper"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}
