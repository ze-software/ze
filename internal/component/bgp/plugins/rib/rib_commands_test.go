package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// Wire attribute bytes for test data.
// These are full wire format: [flags][type][length][value].
var (
	testWireOriginIGP    = []byte{0x40, 0x01, 0x01, 0x00}                               // ORIGIN = IGP
	testWireASPath65001  = []byte{0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9} // AS_PATH = [65001]
	testWireNextHop      = []byte{0x40, 0x03, 0x04, 0x0A, 0x00, 0x00, 0x01}             // NEXT_HOP = 10.0.0.1
	testWireMED100       = []byte{0x80, 0x04, 0x04, 0x00, 0x00, 0x00, 0x64}             // MED = 100
	testWireLocalPref100 = []byte{0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64}             // LOCAL_PREF = 100
	testWireCommunity    = []byte{0xC0, 0x08, 0x04, 0xFD, 0xE8, 0x00, 0x64}             // COMMUNITIES = [65000:100]
)

// concatBytes concatenates multiple byte slices.
func concatBytes(slices ...[]byte) []byte {
	var total int
	for _, s := range slices {
		total += len(s)
	}
	result := make([]byte, 0, total)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// requirePeerRoutes unmarshals JSON and returns the route array for a peer.
func requirePeerRoutes(t *testing.T, jsonStr, topKey, peerAddr string) []any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &result))

	top, ok := result[topKey].(map[string]any)
	require.True(t, ok, "expected %s key", topKey)

	peerRoutes, ok := top[peerAddr].([]any)
	require.True(t, ok, "expected peer routes for %s", peerAddr)
	return peerRoutes
}

// anyToJSONStr converts an any value to a JSON string.
// For json.RawMessage, returns it directly. For other types, marshals to JSON.
func anyToJSONStr(t *testing.T, v any) string {
	t.Helper()
	switch d := v.(type) {
	case json.RawMessage:
		return string(d)
	case string:
		return d
	default:
		b, err := json.Marshal(d)
		require.NoError(t, err)
		return string(b)
	}
}

// requireFirstRoute unmarshals JSON and extracts the first route for a peer.
func attrVal(route map[string]any, key string) any {
	v, ok := route[key]
	if !ok {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		if val, ok := m["value"]; ok {
			return val
		}
	}
	return v
}

func attrSlice(t *testing.T, route map[string]any, key string) []any {
	t.Helper()
	v := attrVal(route, key)
	arr, ok := v.([]any)
	require.True(t, ok, "expected %s array", key)
	return arr
}

func requireFirstRoute(t *testing.T, jsonStr, topKey, peerAddr string) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &result))

	top, ok := result[topKey].(map[string]any)
	require.True(t, ok, "expected %s key", topKey)

	peerRoutes, ok := top[peerAddr].([]any)
	require.True(t, ok, "expected peer routes for %s", peerAddr)
	require.NotEmpty(t, peerRoutes)

	route, ok := peerRoutes[0].(map[string]any)
	require.True(t, ok, "expected route map")
	return route
}

// TestInboundShowWithAttributes verifies enriched show bgp rib received returns attributes.
//
// VALIDATES: AC-6 — show bgp rib received returns origin, as-path, med, local-pref, communities.
// PREVENTS: Show command returning only family/prefix/next-hop without path attributes.
func TestInboundShowWithAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	// Insert a route with full attributes into pool storage
	fam := family.IPv4Unicast
	attrBytes := concatBytes(
		testWireOriginIGP,
		testWireASPath65001,
		testWireNextHop,
		testWireMED100,
		testWireLocalPref100,
		testWireCommunity,
	)
	// NLRI: 10.0.0.0/24 = [prefix-len=24][10][0][0]
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	route := requireFirstRoute(t, anyToJSONStr(t, r.showPipeline("*", []string{"received"})), "adj-rib-in", "192.0.2.1")

	assert.Equal(t, family.IPv4Unicast.String(), route["family"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])
	assert.Equal(t, "igp", attrVal(route, "origin"))
	assert.Equal(t, float64(100), attrVal(route, "med"))
	assert.Equal(t, float64(100), attrVal(route, "local-preference"))

	// AS path comes as []any with float64 values in JSON
	asPath := attrSlice(t, route, "as-path")
	require.Len(t, asPath, 1)
	assert.Equal(t, float64(65001), asPath[0])

	// Communities
	communities := attrSlice(t, route, "community")
	require.Len(t, communities, 1)
	assert.Equal(t, "65000:100", communities[0])
}

// TestInboundShowMinimalAttributes verifies show with only mandatory attributes.
//
// VALIDATES: Missing optional attributes are omitted from output.
// PREVENTS: Null/zero values for missing MED, LOCAL_PREF, communities.
func TestInboundShowMinimalAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlriBytes, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	route := requireFirstRoute(t, anyToJSONStr(t, r.showPipeline("192.0.2.1", []string{"received"})), "adj-rib-in", "192.0.2.1")

	assert.Equal(t, "igp", attrVal(route, "origin"))
	assert.Equal(t, "10.0.0.1", route["next-hop"])

	// Optional attributes should be absent
	_, hasMED := route["med"]
	assert.False(t, hasMED, "MED should be absent when not in route")
	_, hasLP := route["local-preference"]
	assert.False(t, hasLP, "LOCAL_PREF should be absent when not in route")
	_, hasCom := route["community"]
	assert.False(t, hasCom, "communities should be absent when not in route")
}

// TestOutboundShowWithAttributes verifies enriched show bgp rib sent returns attributes.
//
// VALIDATES: AC-7 — show bgp rib sent returns origin, as-path, med, local-pref, communities.
// PREVENTS: Outbound show missing path attributes for route replay verification.
func TestOutboundShowWithAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	med := uint32(100)
	localPref := uint32(200)
	r.ribOut[netip.MustParseAddr("192.0.2.1")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {
				Family:           family.IPv4Unicast,
				Prefix:           "10.0.0.0/24",
				NextHop:          "10.0.0.1",
				Origin:           new(OriginIGP),
				ASPath:           []uint32{65001, 65002},
				MED:              &med,
				LocalPreference:  &localPref,
				Communities:      []attribute.Community{attribute.Community(65000<<16 | 100)},
				LargeCommunities: []attribute.LargeCommunity{{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}},
			},
		},
	})

	route := requireFirstRoute(t, anyToJSONStr(t, r.showPipeline("*", []string{"sent"})), "adj-rib-out", "192.0.2.1")

	assert.Equal(t, family.IPv4Unicast.String(), route["family"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])
	assert.Equal(t, "igp", attrVal(route, "origin"))
	assert.Equal(t, float64(100), attrVal(route, "med"))
	assert.Equal(t, float64(200), attrVal(route, "local-preference"))

	asPath := attrSlice(t, route, "as-path")
	require.Len(t, asPath, 2)
	assert.Equal(t, float64(65001), asPath[0])
	assert.Equal(t, float64(65002), asPath[1])

	communities := attrSlice(t, route, "community")
	require.Len(t, communities, 1)
	assert.Equal(t, "65000:100", communities[0])

	largeCommunities := attrSlice(t, route, "large-community")
	require.Len(t, largeCommunities, 1)
	assert.Equal(t, "65000:1:2", largeCommunities[0])
}

// TestRibStatusEmitsPerPeerRouteCounts verifies status() reports per-peer
// Adj-RIB-In and Adj-RIB-Out sizes, not only the global sums.
//
// VALIDATES: AC-6 — show bgp rib status carries a route-counts map keyed by peer
// address with {in, out}. summary.go merges this into the per-peer summary rows.
// PREVENTS: the birdwatcher routes_* fields staying 0 because the RIB only
// exposed global route totals.
func TestRibStatusEmitsPerPeerRouteCounts(t *testing.T) {
	r := newTestRIBManager(t)

	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	// Peer .1: two received routes (Adj-RIB-In) and three advertised (Adj-RIB-Out).
	in1 := storage.NewPeerRIB("192.0.2.1")
	in1.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 0, 0}, true)
	in1.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 0, 1}, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = in1
	r.ribOut[netip.MustParseAddr("192.0.2.1")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "10.0.0.1", Origin: new(OriginIGP)},
			"10.0.1.0/24": {Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", NextHop: "10.0.0.1", Origin: new(OriginIGP)},
			"10.0.2.0/24": {Family: family.IPv4Unicast, Prefix: "10.0.2.0/24", NextHop: "10.0.0.1", Origin: new(OriginIGP)},
		},
	})
	// Peer .2: one received route, nothing advertised.
	in2 := storage.NewPeerRIB("192.0.2.2")
	in2.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 1, 0}, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.2")] = in2

	status, ok := r.status("").(map[string]any)
	require.True(t, ok)

	counts, ok := status["route-counts"].(map[string]any)
	require.True(t, ok, "status must carry a route-counts map")

	p1, ok := counts["192.0.2.1"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, p1["in"], "peer .1 Adj-RIB-In size")
	assert.Equal(t, 3, p1["out"], "peer .1 Adj-RIB-Out size")

	p2, ok := counts["192.0.2.2"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, p2["in"], "peer .2 Adj-RIB-In size")
	assert.Equal(t, 0, p2["out"], "peer .2 advertised nothing")
}

// TestRibStatusFamilyScopedRouteCounts verifies a family argument scopes the
// per-peer route-counts to that family, while no argument reports all-family
// totals.
//
// VALIDATES: status(fam) counts only `fam`; status("") counts every family.
// PREVENTS: a family-filtered `show bgp summary <afi/safi>` reporting all-family
// route counts (the /ze-review NOTE this fixes) — the counts would include IPv6
// routes for an `ipv4/unicast` filter.
func TestRibStatusFamilyScopedRouteCounts(t *testing.T) {
	r := newTestRIBManager(t)
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)

	// Peer with two received routes in IPv4 and one in IPv6 (Adj-RIB-In).
	in := storage.NewPeerRIB("192.0.2.1")
	in.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 0, 0}, true)
	in.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 0, 1}, true)
	in.Insert(family.IPv6Unicast, attrBytes, []byte{32, 0x20, 0x01, 0x0d, 0xb8}, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = in
	// Advertised: two IPv4, one IPv6 (Adj-RIB-Out).
	r.ribOut[netip.MustParseAddr("192.0.2.1")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "10.0.0.1", Origin: new(OriginIGP)},
			"10.0.1.0/24": {Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", NextHop: "10.0.0.1", Origin: new(OriginIGP)},
		},
		family.IPv6Unicast: {
			"2001:db8::/32": {Family: family.IPv6Unicast, Prefix: "2001:db8::/32", NextHop: "2001:db8::1", Origin: new(OriginIGP)},
		},
	})

	peerCount := func(status any) map[string]any {
		m, ok := status.(map[string]any)
		require.True(t, ok)
		counts, ok := m["route-counts"].(map[string]any)
		require.True(t, ok)
		p, ok := counts["192.0.2.1"].(map[string]any)
		require.True(t, ok)
		return p
	}

	// No family: all-family totals.
	total := peerCount(r.status(""))
	assert.Equal(t, 3, total["in"], "all-family Adj-RIB-In")
	assert.Equal(t, 3, total["out"], "all-family Adj-RIB-Out")

	// IPv4 filter: only the IPv4 routes.
	v4 := peerCount(r.status("ipv4/unicast"))
	assert.Equal(t, 2, v4["in"], "IPv4-scoped Adj-RIB-In")
	assert.Equal(t, 2, v4["out"], "IPv4-scoped Adj-RIB-Out")

	// IPv6 filter: only the IPv6 route.
	v6 := peerCount(r.status("ipv6/unicast"))
	assert.Equal(t, 1, v6["in"], "IPv6-scoped Adj-RIB-In")
	assert.Equal(t, 1, v6["out"], "IPv6-scoped Adj-RIB-Out")

	// Unrecognized family falls back to all-family totals (never panics).
	bogus := peerCount(r.status("not/afamily"))
	assert.Equal(t, 3, bogus["in"], "unknown family falls back to totals")
}

// TestInboundShowFamilyFilter verifies family filter restricts results.
//
// VALIDATES: AC-6 — show bgp rib received with family filter returns only matching family.
// PREVENTS: Family filter being ignored, all families returned.
func TestInboundShowFamilyFilter(t *testing.T) {
	r := newTestRIBManager(t)

	// Insert IPv4 route
	ipv4Family := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriIPv4 := []byte{24, 10, 0, 0} // 10.0.0.0/24

	// Insert IPv6 route
	ipv6Family := family.IPv6Unicast
	nlriIPv6 := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00} // 2001:db8:1::/64

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(ipv4Family, attrBytes, nlriIPv4, true)
	peerRIB.Insert(ipv6Family, attrBytes, nlriIPv6, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Without filter: both families
	allRoutes := requirePeerRoutes(t, anyToJSONStr(t, r.showPipeline("*", []string{"received"})), "adj-rib-in", "192.0.2.1")
	assert.Len(t, allRoutes, 2, "expected both routes without filter")

	// With family filter: only IPv4
	filteredRoutes := requirePeerRoutes(t, anyToJSONStr(t, r.showPipeline("*", []string{"received", "family", family.IPv4Unicast.String()})), "adj-rib-in", "192.0.2.1")
	require.Len(t, filteredRoutes, 1, "expected only IPv4 route")
	first, ok := filteredRoutes[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, family.IPv4Unicast.String(), first["family"])
}

// TestInboundShowPrefixFilter verifies prefix filter restricts results.
//
// VALIDATES: AC-7 — show bgp rib received with prefix filter returns only matching prefix.
// PREVENTS: Prefix filter being ignored, all prefixes returned.
func TestInboundShowPrefixFilter(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlri1 := []byte{24, 10, 0, 0}   // 10.0.0.0/24
	nlri2 := []byte{24, 172, 16, 0} // 172.16.0.0/24

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(fam, attrBytes, nlri1, true)
	peerRIB.Insert(fam, attrBytes, nlri2, true)
	r.bgpPeers[netip.MustParseAddr("192.0.2.1")] = peerRIB

	// Filter by prefix (exact prefix string match)
	routes := requirePeerRoutes(t, anyToJSONStr(t, r.showPipeline("*", []string{"received", "prefix", "10.0.0.0/24"})), "adj-rib-in", "192.0.2.1")
	require.Len(t, routes, 1, "expected only matching prefix")
	first, ok := routes[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.0/24", first["prefix"])
}

// TestOutboundShowMinimalAttributes verifies outbound show omits missing attributes.
//
// VALIDATES: Missing optional attributes are omitted from outbound show output.
// PREVENTS: Null/zero values for missing MED, communities in output.
func TestOutboundShowMinimalAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut[netip.MustParseAddr("192.0.2.2")] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {
				Family:  family.IPv4Unicast,
				Prefix:  "10.0.0.0/24",
				NextHop: "10.0.0.1",
			},
		},
	})

	route := requireFirstRoute(t, anyToJSONStr(t, r.showPipeline("*", []string{"sent"})), "adj-rib-out", "192.0.2.2")

	// Only family, prefix, next-hop should be present
	assert.Equal(t, family.IPv4Unicast.String(), route["family"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])

	_, hasOrigin := route["origin"]
	assert.False(t, hasOrigin, "origin should be absent when empty")
	_, hasMED := route["med"]
	assert.False(t, hasMED, "MED should be absent when nil")
}

// TestInjectUsesProtocolSlot verifies request bgp rib inject stores in bgpPeers.
func TestInjectUsesProtocolSlot(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}

	status, _, err := r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	require.NotNil(t, r.bgpPeers[netip.MustParseAddr("10.0.0.1")])
	assert.Equal(t, 1, r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Len())

	nlri, err := prefixToWire("ipv4/unicast", "10.0.0.0/24", 0, false)
	require.NoError(t, err)
	_, found := r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Lookup(ipv4Uni, nlri)
	assert.True(t, found)
}

// TestInjectAcceptsNexthopAlias verifies the user-facing inject command accepts
// both the historic nhop spelling and the full nexthop spelling.
func TestInjectAcceptsNexthopAlias(t *testing.T) {
	r := newTestRIBManager(t)

	status, _, err := r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "nexthop", "10.0.0.2",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	route := requireFirstRoute(t, anyToJSONStr(t, r.showPipeline("*", []string{"received"})), "adj-rib-in", "10.0.0.1")
	assert.Equal(t, "10.0.0.2", route["next-hop"])
}

// TestInjectRFC5549ExtendedNextHop verifies that injecting an IPv4 unicast prefix
// with an IPv6 next-hop stores the next-hop as an RFC 5549/8950 extended next-hop
// (carried in MP_REACH_NLRI) instead of discarding it. The stored MP_REACH's IPv6
// next-hop is recovered exactly as the forward path recovers it
// (bestCandidateNextHopAddr -> extractMPNextHopAddr), so the route re-advertises
// with the extended next-hop.
// RFC 5549: advertising IPv4 NLRI with an IPv6 next-hop. RFC 8950: encoding.
func TestInjectRFC5549ExtendedNextHop(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}

	status, _, err := r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24", "nexthop", "2001:db8::1",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	nlri, err := prefixToWire("ipv4/unicast", "10.0.0.0/24", 0, false)
	require.NoError(t, err)
	entry, found := r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Lookup(ipv4Uni, nlri)
	require.True(t, found, "injected route must be stored")

	nh := extractMPNextHopAddr(entry.GetBundle())
	assert.Equal(t, "2001:db8::1", nh.String(), "stored MP_REACH must carry the IPv6 extended next-hop")
}

// TestInjectIPv6NextHopNativeFamily verifies the same MP_REACH path carries an
// IPv6 next-hop for a native IPv6 NLRI (ordinary MP-BGP, not RFC 5549), which the
// pre-fix inject path also discarded.
func TestInjectIPv6NextHopNativeFamily(t *testing.T) {
	r := newTestRIBManager(t)
	ipv6Uni := family.Family{AFI: 2, SAFI: 1}

	status, _, err := r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.1", "ipv6/unicast", "2001:db8:1::/48", "nexthop", "2001:db8::2",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)

	nlri, err := prefixToWire("ipv6/unicast", "2001:db8:1::/48", 0, false)
	require.NoError(t, err)
	entry, found := r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Lookup(ipv6Uni, nlri)
	require.True(t, found, "injected route must be stored")

	nh := extractMPNextHopAddr(entry.GetBundle())
	assert.Equal(t, "2001:db8::2", nh.String(), "stored MP_REACH must carry the IPv6 next-hop")
}

// TestWithdrawUsesProtocolSlot verifies request bgp rib withdraw reads from bgpPeers.
func TestWithdrawUsesProtocolSlot(t *testing.T) {
	r := newTestRIBManager(t)
	ipv4Uni := family.Family{AFI: 1, SAFI: 1}

	_, _, err := r.handleCommand("request bgp rib inject", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24",
	})
	require.NoError(t, err)
	require.Equal(t, 1, r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Len())

	status, data, err := r.handleCommand("request bgp rib withdraw", "", []string{
		"10.0.0.1", "ipv4/unicast", "10.0.0.0/24",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", status)
	m, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, m["existed"])

	nlri, err := prefixToWire("ipv4/unicast", "10.0.0.0/24", 0, false)
	require.NoError(t, err)
	_, found := r.bgpPeers[netip.MustParseAddr("10.0.0.1")].Lookup(ipv4Uni, nlri)
	assert.False(t, found)
}
