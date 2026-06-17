package irr

import (
	"log/slog"
	"os"

	irryang "codeberg.org/thomas-mangin/ze/internal/component/firewall/plugins/irr/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

func init() {
	reg := registry.Registration{
		Name:         "firewall-irr",
		Description:  "IRR-based prefix-list filtering for firewall rules",
		Features:     "yang",
		YANG:         irryang.ZeFirewallIrrYANG,
		ConfigRoots:  []string{"firewall"},
		Dependencies: []string{"firewall"},
		RunEngine:    runFirewallIRR,
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				setMetricsRegistry(r)
			}
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return irryang.ZeFirewallIrrYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("firewall-irr: registration failed", "error", err)
		os.Exit(1)
	}
}
