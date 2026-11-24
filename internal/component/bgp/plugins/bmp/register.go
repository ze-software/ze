package bmp

import (
	bmpyang "github.com/ze-software/ze/internal/component/bgp/plugins/bmp/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

func bmpBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":request bgp rib withdraw-router"},
		{Message: events.NamespaceCLI + ":show bgp rib protocol"},
		{Message: events.NamespaceCLI + ":request bgp rib withdraw-protocol"},
		{Message: ribevents.Namespace + ":" + ribevents.EventReplayRequest},
		{Message: bgpevents.Namespace + ":" + bgpevents.SendRaw},
	}
}

func bmpBusReceives() []rpc.MessageReceiveDecl {
	bothDir := func() []rpc.MessageTagDecl {
		return []rpc.MessageTagDecl{{
			Name:   events.TagDirection,
			Values: []string{events.DirectionReceived, events.DirectionSent},
		}}
	}
	sentDir := func() []rpc.MessageTagDecl {
		return []rpc.MessageTagDecl{{
			Name:   events.TagDirection,
			Values: []string{events.DirectionSent},
		}}
	}
	return []rpc.MessageReceiveDecl{
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate, Tags: bothDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdatePrePolicyOut, Tags: sentDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventOpen, Tags: bothDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventNotification, Tags: bothDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventKeepalive, Tags: bothDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventRefresh, Tags: bothDir()},
	}
}

func init() {
	reg := registry.Registration{
		Name:        "bgp-bmp",
		Description: "BMP receiver and sender (RFC 7854, 8671)",
		RFCs:        []string{"7854", "8671"},
		Features:    "yang",
		YANG:        bmpyang.ZeBMPConfYANG,
		// ConfigRoots decides auto-load. The system root is read through
		// WantsConfig (bmp.go) for the Initiation identity only, so a config
		// holding just `system` must not start BMP. The receiver is claimed
		// at `environment bmp`, so a config holding only another environment
		// block (api-server, log) must not start BMP either.
		ConfigRoots: []string{configRootBGP, configPathReceiver},
		RunEngine:   runBMPPlugin,
		Commands:    commandDecls(),
		Sends:       bmpBusSends(),
		Receives:    bmpBusReceives(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		// RFC 9069 Loc-RIB monitoring subscribes to the RIB's best-change
		// events on the in-process EventBus (mirrors the redistribute orchestrator).
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBus(eb)
		},
		// The receiver's panic boundary counts in ze_task_panics_total.
		ConfigureMetrics: func(reg metrics.Registry) {
			bmpPanics().guard.SetMetricsRegistry(reg)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: bgp-bmp: registration failed: " + err.Error())
	}

	// This plugin connects to the collectors the config names, so it owns the
	// readiness check that asks whether any of them can be reached (doctor.go).
	if err := diagnostic.RegisterDoctorCheck(bmpDoctorCheck); err != nil {
		panic("BUG: bgp-bmp: doctor check registration failed: " + err.Error())
	}
}
