package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC 7606 Section 2 treat-as-withdraw at the RIB boundary.
//
// The reactor rewrites a malformed UPDATE into a withdrawal with
// message.SynthesizeWithdraw and dispatches it through the normal received-UPDATE
// path. These tests feed the EXACT SynthesizeWithdraw output into the structured
// handler so they pin the RIB consequence the reactor tests cannot see:
//   - the previously-installed route is removed from the Adj-RIB-In, and
//   - the removal propagates to the Loc-RIB best path (a best-change Withdraw is
//     published on the EventBus).
// Merely observing that a withdrawal-shaped message was dispatched is the
// reactor's concern (TestSessionRFC7606TreatAsWithdrawDispatchesWithdrawal); here
// the route must actually leave the RIB.

// bestChangePrefixes returns every prefix published on the test EventBus whose
// best-change action matches action (in wire order across all batches).
func bestChangePrefixes(bus *testEventBus, action routeaction.Action) []netip.Prefix {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var out []netip.Prefix
	for i := range bus.events {
		batch, ok := bus.events[i].Payload.(*bestChangeBatch)
		if !ok {
			continue
		}
		for j := range batch.Changes {
			if batch.Changes[j].Action == action {
				out = append(out, batch.Changes[j].Prefix)
			}
		}
	}
	return out
}

// feedReceived drives handleReceivedStructured with a WireUpdate built from a raw
// UPDATE body under the given encoding context, exactly as the reactor would after
// synthesizing a withdrawal.
func feedReceived(r *RIBManager, peer netip.Addr, ctxID bgpctx.ContextID, body []byte) {
	feedReceivedFromGroup(r, peer, "", ctxID, body)
}

// feedReceivedFromGroup is feedReceived for a session that belongs to a peer
// group. group is what bgp/server's getStructuredEvent puts on every event for
// such a session, and it is the only identity a session the reactor created
// from a dynamic group shares with the operator's config document.
func feedReceivedFromGroup(r *RIBManager, peer netip.Addr, group string, ctxID bgpctx.ContextID, body []byte) {
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, _ := wu.Attrs()
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: peer.String(),
		PeerGroup:   group,
		PeerAS:      65001,
		LocalAS:     65000,
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
}

// VALIDATES: AC-1 (Loc-RIB) + AC-5 — a treat-as-withdraw re-advertisement removes
// the previously-installed route from the Adj-RIB-In AND publishes a best-change
// Withdraw to the Loc-RIB; a treat-as-withdraw for a never-installed prefix removes
// nothing and publishes no spurious best-change event.
// PREVENTS: a malformed re-announcement leaving a stale route installed (the exact
// blackhole RFC 7606 Section 2 forbids), and spurious churn on a never-seen prefix.
//
// RFC requirement: RFC7606-2-1 negative — the malformed UPDATE's routes are withdrawn
// end-to-end, all the way to the Loc-RIB best path.
func TestRIBTreatAsWithdrawRemovesInstalledRoute(t *testing.T) {
	bus := newTestEventBus()
	r := newTestRIBManagerWithBus(bus)
	peer := netip.MustParseAddr("192.0.2.1")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	// A valid UPDATE announcing 10.0.0.0/8: Withdrawn length 0, 14 octets of
	// well-known mandatory attributes, then the NLRI.
	announce := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
	feedReceived(r, peer, ctxID, announce)
	require.Equal(t, 1, r.bgpPeers[peer].Len(),
		"a valid UPDATE installs the route in the Adj-RIB-In")
	require.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		bestChangePrefixes(bus, ribevents.BestChangeAdd),
		"the install publishes a best-change Add for the Loc-RIB")

	// The same prefix re-announced with a MALFORMED ORIGIN (length 2), run through
	// the RFC 7606 treat-as-withdraw synthesis. Feeding that withdrawal must remove
	// 10.0.0.0/8 from the Adj-RIB-In and publish a best-change Withdraw.
	malformed := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0f, // Total Path Attribute Length 15
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN with length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
	withdrawal, changed := message.SynthesizeWithdraw(malformed)
	require.True(t, changed, "treat-as-withdraw must produce a withdrawal for an announced route")
	feedReceived(r, peer, ctxID, withdrawal)
	assert.Equal(t, 0, r.bgpPeers[peer].Len(),
		"AC-1: treat-as-withdraw removes the route from the Adj-RIB-In")
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		bestChangePrefixes(bus, ribevents.BestChangeWithdraw),
		"AC-1: the withdrawal propagates to the Loc-RIB best path")

	// AC-5: a treat-as-withdraw for a prefix that was never installed must not install
	// anything and must not publish a best-change event (FamilyRIB.Remove is a no-op for
	// an absent prefix; checkBestPathChange emits nothing when there was no best path).
	eventsBefore := bus.eventCount()
	newAnnounce := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x18, 0x0a, 0x09, 0x09, // NLRI 10.9.9.0/24 (never installed)
	}
	newWithdraw, changed2 := message.SynthesizeWithdraw(newAnnounce)
	require.True(t, changed2)
	feedReceived(r, peer, ctxID, newWithdraw)
	assert.Equal(t, 0, r.bgpPeers[peer].Len(),
		"AC-5: withdrawing a never-installed prefix installs nothing")
	assert.Equal(t, eventsBefore, bus.eventCount(),
		"AC-5: withdrawing a never-installed prefix publishes no best-change event")
}

// VALIDATES: AC-6 — on an ADD-PATH session, a treat-as-withdraw UPDATE that
// re-advertises exactly one (pathID, prefix) withdraws precisely that path; the
// sibling path with a different path ID for the same prefix survives. The path ID
// rides through message.SynthesizeWithdraw verbatim (NLRI bytes copied opaquely),
// so the withdrawal keys on the exact ADD-PATH wire entry.
// PREVENTS: a treat-as-withdraw dropping the wrong path, or all paths, of a prefix
// under RFC 7911.
//
// RFC requirement: RFC7606-2-1 negative — ADD-PATH path IDs are preserved so exactly
// the re-advertised path is withdrawn.
func TestRIBTreatAsWithdrawAddPathPreservesPathID(t *testing.T) {
	r := newTestRIBManager(t)
	peer := netip.MustParseAddr("192.0.2.2")
	ctxID, _ := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true}))

	// Announce two paths for 10.0.0.0/24: path ID 42 and path ID 43. ADD-PATH NLRI is
	// [pathID:4][prefix-len:1][prefix-bytes].
	path42 := []byte{0x00, 0x00, 0x00, 0x2a, 0x18, 0x0a, 0x00, 0x00}
	path43 := []byte{0x00, 0x00, 0x00, 0x2b, 0x18, 0x0a, 0x00, 0x00}
	announce := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	}
	announce = append(announce, path42...)
	announce = append(announce, path43...)
	feedReceived(r, peer, ctxID, announce)
	require.Equal(t, 2, r.bgpPeers[peer].Len(), "both ADD-PATH routes install")

	// Re-advertise ONLY path 42 with a malformed ORIGIN, run through the synthesis.
	malformed := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0f, // Total Path Attribute Length 15
		0x40, 0x01, 0x02, 0x00, 0x00, // ORIGIN with length 2 (invalid)
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
	}
	malformed = append(malformed, path42...)
	withdrawal, changed := message.SynthesizeWithdraw(malformed)
	require.True(t, changed)
	feedReceived(r, peer, ctxID, withdrawal)

	assert.Equal(t, 1, r.bgpPeers[peer].Len(),
		"AC-6: exactly the (pathID 42) path is withdrawn; the sibling path survives")
	_, found42 := r.bgpPeers[peer].Lookup(family.IPv4Unicast, path42)
	assert.False(t, found42, "AC-6: path ID 42 must be withdrawn")
	_, found43 := r.bgpPeers[peer].Lookup(family.IPv4Unicast, path43)
	assert.True(t, found43, "AC-6: path ID 43 (same prefix, different path) must remain")
}
