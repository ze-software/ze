// Design: docs/architecture/irr-owner.md -- the irr plugin registration and its state grants

package irr

import (
	"log/slog"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/statestore"
	irryang "github.com/ze-software/ze/internal/plugins/irr/yang"
	"github.com/ze-software/ze/pkg/zefs"
)

func init() {
	// The owner is the only writer of the datasets, the AS-SET pointers, the
	// ASN bindings and the generation counter. Each consumer writes only its
	// own interest key, granted in the consumer's register.go; the owner reads
	// every consumer's interest key and writes none.
	statestore.RegisterPluginKeys(pluginName,
		zefs.KeyIRRDataset, zefs.KeyIRRSet, zefs.KeyIRRASN, zefs.KeyIRRGeneration)
	statestore.RegisterPluginReadKeys(pluginName, zefs.KeyIRRInterest)

	reg := registry.Registration{
		Name:        pluginName,
		Description: "IRR data owner: fetches, schedules and stores the IRR prefix lists every consumer enforces",
		Features:    "yang",
		YANG:        irryang.ZeIrrConfYANG,
		ConfigRoots: []string{configPathIRR},
		RunEngine:   runIRR,
		Commands:    commandDecls(),
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return irryang.ZeIrrConfYANG }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		slog.Error("irr: registration failed", "error", err, "code", diagnostic.CodeDoctorInternalDefect)
		os.Exit(1)
	}
}
