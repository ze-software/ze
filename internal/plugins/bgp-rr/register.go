package bgp_rr

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-rr",
		Description: "Route Reflector / Route Server",
		RFCs:        []string{"4456"},
		RunEngine:   RunRouteServer,
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
		fmt.Fprintf(os.Stderr, "rr: registration failed: %v\n", err)
		os.Exit(1)
	}
}
