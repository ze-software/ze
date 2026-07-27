package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// Build-buffer capacity guards for the QUEUED announce rail.
//
// buildRIBRouteUpdate writes everything into ONE pooled 4096-byte slot:
// attributes growing up from offset 0, the NLRI parked at the tail. attrBuf is
// backing[off:off+4096] out of a 128-slot slab (session.go), so its CAP runs into
// the next peer's buffer and a walk past len does not panic on most slots -- it
// reads and writes the neighbor's memory, and `attrBuf[:off]` is then handed to
// sendUpdateWithSplit and transmitted.
//
// The batch rail's equivalent guards live in reactor_api_batch_capacity_test.go.
// Which rail encodes a given route is decided by Peer.ShouldQueue(), i.e. by
// scheduling, so both rails need the same bound or the daemon's memory safety
// depends on timing.

// bigCommunities returns a COMMUNITIES attribute of n*4 octets.
func bigCommunities(n int) attribute.Communities {
	comms := make(attribute.Communities, n)
	for i := range comms {
		comms[i] = attribute.Community(uint32(i)) //nolint:gosec // G115: bounded by n
	}
	return comms
}

// TestBuildRIBRouteUpdate_RejectsAttributesTooLargeForBuildBuffer drives the
// queued rail with a stored route whose optional attributes cannot fit.
//
// Before the guard, all eleven WriteAttrTo calls in this builder were unbounded:
// the offset ran past len(attrBuf) and the returned `attrBuf[:off]` reslice
// succeeded into the neighboring slab slot. A stored LARGE/COMMUNITIES block
// large enough is reachable from any relayed route.
//
// VALIDATES: a queued route whose attributes exceed the build buffer is rejected
// (nil) rather than emitted from past len(attrBuf).
// PREVENTS: the cross-session disclosure and the last-slot panic on the rail the
// round-1 and round-2 fixes never touched.
func TestBuildRIBRouteUpdate_RejectsAttributesTooLargeForBuildBuffer(t *testing.T) {
	n := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	route := rib.NewRouteWithASPath(n, netip.MustParseAddr("10.0.0.1"),
		[]attribute.Attribute{bigCommunities(400)}, // 1600 octets
		&attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65001}}}})

	// One slab slot's worth of len, with cap running into the "neighbor".
	const slot = 512
	backing := make([]byte, 2*slot)
	for i := range backing[slot:] {
		backing[slot+i] = 0x5A
	}
	attrBuf := backing[0:slot:len(backing)]
	require.Greater(t, cap(attrBuf), len(attrBuf), "fixture must reproduce cap-past-len, or it proves nothing")

	update := buildRIBRouteUpdate(attrBuf, route, 65000, false /*eBGP*/, true /*asn4*/, false /*addPath*/)
	assert.Nil(t, update, "a route that does not fit the slot must be rejected, not resliced past len")
}

// TestBuildRIBRouteUpdate_AttributesNeverReachTheNLRIRegion is the second bound,
// and the one len(attrBuf) alone would not give.
//
// The NLRI is written at the TAIL of the same buffer the attributes grow into. A
// guard that only stopped writes past len(attrBuf) would still let the attributes
// overwrite the prefix the UPDATE is announcing -- silent corruption, not a crash.
// Here the attributes fit the buffer but not the region below the NLRI.
//
// VALIDATES: the build is rejected when the attributes would reach the NLRI
// region, even though they fit within len(attrBuf).
// PREVENTS: an UPDATE announcing a prefix built from overwritten bytes.
func TestBuildRIBRouteUpdate_AttributesNeverReachTheNLRIRegion(t *testing.T) {
	n := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	nlriLen := nlri.LenWithContext(n, false)
	require.Equal(t, 4, nlriLen, "fixture assumes a 4-octet /24 NLRI")

	comms := bigCommunities(20) // 80 octets value, 84 on the wire
	route := rib.NewRouteWithASPath(n, netip.MustParseAddr("10.0.0.1"),
		[]attribute.Attribute{comms},
		&attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: []uint32{65001}}}})

	// Size the buffer so the attributes fit len(buf) but collide with the NLRI
	// tail: find the exact fitting size first, then take the NLRI's room away.
	big := make([]byte, message.MaxMsgLen)
	fitted := buildRIBRouteUpdate(big, route, 65000, false, true, false)
	require.NotNil(t, fitted)
	need := len(fitted.PathAttributes)

	t.Run("exact-fit-is-accepted", func(t *testing.T) {
		buf := make([]byte, need+nlriLen)
		update := buildRIBRouteUpdate(buf, route, 65000, false, true, false)
		require.NotNil(t, update, "attributes plus NLRI exactly filling the buffer must be built")
		assert.Len(t, update.PathAttributes, need)
		assert.Len(t, update.NLRI, nlriLen)
	})

	t.Run("one-octet-short-is-refused", func(t *testing.T) {
		buf := make([]byte, need+nlriLen-1)
		assert.Nil(t, buildRIBRouteUpdate(buf, route, 65000, false, true, false),
			"attributes must not be allowed to reach the NLRI region")
	})
}

// TestBuildWithdrawNLRI_RejectsNLRITooLargeForItsRegion covers the queued
// withdraw builder, whose two regions share one slot in the opposite arrangement:
// the NLRI at a fixed offset of 2048, the MP_UNREACH_NLRI attribute built from
// offset 0 by copying FROM that region.
//
// Both bounds are load-bearing. An NLRI longer than the tail walks past len(buf)
// (WriteNLRI's index writes panic); an attribute block longer than 2048 overwrites
// the NLRI bytes it is still copying from, which corrupts the withdrawal instead
// of failing it.
//
// VALIDATES: buildWithdrawNLRI returns nil rather than writing past its regions.
// PREVENTS: a panic, and an overlapping copy silently corrupting an MP withdrawal.
func TestBuildWithdrawNLRI_RejectsNLRITooLargeForItsRegion(t *testing.T) {
	v6 := nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::1/128"), 0)

	t.Run("mp-nlri-past-the-tail-region", func(t *testing.T) {
		// 2048 + 17 > 2050: the NLRI does not fit above nlriRegion.
		assert.Nil(t, buildWithdrawNLRI(make([]byte, 2050), v6, false),
			"an NLRI that does not fit above the 2048-byte region must be rejected")
	})

	t.Run("full-size-buffer-still-builds", func(t *testing.T) {
		update := buildWithdrawNLRI(make([]byte, message.MaxMsgLen), v6, false)
		require.NotNil(t, update, "the ordinary case must keep working")
		assert.NotEmpty(t, update.PathAttributes)
	})

	t.Run("ipv4-into-a-buffer-shorter-than-the-nlri", func(t *testing.T) {
		v4 := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
		assert.Nil(t, buildWithdrawNLRI(make([]byte, 2), v4, false),
			"an NLRI longer than the whole buffer must be rejected, not written past len")
	})
}

// TestSendUpdateWithSplit_RejectedBuildIsRouteScoped pins the contract that lets
// every builder above answer nil.
//
// Splitter.Split dereferences its *Update, so a nil from a rejected build would
// panic at the send site -- the very crash the guards exist to stop, moved one
// frame down. It must become an error, and specifically a ROUTE-scoped one: the
// initial-sync drain loops skip the offending route for these and tear the session
// down for anything else, so mis-classifying it would drop a peer over one
// unencodable route.
//
// VALIDATES: a nil update yields errBuildRejected, and errBuildRejected is
// route-scoped.
// PREVENTS: a nil-dereference panic in Splitter.Split, and a session teardown
// caused by one oversized queued route.
func TestSendUpdateWithSplit_RejectedBuildIsRouteScoped(t *testing.T) {
	p := NewPeer(&PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000,
	})

	err := p.sendUpdateWithSplit(nil, message.MaxMsgLen, false)
	require.ErrorIs(t, err, errBuildRejected, "a rejected build must not reach Splitter.Split")
	assert.True(t, isRouteScopedSendError(err), "a rejected build condemns the route, not the session")

	assert.True(t, isRouteScopedSendError(message.ErrAttributesTooLarge))
	assert.True(t, isRouteScopedSendError(message.ErrNLRITooLarge))
	assert.False(t, isRouteScopedSendError(ErrConnectionClosed),
		"a connection error must still stop the drain loop")
}
