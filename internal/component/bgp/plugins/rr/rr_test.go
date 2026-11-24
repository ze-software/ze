// Design: docs/guide/route-reflection.md -- route reflector session scoping and replay.
// Related: rr.go -- handleStructuredState, the peer-up replay under test.
// Related: actor.go -- RFC 4456 Section 6 target selection.

package rr

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRRSkipsReplayOnAttachedUp proves R-11 for the route reflector: an up with
// reason attached marks the peer up and starts no replay, so ReplayGen does not
// move and no replayForPeer goroutine runs.
//
// VALIDATES: an attached internal peer is marked up without starting peer-up replay.
// PREVENTS: a live attach reflecting every stored route into a session whose
// initial routing update already went out.
func TestRRSkipsReplayOnAttachedUp(t *testing.T) {
	const target = "10.0.0.2"
	original := rrOriginalFixture(t, target, 702, 42)
	rr := &routeReflector{peers: map[peerIdentity]*peerState{original: {Address: target, Identity: original}}}
	require.NoError(t, rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: target, PeerOwner: original.owner,
		ConnectionID: original.connection, DestinationSession: 142, SessionGeneration: 1042, State: rpc.SessionStateUp, Reason: rpc.ReasonAttached, IsIBGP: true}))
	rr.mu.Lock()
	defer rr.mu.Unlock()
	assert.True(t, rr.peers[original].Up, "the session is up for the reflector")
	assert.Zero(t, rr.peers[original].ReplayGen, "an attached up starts no replay")
}

// TestRRTextSkipsReplayOnAttachedUp proves the same rule on the text event path
// (dispatchText): a state line carrying "reason attached" marks the peer up and
// starts no replay.
//
// VALIDATES: an attached internal peer over text events is marked up without replay.
// PREVENTS: an external route reflector fed text events replaying every stored
// route into a session a reload only attached it to.
func TestRRTextSkipsReplayOnAttachedUp(t *testing.T) {
	const target = "10.0.0.2"
	original := rrOriginalFixture(t, target, 702, 42)
	rr := &routeReflector{peers: map[peerIdentity]*peerState{original: {Address: target, Identity: original}}}
	require.NoError(t, rr.dispatchText("peer "+target+" remote as 65001 state up peer-owner 702 connection-id 42 destination-session 142 session-generation 1042 reason "+rpc.ReasonAttached+" is-ibgp true\n"))
	rr.mu.Lock()
	defer rr.mu.Unlock()
	assert.True(t, rr.peers[original].Up, "the session is up for the reflector")
	assert.Zero(t, rr.peers[original].ReplayGen, "an attached up starts no replay")
}

// TestReflectorServesInternalPeers proves RFC 4456 Section 6 session-type scoping
// for bgp-rr over both structured and native text state transports.
//
// VALIDATES: AC-19 and S14 — eBGP sources reflect to all internal peers (clients
// and non-clients); internal clients reflect to all other internal peers; internal
// non-clients reflect to internal clients only; eBGP peers never become targets
// and never schedule a peer-up replay; bgp-rr declares session-type=ibgp claim scope.
// PREVENTS: bgp-rr forwarding routes to eBGP peers, reflecting non-client routes
// to other non-clients, or replaying into eBGP sessions when loaded beside bgp-rs.
func TestReflectorServesInternalPeers(t *testing.T) {
	require.Equal(t,
		[]rpc.FilterSessionFact{{Fact: "session-type", Values: []string{"ibgp"}}},
		registry.ClaimSessionScopeFor("bgp-rr"),
	)

	type peerSpec struct {
		addr     string
		owner    uint64
		conn     uint64
		session  uint64
		isIBGP   bool
		rrClient bool
	}
	ebgp1Spec := peerSpec{addr: "192.0.2.1", owner: 701, conn: 41, session: 141, isIBGP: false, rrClient: false}
	ebgp2Spec := peerSpec{addr: "192.0.2.2", owner: 702, conn: 42, session: 142, isIBGP: false, rrClient: false}
	client1Spec := peerSpec{addr: "192.0.2.11", owner: 711, conn: 51, session: 151, isIBGP: true, rrClient: true}
	client2Spec := peerSpec{addr: "192.0.2.12", owner: 712, conn: 52, session: 152, isIBGP: true, rrClient: true}
	nonClient1Spec := peerSpec{addr: "192.0.2.21", owner: 721, conn: 61, session: 161, isIBGP: true, rrClient: false}
	nonClient2Spec := peerSpec{addr: "192.0.2.22", owner: 722, conn: 62, session: 162, isIBGP: true, rrClient: false}

	for _, transport := range []struct {
		name string
		up   func(rr *routeReflector, spec peerSpec, reason string, initialReplay uint64) error
	}{
		{
			name: "structured",
			up: func(rr *routeReflector, spec peerSpec, reason string, initialReplay uint64) error {
				return rr.handleStructuredState(&rpc.StructuredEvent{
					PeerAddress:          spec.addr,
					PeerOwner:            spec.owner,
					ConnectionID:         spec.conn,
					DestinationSession:   spec.session,
					SessionGeneration:    spec.conn + 1000,
					InitialReplay:        initialReplay,
					State:                rpc.SessionStateUp,
					Reason:               reason,
					IsIBGP:               spec.isIBGP,
					RouteReflectorClient: spec.rrClient,
				})
			},
		},
		{
			name: "native-text",
			up: func(rr *routeReflector, spec peerSpec, reason string, initialReplay uint64) error {
				var tb textbuf.Buffer
				tb.Str("peer ").Str(spec.addr).Str(" remote as 65001 state up")
				tb.Str(" peer-owner ").Uint(spec.owner)
				tb.Str(" connection-id ").Uint(spec.conn)
				tb.Str(" destination-session ").Uint(spec.session)
				tb.Str(" session-generation ").Uint(spec.conn + 1000)
				if reason != "" {
					tb.Str(" reason ").Str(reason)
				}
				if initialReplay != 0 {
					tb.Str(" initial-replay ").Uint(initialReplay)
				}
				if spec.isIBGP {
					tb.Str(" is-ibgp true")
				}
				if spec.rrClient {
					tb.Str(" rr-client true")
				}
				tb.Byte('\n')
				return rr.dispatchText(tb.String())
			},
		},
	} {
		t.Run(transport.name, func(t *testing.T) {
			rr := &routeReflector{}
			t.Cleanup(rr.stopDepartures)

			// Bring up eBGP peers with a real InitialReplay (no ReasonAttached):
			// they MUST stay Up as live sources and MUST NOT start a peer replay.
			require.NoError(t, transport.up(rr, ebgp1Spec, "", 901))
			require.NoError(t, transport.up(rr, ebgp2Spec, "", 902))
			for _, spec := range []peerSpec{client1Spec, client2Spec, nonClient1Spec, nonClient2Spec} {
				require.NoError(t, transport.up(rr, spec, rpc.ReasonAttached, 0))
			}

			ebgp1 := rrOriginalFixture(t, ebgp1Spec.addr, ebgp1Spec.owner, ebgp1Spec.conn)
			ebgp2 := rrOriginalFixture(t, ebgp2Spec.addr, ebgp2Spec.owner, ebgp2Spec.conn)
			client1 := rrOriginalFixture(t, client1Spec.addr, client1Spec.owner, client1Spec.conn)
			client2 := rrOriginalFixture(t, client2Spec.addr, client2Spec.owner, client2Spec.conn)
			nonClient1 := rrOriginalFixture(t, nonClient1Spec.addr, nonClient1Spec.owner, nonClient1Spec.conn)
			nonClient2 := rrOriginalFixture(t, nonClient2Spec.addr, nonClient2Spec.owner, nonClient2Spec.conn)

			rr.mu.RLock()
			require.True(t, rr.peers[ebgp1].Up, "eBGP peer remains a live source")
			require.False(t, rr.peers[ebgp1].Internal, "eBGP peer is not internal")
			require.Zero(t, rr.peers[ebgp1].ReplayGen, "eBGP peer UP must not schedule peer replay")
			require.Zero(t, rr.peers[ebgp2].ReplayGen, "eBGP peer UP must not schedule peer replay")
			rr.mu.RUnlock()
			require.Empty(t, rr.replays.snapshot(), "eBGP peer UP must not start a replay goroutine")

			targetAddrs := func(src peerIdentity) []string {
				got := rr.targets(src, family.Family{})
				addrs := make([]string, 0, len(got))
				for _, target := range got {
					addrs = append(addrs, target.identity.address.String())
				}
				slices.Sort(addrs)
				return addrs
			}

			// 1. eBGP source -> all internal peers (clients and non-clients), never another eBGP peer.
			require.Equal(t,
				[]string{client1Spec.addr, client2Spec.addr, nonClient1Spec.addr, nonClient2Spec.addr},
				targetAddrs(ebgp1),
			)

			// 2. Internal client -> other internal client and all internal non-clients, never eBGP peers.
			require.Equal(t,
				[]string{client2Spec.addr, nonClient1Spec.addr, nonClient2Spec.addr},
				targetAddrs(client1),
			)

			// 3. Internal non-client -> internal clients only, never another non-client or eBGP peers.
			require.Equal(t,
				[]string{client1Spec.addr, client2Spec.addr},
				targetAddrs(nonClient1),
			)

			// 4. targetCurrent rejects eBGP peers and accepts internal peers.
			require.False(t, rr.targetCurrent(peerTarget{identity: ebgp1, session: ebgp1Spec.session, generation: 0}))
			require.True(t, rr.targetCurrent(peerTarget{identity: client2, session: client2Spec.session, generation: 0}))
			require.True(t, rr.targetCurrent(peerTarget{identity: nonClient2, session: nonClient2Spec.session, generation: 0}))
			require.False(t, rr.targetCurrent(peerTarget{identity: ebgp2, session: ebgp2Spec.session, generation: 0}))
			require.True(t, rr.targetCurrent(peerTarget{identity: client1, session: client1Spec.session, generation: 0}))
			require.True(t, rr.targetCurrent(peerTarget{identity: nonClient1, session: nonClient1Spec.session, generation: 0}))
		})
	}
}
