package irr

import (
	"log/slog"
	"os"

	irryang "github.com/ze-software/ze/internal/component/firewall/plugins/irr/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
)

func init() {
	registerIRRDoctor()

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

// registerIRRDoctor registers the IRR data-freshness doctor check and the two
// diagnostic codes it emits. It lives in register.go so the side effect, and the
// exit on a registration failure, stay in the plugin's registration file
// (ai/patterns/registration.md).
func registerIRRDoctor() {
	for _, meta := range irrDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			slog.Error("firewall-irr: diagnostic code registration failed", "code", meta.Code, "error", err)
			os.Exit(1)
		}
	}
	check := diagnostic.DoctorCheck{
		Name:         "firewall-irr-data-freshness",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Component:    "firewall-irr",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeIRRStaleData, codeIRRNoData},
		Check:        checkIRRDataFreshness,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		slog.Error("firewall-irr: doctor check registration failed", "error", err)
		os.Exit(1)
	}
}
