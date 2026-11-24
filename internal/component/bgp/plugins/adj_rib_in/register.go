package adj_rib_in

import (
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func adjRIBInBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":request peer"},
		{Message: events.NamespaceCLI + ":request bgp rib flowspec-gate"},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationChange},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationGate},
		{Message: ribevents.Namespace + ":" + ribevents.EventGateQuery},
		{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate},
	}
}

func adjRIBInBusReceives() []rpc.MessageReceiveDecl {
	recvDir := func() []rpc.MessageTagDecl {
		return []rpc.MessageTagDecl{{
			Name:   events.TagDirection,
			Values: []string{events.DirectionReceived},
		}}
	}
	return []rpc.MessageReceiveDecl{
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate, Tags: recvDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdateFiltered, Tags: recvDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: ribevents.Namespace + ":" + ribevents.EventFlowSpecGate},
		{Message: ribevents.Namespace + ":" + ribevents.EventGateQuery},
		{Message: retention.Namespace + ":" + retention.EventType},
		{Message: retention.Namespace + ":" + retention.CleanupEventType},
	}
}

func init() {
	reg := registry.Registration{
		Name:        "bgp-adj-rib-in",
		Description: "Adj-RIB-In storage (raw hex replay)",
		RFCs:        []string{"4271"},
		RunEngine:   runAdjRIBInPlugin,
		Commands:    commandDecls(),
		Sends:       adjRIBInBusSends(),
		Receives:    adjRIBInBusReceives(),
		// The peer-up replay of stored routes is part of the establishing peer's
		// initial routing update, and signalSessionReady reports on every peer-up
		// (rib.go) -- including when bgp-rs holds the replay role and this plugin
		// sends nothing, which is finished the instant the event arrives.
		SignalsSessionReady: true,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			SetMetricsRegistry(reg)
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
		panic("BUG: adj-rib-in: registration failed: " + err.Error())
	}
}
