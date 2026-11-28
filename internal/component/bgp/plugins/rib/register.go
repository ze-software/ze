package rib

import (
	ribyang "github.com/ze-software/ze/internal/component/bgp/plugins/rib/yang"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

const pluginName = "bgp-rib"

const eventFlowSpecChange = "flowspec-change"

func ribBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":request peer"},
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in eligibility"},
		{Message: ribevents.Namespace + ":" + ribevents.EventBestChange},
		{Message: bgpevents.Namespace + ":" + redistevents.EventType},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationChange},
		{Message: ribevents.Namespace + ":" + eventFlowSpecChange},
		{Message: ribevents.Namespace + ":" + ribevents.EventSelectionDeferred},
		{Message: ribevents.Namespace + ":" + ribevents.EventSelectionComplete},
		{Message: ribevents.Namespace + ":" + ribevents.EventFlowSpecGate},
		{Message: ribevents.Namespace + ":" + ribevents.EventGateQuery},
		{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate},
		{Message: bgpevents.Namespace + ":" + bgpevents.SendRefresh},
		{Message: sysribevents.Namespace + ":" + sysribevents.EventRouteInstall},
		{Message: sysribevents.Namespace + ":" + sysribevents.EventRouteMetrics},
	}
}

func ribBusReceives() []rpc.MessageReceiveDecl {
	return []rpc.MessageReceiveDecl{
		{
			Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate,
			Tags: []rpc.MessageTagDecl{{
				Name:   events.TagDirection,
				Values: []string{events.DirectionReceived, events.DirectionSent},
			}},
		},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventRefresh},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventNegotiated},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventEOR},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationChange},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationGate},
		{Message: ribevents.Namespace + ":" + ribevents.EventGateQuery},
		{Message: ribevents.Namespace + ":" + ribevents.EventReplayRequest},
		{Message: redistevents.ReplayNamespace + ":" + redistevents.ReplayRequestEventType},
	}
}

func init() {
	registerDoctorCodes()
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        codeRemoteMetricFailure,
		Title:       "BGP next-hop reachability lookup failed",
		Description: "The BGP RIB could not read next-hop reachability from the engine. Affected received paths remain stored but cannot be selected until the connection recovers. A failed revision feed stops the plugin, preserving its mandatory dependency. Inspect ze show doctor for the plugin connection problem and restore that connection.",
		Examples:    []string{remoteMetricFix, "ze show diagnostic doctor-bgp-next-hop-metrics"},
	}); err != nil {
		panic("BUG: bgp rib: next-hop metric diagnostic registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeRemoteMetricFailure}); err != nil {
		panic("BUG: bgp rib: next-hop metric event registration failed: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorBGPSelectedOutput}); err != nil {
		panic("BUG: bgp rib: selected output event registration failed: " + err.Error())
	}
	// Publish the TABLE_DUMP_V2 snapshot provider through the leaf registry
	// seam. MRT reads it from there, so neither plugin imports the other and
	// the always-on hub no longer names a BGP package (spec-feature-gate-10).
	registry.SetRIBDumpCallback(RIBDumpBridge)

	// `show bgp rib ... graph` answers the AS topology as data; its text
	// rendering is the box diagram, drawn wherever the pipe chain runs.
	command.MustRegisterTextRenderer(graphAnswerKey, renderGraphText)

	_ = events.RegisterNamespace(ribevents.Namespace,
		ribevents.EventCache, ribevents.EventRoute, ribevents.EventBestChange, ribevents.EventReplayRequest,
		ribevents.EventValidationChange, ribevents.EventValidationGate, eventFlowSpecChange,
		ribevents.EventFlowSpecGate, ribevents.EventGateQuery,
		ribevents.EventSelectionDeferred, ribevents.EventSelectionComplete,
	)

	reg := registry.Registration{
		Name:                    pluginName,
		Description:             "Route Information Base storage",
		RFCs:                    []string{"4271"},
		Features:                "yang",
		ConfigRoots:             []string{configRootBGP},
		ConfigReads:             []string{"rib"},
		InProcessConfigVerifier: verifyRIBConfig,
		YANG:                    ribyang.ZeRibYANG,
		RunEngine:               runRIBPlugin,
		Commands:                commandDecls(),
		Dissemination:           disseminationDecls(),
		Sends:                   ribBusSends(),
		Receives:                ribBusReceives(),
		// The Adj-RIB-Out replay on the down-to-up edge IS this peer's initial
		// routing update, and replayRoutesWithCursor reports when it is out --
		// including when there was nothing to replay (rib_replay.go).
		SignalsSessionReady: true,
		// Live withdrawals and captured lifecycle cleanup MUST follow this
		// session's exact valid-history restore, never overtake it.
		FencesLiveForwards: true,
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
		panic("BUG: bgp-rib: registration failed: " + err.Error())
	}
}
