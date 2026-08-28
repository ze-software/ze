package rib

import (
	"fmt"
	"os"

	ribyang "github.com/ze-software/ze/internal/component/bgp/plugins/rib/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

func init() {
	// Publish the TABLE_DUMP_V2 snapshot provider through the leaf registry
	// seam. MRT reads it from there, so neither plugin imports the other and
	// the always-on hub no longer names a BGP package (spec-feature-gate-10).
	registry.SetRIBDumpCallback(RIBDumpBridge)

	_ = events.RegisterNamespace(ribevents.Namespace,
		ribevents.EventCache, ribevents.EventRoute, ribevents.EventBestChange, ribevents.EventReplayRequest,
	)

	reg := registry.Registration{
		Name:        "bgp-rib",
		Description: "Route Information Base storage",
		RFCs:        []string{"4271"},
		Features:    "yang",
		ConfigRoots: []string{configRootBGP},
		YANG:        ribyang.ZeRibYANG,
		RunEngine:   runRIBPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			SetEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = GetYANG
		cfg.ConfigLogger = func(level string) {
			SetLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "rib: registration failed: %v\n", err)
		os.Exit(1)
	}
}
