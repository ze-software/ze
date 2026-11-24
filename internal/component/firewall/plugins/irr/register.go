package irr

import (
	"github.com/ze-software/ze/internal/component/firewall"
	irryang "github.com/ze-software/ze/internal/component/firewall/plugins/irr/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func firewallIRRSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: store.NoticeNamespace + ":" + store.EventInterestChanged},
	}
}

func firewallIRRReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: store.NoticeNamespace + ":" + store.EventDatasetStored},
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
	// meta/irr-interest/firewall-irr tells the irr plugin what this plugin
	// enforces, and it writes no other consumer's interest key; it reads the
	// datasets, pointers, bindings and generation the irr plugin writes
	// (docs/architecture/irr-owner.md). It holds no key of its own beyond the
	// interest key: the lists it enforces are the irr plugin's.
	statestore.RegisterPluginKeyInstance(pluginName, zefs.KeyIRRInterest, pluginName)
	statestore.RegisterPluginReadKeys(pluginName,
		zefs.KeyIRRDataset, zefs.KeyIRRSet, zefs.KeyIRRASN, zefs.KeyIRRGeneration)
	registerIRRDoctor()
	if err := firewall.RegisterConfigTableBuilder(pluginName, buildIfaceBindingShapeTables); err != nil {
		panic("BUG: firewall-irr: config table builder registration failed: " + err.Error())
	}

	reg := registry.Registration{
		Name:        pluginName,
		Description: "IRR-based prefix-list filtering for firewall rules",
		Features:    "yang",
		YANG:        irryang.ZeFirewallIrrYANG,
		ConfigRoots: []string{configRoot},
		// The irr plugin fetches and stores the lists this plugin enforces, so
		// it starts whenever this plugin runs, and before it.
		Dependencies: []string{dependencyFirewall, store.OwnerPlugin},
		Sends:        firewallIRRSends(),
		Receives:     firewallIRRReceives(),
		RunEngine:    runFirewallIRR,
		Commands:     commandDecls(),
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return irryang.ZeFirewallIrrYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: firewall-irr: registration failed: " + err.Error())
	}
}

// registerIRRDoctor registers the IRR data-freshness doctor check and the
// diagnostic codes it emits. It lives in register.go so the side effect, and
// the panic on a registration failure, stay in the plugin's registration file
// (ai/patterns/registration.md).
func registerIRRDoctor() {
	for _, meta := range irrDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: firewall-irr: diagnostic code registration failed (" + meta.Code + "): " + err.Error())
		}
	}
	check := diagnostic.DoctorCheck{
		Name:         "firewall-irr-data-freshness",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Component:    "firewall-irr",
		Dependencies: []string{"config-tree"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{codeIRRStaleData, codeIRRNoData, codeIRROversizedData},
		Check:        checkIRRDataFreshness,
	}
	if err := diagnostic.RegisterDoctorCheck(check); err != nil {
		panic("BUG: firewall-irr: doctor check registration failed: " + err.Error())
	}
}
