package reactor

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// Build-buffer capacity guard for the batch announce rail.
//
// attrBuf comes from getBuildBuf, which returns backing[off:off+4096] out of a
// 128-slot slab (session.go), so the slice's CAP runs into the NEXT peer's buffer
// while its LEN is the slot. A build that walks past len(attrBuf) therefore does
// not panic on most slots -- it silently reads and writes the neighbor's memory,
// and the resulting attrBuf[:attrOff] is handed to sendUpdateWithSplit and
// transmitted. On the slab's last slot cap == len, so the same reslice panics and
// takes the daemon down instead.
//
// These tests live in their own file rather than beside the ordering tests: that
// file's cases carry `RFC requirement:` tags for RFC 4271 Section 5 attribute
// ordering, and this is a memory-safety concern, not an ordering one.

// TestAnnounceNLRIBatch_RejectsBatchTooLargeForBuildBuffer drives the guard from
// the API entry point with enough NLRI to overflow the 4K build slot.
//
// It asserts the FAILURE IS REPORTED, deliberately not that the emitted bytes are
// right. A byte-content assertion could not have caught the defect: on most slots
// the leaked neighbor bytes are indistinguishable from uninitialized buffer, and
// on the last slot the test would panic before asserting anything. An error
// return is the only observation that separates "rejected" from "sent something
// wrong".
//
// VALIDATES: an announce whose attributes cannot fit the build buffer is rejected
// with errAnnounceTooLarge, and no UPDATE is sent for it.
// PREVENTS: the reslice past len(attrBuf) that put another peer's buffer contents
// on the wire, and the panic on the slab's final slot.
func TestAnnounceNLRIBatch_RejectsBatchTooLargeForBuildBuffer(t *testing.T) {
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000, // iBGP
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{family.IPv6Unicast: true},
		ExtendedMessage: false,
	})

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	adapter := &reactorAPIAdapter{r: r}

	// 240 IPv6 /128s at 17 wire bytes each (1 length octet + 16 address) = 4080,
	// which with the MP_REACH header, AFI/SAFI, next-hop and reserved octet
	// overflows the 4096-byte slot.
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   manyIPv6Host(240),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	err := adapter.AnnounceNLRIBatch(selector.All(), batch)
	require.Error(t, err, "an announce that cannot be encoded must not report success")
	assert.ErrorIs(t, err, errAnnounceTooLarge)
}

// TestInsertAttrOrdered_RefusesToExceedBuffer pins the guard itself at its own
// boundary: the last attribute that fits is written, and one octet more is
// refused rather than written past len.
//
// VALIDATES: insertAttrOrdered returns ok=false and leaves attrOff untouched when
// the attribute would not fit, and ok=true at exactly the fitting size.
// PREVENTS: an off-by-one in the capacity check re-opening the out-of-slot write.
func TestInsertAttrOrdered_RefusesToExceedBuffer(t *testing.T) {
	// LOCAL_PREF is 7 wire bytes (3 header + 4 value).
	const localPrefWire = 7
	lp := attribute.LocalPref(100)
	require.Equal(t, localPrefWire, attrWireLen(lp), "fixture assumes a 7-byte LOCAL_PREF")

	t.Run("exact-fit-is-accepted", func(t *testing.T) {
		buf := make([]byte, localPrefWire)
		off, ok := insertAttrOrdered(buf, 0, lp)
		require.True(t, ok, "an attribute that exactly fills the buffer must be written")
		assert.Equal(t, localPrefWire, off)
	})

	t.Run("one-octet-short-is-refused", func(t *testing.T) {
		buf := make([]byte, localPrefWire-1)
		off, ok := insertAttrOrdered(buf, 0, lp)
		assert.False(t, ok, "must refuse rather than write past len(buf)")
		assert.Equal(t, 0, off, "a refused insert must not advance the offset")
	})

	t.Run("refused-when-existing-content-leaves-too-little", func(t *testing.T) {
		buf := make([]byte, localPrefWire+3)
		off, ok := insertAttrOrdered(buf, 4, lp) // 4 used, 6 free, needs 7
		assert.False(t, ok)
		assert.Equal(t, 4, off, "a refused insert must leave the block length unchanged")
	})
}

// TestAnnounceNLRIBatch_RejectsNLRIsTooLargeForBuildBuffer drives the SECOND
// write into a pooled slot: the NLRI loop, which the round-1 capacity guard did
// not cover.
//
// insertAttrOrdered bounds the attribute inserts, so 240 IPv6 /128s (the case
// above) is caught there. The NLRI loop runs BEFORE any of that, writes into a
// different pooled slot, and had no bound at all: nlri.WriteNLRI ends in an index
// expression (INET.WriteTo), so it panics past len rather than clamping, taking
// the daemon down. 900 IPv6 /128s at 17 wire bytes is 15300 bytes into a 4096
// slot.
//
// VALIDATES: an announce whose NLRIs cannot fit the build buffer is rejected with
// errAnnounceTooLarge.
// PREVENTS: "panic: index out of range [4097] with length 4096" in
// nlri.(*INET).WriteTo, reachable from a plugin with `update ... nlri add
// <~250 IPv6 prefixes>`.
func TestAnnounceNLRIBatch_RejectsNLRIsTooLargeForBuildBuffer(t *testing.T) {
	adapter := establishedIPv6Adapter(t)

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   manyIPv6Host(900),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	err := adapter.AnnounceNLRIBatch(selector.All(), batch)
	require.Error(t, err, "an announce whose NLRIs cannot be encoded must not report success")
	assert.ErrorIs(t, err, errAnnounceTooLarge)
}

// TestWithdrawNLRIBatch_RejectsNLRIsTooLargeForBuildBuffer is the same guard on
// the withdraw rail, which carried an identical unbounded NLRI loop.
//
// A withdraw that cannot be encoded is not a lesser failure than an announce that
// cannot: the peer keeps forwarding to prefixes it was never told about. Before
// the guard this panicked in exactly the same place.
//
// VALIDATES: a withdraw whose NLRIs cannot fit the build buffer is rejected with
// errWithdrawTooLarge.
// PREVENTS: the same index-out-of-range panic reached through
// `update ... nlri del`.
func TestWithdrawNLRIBatch_RejectsNLRIsTooLargeForBuildBuffer(t *testing.T) {
	adapter := establishedIPv6Adapter(t)

	batch := bgptypes.NLRIBatch{
		Family: family.IPv6Unicast,
		NLRIs:  manyIPv6Host(900),
	}

	err := adapter.WithdrawNLRIBatch(selector.All(), batch)
	require.Error(t, err, "a withdraw whose NLRIs cannot be encoded must not report success")
	assert.ErrorIs(t, err, errWithdrawTooLarge)
}

// TestBuildBatchAnnounce_InvalidNextHopWithOversizeAttrs drives the THIRD write:
// writeMandatoryAttrs, and specifically the one route to its result that no
// downstream insert can catch.
//
// buildWireModeUpdate's inserts each reject a bad attrOff, but the IPv4 branch
// inserts NEXT_HOP only when the next-hop is VALID, and LOCAL_PREF only for iBGP.
// resolveNextHop deliberately passes an invalid explicit next-hop through
// (TestResolveNextHop_ExplicitInvalid), so an eBGP announce with one reached
// `attrBuf[:attrOff]` with nothing having checked attrOff -- and attrBuf's cap runs
// into the next peer's slab slot, so that reslice succeeds and transmits the
// neighbor's memory.
//
// The buffer here is deliberately SHORTER than its capacity, exactly as
// getBuildBuf's slices are, so a regression reproduces the disclosure rather than
// panicking.
//
// VALIDATES: an attribute block larger than the build buffer is rejected, on the
// path where no ordered insert runs.
// PREVENTS: the cross-session memory disclosure re-opening through
// writeMandatoryAttrs after the round-1 guard closed it through insertAttrOrdered.
func TestBuildBatchAnnounce_InvalidNextHopWithOversizeAttrs(t *testing.T) {
	// A COMMUNITIES attribute far larger than the 512-byte "slot" below.
	comms := make(attribute.Communities, 400) // 1600 octets
	for i := range comms {
		comms[i] = attribute.Community(uint32(i)) //nolint:gosec // G115: bounded by loop
	}
	b := attribute.NewBuilder()
	b.SetOrigin(0)
	b.SetASPath([]uint32{65001})
	for _, c := range comms {
		b.AddCommunity(uint16(c>>16), uint16(c)) //nolint:gosec // G115: bounded by loop
	}

	batch := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.Addr{}),
		Wire:    attribute.NewAttributesWire(b.Build(), bgpctx.APIContextID),
	}

	// backing models one slab: the builder gets slot[0], whose cap runs into slot[1].
	const slot = 512
	backing := make([]byte, 2*slot)
	for i := range backing[slot:] {
		backing[slot+i] = 0x5A // the "neighbor's" bytes
	}
	attrBuf := backing[0:slot:len(backing)]
	require.Greater(t, cap(attrBuf), len(attrBuf), "fixture must reproduce cap-past-len, or it proves nothing")
	nlriBuf := make([]byte, message.MaxMsgLen)

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}
	update := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.Addr{} /*invalid next-hop*/, false /*eBGP*/, false /*rsClient*/, true /*asn4*/, false /*addPath*/, 65000)

	require.Nil(t, update, "a block that does not fit the slot must be rejected, not resliced past len")
}

// TestWriteMandatoryAttrs_RejectsBlockLargerThanBuffer pins the guard itself, on
// each of the four arms, at its boundary.
//
// VALIDATES: writeMandatoryAttrs returns -1 rather than an offset past len(buf),
// and accepts at exactly the fitting size.
// PREVENTS: an off-by-one restoring the reslice-past-len on any one arm while the
// other three stay guarded.
func TestWriteMandatoryAttrs_RejectsBlockLargerThanBuffer(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}

	// Arm 1: ORIGIN and AS_PATH both present, copied verbatim.
	full := attribute.NewBuilder()
	full.SetOrigin(0)
	full.SetASPath([]uint32{65001})
	full.AddCommunity(65000, 1)
	packed := attribute.NewAttributesWire(full.Build(), bgpctx.APIContextID)
	packedLen := len(full.Build())

	t.Run("verbatim-exact-fit", func(t *testing.T) {
		n := adapter.writeMandatoryAttrs(make([]byte, packedLen), packed,
			true /*isIBGP*/, false, true, true, true, 65000, 0)
		assert.Equal(t, packedLen, n, "a block that exactly fills the buffer must be written")
	})
	t.Run("verbatim-one-octet-short", func(t *testing.T) {
		n := adapter.writeMandatoryAttrs(make([]byte, packedLen-1), packed,
			true /*isIBGP*/, false, true, true, true, 65000, 0)
		assert.Equal(t, -1, n, "must reject rather than return an offset past len(buf)")
	})

	// Arms 2-4: a block missing ORIGIN and/or AS_PATH, so the builder synthesizes
	// them and the returned offset is a SUM the clamped copy cannot bound.
	//
	// Each fixture carries 64 communities (259 wire octets) so the one-octet-short
	// buffer stays far above the function's fixed minimum for the synthesized
	// ORIGIN + AS_PATH. Sized any smaller, that minimum answers -1 first and the
	// sub-test passes without ever reaching the arm it names -- which is exactly
	// what it did until removing all three arm bounds left it green.
	manyCommunities := func(b *attribute.Builder) {
		for i := range 64 {
			b.AddCommunity(65000, uint16(i)) //nolint:gosec // G115: bounded by loop
		}
	}
	for _, tc := range []struct {
		name  string
		build func(*attribute.Builder)
	}{
		{"both-mandatory-missing", func(b *attribute.Builder) { manyCommunities(b) }},
		{"origin-missing", func(b *attribute.Builder) { b.SetASPath([]uint32{65001}); manyCommunities(b) }},
		{"as-path-missing", func(b *attribute.Builder) { b.SetOrigin(0); manyCommunities(b) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := attribute.NewBuilder()
			tc.build(b)
			wire := attribute.NewAttributesWire(b.Build(), bgpctx.APIContextID)

			// Generous buffer: accepted, and the offset stays inside it.
			big := make([]byte, message.MaxMsgLen)
			n := adapter.writeMandatoryAttrs(big, wire, false /*eBGP*/, false, true, true, true, 65000, 0)
			require.Positive(t, n, "must encode into a full-size buffer")
			require.LessOrEqual(t, n, len(big))

			// Exactly what it needs: accepted.
			assert.Equal(t, n, adapter.writeMandatoryAttrs(make([]byte, n), wire, false, false, true, true, true, 65000, 0),
				"a block that exactly fills the buffer must be written")

			// One octet short of what it just needed: rejected. The buffer must
			// still clear the fixed minimum, or a different guard answers.
			tight := make([]byte, n-1)
			require.Greater(t, len(tight), 64, "fixture must exercise the arm's own bound, not the fixed minimum")
			assert.Equal(t, -1, adapter.writeMandatoryAttrs(tight, wire, false, false, true, true, true, 65000, 0),
				"must reject rather than return an offset past len(buf)")
		})
	}
}

// TestNLRILenMatchesWriteNLRI is the invariant writeBatchNLRI rests on, checked
// directly rather than only through its consequences -- the NLRI twin of
// TestAttrWireLen_MatchesWriteAttrTo.
//
// writeBatchNLRI admits an NLRI when LenWithContext(n, addPath) fits the remaining
// buffer, and then calls WriteNLRI. If WriteNLRI ever wrote MORE than
// LenWithContext reports, the guard would wave through exactly the write that
// panics, and no capacity test above would notice.
//
// VALIDATES: WriteNLRI writes precisely LenWithContext bytes, with and without
// ADD-PATH.
// PREVENTS: a Len()/WriteTo() divergence in an NLRI type re-opening the
// out-of-range panic through the guard meant to stop it.
func TestNLRILenMatchesWriteNLRI(t *testing.T) {
	wireV6, err := nlri.NewWireNLRI(family.IPv6Unicast, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}, false)
	require.NoError(t, err)
	wireAddPath, err := nlri.NewWireNLRI(family.IPv6Unicast, []byte{0, 0, 0, 7, 0x20, 0x20, 0x01, 0x0d, 0xb8}, true)
	require.NoError(t, err)

	cases := []struct {
		name string
		n    nlri.NLRI
	}{
		{"inet-ipv4-24", nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)},
		{"inet-ipv4-32", nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.1/32"), 7)},
		{"inet-ipv4-default", nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("0.0.0.0/0"), 0)},
		{"inet-ipv6-128", nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix("2001:db8::1/128"), 0)},
		{"wire-passthrough", wireV6},
		{"wire-passthrough-addpath", wireAddPath},
	}

	for _, c := range cases {
		for _, addPath := range []bool{false, true} {
			t.Run(c.name, func(t *testing.T) {
				buf := make([]byte, message.MaxMsgLen)
				wrote := nlri.WriteNLRI(c.n, buf, 0, addPath)
				assert.Equal(t, wrote, nlri.LenWithContext(c.n, addPath),
					"LenWithContext must equal what WriteNLRI writes, or writeBatchNLRI's bound is a lie")
			})
		}
	}
}

// TestAttrWireLen_ExtendedLengthBoundary walks attrWireLen across the one value
// at which its answer changes shape: 255 octets take a 3-octet header, 256 take
// the 4-octet extended-length one (WriteHeaderTo promotes at length > 255).
//
// attrWireLen is now load-bearing twice over. insertAttrOrdered shifts the tail
// right by exactly this many bytes before writing -- so an over-estimate leaves a
// gap of whatever the pooled buffer last held, and an under-estimate overwrites
// the following attribute. Both capacity guards added here (insertAttrOrdered,
// attrWriter) also ADMIT a write on this number, so an under-estimate at the
// boundary waves through the very out-of-slot write they exist to stop.
//
// TestAttrWireLen_MatchesWriteAttrTo covers the attribute types the builders
// insert; none of them can be sized to 255 or 256 octets (communities come in
// 4-, 8- and 12-byte units), so the boundary itself was never exercised. An
// opaque attribute takes an arbitrary length and can sit on it.
//
// VALIDATES: attrWireLen equals what WriteAttrTo writes at 254, 255, 256 and 257
// octets of value.
// PREVENTS: an off-by-one at the extended-length promotion re-opening the
// out-of-slot write through the guard meant to close it.
func TestAttrWireLen_ExtendedLengthBoundary(t *testing.T) {
	for _, valueLen := range []int{254, 255, 256, 257} {
		t.Run(textLen(valueLen), func(t *testing.T) {
			attr := attribute.NewOpaqueAttribute(0xC0, 40, make([]byte, valueLen))

			buf := make([]byte, message.MaxMsgLen)
			wrote := attribute.WriteAttrTo(attr, buf, 0)
			assert.Equal(t, wrote, attrWireLen(attr),
				"attrWireLen must equal what WriteAttrTo writes across the 255/256 header boundary")

			wantHdr := 3
			if valueLen > 255 {
				wantHdr = 4
			}
			assert.Equal(t, wantHdr+valueLen, wrote, "header size must flip at 256 octets, not before or after")

			// And the guard built on it must accept an exact fit and refuse one octet less.
			exact := make([]byte, wrote)
			_, ok := insertAttrOrdered(exact, 0, attr)
			assert.True(t, ok, "an attribute that exactly fills the buffer must be written")

			short := make([]byte, wrote-1)
			_, ok = insertAttrOrdered(short, 0, attr)
			assert.False(t, ok, "one octet short must be refused, not written past len")
		})
	}
}

// textLen names a sub-test after a byte count.
func textLen(n int) string {
	return "value-" + strconv.Itoa(n) + "-octets"
}

// establishedIPv6Adapter returns an adapter with one established IPv6-unicast
// iBGP peer, the fixture both capacity tests drive the API entry point through.
func establishedIPv6Adapter(t *testing.T) *reactorAPIAdapter {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65000, // iBGP
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:        map[family.Family]bool{family.IPv6Unicast: true},
		ExtendedMessage: false,
	})

	r := &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}
	return &reactorAPIAdapter{r: r}
}

// manyIPv6Host returns n distinct IPv6 /128 NLRIs.
func manyIPv6Host(n int) []nlri.NLRI {
	out := make([]nlri.NLRI, 0, n)
	for i := range n {
		addr := netip.AddrFrom16([16]byte{
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, byte(i >> 8), byte(i), //nolint:gosec // G115: bounded by n
		})
		out = append(out, nlri.NewINET(family.IPv6Unicast, netip.PrefixFrom(addr, 128), 0))
	}
	return out
}
