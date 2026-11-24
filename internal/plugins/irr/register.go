// Design: docs/architecture/irr-owner.md -- the irr plugin registration and its state grants

package irr

import (
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/statestore"
	irryang "github.com/ze-software/ze/internal/plugins/irr/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

func irrSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: store.NoticeNamespace + ":" + store.EventDatasetStored},
	}
}

func irrReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{Message: store.NoticeNamespace + ":" + store.EventInterestChanged},
	}
}

func init() {
	// The owner is the only writer of the datasets, the AS-SET pointers, the
	// ASN bindings and the generation counter. Each consumer writes only its
	// own interest key, granted in the consumer's register.go; the owner reads
	// every consumer's interest key and writes none.
	statestore.RegisterPluginKeys(pluginName,
		zefs.KeyIRRDataset, zefs.KeyIRRSet, zefs.KeyIRRASN, zefs.KeyIRRGeneration)
	statestore.RegisterPluginReadKeys(pluginName, zefs.KeyIRRInterest)
	// The owner converts each list firewall-irr stored before it existed, and
	// removes the old key once converted (legacy.go).
	statestore.RegisterPluginKeys(pluginName, zefs.KeyIRRPrefixCache)
	for _, meta := range irrDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: irr: diagnostic code " + meta.Code + " registration failed: " + err.Error())
		}
	}

	reg := registry.Registration{
		Name:        pluginName,
		Description: "IRR data owner: fetches, schedules and stores the IRR prefix lists every consumer enforces",
		Features:    "yang",
		YANG:        irryang.ZeIrrConfYANG,
		ConfigRoots: []string{configPathIRR},
		Sends:       irrSends(),
		Receives:    irrReceives(),
		RunEngine:   runIRR,
		Commands:    commandDecls(),
		// The owner is the only IRR fetcher, so it registers the IRR refresh
		// metrics (metrics.go).
		ConfigureMetrics: setMetricsRegistry,
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return irryang.ZeIrrConfYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: irr: registration failed: " + err.Error())
	}
}
