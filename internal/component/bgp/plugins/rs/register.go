package rs

import (
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rs/yang"
	"github.com/ze-software/ze/internal/component/bgp/retention"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func rsParticipantNodes() []rpc.ParticipantNodeDecl {
	return []rpc.ParticipantNodeDecl{{
		Module:     "ze-rs-conf",
		Prefix:     "rs",
		SchemaPath: "/bgp:bgp/rs:route-server",
	}}
}

func rsBusSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in claim-replay"},
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in replay-routes"},
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in replay-path"},
		{Message: events.NamespaceCLI + ":request bgp adj-rib-in eligibility"},
		{Message: events.NamespaceCLI + ":request bgp rib flowspec-gate"},
		{Message: events.NamespaceCLI + ":request peer"},
		{Message: ribevents.Namespace + ":" + ribevents.EventGateQuery},
		{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate},
	}
}

// rsBusReceives returns the bus messages bgp-rs receives. It excludes
// update-filtered by design (RFC 7947 Section 2.1) and constrains update and
// open to received direction only (server.go).
func rsBusReceives() []rpc.MessageReceiveDecl {
	recvDir := func() []rpc.MessageTagDecl {
		return []rpc.MessageTagDecl{{
			Name:   events.TagDirection,
			Values: []string{events.DirectionReceived},
		}}
	}
	return []rpc.MessageReceiveDecl{
		{Message: bgpevents.Namespace + ":" + bgpevents.EventUpdate, Tags: recvDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventState},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventOpen, Tags: recvDir()},
		{Message: bgpevents.Namespace + ":" + bgpevents.EventRefresh},
		{Message: retention.Namespace + ":" + retention.EventType},
		{Message: retention.Namespace + ":" + retention.CleanupEventType},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationChange},
		{Message: ribevents.Namespace + ":" + ribevents.EventFlowSpecGate},
		{Message: ribevents.Namespace + ":" + ribevents.EventValidationGate},
	}
}

func init() {
	_ = events.RegisterNamespace(ribevents.Namespace, ribevents.EventValidationChange)
	// Activate the reactor's route-server fast-path forwarding capability
	// (BGP-owned filterapi seam, not the generic registry). This is the sole
	// caller of EnableRSForwarding: deleting this plugin package removes the
	// activation, leaving the reactor RS fast path inert -- the "delete the
	// folder and the feature vanishes" invariant. The reactor reads the flag
	// once at construction; it never imports this package.
	filterapi.EnableRSForwarding()

	reg := registry.Registration{
		Name:         rsPluginName,
		Description:  "Route Server",
		RFCs:         []string{"7947"},
		ConfigRoots:  []string{"bgp"},
		Features:     "yang",
		YANG:         yang.ZeRsConfYANG,
		Dependencies: []string{"bgp-rib"},
		// bgp-adj-rib-in is optional. Without it, peer-up replay uses the
		// mandatory RIB for authorized FlowSpec paths; other families require
		// the receive store for replay.
		OptionalDependencies: []string{"bgp-adj-rib-in"},
		// This plugin drives peer-up replay explicitly (replayForPeer), so
		// bgp-adj-rib-in must not also self-replay -- with both firing, a route
		// learned just before a peer established went out twice. Declaring the
		// role here rather than dispatching a command at OnAllPluginsReady is
		// what makes the stand-down deterministic: the engine delivers the claim
		// on bgp-adj-rib-in's Stage-2 configure, which completes before peers
		// start. See ClaimPeerUpReplay in server_handlers.go.
		Claims:            []string{ClaimPeerUpReplay, disseminationRole},
		ClaimSessionScope: []rpc.FilterSessionFact{{Fact: "rs-client", Values: []string{"true"}}},
		Dissemination:     disseminationDecls(),
		// handleState makes a peer a live forward target (Up + ForwardFrom, one
		// critical section, server_handlers.go). An UPDATE taken delivery of
		// before that lands at or below the peer's cut, so it belongs to the
		// announce-only Adj-RIB-In replay and its withdrawals are lost. Declaring
		// the barrier makes the engine hold this peer's initial-sync End-of-RIB
		// until this plugin has taken delivery of the peer-up event, so a peer
		// that waits for the End-of-RIB before sending cannot land in that window.
		PeerUpBarrier: true,
		// The route-server replay to a client that establishes is that client's
		// initial routing update, and sendEOR reports when it is out
		// (server_handlers.go). Distinct from PeerUpBarrier above: that one says
		// this plugin has REGISTERED the peer, this one says its ROUTES are out.
		SignalsSessionReady: true,
		// Its peer-up replay and its live forwards reach a peer on two rails:
		// the engine holds the live ones until the replay is reported.
		FencesLiveForwards: true,
		// The reactor fast path (rs-fast-path) forwards in place of this
		// plugin's cached forward, so it reads the grants that forward
		// answers to: the source's `receive [ update-received ]` to this
		// plugin, without which this plugin never sees the UPDATE, and the
		// destination's `send [ update ]`, which ForwardCached is checked
		// against. Compiled-in only, like the fast path itself
		// (filterapi.EnableRSForwarding above): the engine reads this field
		// from the registry linked into the daemon (registry.RelaysOnRSFastPath)
		// and Stage 1 carries no such field, so it holds whether this plugin
		// runs in-process or forked
		// (test/plugin/bgp-rs-opaque-transparency-forked.ci sets rs-fast-path
		// with bgp-rs forked), and a third-party route server cannot declare it.
		RelaysOnRSFastPath: true,
		RunEngine:          RunRouteServer,
		// RFC 7947 Section 2.2 transparency toward every route-server client,
		// served by evaluateRSFilter and inserted by the client's session
		// fact (transparency.go).
		Filters:          rsFilterDecls(),
		FilterInsertions: rsInsertions(),
		Commands:         commandDecls(),
		ParticipantNodes: rsParticipantNodes(),
		Sends:            rsBusSends(),
		Receives:         rsBusReceives(),
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
		panic("BUG: rs: registration failed: " + err.Error())
	}
}
