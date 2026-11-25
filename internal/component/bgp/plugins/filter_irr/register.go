package filter_irr

import (
	firryang "github.com/ze-software/ze/internal/component/bgp/plugins/filter_irr/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func filterIRRBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: "bgp:refresh"},
		{Message: events.NamespaceCLI + ":show bgp peer"},
		{Message: events.NamespaceCLI + ":request peer"},
		{Message: store.NoticeNamespace + ":" + store.EventInterestChanged},
	}
}

func filterIRRBusReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: store.NoticeNamespace + ":" + store.EventDatasetStored},
	}
}

func init() {
	// The filter tells the irr plugin what it enforces through its own interest
	// key, meta/irr-interest/bgp-filter-irr, and no other consumer's; it reads
	// the datasets, pointers, bindings and generation the irr plugin writes
	// (docs/architecture/irr-owner.md).
	statestore.RegisterPluginKeyInstance(pluginName, zefs.KeyIRRInterest, pluginName)
	statestore.RegisterPluginReadKeys(pluginName,
		zefs.KeyIRRDataset, zefs.KeyIRRSet, zefs.KeyIRRASN, zefs.KeyIRRGeneration)
	for _, meta := range irrDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: bgp-filter-irr: diagnostic code " + meta.Code + " registration failed: " + err.Error())
		}
	}

	reg := registry.Registration{
		Name:        pluginName,
		Description: "IRR-based prefix-list filter for eBGP peers",
		Features:    "yang",
		YANG:        firryang.ZeFilterIrrYANG,
		ConfigRoots: []string{configRootBGP},
		// The irr plugin fetches and stores the lists this filter enforces, so it
		// starts whenever the filter runs, and before it.
		Dependencies: []string{configRootBGP, store.OwnerPlugin},
		RunEngine:    runFilterIRR,
		Commands:     commandDecls(),
		Sends:        filterIRRBusSends(),
		Receives:     filterIRRBusReceives(),
		Filters:      filterDecls(), // the Stage 1 declaration: config load knows irr serves filters
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return firryang.ZeFilterIrrYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: bgp-filter-irr: registration failed: " + err.Error())
	}
}
