package filter_irr

import (
	"log/slog"
	"os"

	firryang "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/filter_irr/yang"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

func init() {
	reg := registry.Registration{
		Name:         "bgp-filter-irr",
		Description:  "IRR-based prefix-list filter for eBGP peers",
		Features:     "yang",
		YANG:         firryang.ZeFilterIrrYANG,
		ConfigRoots:  []string{"bgp"},
		Dependencies: []string{"bgp"},
		RunEngine:    runFilterIRR,
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return firryang.ZeFilterIrrYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("bgp-filter-irr: registration failed", "error", err)
		os.Exit(1)
	}
}
