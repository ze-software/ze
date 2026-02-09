package rib

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	ribschema "codeberg.org/thomas-mangin/ze/internal/plugin/rib/schema"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "rib",
		Description: "Route Information Base storage",
		RFCs:        []string{"4271"},
		Features:    "yang",
		YANG:        ribschema.ZeRibYANG,
		RunEngine:   RunRIBPlugin,
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
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rib: registration failed: %v\n", err)
		os.Exit(1)
	}
}
