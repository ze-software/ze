package gr

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	grschema "codeberg.org/thomas-mangin/ze/internal/plugins/gr/schema"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:         "gr",
		Description:  "Graceful Restart capability plugin",
		RFCs:         []string{"4724"},
		SupportsCapa: true,
		Features:     "capa yang",
		ConfigRoots:  []string{"bgp"},
		YANG:         grschema.ZeGracefulRestartYANG,
		RunEngine:    RunGRPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		cfg.RunCLIDecode = RunCLIDecode
		cfg.RunEngine = RunGRPlugin
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "gr: registration failed: %v\n", err)
		os.Exit(1)
	}
}
