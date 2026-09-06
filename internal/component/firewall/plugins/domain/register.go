// Design: docs/architecture/firewall/firewall-domain-group.md -- firewall domain-group plugin registration

package domain

import (
	"log/slog"
	"os"

	domainyang "github.com/ze-software/ze/internal/component/firewall/plugins/domain/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
)

// configRoot is the YANG container this plugin reads, and dependencyFirewall is
// the plugin it needs beside it. The two hold the same text and name two
// different things.
const (
	configRoot         = "firewall"
	dependencyFirewall = "firewall"
)

func init() {
	registerDomainDoctor()

	reg := registry.Registration{
		Name:         "firewall-domain",
		Description:  "DNS-sourced address groups for firewall rules",
		Features:     "yang",
		YANG:         domainyang.ZeFirewallDomainGroupYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{dependencyFirewall},
		RunEngine:    runFirewallDomain,
		ConfigureMetrics: func(reg metrics.Registry) {
			setMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return domainyang.ZeFirewallDomainGroupYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("firewall-domain: registration failed", "error", err)
		os.Exit(1)
	}
}

// registerDomainDoctor registers the domain-group data check and the two
// diagnostic codes it emits. It lives in register.go so the side effect, and
// the exit on a registration failure, stay in the plugin's registration file
// (ai/patterns/registration.md).
func registerDomainDoctor() {
	for _, meta := range domainDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			slog.Error("firewall-domain: diagnostic code registration failed", "code", meta.Code, "error", err)
			os.Exit(1)
		}
	}
	check := diagnostic.DoctorCheck{
		Name:         "firewall-domain-group-data",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Component:    "firewall-domain",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeDomainGroupStaleData, codeDomainGroupNoData},
		Check:        checkDomainGroupData,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		slog.Error("firewall-domain: doctor check registration failed", "error", err)
		os.Exit(1)
	}
}
