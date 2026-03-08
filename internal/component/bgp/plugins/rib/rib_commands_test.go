package rib

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
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

// requireFirstRoute unmarshals JSON and extracts the first route for a peer.
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

// TestInboundShowWithAttributes verifies enriched rib show in returns attributes.
//
// VALIDATES: AC-6 — rib show in returns origin, as-path, med, local-pref, communities.
// PREVENTS: Show command returning only family/prefix/next-hop without path attributes.
func TestInboundShowWithAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	// Insert a route with full attributes into pool storage
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
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
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	route := requireFirstRoute(t, r.inboundShowJSON("*", nil), "adj-rib-in", "192.0.2.1")

	assert.Equal(t, "ipv4/unicast", route["family"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])
	assert.Equal(t, "igp", route["origin"])
	assert.Equal(t, float64(100), route["med"])
	assert.Equal(t, float64(100), route["local-preference"])

	// AS path comes as []any with float64 values in JSON
	asPath, ok := route["as-path"].([]any)
	require.True(t, ok, "expected as-path array")
	require.Len(t, asPath, 1)
	assert.Equal(t, float64(65001), asPath[0])

	// Communities
	communities, ok := route["community"].([]any)
	require.True(t, ok, "expected community array")
	require.Len(t, communities, 1)
	assert.Equal(t, "65000:100", communities[0])
}

// TestInboundShowMinimalAttributes verifies show with only mandatory attributes.
//
// VALIDATES: Missing optional attributes are omitted from output.
// PREVENTS: Null/zero values for missing MED, LOCAL_PREF, communities.
func TestInboundShowMinimalAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriBytes := []byte{24, 10, 0, 0}

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlriBytes)
	r.ribInPool["192.0.2.1"] = peerRIB

	route := requireFirstRoute(t, r.inboundShowJSON("*", nil), "adj-rib-in", "192.0.2.1")

	assert.Equal(t, "igp", route["origin"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])

	// Optional attributes should be absent
	_, hasMED := route["med"]
	assert.False(t, hasMED, "MED should be absent when not in route")
	_, hasLP := route["local-preference"]
	assert.False(t, hasLP, "LOCAL_PREF should be absent when not in route")
	_, hasCom := route["community"]
	assert.False(t, hasCom, "communities should be absent when not in route")
}

// TestOutboundShowWithAttributes verifies enriched rib show out returns attributes.
//
// VALIDATES: AC-7 — rib show out returns origin, as-path, med, local-pref, communities.
// PREVENTS: Outbound show missing path attributes for route replay verification.
func TestOutboundShowWithAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	med := uint32(100)
	localPref := uint32(200)
	r.ribOut["192.0.2.1"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {
			Family:           "ipv4/unicast",
			Prefix:           "10.0.0.0/24",
			NextHop:          "10.0.0.1",
			Origin:           "igp",
			ASPath:           []uint32{65001, 65002},
			MED:              &med,
			LocalPreference:  &localPref,
			Communities:      []string{"65000:100"},
			LargeCommunities: []string{"65000:1:2"},
		},
	}

	route := requireFirstRoute(t, r.outboundShowJSON("*"), "adj-rib-out", "192.0.2.1")

	assert.Equal(t, "ipv4/unicast", route["family"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])
	assert.Equal(t, "igp", route["origin"])
	assert.Equal(t, float64(100), route["med"])
	assert.Equal(t, float64(200), route["local-preference"])

	asPath, ok := route["as-path"].([]any)
	require.True(t, ok, "expected as-path array")
	require.Len(t, asPath, 2)
	assert.Equal(t, float64(65001), asPath[0])
	assert.Equal(t, float64(65002), asPath[1])

	communities, ok := route["community"].([]any)
	require.True(t, ok, "expected community array")
	require.Len(t, communities, 1)
	assert.Equal(t, "65000:100", communities[0])

	largeCommunities, ok := route["large-community"].([]any)
	require.True(t, ok, "expected large-community array")
	require.Len(t, largeCommunities, 1)
	assert.Equal(t, "65000:1:2", largeCommunities[0])
}

// TestInboundShowFamilyFilter verifies family filter restricts results.
//
// VALIDATES: AC-6 — rib show in with family filter returns only matching family.
// PREVENTS: Family filter being ignored, all families returned.
func TestInboundShowFamilyFilter(t *testing.T) {
	r := newTestRIBManager(t)

	// Insert IPv4 route
	ipv4Family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlriIPv4 := []byte{24, 10, 0, 0} // 10.0.0.0/24

	// Insert IPv6 route
	ipv6Family := nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	nlriIPv6 := []byte{64, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00} // 2001:db8:1::/64

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(ipv4Family, attrBytes, nlriIPv4)
	peerRIB.Insert(ipv6Family, attrBytes, nlriIPv6)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Without filter: both families
	allRoutes := requirePeerRoutes(t, r.inboundShowJSON("*", nil), "adj-rib-in", "192.0.2.1")
	assert.Len(t, allRoutes, 2, "expected both routes without filter")

	// With family filter: only IPv4
	filteredRoutes := requirePeerRoutes(t, r.inboundShowJSON("*", []string{"ipv4/unicast"}), "adj-rib-in", "192.0.2.1")
	require.Len(t, filteredRoutes, 1, "expected only IPv4 route")
	first, ok := filteredRoutes[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ipv4/unicast", first["family"])
}

// TestInboundShowPrefixFilter verifies prefix filter restricts results.
//
// VALIDATES: AC-7 — rib show in with prefix filter returns only matching prefix.
// PREVENTS: Prefix filter being ignored, all prefixes returned.
func TestInboundShowPrefixFilter(t *testing.T) {
	r := newTestRIBManager(t)

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	attrBytes := concatBytes(testWireOriginIGP, testWireNextHop)
	nlri1 := []byte{24, 10, 0, 0}   // 10.0.0.0/24
	nlri2 := []byte{24, 172, 16, 0} // 172.16.0.0/24

	peerRIB := storage.NewPeerRIB("192.0.2.1")
	peerRIB.Insert(family, attrBytes, nlri1)
	peerRIB.Insert(family, attrBytes, nlri2)
	r.ribInPool["192.0.2.1"] = peerRIB

	// Filter by prefix
	routes := requirePeerRoutes(t, r.inboundShowJSON("*", []string{"10.0.0.0/24"}), "adj-rib-in", "192.0.2.1")
	require.Len(t, routes, 1, "expected only matching prefix")
	first, ok := routes[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.0/24", first["prefix"])
}

// TestParseShowFilters verifies family vs prefix disambiguation.
//
// VALIDATES: parseShowFilters distinguishes family ("ipv4/unicast") from prefix ("10.0.0.0/24", "fc00::/7").
// PREVENTS: IPv6 ULA prefix "fc00::/7" being misclassified as family filter.
func TestParseShowFilters(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFamily string
		wantPrefix string
	}{
		{"family only", []string{"ipv4/unicast"}, "ipv4/unicast", ""},
		{"ipv4 prefix", []string{"10.0.0.0/24"}, "", "10.0.0.0/24"},
		{"ipv6 prefix", []string{"2001:db8::/32"}, "", "2001:db8::/32"},
		{"ipv6 ula prefix", []string{"fc00::/7"}, "", "fc00::/7"},
		{"both", []string{"ipv4/unicast", "10.0.0.0/24"}, "ipv4/unicast", "10.0.0.0/24"},
		{"no slash", []string{"hello"}, "", ""},
		{"empty", nil, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			family, prefix := parseShowFilters(tt.args)
			assert.Equal(t, tt.wantFamily, family, "family")
			assert.Equal(t, tt.wantPrefix, prefix, "prefix")
		})
	}
}

// TestOutboundShowMinimalAttributes verifies outbound show omits missing attributes.
//
// VALIDATES: Missing optional attributes are omitted from outbound show output.
// PREVENTS: Null/zero values for missing MED, communities in output.
func TestOutboundShowMinimalAttributes(t *testing.T) {
	r := newTestRIBManager(t)

	r.ribOut["192.0.2.2"] = map[string]*Route{
		"ipv4/unicast:10.0.0.0/24": {
			Family:  "ipv4/unicast",
			Prefix:  "10.0.0.0/24",
			NextHop: "10.0.0.1",
		},
	}

	route := requireFirstRoute(t, r.outboundShowJSON("*"), "adj-rib-out", "192.0.2.2")

	// Only family, prefix, next-hop should be present
	assert.Equal(t, "ipv4/unicast", route["family"])
	assert.Equal(t, "10.0.0.0/24", route["prefix"])
	assert.Equal(t, "10.0.0.1", route["next-hop"])

	_, hasOrigin := route["origin"]
	assert.False(t, hasOrigin, "origin should be absent when empty")
	_, hasMED := route["med"]
	assert.False(t, hasMED, "MED should be absent when nil")
}
