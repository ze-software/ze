// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-4, AC-5 -- the tag the
// consumer hands the engine becomes the External Route Tag of the AS-External-LSA
// (RFC 2328 Appendix A.4.5): the route's own tag, or the tag an `ospf filter export`
// filter set, which is how `ze config migrate` carries a retired per-source `tag`
// for a route that carries none (`match { tag [ 0 ] }`).
// PREVENTS: a route tag reaching the engine and being dropped before origination.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestEngineInjectExternalRouteTagWins drives the whole engine path: the Type 5 the
// engine originates carries the route's tag.
func TestEngineInjectExternalRouteTagWins(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"static":{"source":"static"}}}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.5.0.0/24"), "static", defaultExternal(4242)))
	body, ok := externalBody(t, eng, rid, "10.5.0.0/24")
	require.True(t, ok, "Type 5 originated for the injected prefix")
	assert.Equal(t, uint32(4242), body.ExternalRouteTag, "the route's own tag is the External Route Tag")
}

// TestEngineInjectExternalUntaggedRouteTakesConfiguredTag proves a tag the export
// chain set for an untagged route (the migrated per-source tag 7) reaches the LSA.
func TestEngineInjectExternalUntaggedRouteTakesConfiguredTag(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1","redistribute":{"static":{"source":"static"}}}}`)

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.6.0.0/24"), "static", defaultExternal(7)))
	body, ok := externalBody(t, eng, rid, "10.6.0.0/24")
	require.True(t, ok)
	assert.Equal(t, uint32(7), body.ExternalRouteTag, "an untagged route takes the tag the export chain set")
}

// TestEngineInjectExternalNSSARouteTag proves the Type 7 an NSSA-internal ASBR
// originates carries the route's own tag too, not only the AS-wide Type 5. RFC 3101
// Section 2.4 gives the Type 7 the same External Route Tag field as the Type 5.
func TestEngineInjectExternalNSSARouteTag(t *testing.T) {
	eng, rid := newRedistEngine(t, `{"ospf":{"router-id":"10.0.3.1","areas":{"area":{"0.0.0.5":{"area-id":"0.0.0.5","area-type":"nssa"}}},"interfaces":{"interface":{"eth0":{"area":"0.0.0.5"}}},"redistribute":{"static":{"source":"static"}}}}`)
	nssa := types.AreaID{0, 0, 0, 5}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}

	require.NoError(t, eng.InjectExternal(netip.MustParsePrefix("10.9.0.0/24"), "static", defaultExternal(4242)))
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
