package rib

import (
	"fmt"
	"os"

	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	ribschema "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/schema"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	_ = events.RegisterNamespace(ribevents.Namespace,
		ribevents.EventCache, ribevents.EventRoute, ribevents.EventBestChange, ribevents.EventReplayRequest,
	)

	reg := registry.Registration{
		Name:        "bgp-rib",
		Description: "Route Information Base storage",
		RFCs:        []string{"4271"},
		Features:    "yang",
		ConfigRoots: []string{"bgp"},
		YANG:        ribschema.ZeRibYANG,
		RunEngine:   RunRIBPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			SetLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg any) {
			if r, ok := reg.(metrics.Registry); ok {
				SetMetricsRegistry(r)
			}
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				SetEventBus(e)
			}
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
