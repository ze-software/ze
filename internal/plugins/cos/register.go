package cos

import (
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	coreCos "codeberg.org/thomas-mangin/ze/internal/core/cos"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/internal/plugins/cos/yang"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func init() {
	reg := registry.Registration{
		Name:                    Name,
		Description:             "802.1p class-of-service profile definitions",
		Features:                "yang",
		YANG:                    yang.ZeCosConfYANG,
		ConfigRoots:             []string{"class-of-service"},
		InProcessConfigVerifier: verifyCoSConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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

func verifyCoSConfig(sections []sdk.ConfigSection) error {
	coreCos.Clear()
	for _, sec := range sections {
		if sec.Root != "class-of-service" {
			continue
		}
		if err := parseAndRegisterProfiles(sec.Data); err != nil {
			return err
		}
	}
	return nil
}

func runPlugin(conn net.Conn) int {
	logger().Debug("cos plugin starting")

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnConfigVerify(verifyCoSConfig)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "class-of-service" {
				continue
			}
			if err := parseAndRegisterProfiles(sec.Data); err != nil {
				return err
			}
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"class-of-service"},
		VerifyBudget: 1,
	}); err != nil {
		logger().Error("cos plugin failed", "error", err)
		return 1
	}
	return 0
}
