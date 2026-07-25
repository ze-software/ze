package irr

import (
	"log/slog"
	"os"

	irryang "github.com/ze-software/ze/internal/component/firewall/plugins/irr/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/metrics"
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
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
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
