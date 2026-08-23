package adj_rib_in

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgp "github.com/ze-software/ze/internal/component/bgp"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSplitRawNLRIHexSkipsPathIdentifiers verifies the raw-section split reads
// the prefix length from the right octet under an ADD-PATH source.
//
// VALIDATES: RFC 7911 Section 3 -- the NLRI encoding is extended by prepending a
// four-octet Path Identifier, so the length byte moves by four. The split
// returns the bare RFC 4271 prefix either way, which is what the structured
// ingest path stores too.
// PREVENTS: the defect this replaces. The split took no ADD-PATH argument, so it
// read the first octet of a Path Identifier as a prefix length; the route was
// keyed correctly from the parsed NLRI list and stored under the bytes of a
// different prefix. Those bytes reach an operator as `nlri-hex` under
// `show bgp adj-rib-in`, and reach a peer through a replay.
func TestSplitRawNLRIHexSkipsPathIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rawHex  string
		addPath bool
		want    []string
	}{
		{"no add-path, one prefix", "180a0000", false, []string{"180a0000"}},
		{"no add-path, two prefixes", "180a0000180b0000", false, []string{"180a0000", "180b0000"}},
		{"add-path identifier 1", "00000001180a0000", true, []string{"180a0000"}},
		// Identifier 0 is legal (RFC 7911 Section 3 reserves no value) and its
		// four octets are present on the wire like any other.
		{"add-path identifier 0", "00000000180a0000", true, []string{"180a0000"}},
		{"add-path, two paths for one prefix", "00000001180a000000000002180a0000", true, []string{"180a0000", "180a0000"}},
		{"add-path, truncated identifier", "000001", true, nil},
		{"add-path, identifier with no prefix", "00000001", true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, splitRawNLRIHex(tc.rawHex, family.IPv4Unicast, tc.addPath))
		})
	}

	assert.Nil(t, splitRawNLRIHex("00000001180a0000", ipv4VPN, true),
		"a complex family does not split into per-route prefixes")
}

// TestPrefixToWireHexWritesBarePrefix verifies the text-prefix fallback writes
// RFC 4271 NLRI and nothing else.
//
// VALIDATES: one storage framing. The Path Identifier travels on
// RawRoute.PathID, so the bytes carry it never.
// PREVENTS: the asymmetry this replaces -- four octets written only when the
// identifier was non-zero, which stored a legal identifier of 0 as a bare prefix
// and made the two indistinguishable.
func TestPrefixToWireHexWritesBarePrefix(t *testing.T) {
	assert.Equal(t, "180a0000", prefixToWireHex(family.IPv4Unicast, "10.0.0.0/24"))
	assert.Equal(t, "20c1000201", prefixToWireHex(family.IPv4Unicast, "193.0.2.1/32"))
	assert.Equal(t, "", prefixToWireHex(family.IPv4Unicast, "not-a-prefix"))
}

// TestHandleReceivedAddPathStoresIdentifier verifies the legacy JSON ingest path
// records the Path Identifier and the framing beside the bare prefix.
//
// VALIDATES: AC-1 storage half. A forked bgp-adj-rib-in gets its events as JSON
// (it has no bridge, so onMessageReceived sends a formatted payload), and this is
// the handler they land on.
// PREVENTS: a replay from that plugin announcing the wrong prefix, and a relay
// that cannot tell a stored identifier of 0 from a producer that recorded none.
func TestHandleReceivedAddPathStoresIdentifier(t *testing.T) {
	r := newTestManager(t)

	r.handleReceived(&bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 100},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "00000007180a0000"},
		AddPath:       map[family.Family]bool{family.IPv4Unicast: true},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {{
				NextHop: "10.0.0.1",
				Action:  routeaction.Add,
				NLRIs:   []any{map[string]any{"prefix": "10.0.0.0/24", "path-id": float64(7)}},
			}},
		},
	})

	route := onlyStoredRoute(t, r)
	assert.Equal(t, "180a0000", route.NLRIHex, "the four Path Identifier octets are not stored in the bytes")
	assert.Equal(t, uint32(7), route.PathID)
	assert.Equal(t, rpc.NLRIFramingPrefixOnly, route.NLRIFraming)
}

// TestHandleReceivedStructuredAddPathStoresIdentifier verifies the in-process
// ingest path records the same pair.
//
// VALIDATES: AC-1 storage half on the bridge path, which is the one an internal
// bgp-adj-rib-in takes.
// PREVENTS: the identifier surviving only in compactRouteKey, which is where it
// stopped: two paths for one prefix were two stored entries whose payloads were
// identical, so the relay could not tell them apart.
func TestHandleReceivedStructuredAddPathStoresIdentifier(t *testing.T) {
	r := newTestManager(t)

	// ORIGIN igp, AS_PATH [65001], NEXT_HOP 10.0.0.1, then two paths for
	// 10.0.0.0/24 under identifiers 1 and 0.
	body, err := hex.DecodeString("00000014" +
		"40010100" + "40020602010000fde9" + "400304" + "0a000001" +
		"00000001180a0000" + "00000000180a0000")
	require.NoError(t, err)

	ctxID, regErr := bgpctx.Registry.Register(
		bgpctx.EncodingContextWithAddPath(true, map[family.Family]bool{family.IPv4Unicast: true}))
	require.NoError(t, regErr)
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)

	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: "10.0.0.1",
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})

	routes, ok := r.ribIn[netip.MustParseAddr("10.0.0.1")]
	require.True(t, ok, "should have peer entry")
	require.Equal(t, 2, routes.Len(), "two Path Identifiers for one prefix are two paths")

	byPathID := map[uint32]*RawRoute{}
	routes.Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		byPathID[rt.PathID] = rt
		return true
	})
	require.Len(t, byPathID, 2, "the two stored paths carry distinct identifiers")
	for pathID, rt := range byPathID {
		assert.Equal(t, "180a0000", rt.NLRIHex, "path %d stores the bare prefix", pathID)
		assert.Equal(t, rpc.NLRIFramingPrefixOnly, rt.NLRIFraming)
	}
	assert.Contains(t, byPathID, uint32(0), "identifier 0 is a path, not an absence")
	assert.Contains(t, byPathID, uint32(1))
}

// TestBuildReplayRoutesCarriesFraming verifies the two fields reach the engine.
//
// VALIDATES: AC-1/AC-2 across the RPC. The identifier reached storage before this
// change and stopped at rpc.StoredRoute, which is why an ADD-PATH source could
// not be replayed at all.
// PREVENTS: the relay having to infer the framing from the bytes, where a bare
// prefix and an identifier-plus-prefix are both well formed.
func TestBuildReplayRoutesCarriesFraming(t *testing.T) {
	r := newTestManager(t)
	r.ribIn[netip.MustParseAddr("10.0.0.1")] = newSeqMap()
	r.ribIn[netip.MustParseAddr("10.0.0.1")].Put(
		routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/24", 0), 1, &RawRoute{
			Family:      family.IPv4Unicast,
			AttrHex:     "40010100",
			NHopHex:     "0a000001",
			NLRIHex:     "180a0000",
			PathID:      0,
			NLRIFraming: rpc.NLRIFramingPrefixOnly,
		})

	routes, _ := r.buildReplayRoutes(netip.MustParseAddr("10.0.0.99"), 0, unboundedReplay())
	require.Len(t, routes, 1)
	assert.Equal(t, rpc.StoredRoute{
		SourcePeer:  "10.0.0.1",
		Family:      "ipv4/unicast",
		AttrHex:     "40010100",
		NextHopHex:  "0a000001",
		NLRIHex:     "180a0000",
		PathID:      0,
		NLRIFraming: rpc.NLRIFramingPrefixOnly,
	}, routes[0])
}

// onlyStoredRoute returns the single route stored for a peer, failing when the
// count is not one.
func onlyStoredRoute(t *testing.T, r *AdjRIBInManager) *RawRoute {
	t.Helper()
	// Every caller drives the one source peer testPeerJSON names, so the address
	// is a property of the fixture rather than an argument.
	const peer = "10.0.0.1"
	routes, ok := r.ribIn[netip.MustParseAddr(peer)]
	require.True(t, ok, "no entry for peer %s", peer)
	require.Equal(t, 1, routes.Len(), "expected exactly one stored route")
	var out *RawRoute
	routes.Range(func(_ compactRouteKey, _ uint64, rt *RawRoute) bool {
		out = rt
		return true
	})
	require.NotNil(t, out)
	return out
}

// rfc2545Attrs builds an attribute block whose MP_REACH_NLRI carries an IPv6
// next hop in RFC 2545 Section 3 form 2. The global address is 2001:db8::1 and
// the link-local address fe80::1 follows it, under a Length octet of 32. The
// NLRI is 2001:db8:1::/48.
func rfc2545Attrs() (attrHex, nextHopHex, nlriHex string) {
	nextHopHex = "20010db8000000000000000000000001" + "fe800000000000000000000000000001"
	nlriHex = "3020010db80001"
	// AFI 2, SAFI 1, Next Hop Length 32, the pair, Reserved, the NLRI.
	mpValue := "0002" + "01" + "20" + nextHopHex + "00" + nlriHex
	// 4 + 32 + 1 + 7 = 44 octets of value, so the header is 80 0e 2c.
	mpAttr := "800e2c" + mpValue
	attrHex = "40010100" + "40020602010000fde9" + mpAttr
	return attrHex, nextHopHex, nlriHex
}

// TestHandleReceivedStructuredStoresWholeRFC2545NextHop pins the structured
// ingest path against RFC 2545 Section 3.
//
// VALIDATES: the stored next hop is the WHOLE Network Address of Next Hop field,
// so a 32-octet global plus link-local pair survives into RawRoute.NHopHex.
// PREVENTS: the truncation this replaces. MPReachWire.NextHop returns one
// netip.Addr and keeps the first 16 octets, so the store dropped the link-local
// half. RFC 2545's own pitfall list names the consequence: a speaker holding
// only the global address cannot rebuild form 2. The relay then emitted 16
// octets where the live forward rail relays the source's 32, and an on-link peer
// lost the next hop it resolves on its own link (RFC2545-3-1, RFC2545-3-2).
func TestHandleReceivedStructuredStoresWholeRFC2545NextHop(t *testing.T) {
	r := newTestManager(t)

	attrHex, nextHopHex, nlriHex := rfc2545Attrs()
	attrBytes, err := hex.DecodeString(attrHex)
	require.NoError(t, err)
	// Withdrawn Routes Length 0, then the attribute section, then no body NLRI:
	// an IPv6 route travels inside MP_REACH.
	body := append([]byte{0x00, 0x00, byte(len(attrBytes) >> 8), byte(len(attrBytes))}, attrBytes...)

	ctxID, regErr := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, regErr)
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)

	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: "10.0.0.1",
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})

	route := onlyStoredRoute(t, r)
	assert.Equal(t, nextHopHex, route.NHopHex,
		"both halves of the RFC 2545 pair must reach storage")
	assert.Len(t, route.NHopHex, 64, "32 octets, not 16")
	assert.Equal(t, nlriHex, route.NLRIHex)
}

// TestHandleReceivedStoresWholeRFC2545NextHop pins the SIBLING ingest path, the
// one a forked bgp-adj-rib-in takes.
//
// VALIDATES: the legacy JSON path reads the next hop out of the raw attribute
// block, which holds the field verbatim.
// PREVENTS: fixing only the in-process path. The event's own next-hop is an
// address STRING, so it can never express form 2, and a forked plugin would have
// gone on storing 16 octets while the in-process one stored 32
// (ai/rules/completion.md: the sibling path is in scope).
func TestHandleReceivedStoresWholeRFC2545NextHop(t *testing.T) {
	r := newTestManager(t)

	attrHex, nextHopHex, nlriHex := rfc2545Attrs()

	r.handleReceived(&bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 101},
		Peer:          testPeerJSON(t),
		RawAttributes: attrHex,
		RawNLRI:       map[family.Family]string{family.IPv6Unicast: nlriHex},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv6Unicast: {{
				// The string the event carries names the global address alone.
				// It is the value this path used to store.
				NextHop: "2001:db8::1",
				Action:  routeaction.Add,
				NLRIs:   []any{map[string]any{"prefix": "2001:db8:1::/48"}},
			}},
		},
	})

	route := onlyStoredRoute(t, r)
	assert.Equal(t, nextHopHex, route.NHopHex,
		"the raw MP_REACH field wins over the event's address string")
	assert.Len(t, route.NHopHex, 64, "32 octets, not 16")
}

// TestHandleReceivedKeepsEventNextHopWithoutMPReach bounds the change above.
//
// VALIDATES: a legacy IPv4 unicast announcement, which carries its next hop in
// the well-known NEXT_HOP attribute and no MP_REACH at all, still stores the
// event's address.
// PREVENTS: the MP_REACH read becoming the only source. One UPDATE can carry
// MP_REACH for one family and body NLRI for IPv4 unicast. So the field is used
// only for the family that attribute names.
func TestHandleReceivedKeepsEventNextHopWithoutMPReach(t *testing.T) {
	r := newTestManager(t)

	r.handleReceived(&bgp.Event{
		Message:       &bgp.MessageInfo{Type: rpc.EventKindUpdate, ID: 102},
		Peer:          testPeerJSON(t),
		RawAttributes: "40010100" + "40020602010000fde9" + "4003040a000001",
		RawNLRI:       map[family.Family]string{family.IPv4Unicast: "180a0000"},
		FamilyOps: map[family.Family][]bgp.FamilyOperation{
			family.IPv4Unicast: {{
				NextHop: "10.0.0.1",
				Action:  routeaction.Add,
				NLRIs:   []any{map[string]any{"prefix": "10.0.0.0/24"}},
			}},
		},
	})

	route := onlyStoredRoute(t, r)
	assert.Equal(t, "0a000001", route.NHopHex)
}

// TestMPReachNextHopHexRefusesWhatItCannotRead covers the derivation's own edges.
//
// VALIDATES: mpReachNextHopHex reports nothing rather than something wrong when
// the block is absent, unparseable, or carries no MP_REACH.
// PREVENTS: a guard whose zero value looks like an answer. "" is what routes the
// caller back to the event's address string, so each of these must produce it
// (ai/rules/evidence.md).
func TestMPReachNextHopHexRefusesWhatItCannotRead(t *testing.T) {
	attrHex, nextHopHex, _ := rfc2545Attrs()

	got, fam := mpReachNextHopHex(attrHex)
	assert.Equal(t, nextHopHex, got)
	assert.Equal(t, family.IPv6Unicast, fam)

	for _, tc := range []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"not-hex", "zz"},
		{"odd-length", "400101"},
		{"no-mp-reach", "40010100" + "4003040a000001"},
		{"mp-reach-declares-more-than-it-holds", "800e050002012000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hexOut, famOut := mpReachNextHopHex(tc.in)
			assert.Empty(t, hexOut)
			assert.Equal(t, family.Family{}, famOut)
		})
	}
}
