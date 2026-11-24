// Design: docs/architecture/firewall/firewall-domain-group.md -- firewall domain-group plugin registration

package domain

import (
	domainyang "github.com/ze-software/ze/internal/component/firewall/plugins/domain/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func firewallDomainSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: "resolve:dns"},
	}
}

// configRoot is the YANG container this plugin reads, and dependencyFirewall is
// the plugin it needs beside it. The two hold the same text and name two
// different things.
const (
	configRoot         = "firewall"
	dependencyFirewall = "firewall"
)

func init() {
	statestore.RegisterPluginKeys("firewall-domain", zefs.KeyFirewallDomainGroup)
	registerDomainDoctor()

	reg := registry.Registration{
		Name:         "firewall-domain",
		Description:  "DNS-sourced address groups for firewall rules",
		Features:     "yang",
		YANG:         domainyang.ZeFirewallDomainGroupYANG,
		ConfigRoots:  []string{configRoot},
		Dependencies: []string{dependencyFirewall},
		Sends:        firewallDomainSends(),
		RunEngine:    runFirewallDomain,
		Commands:     commandDecls(),
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
		panic("BUG: firewall-domain: registration failed: " + err.Error())
	}
}

// registerDomainDoctor registers the domain-group data check and the
// diagnostic codes it emits (domainDiagnosticCodes). It lives in register.go so
// the side effect, and the panic on a registration failure, stay in the plugin's
// registration file (ai/patterns/registration.md).
func registerDomainDoctor() {
	for _, meta := range domainDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: firewall-domain: diagnostic code registration failed (" + meta.Code + "): " + err.Error())
		}
	}
	// An unsaved answer is also raised on the report bus (unsaved_issue.go),
	// and the daemon accepts a plugin's raise only under a registered event
	// code. Registered after its metadata, which RegisterEventCode requires.
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDomainGroupUnsavedAnswer}); err != nil {
		panic("BUG: firewall-domain: event code registration failed (" + codeDomainGroupUnsavedAnswer + "): " + err.Error())
	}
	check := diagnostic.DoctorCheck{
		Name:         "firewall-domain-group-data",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Component:    "firewall-domain",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		// A literal, so `./le arch doctor-coverage check` reads the codes this
		// check declares. The runtime check declares the unsaved-answer code
		// (unsavedAnswerCheckDecl).
		Codes: []string{
			codeDomainGroupStaleData,
			codeDomainGroupNoData,
			codeDomainGroupUnreadableData,
		},
		Check: checkDomainGroupData,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: firewall-domain: doctor check registration failed: " + err.Error())
	}
}
