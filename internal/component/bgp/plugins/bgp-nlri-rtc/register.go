package bgp_nlri_rtc

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-rtc",
		Description: "Route Target Constraint family plugin (RFC 4684)",
		RFCs:        []string{"4684"},
		Features:    "nlri",
		Families:    []string{"ipv4/rtc"},
		RunEngine:   RunRTCPlugin,
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
		fmt.Fprintf(os.Stderr, "rtc: registration failed: %v\n", err)
		os.Exit(1)
	}
}
