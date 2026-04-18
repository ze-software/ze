package rs

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-rs",
		Description: "Route Server",
		RFCs:        []string{"7947"},
		// bgp-adj-rib-in is optional: bgp-rs uses it for replay-on-peer-up
		// when present, and gracefully disables replay with a one-shot WARN
		// when absent. See spec-rs-fastpath-2-adjrib learned summary.
		OptionalDependencies: []string{"bgp-adj-rib-in"},
		RunEngine:            RunRouteServer,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rs: registration failed: %v\n", err)
		os.Exit(1)
	}
}
