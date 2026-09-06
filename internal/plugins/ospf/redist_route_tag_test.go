// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-4, AC-5 -- a redistributed
// route's own tag becomes the External Route Tag of the AS-External-LSA (RFC 2328
// Appendix A.4.5), and the per-source `tag` under `ospf { redistribute { source ... } }`
// is the fallback for a route that carries none.
// PREVENTS: the per-source tag silently overriding a per-route tag, and a route tag
// reaching the engine and being dropped before origination.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestExternalRouteTagPrecedence pins the precedence guard itself: a nonzero route tag
// wins, and zero means "this route carries no tag", so the configured value applies.
func TestExternalRouteTagPrecedence(t *testing.T) {
	cases := []struct {
		name       string
		routeTag   uint32
		configured uint32
		want       uint32
	}{
		{name: "route tag wins over configured", routeTag: 4242, configured: 7, want: 4242},
		{name: "no route tag falls back to configured", routeTag: 0, configured: 7, want: 7},
		{name: "route tag applies with nothing configured", routeTag: 4242, configured: 0, want: 4242},
		{name: "neither set originates zero", routeTag: 0, configured: 0, want: 0},
		{name: "maximum route tag is preserved", routeTag: 0xFFFFFFFF, configured: 7, want: 0xFFFFFFFF},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, externalRouteTag(tc.routeTag, tc.configured))
		})
	}
}

// TestEngineInjectExternalRouteTagWins drives the whole engine path: the Type 5 the
// engine originates carries the route's tag, not the configured one.
func TestEngineInjectExternalRouteTagWins(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"static":{"source":"static","tag":"7"}}}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.5.0.0/24"), "static", 4242))
	body, ok := externalBody(t, eng, rid, "10.5.0.0/24")
	require.True(t, ok, "Type 5 originated for the injected prefix")
	assert.Equal(t, uint32(4242), body.ExternalRouteTag, "the route's own tag is the External Route Tag")
}

// TestEngineInjectExternalUntaggedRouteTakesConfiguredTag proves the existing
// configuration keeps working: a route with no tag still takes the per-source value.
func TestEngineInjectExternalUntaggedRouteTakesConfiguredTag(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"static":{"source":"static","tag":"7"}}}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.6.0.0/24"), "static", 0))
	body, ok := externalBody(t, eng, rid, "10.6.0.0/24")
	require.True(t, ok)
	assert.Equal(t, uint32(7), body.ExternalRouteTag, "an untagged route takes the configured tag")
}

// TestEngineInjectExternalNSSARouteTag proves the Type 7 an NSSA-internal ASBR
// originates carries the route's own tag too, not only the AS-wide Type 5. RFC 3101
// Section 2.4 gives the Type 7 the same External Route Tag field as the Type 5.
func TestEngineInjectExternalNSSARouteTag(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.3.1","areas":{"area":{"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.5"}}},"redistribute":{"static":{"source":"static","tag":"7"}}}}`)
	nssa := types.AreaID{0, 0, 0, 5}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.9.0.0/24"), "static", 4242))
	require.Equal(t, 1, selfNSSACount(eng, nssa, rid), "Type 7 originated into the attached NSSA")

	key := types.LSAKey{
		Type:              types.LSTypeNSSA,
		LinkStateID:       types.LinkStateID(netip.MustParseAddr("10.9.0.0").As4()),
		AdvertisingRouter: rid,
	}
	lsa, ok := eng.lsdb.LookupLSA(nssa, key)
	require.True(t, ok, "the Type 7 is in the NSSA store")
	body, err := lsa.DecodeExternal()
	require.NoError(t, err)
	assert.Equal(t, uint32(4242), body.ExternalRouteTag, "the route tag reaches the Type 7 as well")
}
