// Tests for RFC 6793 AS4_PATH generation on the origin-as / export announce
// paths: reactor_api_batch.go (established peers) and peer_rib_routes.go (peers
// that connect after the announce, via the queue). A NEW speaker sending a
// two-octet AS_PATH that had to substitute AS_TRANS for a non-mappable four-octet
// AS MUST also send the four-octet sequence in an AS4_PATH (§4.2.2); when every
// AS is mappable, or the peer negotiated 4-octet support, it MUST NOT.

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/rib"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

const (
	// nonMappableAS has a non-zero high 16 bits, so it cannot be represented in a
	// two-octet AS_PATH and forces AS_TRANS (RFC 6793 §4.2.1). Low 16 bits are
	// 0x86A0, distinct from AS_TRANS (0x5BA0), so a truncation bug would be visible.
	nonMappableAS uint32 = 100000 // 0x000186A0
	mappableAS    uint32 = 65000  // 0x0000FDE8
	asTransWire          = 23456  // 0x5BA0
)

// findPathAttr walks the path-attribute TLVs in b and returns the flags and value
// bytes of the first attribute matching code (independent of ASN4 context, since
// it reads only the explicit length fields).
func findPathAttr(b []byte, code byte) (flags byte, value []byte, ok bool) {
	pos := 0
	for pos+2 <= len(b) {
		fl := b[pos]
		tc := b[pos+1]
		var hdr, ln int
		if fl&0x10 != 0 { // extended length
			if pos+4 > len(b) {
				break
			}
			ln = int(b[pos+2])<<8 | int(b[pos+3])
			hdr = 4
		} else {
			if pos+3 > len(b) {
				break
			}
			ln = int(b[pos+2])
			hdr = 3
		}
		if pos+hdr+ln > len(b) {
			break
		}
		if tc == code {
			return fl, b[pos+hdr : pos+hdr+ln], true
		}
		pos += hdr + ln
	}
	return 0, nil, false
}

func ipv4Batch(originAS uint32) bgptypes.NLRIBatch {
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	if err != nil {
		panic(err)
	}
	return bgptypes.NLRIBatch{
		Family:   family.IPv4Unicast,
		NLRIs:    []nlri.NLRI{wn},
		NextHop:  bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
		OriginAS: originAS,
	}
}

// buildAnnounce drives buildBatchAnnounceUpdate with localAS == mappableAS (a
// two-octet local AS), so the non-mappable AS under test is always the origin.
func buildAnnounce(t *testing.T, batch bgptypes.NLRIBatch, isIBGP, asn4 bool) *message.Update {
	t.Helper()
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: mappableAS}}}
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("10.0.0.1"), isIBGP, false, asn4, false, mappableAS)
	require.NotNil(t, update)
	return update
}

// --- writeASPath: the AS_TRANS truncation fix -------------------------------

// TestWriteASPath_NonMappableLocalAS_MapsToASTrans guards the fix that a
// four-octet local AS toward an OLD peer is encoded as AS_TRANS, not truncated to
// its low 16 bits. RFC 6793 §4.2.1.
func TestWriteASPath_NonMappableLocalAS_MapsToASTrans(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: nonMappableAS}}}
	buf := make([]byte, 64)
	n := adapter.writeASPath(buf, false /*eBGP*/, false /*asn4*/, nonMappableAS, 0)

	// [0x40, AS_PATH, len=4, ASSequence, count=1, AS_TRANS(2 bytes)]
	require.Equal(t, 7, n)
	assert.Equal(t, byte(attribute.AttrASPath), buf[1])
	as := int(buf[5])<<8 | int(buf[6])
	assert.Equal(t, asTransWire, as, "non-mappable localAS must map to AS_TRANS, not truncate to 0x86A0")
	assert.NotEqual(t, 0x86A0, as)
}

// TestWriteASPath_MappableCases_ByteCompat verifies the refactor left the common
// (mappable) encodings byte-identical.
func TestWriteASPath_MappableCases_ByteCompat(t *testing.T) {
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: mappableAS}}}
	buf := make([]byte, 64)

	// iBGP, no origin-as -> empty AS_PATH
	assert.Equal(t, []byte{0x40, 0x02, 0x00}, buf[:adapter.writeASPath(buf, true, false, mappableAS, 0)])

	// eBGP, asn4 -> 4-octet [localAS]
	assert.Equal(t, []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE8},
		buf[:adapter.writeASPath(buf, false, true, mappableAS, 0)])

	// eBGP, 2-octet -> [localAS]
	assert.Equal(t, []byte{0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE8},
		buf[:adapter.writeASPath(buf, false, false, mappableAS, 0)])
}

// --- writeAnnounceAS4Path: the emit predicate -------------------------------

func TestWriteAnnounceAS4Path_Cases(t *testing.T) {
	buf := make([]byte, 64)

	// eBGP origin-as, OLD peer, non-mappable origin -> AS4_PATH [localAS, originAS]
	n := writeAnnounceAS4Path(buf, 0, false, false, mappableAS, nonMappableAS)
	require.Positive(t, n)
	flags, value, ok := findPathAttr(buf[:n], byte(attribute.AttrAS4Path))
	require.True(t, ok, "AS4_PATH must be emitted")
	assert.Equal(t, byte(attribute.FlagOptional|attribute.FlagTransitive), flags, "AS4_PATH is optional transitive (0xC0)")
	as4, err := attribute.ParseAS4Path(value)
	require.NoError(t, err)
	require.Len(t, as4.Segments, 1)
	assert.Equal(t, []uint32{mappableAS, nonMappableAS}, as4.Segments[0].ASNs)

	// iBGP origin-as, OLD peer -> AS4_PATH [originAS]
	n = writeAnnounceAS4Path(buf, 0, true, false, mappableAS, nonMappableAS)
	_, value, ok = findPathAttr(buf[:n], byte(attribute.AttrAS4Path))
	require.True(t, ok)
	as4, err = attribute.ParseAS4Path(value)
	require.NoError(t, err)
	assert.Equal(t, []uint32{nonMappableAS}, as4.Segments[0].ASNs)

	// NEW peer (asn4) -> never emit
	assert.Zero(t, writeAnnounceAS4Path(buf, 0, false, true, mappableAS, nonMappableAS))

	// all-mappable origin -> MUST NOT emit
	assert.Zero(t, writeAnnounceAS4Path(buf, 0, false, false, mappableAS, 112))

	// plain export, mappable localAS -> no emit
	assert.Zero(t, writeAnnounceAS4Path(buf, 0, false, false, mappableAS, 0))

	// plain export, non-mappable localAS -> AS4_PATH [localAS]
	n = writeAnnounceAS4Path(buf, 0, false, false, nonMappableAS, 0)
	_, value, ok = findPathAttr(buf[:n], byte(attribute.AttrAS4Path))
	require.True(t, ok)
	as4, err = attribute.ParseAS4Path(value)
	require.NoError(t, err)
	assert.Equal(t, []uint32{nonMappableAS}, as4.Segments[0].ASNs)
}

// --- buildBatchAnnounceUpdate integration -----------------------------------

// TestAnnounceAS4Path_OriginAS_OldPeer_eBGP is the core case: a four-octet
// origin-as toward an OLD eBGP peer emits AS_PATH with AS_TRANS AND a matching
// AS4_PATH carrying the real four-octet sequence.
func TestAnnounceAS4Path_OriginAS_OldPeer_eBGP(t *testing.T) {
	update := buildAnnounce(t, ipv4Batch(nonMappableAS), false /*eBGP*/, false /*OLD*/)

	// AS_PATH (two-octet) ends with AS_TRANS for the non-mappable origin.
	_, asPath, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrASPath))
	require.True(t, ok, "AS_PATH present")
	require.Len(t, asPath, 6) // type, count, localAS(2), AS_TRANS(2)
	assert.Equal(t, asTransWire, int(asPath[4])<<8|int(asPath[5]), "origin encoded as AS_TRANS")

	// AS4_PATH carries [localAS, originAS] in four octets.
	_, as4v, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	require.True(t, ok, "AS4_PATH must accompany the AS_TRANS AS_PATH")
	as4, err := attribute.ParseAS4Path(as4v)
	require.NoError(t, err)
	assert.Equal(t, []uint32{mappableAS, nonMappableAS}, as4.Segments[0].ASNs)
}

// TestAnnounceAS4Path_OriginAS_NewPeer_NoAS4Path: to a 4-octet-capable peer the
// real ASNs ride in AS_PATH and AS4_PATH MUST NOT be sent.
func TestAnnounceAS4Path_OriginAS_NewPeer_NoAS4Path(t *testing.T) {
	update := buildAnnounce(t, ipv4Batch(nonMappableAS), false, true /*NEW*/)

	_, _, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	assert.False(t, ok, "no AS4_PATH toward a NEW (4-octet) peer")

	// AS_PATH (four-octet) carries the real origin AS in its last 4 bytes.
	_, asPath, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrASPath))
	require.True(t, ok)
	require.Len(t, asPath, 10) // type, count, localAS(4), originAS(4)
	got := uint32(asPath[6])<<24 | uint32(asPath[7])<<16 | uint32(asPath[8])<<8 | uint32(asPath[9])
	assert.Equal(t, nonMappableAS, got)
}

// TestAnnounceAS4Path_AllMappable_NoAS4Path: a two-octet origin (e.g. the AS112
// default 112) needs no AS4_PATH even toward an OLD peer.
func TestAnnounceAS4Path_AllMappable_NoAS4Path(t *testing.T) {
	update := buildAnnounce(t, ipv4Batch(112), false, false)
	_, _, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	assert.False(t, ok, "all-mappable path MUST NOT carry AS4_PATH")
}

// TestAnnounceAS4Path_iBGP_OldPeer: iBGP origin-as toward an OLD peer emits
// AS4_PATH [originAS] (no local-AS prepend on iBGP).
func TestAnnounceAS4Path_iBGP_OldPeer(t *testing.T) {
	update := buildAnnounce(t, ipv4Batch(nonMappableAS), true /*iBGP*/, false)
	_, as4v, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	require.True(t, ok)
	as4, err := attribute.ParseAS4Path(as4v)
	require.NoError(t, err)
	assert.Equal(t, []uint32{nonMappableAS}, as4.Segments[0].ASNs)
}

// TestAnnounceAS4Path_VerbatimWire_NoSynthAS4Path: when the batch supplies its own
// AS_PATH (verbatim wire), this builder copies it and does not synthesize an
// AS4_PATH -- that path owns its own encoding.
func TestAnnounceAS4Path_VerbatimWire_NoSynthAS4Path(t *testing.T) {
	// ORIGIN IGP + a verbatim two-octet AS_PATH [65001].
	wireAttrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x04, 0x02, 0x01, 0xFD, 0xE9}
	batch := ipv4Batch(nonMappableAS) // OriginAS set but must be ignored: AS_PATH present
	batch.Wire = attribute.NewAttributesWire(wireAttrs, 0)

	update := buildAnnounce(t, batch, false, false)
	_, _, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	assert.False(t, ok, "verbatim AS_PATH must not trigger a synthesized AS4_PATH")
}

// TestAnnounceAS4Path_IPv6_OldPeer: AS4_PATH is appended after MP_REACH_NLRI for a
// non-IPv4 family and still parses.
func TestAnnounceAS4Path_IPv6_OldPeer(t *testing.T) {
	wn, err := nlri.NewWireNLRI(family.IPv6Unicast, []byte{0x20, 0x20, 0x01, 0x0d, 0xb8}, false)
	require.NoError(t, err)
	batch := bgptypes.NLRIBatch{
		Family:   family.IPv6Unicast,
		NLRIs:    []nlri.NLRI{wn},
		NextHop:  bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
		OriginAS: nonMappableAS,
	}
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: mappableAS}}}
	attrBuf := make([]byte, message.MaxMsgLen)
	nlriBuf := make([]byte, message.MaxMsgLen)
	update := adapter.buildBatchAnnounceUpdate(attrBuf, nlriBuf, batch, netip.MustParseAddr("2001:db8::1"), false, false, false, false, mappableAS)

	_, as4v, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	require.True(t, ok, "AS4_PATH present for IPv6 too")
	as4, err := attribute.ParseAS4Path(as4v)
	require.NoError(t, err)
	assert.Equal(t, []uint32{mappableAS, nonMappableAS}, as4.Segments[0].ASNs)
}

// --- buildRIBRouteUpdate (queued / re-announce) integration -----------------

func ribRouteWithASPath(t *testing.T, asns []uint32) *rib.Route {
	t.Helper()
	wn, err := nlri.NewWireNLRI(family.IPv4Unicast, []byte{0x18, 0x0a, 0x00, 0x00}, false)
	require.NoError(t, err)
	asPath := &attribute.ASPath{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}}}
	return rib.NewRouteWithASPath(wn, netip.MustParseAddr("10.0.0.1"), []attribute.Attribute{attribute.OriginIGP}, asPath)
}

// TestRIBRouteUpdate_NonMappableASPath_OldPeer: a queued route whose stored
// AS_PATH holds a four-octet AS emits AS4_PATH when re-announced to an OLD peer.
func TestRIBRouteUpdate_NonMappableASPath_OldPeer(t *testing.T) {
	attrBuf := make([]byte, message.MaxMsgLen)
	update := buildRIBRouteUpdate(attrBuf, ribRouteWithASPath(t, []uint32{mappableAS, nonMappableAS}), mappableAS, false, false /*OLD*/, false)

	_, as4v, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	require.True(t, ok, "queued re-announce to OLD peer must carry AS4_PATH")
	as4, err := attribute.ParseAS4Path(as4v)
	require.NoError(t, err)
	assert.Equal(t, []uint32{mappableAS, nonMappableAS}, as4.Segments[0].ASNs)
}

// TestRIBRouteUpdate_NonMappableASPath_NewPeer: no AS4_PATH toward a 4-octet peer.
func TestRIBRouteUpdate_NonMappableASPath_NewPeer(t *testing.T) {
	attrBuf := make([]byte, message.MaxMsgLen)
	update := buildRIBRouteUpdate(attrBuf, ribRouteWithASPath(t, []uint32{mappableAS, nonMappableAS}), mappableAS, false, true /*NEW*/, false)
	_, _, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	assert.False(t, ok)
}

// TestRIBRouteUpdate_AllMappable_NoAS4Path: an all-mappable stored path needs no
// AS4_PATH.
func TestRIBRouteUpdate_AllMappable_NoAS4Path(t *testing.T) {
	attrBuf := make([]byte, message.MaxMsgLen)
	update := buildRIBRouteUpdate(attrBuf, ribRouteWithASPath(t, []uint32{mappableAS, 112}), mappableAS, false, false, false)
	_, _, ok := findPathAttr(update.PathAttributes, byte(attribute.AttrAS4Path))
	assert.False(t, ok)
}

// --- WriteAnnounceUpdate (single-route session send) integration ------------

// announceMsgPathAttrs extracts the path-attributes section of a full UPDATE
// message written by WriteAnnounceUpdate.
func announceMsgPathAttrs(t *testing.T, buf []byte, n int) []byte {
	t.Helper()
	const attrLenPos = message.MarkerLen + 2 /*len*/ + 1 /*type*/ + 2 /*withdrawn len*/
	const attrStart = attrLenPos + 2
	attrLen := int(buf[attrLenPos])<<8 | int(buf[attrLenPos+1])
	require.LessOrEqual(t, attrStart+attrLen, n)
	return buf[attrStart : attrStart+attrLen]
}

// TestWriteAnnounceUpdate_NonMappableLocalAS_OldPeer_IPv4: a four-octet local AS
// exported to an OLD eBGP peer emits AS_TRANS in AS_PATH plus AS4_PATH [localAS].
func TestWriteAnnounceUpdate_NonMappableLocalAS_OldPeer_IPv4(t *testing.T) {
	buf := make([]byte, 4096)
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}
	n := WriteAnnounceUpdate(buf, 0, route, nonMappableAS, false /*eBGP*/, false /*OLD*/, false)
	attrs := announceMsgPathAttrs(t, buf, n)

	_, asPath, ok := findPathAttr(attrs, byte(attribute.AttrASPath))
	require.True(t, ok)
	assert.Equal(t, asTransWire, int(asPath[len(asPath)-2])<<8|int(asPath[len(asPath)-1]), "localAS encoded as AS_TRANS")

	_, as4v, ok := findPathAttr(attrs, byte(attribute.AttrAS4Path))
	require.True(t, ok, "AS4_PATH must accompany the AS_TRANS AS_PATH")
	as4, err := attribute.ParseAS4Path(as4v)
	require.NoError(t, err)
	assert.Equal(t, []uint32{nonMappableAS}, as4.Segments[0].ASNs)
}

// TestWriteAnnounceUpdate_NonMappableLocalAS_OldPeer_IPv6: same, IPv6 (AS4_PATH
// after MP_REACH_NLRI).
func TestWriteAnnounceUpdate_NonMappableLocalAS_OldPeer_IPv6(t *testing.T) {
	buf := make([]byte, 4096)
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}
	n := WriteAnnounceUpdate(buf, 0, route, nonMappableAS, false, false, false)
	attrs := announceMsgPathAttrs(t, buf, n)

	_, as4v, ok := findPathAttr(attrs, byte(attribute.AttrAS4Path))
	require.True(t, ok, "AS4_PATH present for IPv6 too")
	as4, err := attribute.ParseAS4Path(as4v)
	require.NoError(t, err)
	assert.Equal(t, []uint32{nonMappableAS}, as4.Segments[0].ASNs)
}

// TestWriteAnnounceUpdate_NewPeer_NoAS4Path: to a 4-octet peer no AS4_PATH.
func TestWriteAnnounceUpdate_NewPeer_NoAS4Path(t *testing.T) {
	buf := make([]byte, 4096)
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.168.1.1")),
	}
	n := WriteAnnounceUpdate(buf, 0, route, nonMappableAS, false, true /*NEW*/, false)
	_, _, ok := findPathAttr(announceMsgPathAttrs(t, buf, n), byte(attribute.AttrAS4Path))
	assert.False(t, ok)
}

// --- predicates & shape ------------------------------------------------------

func TestAnyNonMappableAS(t *testing.T) {
	assert.False(t, anyNonMappableAS(nil))
	assert.False(t, anyNonMappableAS([]uint32{1, 65535, 112}))
	assert.True(t, anyNonMappableAS([]uint32{65535, 65536}))
}

// TestAsPathHasNonMappableAS_SkipsConfed: a non-mappable AS confined to a confed
// segment must not trigger AS4_PATH (RFC 6793 §3 excludes confed from AS4_PATH).
func TestAsPathHasNonMappableAS_SkipsConfed(t *testing.T) {
	assert.False(t, asPathHasNonMappableAS(nil))

	confedOnly := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASConfedSequence, ASNs: []uint32{nonMappableAS}},
	}}
	assert.False(t, asPathHasNonMappableAS(confedOnly), "non-mappable AS in confed segment is excluded")

	mixed := &attribute.ASPath{Segments: []attribute.ASPathSegment{
		{Type: attribute.ASConfedSequence, ASNs: []uint32{nonMappableAS}},
		{Type: attribute.ASSequence, ASNs: []uint32{mappableAS, nonMappableAS}},
	}}
	assert.True(t, asPathHasNonMappableAS(mixed), "non-mappable AS in a real segment triggers AS4_PATH")
}

// TestAnnounceASPathASNs_Shape pins the synthesized AS_PATH shape shared by
// writeASPath and writeAnnounceAS4Path so they can never disagree.
func TestAnnounceASPathASNs_Shape(t *testing.T) {
	shape := func(isIBGP bool, localAS, originAS uint32) []uint32 {
		var s [2]uint32
		return announceASPathASNs(s[:0], isIBGP, localAS, originAS)
	}
	assert.Equal(t, []uint32{112}, shape(true, 65000, 112))         // iBGP origin-as
	assert.Equal(t, []uint32{65000, 112}, shape(false, 65000, 112)) // eBGP origin-as
	assert.Empty(t, shape(true, 65000, 0))                          // iBGP plain export
	assert.Equal(t, []uint32{65000}, shape(false, 65000, 0))        // eBGP plain export
}
