package bgp_nlri_mvpn

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func init() {
	reg := registry.Registration{
		Name:        "bgp-mvpn",
		Description: "Multicast VPN family plugin (RFC 6514)",
		RFCs:        []string{"6514"},
		Features:    "nlri",
		Families:    []string{"ipv4/mvpn", "ipv6/mvpn"},
		RunEngine:   RunMVPNPlugin,
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
		fmt.Fprintf(os.Stderr, "mvpn: registration failed: %v\n", err)
		os.Exit(1)
	}
}
