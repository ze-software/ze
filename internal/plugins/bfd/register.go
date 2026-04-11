// IMPORTANT: this plugin is INTENTIONALLY not blank-imported by
// `internal/component/plugin/all/all.go`. Running `make generate` will
// pick it up and regenerate all.go to include it -- DO NOT do that until
// the wiring spec lands. RunBFDPlugin is a no-op stub today; if a
// generated all.go references it, the engine will spawn the plugin and
// see it exit immediately, which presents as a startup failure.
//
// To wire BFD into the engine: replace RunBFDPlugin with the real entry
// (parse YANG, open transport.UDP per VRF, expose api.Service over RPC),
// then run `make generate`.

package bfd

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	bfdschema "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/schema"
)

func init() {
	reg := registry.Registration{
		Name:        "bfd",
		Description: "Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)",
		Features:    "yang",
		RFCs:        []string{"5880", "5881", "5882", "5883"},
		ConfigRoots: []string{"bfd"},
		YANG:        bfdschema.ZeBFDConfYANG,
		RunEngine:   RunBFDPlugin,
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			UseLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "bfd: registration failed: %v\n", err)
		os.Exit(1)
	}
}
