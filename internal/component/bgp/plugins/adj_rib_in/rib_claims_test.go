// Related: rib_claims.go -- the stand-down decision under test

package adj_rib_in

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/seqmap"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// claimedManagerWithRoute returns a manager holding one stored route, with the
// peer-up replay role already claimed by another plugin, and the destinations
// its relay was asked for.
func claimedManagerWithRoute(t *testing.T) (*AdjRIBInManager, *[]string) {
	t.Helper()
	r := newTestManager(t)
	m := seqmap.New[compactRouteKey, *RawRoute]()
	m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
		Family: family.IPv4Unicast, AttrHex: "40010100",
		NHopHex: "0a000001", NLRIHex: "180a0000",
	})
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = m

	var destinations []string
	r.routeRelayer = func(dest string, routes []rpc.StoredRoute) error {
		if len(routes) > 0 {
			destinations = append(destinations, dest)
		}
		return nil
	}

	r.applyStartupClaims(func(role string) bool { return role == claimPeerUpReplay })
	require.True(t, r.replayOwned.Load(), "the fixture needs the role claimed")
	return r, &destinations
}

// TestSelfReplayCoversAPeerTheOwnerIsNotFed pins the per-peer scope of a
// daemon-wide stand-down.
//
// VALIDATES: spec-fixit-stored-route-relay-hardening AC-4 -- a peer that gives
// `state` to bgp-adj-rib-in and not to the plugin holding the peer-up-replay
// role is still replayed. Both ingest paths are driven: the structured bridge an
// in-process plugin takes delivery on, and the JSON event a forked one parses.
// PREVENTS: that peer being served by nobody. The claim is daemon-wide and
// delivery is per-peer, so bgp-rs stands adj-rib-in down for every peer while
// replaying only the peers it is fed -- and it never makes the others forward
// targets either (rs/server_forward.go selectForwardTargets keys on peer.Up,
// which only rs/server_handlers.go handleState sets).
func TestSelfReplayCoversAPeerTheOwnerIsNotFed(t *testing.T) {
	const dest = "10.0.0.2"

	jsonUp := func(unheld []string) *bgp.Event {
		return &bgp.Event{
			Type:        "state",
			State:       "up",
			UnheldRoles: unheld,
			Peer: mustMarshal(t, bgp.PeerInfoJSON{
				Remote: bgp.PeerRemoteInfo{Address: dest, AS: 65002},
			}),
		}
	}
	structuredUp := func(unheld []string) *rpc.StructuredEvent {
		return &rpc.StructuredEvent{
			EventType:   rpc.EventKindState,
			PeerAddress: dest,
			State:       rpc.SessionStateUp,
			UnheldRoles: unheld,
		}
	}

	t.Run("json ingest", func(t *testing.T) {
		t.Run("owner not fed this peer: self-replay", func(t *testing.T) {
			r, got := claimedManagerWithRoute(t)
			r.handleState(jsonUp([]string{claimPeerUpReplay}))
			assert.Equal(t, []string{dest}, *got,
				"the engine retracted the claim for this peer, so this plugin owes it a replay")
		})

		t.Run("owner fed this peer: stand down", func(t *testing.T) {
			r, got := claimedManagerWithRoute(t)
			r.handleState(jsonUp(nil))
			assert.Empty(t, *got,
				"the claim holds for this peer, so the owner replays it and this plugin must not")
		})

		t.Run("an unrelated retraction changes nothing", func(t *testing.T) {
			r, got := claimedManagerWithRoute(t)
			r.handleState(jsonUp([]string{"some-other-role"}))
			assert.Empty(t, *got,
				"only the peer-up-replay role decides this plugin's peer-up behavior")
		})
	})

	t.Run("structured ingest", func(t *testing.T) {
		t.Run("owner not fed this peer: self-replay", func(t *testing.T) {
			r, got := claimedManagerWithRoute(t)
			r.handleStructuredState(structuredUp([]string{claimPeerUpReplay}))
			assert.Equal(t, []string{dest}, *got,
				"the structured path owes the same replay as the JSON one")
		})

		t.Run("owner fed this peer: stand down", func(t *testing.T) {
			r, got := claimedManagerWithRoute(t)
			r.handleStructuredState(structuredUp(nil))
			assert.Empty(t, *got, "the claim holds for this peer on this path too")
		})
	})

	// The retraction says who holds a role. It says nothing about who claimed
	// one, so it must not turn self-replay off where it was never on.
	t.Run("unclaimed plugin ignores the retraction", func(t *testing.T) {
		r := newTestManager(t)
		m := seqmap.New[compactRouteKey, *RawRoute]()
		m.Put(routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
			Family: family.IPv4Unicast, AttrHex: "40010100",
			NHopHex: "0a000001", NLRIHex: "180a0000",
		})
		r.ribIn[netip.MustParseAddr("10.0.0.1")] = m
		var got []string
		r.routeRelayer = func(d string, routes []rpc.StoredRoute) error {
			if len(routes) > 0 {
				got = append(got, d)
			}
			return nil
		}

		r.handleState(jsonUp(nil))
		assert.Equal(t, []string{dest}, got,
			"with nothing claimed, peer-up replays whatever the engine says about roles")
	})
}
