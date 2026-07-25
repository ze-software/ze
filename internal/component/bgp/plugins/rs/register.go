package rs

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rs/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
)

func init() {
	// Activate the reactor's route-server fast-path forwarding capability
	// (BGP-owned filterapi seam, not the generic registry). This is the sole
	// caller of EnableRSForwarding: deleting this plugin package removes the
	// activation, leaving the reactor RS fast path inert -- the "delete the
	// folder and the feature vanishes" invariant. The reactor reads the flag
	// once at construction; it never imports this package.
	filterapi.EnableRSForwarding()

	reg := registry.Registration{
		Name:        "bgp-rs",
		Description: "Route Server",
		RFCs:        []string{"7947"},
		ConfigRoots: []string{"bgp"},
		Features:    "yang",
		YANG:        yang.ZeRsConfYANG,
		// bgp-adj-rib-in is optional: bgp-rs uses it for replay-on-peer-up
		// when present, and gracefully disables replay with a one-shot WARN
		// when absent. See spec-rs-fastpath-2-adjrib learned summary.
		OptionalDependencies: []string{"bgp-adj-rib-in"},
		RunEngine:            RunRouteServer,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rs: registration failed: %v\n", err)
		os.Exit(1)
	}
}
