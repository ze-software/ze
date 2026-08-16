// rfc-test-change-approved: 2026-08-08 Thomas approved the buildBatchAnnounceUpdate
// signature change that carries the true cause to the caller (an (*message.Update,
// error) pair in place of a bare *message.Update, so a refused build reports WHY
// instead of a silent nil). Every hunk in this RFC4271-5-7 file is that caller
// adaptation, `update :=` becoming `update, _ :=`. No assertion, fixture, or
// expected value changed.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/rib"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// Attribute type-code ordering on the announce rails.
//
// An announce reaches the wire through one of two builders, and which one runs is
// decided by Peer.ShouldQueue() (reactor_api_batch.go): a route injected while the
// destination peer is still draining its initial sync is queued and later drained
// through buildRIBRouteUpdate (peer_rib_routes.go), while the same route injected
// after establishment is built by buildBatchAnnounceUpdate. Nothing in the route
// selects the builder -- scheduling does.
//
// The batch builder used to APPEND LOCAL_PREF (5), MP_REACH_NLRI (14) and AS4_PATH
// (17) to a block that already held the caller's attributes verbatim, so a route
// carrying EXTENDED_COMMUNITIES (16) or LARGE_COMMUNITIES (32) came out as
// 1,2,16,5,14 there and 1,2,5,14,16 through the queue. One route, two byte
// strings, chosen by timing: test/plugin/ddos-flowspec-announce.ci pinned the
// queued encoding and failed whenever the batch rail won the race.
//
// These tests pin BOTH halves: ascending type-code order (RFC 4271 Section 5), and
// byte-for-byte agreement between the two rails.
//
// AIGP (code 26, RFC 7311) is deliberately NOT a case in orderCases below, and
// that is not a gap: TestAnnounceRailsPreserveUnlistedAttributes in
// reactor_api_batch_attr_preserve_test.go drives AIGP through both rails and
// asserts the same two properties this table exists for -- the exact wire order
// (wantCodes [1 2 5 8 14 26 40] and [1 2 3 5 8 26 40], plus assertAscending) and
// byte-for-byte rail agreement (hex(queued) == hex(batch)). Restating it here
// would duplicate that coverage at equal strength while mixing an
// attribute-PRESERVATION concern into a table whose cases carry RFC-requirement
// tags for Section 5 ORDERING; the file split is described at the top of that
// file. Checked 2026-07-27 when the question was reopened.

// attrCodes returns the attribute type codes present in a packed attribute block,
// in wire order.
func attrCodes(t *testing.T, b []byte) []int {
	t.Helper()
	var out []int
	pos := 0
	for pos+3 <= len(b) {
		flags := b[pos]
		var hdr, ln int
		if flags&0x10 != 0 {
			require.LessOrEqual(t, pos+4, len(b), "truncated extended-length attribute header")
			hdr, ln = 4, int(b[pos+2])<<8|int(b[pos+3])
		} else {
			hdr, ln = 3, int(b[pos+2])
		}
		require.LessOrEqual(t, pos+hdr+ln, len(b), "attribute length runs past the block")
		out = append(out, int(b[pos+1]))
		pos += hdr + ln
	}
	require.Equal(t, len(b), pos, "attribute block did not walk cleanly to its end")
	return out
}

// assertAscending fails when codes are not in non-decreasing order.
func assertAscending(t *testing.T, codes []int) {
	t.Helper()
	for i := 1; i < len(codes); i++ {
		assert.LessOrEqualf(t, codes[i-1], codes[i],
			"attributes out of type-code order at index %d: %v", i, codes)
	}
}

// orderCase is one announce, described once and driven through both rails.
type orderCase struct {
	name    string
	fam     family.Family
	nlri    string // hex of the family's wire NLRI
	nextHop string
	isIBGP  bool
	asn4    bool
	localAS uint32
	build   func(*attribute.Builder)
	want    []int // expected type codes, in wire order
}

func extCommunity(t *testing.T, h string) attribute.ExtendedCommunity {
	t.Helper()
	raw, err := hex.DecodeString(h)
	require.NoError(t, err)
	require.Len(t, raw, 8)
	var ec attribute.ExtendedCommunity
	copy(ec[:], raw)
	return ec
}

func orderCases(t *testing.T) []orderCase {
	t.Helper()
	return []orderCase{
		{
			// The ddos-flowspec-announce.ci shape: a FlowSpec route whose
			// rate-limit rides in EXTENDED_COMMUNITIES (16), so MP_REACH (14)
			// and LOCAL_PREF (5) must be placed BEFORE an attribute that is
			// already in the block.
			name:    "flowspec-with-ext-community-ibgp",
			fam:     family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
			nlri:    "0B0118C00002038106058150",
			nextHop: "127.0.0.1",
			isIBGP:  true,
			asn4:    true,
			localAS: 1,
			build: func(b *attribute.Builder) {
				b.AddExtendedCommunity(extCommunity(t, "8006000046160000"))
			},
			want: []int{1, 2, 5, 14, 16},
		},
		{
			// MP_REACH (14) has to land BETWEEN two attributes the caller
			// supplied: COMMUNITIES (8) below it and LARGE_COMMUNITIES (32)
			// above it. Appending can never produce this.
			name:    "ipv6-unicast-communities-straddling-mpreach",
			fam:     family.IPv6Unicast,
			nlri:    "202001 0db8",
			nextHop: "2001:db8::1",
			isIBGP:  true,
			asn4:    true,
			localAS: 65000,
			build: func(b *attribute.Builder) {
				b.AddCommunity(65000, 100)
				b.AddLargeCommunity(65000, 1, 2)
			},
			want: []int{1, 2, 5, 8, 14, 32},
		},
		{
			// test/plugin/nexthop.ci's exact shape: ORIGIN + LOCAL_PREF + an
			// IPv6 MP_REACH, nothing above code 14. Included to SETTLE that
			// test's intermittency rather than assume it: nothing here needs
			// reordering, both rails already agreed, so nexthop's failures are
			// not this defect. If this case ever diverges, that conclusion is
			// wrong and the test says so.
			name:    "nexthop-ci-shape-no-high-coded-attrs",
			fam:     family.IPv6Unicast,
			nlri:    "802605 0000000000000000 0000000000000002",
			nextHop: "2001::1",
			isIBGP:  true,
			asn4:    true,
			localAS: 65512,
			build: func(b *attribute.Builder) {
				b.SetOrigin(0)
				b.SetLocalPref(500)
			},
			want: []int{1, 2, 5, 14},
		},
		{
			// AS4_PATH (17) against LARGE_COMMUNITIES (32), through BOTH rails.
			//
			// Added after a reviewer's probe caught this: the first version of
			// this fix ordered AS4_PATH on the batch rail but left
			// buildRIBRouteUpdate appending it after every optional attribute,
			// so the batch rail emitted [1 2 3 17 32] and the queued rail
			// [1 2 3 32 17] -- a NEW two-rail divergence introduced by the fix
			// itself. The standalone
			// TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity drives
			// the batch rail alone and could not see it.
			name:    "as4path-against-large-community-ebgp-old-peer",
			fam:     family.IPv4Unicast,
			nlri:    "180a0000",
			nextHop: "10.0.0.1",
			isIBGP:  false,      // eBGP: the builder synthesizes the AS_PATH
			asn4:    false,      // OLD peer: RFC 6793 §4.2.2 owes an AS4_PATH
			localAS: 4200000000, // non-mappable, so AS_TRANS + AS4_PATH
			build: func(b *attribute.Builder) {
				b.AddLargeCommunity(65000, 1, 2)
			},
			want: []int{1, 2, 3, 17, 32},
		},
		{
			// IPv4 unicast has no MP_REACH, but LOCAL_PREF (5) still has to
			// precede an EXTENDED_COMMUNITIES (16) the caller supplied.
			name:    "ipv4-unicast-ext-community-localpref-before-16",
			fam:     family.IPv4Unicast,
			nlri:    "180a0000",
			nextHop: "10.0.0.1",
			isIBGP:  true,
			asn4:    true,
			localAS: 65000,
			build: func(b *attribute.Builder) {
				b.AddExtendedCommunity(extCommunity(t, "8006000046160000"))
			},
			want: []int{1, 2, 3, 5, 16},
		},
	}
}

// buildBatchRail encodes one orderCase through buildBatchAnnounceUpdate, the
// builder used when the destination peer is established.
func buildBatchRail(t *testing.T, c orderCase) []byte {
	t.Helper()
	wireNLRI, err := hex.DecodeString(stripSpaces(c.nlri))
	require.NoError(t, err)
	wn, err := nlri.NewWireNLRI(c.fam, wireNLRI, false)
	require.NoError(t, err)

	b := attribute.NewBuilder()
	c.build(b)
	batch := bgptypes.NLRIBatch{
		Family:  c.fam,
		NLRIs:   []nlri.NLRI{wn},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(c.nextHop)),
		Wire:    attribute.NewAttributesWire(b.Build(), bgpctx.APIContextID),
	}

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: c.localAS}}}
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
		netip.MustParseAddr(c.nextHop), c.isIBGP, false /*rsClient*/, c.asn4, false /*addPath*/, c.localAS)
	require.NotNil(t, update)
	return update.PathAttributes
}

// buildQueuedRail encodes the SAME orderCase through buildRIBRouteUpdate, the
// builder used when the route was queued while the peer drained its initial sync.
func buildQueuedRail(t *testing.T, c orderCase) []byte {
	t.Helper()
	wireNLRI, err := hex.DecodeString(stripSpaces(c.nlri))
	require.NoError(t, err)
	wn, err := nlri.NewWireNLRI(c.fam, wireNLRI, false)
	require.NoError(t, err)

	b := attribute.NewBuilder()
	c.build(b)
	aw := attribute.NewAttributesWire(b.Build(), bgpctx.APIContextID)
	attrs, err := aw.All()
	require.NoError(t, err)

	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: c.localAS}}}
	asPath := adapter.buildBatchASPath(nil, 0, c.isIBGP, false /*rsClient*/, c.localAS)
	route := rib.NewRouteWithASPath(wn, netip.MustParseAddr(c.nextHop), attrs, asPath)

	attrBuf := make([]byte, message.MaxMsgLen)
	update := buildRIBRouteUpdate(attrBuf, route, c.localAS, c.isIBGP, c.asn4, false /*addPath*/)
	require.NotNil(t, update)
	return update.PathAttributes
}

func stripSpaces(s string) string {
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// TestAnnounceBatchRail_AscendingTypeCodeOrder pins the batch builder's output to
// ascending attribute type-code order.
//
// RFC requirement: RFC4271-5-7 positive -- "The sender of an UPDATE message SHOULD
// order path attributes within the UPDATE message in ascending order of attribute
// type" (RFC 4271 Section 5).
// VALIDATES: buildBatchAnnounceUpdate places LOCAL_PREF (5), NEXT_HOP (3) and
// MP_REACH_NLRI (14) at their type-code position inside a block that already holds
// caller-supplied attributes with higher codes.
// PREVENTS: regressing to appending them, which emitted 1,2,16,5,14 for a FlowSpec
// announce carrying a rate-limit extended community.
func TestAnnounceBatchRail_AscendingTypeCodeOrder(t *testing.T) {
	for _, c := range orderCases(t) {
		t.Run(c.name, func(t *testing.T) {
			codes := attrCodes(t, buildBatchRail(t, c))
			assert.Equal(t, c.want, codes, "batch rail attribute order")
			assertAscending(t, codes)
		})
	}
}

// TestAnnounceQueuedRail_AscendingTypeCodeOrder pins the same property on the
// queued builder, so neither rail can drift alone.
//
// RFC requirement: RFC4271-5-7 positive -- RFC 4271 Section 5 ascending attribute
// type order, on the initial-sync drain path.
// VALIDATES: buildRIBRouteUpdate emits ascending type codes for the same routes.
// PREVENTS: a future edit "fixing" one rail by matching the other's broken order.
func TestAnnounceQueuedRail_AscendingTypeCodeOrder(t *testing.T) {
	for _, c := range orderCases(t) {
		t.Run(c.name, func(t *testing.T) {
			codes := attrCodes(t, buildQueuedRail(t, c))
			assert.Equal(t, c.want, codes, "queued rail attribute order")
			assertAscending(t, codes)
		})
	}
}

// TestAnnounceRailsAgreeByteForByte is the invariant the intermittent failure was
// really about: the encoding of a route must not depend on which builder ran, and
// which builder runs is decided by Peer.ShouldQueue() -- by scheduling.
//
// VALIDATES: buildBatchAnnounceUpdate and buildRIBRouteUpdate produce identical
// path-attribute bytes for the same route.
// PREVENTS: test/plugin/ddos-flowspec-announce.ci (and any other hex-pinned
// announce test) passing or failing according to whether the peer had finished its
// initial sync when the plugin's command arrived.
func TestAnnounceRailsAgreeByteForByte(t *testing.T) {
	for _, c := range orderCases(t) {
		t.Run(c.name, func(t *testing.T) {
			batch := buildBatchRail(t, c)
			queued := buildQueuedRail(t, c)
			assert.Equal(t, hex.EncodeToString(queued), hex.EncodeToString(batch),
				"the two announce rails must encode the same route to the same bytes")
		})
	}
}

// TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity covers the third
// appended attribute. AS4_PATH is type 17, which the old comment called "higher
// than every attribute this builder emits" -- true of the attributes the builder
// writes itself, false of the caller's block, where LARGE_COMMUNITIES is 32.
//
// RFC requirement: RFC4271-5-7 positive -- RFC 4271 Section 5 ascending attribute
// type order, with the RFC 6793 Section 4.2.2 AS4_PATH in the block.
// VALIDATES: AS4_PATH lands before LARGE_COMMUNITIES.
// PREVENTS: emitting 1,2,3,32,17 toward an OLD peer with a four-octet local AS.
func TestAnnounceBatchRail_AS4PathOrderedAgainstLargeCommunity(t *testing.T) {
	const fourOctetAS = uint32(4200000000) // non-mappable: forces AS_TRANS + AS4_PATH
	c := orderCase{
		fam:     family.IPv4Unicast,
		nlri:    "180a0000",
		nextHop: "10.0.0.1",
		isIBGP:  false, // eBGP, so the builder synthesizes the AS_PATH
		asn4:    false, // OLD peer, so RFC 6793 owes an AS4_PATH
		localAS: fourOctetAS,
		build: func(b *attribute.Builder) {
			b.AddLargeCommunity(65000, 1, 2)
		},
	}
	codes := attrCodes(t, buildBatchRail(t, c))
	assert.Equal(t, []int{1, 2, 3, 17, 32}, codes, "AS4_PATH must precede LARGE_COMMUNITIES")
	assertAscending(t, codes)
}

// TestAnnounceMergeInsert_ExtendedLengthShift guards the one way a merge insert
// can corrupt rather than merely misorder: attribute.AttrWireLen and WriteHeaderTo
// must agree on the header size, or the size query and the write disagree about
// where the next attribute starts. A value longer than 255 octets takes the
// 4-octet extended-length header.
//
// this was TestInsertAttrOrdered_ExtendedLengthShift. insertAttrOrdered
// shifted a byte block in place; the shared writer emits into a freshly sized
// region instead, so the same property is asserted against announceAttrs.emit.
// Same fixture, same two assertions.
//
// VALIDATES: emitting a >255-octet attribute in front of an existing one keeps
// both attributes intact and walkable.
// PREVENTS: a 3-vs-4 byte header disagreement silently truncating the block.
func TestAnnounceMergeInsert_ExtendedLengthShift(t *testing.T) {
	buf := make([]byte, message.MaxMsgLen)

	// Start with LARGE_COMMUNITIES (32) already in the base block.
	large := attribute.LargeCommunities{{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}}
	baseLen := attribute.WriteAttrTo(large, buf, 0)
	base := make([]byte, baseLen)
	copy(base, buf[:baseLen])

	// Contribute a COMMUNITIES (8) attribute whose value exceeds 255 octets.
	comms := make(attribute.Communities, 100) // 400 octets
	for i := range comms {
		comms[i] = attribute.Community(uint32(i)) //nolint:gosec // G115: bounded by loop
	}
	require.Greater(t, comms.Len(), 255, "fixture must exercise the extended-length header")

	var plan announceAttrs
	plan.begin()
	defer plan.release()
	plan.add(comms, nil)
	off, ok := plan.emit(base, buf)
	require.True(t, ok, "emit must fit in a MaxMsgLen buffer")

	codes := attrCodes(t, buf[:off])
	assert.Equal(t, []int{8, 32}, codes)

	_, value, ok := findPathAttr(buf[:off], byte(attribute.AttrLargeCommunity))
	require.True(t, ok, "LARGE_COMMUNITIES must survive the merge")
	assert.Len(t, value, 12, "LARGE_COMMUNITIES value must be intact after the merge")
}

// TestBatchBuild_EmitsNoUnwrittenBufferBytes checks the consequence of the
// attrWireLen invariant end-to-end: every byte the batch builder hands back must
// be a byte it wrote, not residue from whatever the pooled buffer held before.
//
// The buffers come from getBuildBuf() and are reused across UPDATEs, so a shift
// that opens a gap it does not fill would emit the previous message's bytes as
// attribute content -- a cross-UPDATE information leak that no ordering assertion
// can see. Pre-filling with a sentinel and rebuilding the walk turns that into a
// visible failure.
//
// VALIDATES: the emitted attribute block walks cleanly and its declared lengths
// account for every byte, with the buffer pre-poisoned.
// PREVENTS: the announce writer sizing more than it writes and leaking pool memory
// onto the wire.
func TestBatchBuild_EmitsNoUnwrittenBufferBytes(t *testing.T) {
	for _, c := range orderCases(t) {
		t.Run(c.name, func(t *testing.T) {
			wireNLRI, err := hex.DecodeString(stripSpaces(c.nlri))
			require.NoError(t, err)
			wn, err := nlri.NewWireNLRI(c.fam, wireNLRI, false)
			require.NoError(t, err)

			b := attribute.NewBuilder()
			c.build(b)
			batch := bgptypes.NLRIBatch{
				Family:  c.fam,
				NLRIs:   []nlri.NLRI{wn},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(c.nextHop)),
				Wire:    attribute.NewAttributesWire(b.Build(), bgpctx.APIContextID),
			}

			// Poison both buffers: every byte the builder does not write stays 0x5A.
			attrBuf := make([]byte, message.MaxMsgLen)
			nlriBuf := make([]byte, message.MaxMsgLen)
			for i := range attrBuf {
				attrBuf[i] = 0x5A
			}
			for i := range nlriBuf {
				nlriBuf[i] = 0x5A
			}

			adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: c.localAS}}}
			update, _ := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch,
				netip.MustParseAddr(c.nextHop), c.isIBGP, false, c.asn4, false, c.localAS)
			require.NotNil(t, update)

			// attrCodes requires the block to walk exactly to its end: a gap of
			// unwritten sentinel would either desynchronise the walk or leave a
			// trailing remainder, and either fails here.
			codes := attrCodes(t, update.PathAttributes)
			assert.Equal(t, c.want, codes, "poisoned-buffer build must match the clean build")
		})
	}
}

// TestAnnounceNLRIBatch_RejectsBatchTooLargeForBuildBuffer was MOVED,
// not dropped -- it lives verbatim (with two extra boundary sub-tests) in
// reactor_api_batch_capacity_test.go. It was added here by mistake: this file's
// cases carry `RFC requirement: RFC4271-5-7` tags for attribute ordering, and a
// build-buffer capacity guard is a memory-safety concern, not an ordering one.
// Coverage is strictly greater after the move.

// TestAttrWireLen_MatchesWriteAttrTo is the invariant the announce writer's size
// query rests on, checked directly instead of only through its consequences.
//
// The size query sums attribute.AttrWireLen(attr) per contributed attribute and the
// write then emits each one with WriteAttrTo's own header rule. If the two ever
// disagree, an over-estimate leaves a gap of whatever the pooled buffer happened to
// hold -- stale bytes from a previous UPDATE emitted as attribute content -- and an
// under-estimate overwrites the attribute that follows. Neither shows up as a
// mis-ordering, so the ordering tests above cannot catch it.
//
// The attributes below are exactly the ones the batch builder contributes (NEXT_HOP,
// LOCAL_PREF, MP_REACH_NLRI, AS4_PATH), plus the two shapes most likely to break
// the identity: an AS4_PATH carrying a confederation segment, which RFC 6793
// Section 3 makes WriteTo skip, and a value over the 255-octet extended-length
// boundary.
//
// VALIDATES: attribute.AttrWireLen(attr) == WriteAttrTo(attr, buf, 0) for every
// attribute the announce rails contribute.
// PREVENTS: a pool-memory leak into the wire (or a clobbered neighbor) from a
// Len()/WriteTo() mismatch in any attribute type the announce writer sizes.
func TestAttrWireLen_MatchesWriteAttrTo(t *testing.T) {
	bigComms := make(attribute.Communities, 100) // 400 octets: extended-length header
	for i := range bigComms {
		bigComms[i] = attribute.Community(uint32(i)) //nolint:gosec // G115: bounded by loop
	}

	cases := []struct {
		name string
		attr attribute.Attribute
	}{
		{"next-hop-ipv4", &attribute.NextHop{Addr: netip.MustParseAddr("10.0.0.1")}},
		{"local-pref", attribute.LocalPref(100)},
		{"mpreach-ipv6-unicast", attribute.NewMPReachNLRI(2, 1,
			[]netip.Addr{netip.MustParseAddr("2001:db8::1")}, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8})},
		{"mpreach-ipv4-flowspec", attribute.NewMPReachNLRI(1, 133,
			[]netip.Addr{netip.MustParseAddr("127.0.0.1")}, []byte{0x0B, 0x01, 0x18, 0xC0, 0x00, 0x02, 0x03, 0x81, 0x06, 0x05, 0x81, 0x50})},
		{"as4path-plain", &attribute.AS4Path{Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{4200000000}},
		}}},
		{"as4path-with-confed-segment", &attribute.AS4Path{Segments: []attribute.ASPathSegment{
			{Type: attribute.ASConfedSequence, ASNs: []uint32{65001, 65002}},
			{Type: attribute.ASSequence, ASNs: []uint32{4200000000}},
		}}},
		{"communities-extended-length", bigComms},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := make([]byte, message.MaxMsgLen)
			wrote := attribute.WriteAttrTo(c.attr, buf, 0)
			assert.Equal(t, wrote, attribute.AttrWireLen(c.attr),
				"attrWireLen must equal what WriteAttrTo writes, or insertAttrOrdered shifts by the wrong amount")
		})
	}
}
