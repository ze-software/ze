// Design: docs/architecture/api/architecture.md -- lifecycle deliveries follow compiled bus filters (AC-39, S32)
// Related: events.go -- peerStateRecipients, onPeerBound, onPeerUnbound, peerStateUnheldRoles

package server

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLifecycleDeliveriesAreRegisteredReceptions verifies that peer bind,
// peer unbind, and OPEN-validator removal deliveries all pass through the
// compiled bus filter (PeerScopedProcs / PeerScopedProcsNamed), appear as
// registered receptions with origin "registered", and are suppressed when a
// participant's bus block narrows them out.
//
// VALIDATES: AC-39, S32 (no delivery bypasses CompiledBus); AC-19, S13
// (peerStateUnheldRoles honors ClaimSessionScope and structured events carry
// session facts IsIBGP / RSClient / RouteReflectorClient).
// PREVENTS: lifecycle deliveries bypassing receive allow-lists or from-lists,
// or bgp-rs / bgp-rr claiming replay on peers outside their session scope.
func TestLifecycleDeliveriesAreRegisteredReceptions(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	require.NoError(t, srv.StartWithContext(ctx))
	t.Cleanup(srv.Stop)

	var mu sync.Mutex
	received := map[string][]string{}

	makeProc := func(name string, wantsOpen bool, receives []rpc.MessageReceiveDecl) *process.Process {
		p := selectedStateBridgeProcess(t, name, make(chan selectedStateObservation, 1))
		p.SetRegistration(&plugin.PluginRegistration{
			WantsValidateOpen: wantsOpen,
			Receives:          receives,
			Done:              true,
		})
		p.Bridge().SetDeliverStructured(func(values []any) error {
			mu.Lock()
			for _, v := range values {
				if se, ok := v.(*rpc.StructuredEvent); ok {
					received[name] = append(received[name], se.State.String()+":"+se.Reason)
				}
			}
			mu.Unlock()
			return nil
		})
		return p
	}

	stateMsg := bgpevents.Namespace + ":" + bgpevents.EventState
	negMsg := bgpevents.Namespace + ":" + bgpevents.EventNegotiated

	collector := makeProc("collector", false, []rpc.MessageReceiveDecl{
		{Message: stateMsg},
		{Message: negMsg},
	})
	validator := makeProc("open-validator", true, []rpc.MessageReceiveDecl{
		{Message: stateMsg},
	})

	stateET := events.LookupEventTypeID(bgpevents.EventState)
	negET := events.LookupEventTypeID(bgpevents.EventNegotiated)
	srv.Subscriptions().Add(collector, &pluginserver.Subscription{Namespace: bgpNS(), EventType: stateET, Direction: events.DirUnspecified})
	srv.Subscriptions().Add(collector, &pluginserver.Subscription{Namespace: bgpNS(), EventType: negET, Direction: events.DirUnspecified})
	srv.Subscriptions().Add(validator, &pluginserver.Subscription{Namespace: bgpNS(), EventType: stateET, Direction: events.DirUnspecified})

	peer := &plugin.PeerInfo{
		Name:                 "lab",
		Address:              netip.MustParseAddr("192.0.2.9"),
		PeerAS:               rpc.RemoteASOf(65001),
		LocalAS:              65000,
		IsIBGP:               false,
		RSClient:             true,
		RouteReflectorClient: false,
	}
	peerEntry := pluginserver.DeliveryPeer{
		Addr:           peer.AddrStr(),
		Name:           peer.Name,
		Path:           "/bgp/peer/lab",
		SecondaryPaths: []string{"/bgp/ip/192.0.2.9"},
	}

	endpoints := func(valBlock, colBlock plugin.BusBlock) []pluginserver.BusParticipantInput {
		return []pluginserver.BusParticipantInput{
			{
				Path:         plugin.EndpointPath("collector"),
				EndpointName: "collector",
				Receives:     []rpc.MessageReceiveDecl{{Message: stateMsg}, {Message: negMsg}},
				OwnBlock:     colBlock,
			},
			{
				Path:         plugin.EndpointPath("open-validator"),
				EndpointName: "open-validator",
				Receives:     []rpc.MessageReceiveDecl{{Message: stateMsg}},
				OwnBlock:     valBlock,
			},
		}
	}

	// 1. Default registration: both collector and open-validator receive
	// lifecycle deliveries with origin "registered".
	require.NoError(t, srv.UpdateDeliveryGraphWithEndpoints(bgpevents.Namespace, []pluginserver.DeliveryPeer{peerEntry}, endpoints(plugin.BusBlock{}, plugin.BusBlock{})))

	inspected := srv.DeliveryGraph().Inspect()
	require.Len(t, inspected, 1)
	for _, pe := range inspected[0].Processes {
		assert.Equal(t, pluginserver.BusOriginRegistered.String(), pe.Origin,
			"registered lifecycle reception for %s must show origin registered", pe.Process)
	}

	enc := format.NewJSONEncoder("test")
	onPeerBound(srv, enc, peer, nil, []string{"collector"}, nil)
	onPeerUnbound(srv, peer, rpc.SessionStateDown, "detached", []string{"collector"})
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)

	mu.Lock()
	assert.Equal(t, []string{"up:attached", "down:detached", "down:removed"}, received["collector"])
	assert.Equal(t, []string{"down:removed"}, received["open-validator"])
	received["collector"] = nil
	received["open-validator"] = nil
	mu.Unlock()

	// 2. Narrowed out via bus block: receive { allow none; } on open-validator
	// and receive { from none; } on collector MUST suppress bind and
	// OPEN-validator removal; an endpoint never admitted on the peer receives
	// nothing on bind, unbind, or removal with zero filter bypass.
	neverAdmitted := makeProc("never-admitted", false, []rpc.MessageReceiveDecl{{Message: stateMsg}})
	srv.Subscriptions().Add(neverAdmitted, &pluginserver.Subscription{Namespace: bgpNS(), EventType: stateET, Direction: events.DirUnspecified})

	narrowVal := plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{AllowPresent: true, AllowNone: true},
	}
	narrowCol := plugin.BusBlock{
		Receive: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
	}
	eps := append(endpoints(narrowVal, narrowCol), pluginserver.BusParticipantInput{
		Path:         plugin.EndpointPath("never-admitted"),
		EndpointName: "never-admitted",
		Receives:     []rpc.MessageReceiveDecl{{Message: stateMsg}},
		OwnBlock:     narrowCol,
	})
	require.NoError(t, srv.UpdateDeliveryGraphWithEndpoints(bgpevents.Namespace, []pluginserver.DeliveryPeer{peerEntry}, eps))

	onPeerBound(srv, enc, peer, nil, []string{"collector", "never-admitted"}, nil)
	onPeerUnbound(srv, peer, rpc.SessionStateDown, "detached", []string{"never-admitted"})
	onPeerStateChange(srv, peer, rpc.SessionStateDown, rpc.ReasonPeerRemoved)

	mu.Lock()
	assert.Empty(t, received["collector"], "narrowed collector must not receive bind or removal")
	assert.Empty(t, received["never-admitted"], "unadmitted endpoint must not receive bind, unbind, or removal")
	assert.Empty(t, received["open-validator"], "narrowed OPEN validator must not receive removal")
	mu.Unlock()

	// 3. Structured events carry IsIBGP, RSClient, and RouteReflectorClient.
	seState := getStructuredStateEvent(peer, rpc.SessionStateUp, "", nil)
	assert.Equal(t, peer.IsIBGP, seState.IsIBGP)
	assert.Equal(t, peer.RSClient, seState.RSClient)
	assert.Equal(t, peer.RouteReflectorClient, seState.RouteReflectorClient)
	rpc.PutStructuredEvent(seState)

	seMsg := getStructuredEvent(peer, &bgptypes.RawMessage{Type: msgtype.TypeKEEPALIVE, Direction: rpc.DirectionReceived})
	assert.Equal(t, peer.IsIBGP, seMsg.IsIBGP)
	assert.Equal(t, peer.RSClient, seMsg.RSClient)
	assert.Equal(t, peer.RouteReflectorClient, seMsg.RouteReflectorClient)
	rpc.PutStructuredEvent(seMsg)

	// 4. peerStateUnheldRoles respects ClaimSessionScope (S13): RS claims
	// rs-client peers, RR claims iBGP peers, ordinary eBGP leaves replay unheld.
	snap := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snap) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:              "test-rs-claimant",
		Claims:            []string{"bgp-peer-up-replay"},
		ClaimSessionScope: []rpc.FilterSessionFact{{Fact: "rs-client", Values: []string{"true"}}},
		Sends:             []rpc.MessageSendDecl{{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate}},
		Receives:          []rpc.MessageReceiveDecl{{Message: stateMsg}},
		RunEngine:         func(net.Conn) int { return 0 },
		CLIHandler:        func([]string) int { return 0 },
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:              "test-rr-claimant",
		Claims:            []string{"bgp-peer-up-replay"},
		ClaimSessionScope: []rpc.FilterSessionFact{{Fact: "session-type", Values: []string{"ibgp"}}},
		Sends:             []rpc.MessageSendDecl{{Message: bgpevents.Namespace + ":" + bgpevents.SendUpdate}},
		Receives:          []rpc.MessageReceiveDecl{{Message: stateMsg}},
		RunEngine:         func(net.Conn) int { return 0 },
		CLIHandler:        func([]string) int { return 0 },
	}))

	rsProc := process.NewProcess(plugin.PluginConfig{Name: "test-rs-claimant", CompiledPlugin: "test-rs-claimant", Internal: true})
	rrProc := process.NewProcess(plugin.PluginConfig{Name: "test-rr-claimant", CompiledPlugin: "test-rr-claimant", Internal: true})
	assert.True(t, peerMatchesClaimScope(&plugin.PeerInfo{RSClient: true, IsIBGP: false}, registry.ClaimSessionScopeFor(rsProc.RegistrationName())))
	assert.False(t, peerMatchesClaimScope(&plugin.PeerInfo{RSClient: false, IsIBGP: false}, registry.ClaimSessionScopeFor(rsProc.RegistrationName())))
	assert.True(t, peerMatchesClaimScope(&plugin.PeerInfo{RSClient: false, IsIBGP: true}, registry.ClaimSessionScopeFor(rrProc.RegistrationName())))
	assert.False(t, peerMatchesClaimScope(&plugin.PeerInfo{RSClient: false, IsIBGP: false}, registry.ClaimSessionScopeFor(rrProc.RegistrationName())))
}
