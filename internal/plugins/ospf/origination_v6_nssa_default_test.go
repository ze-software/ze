// VALIDATES: RFC 3101 Section 2.4 / Section 2.7 NSSA default-route origination on the
// OSPFv3 address family, mapped by RFC 5340 Section 4.4.3.7.
// PREVENTS: an OSPFv3 NSSA border router reaching the OSPFv2 producer and keying its
// default 0x0007 (link-local scope under RFC 5340 Appendix A.4.2.1) instead of the
// NSSA-LSA 0x2007; a no-summary OSPFv3 NSSA receiving no default at all; and an
// unrelated redistribution withdrawal sweeping the default out of the area.
package ospf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// v6DualAreaNSSAABR builds an OSPFv3 engine attached to the backbone and to one NSSA, which
// is what makes it an NSSA border router. nssaLeaves are extra JSON leaves for the NSSA area,
// so a caller can ask for `no-summary true`. Connected redistribution is always configured,
// so a caller can inject an external without a second engine shape.
func v6DualAreaNSSAABR(t *testing.T, nssaLeaves string) (*engine, types.RouterID, types.AreaID) {
	t.Helper()
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{`+
		`"0.0.0.0":{"area-id":"0.0.0.0"},`+
		`"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa"`+nssaLeaves+`}}},`+
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.0"},"eth1":{"area":"0.0.0.6"}}},`+
		`"redistribute":{"connected":{"source":"connected"}}}}`)
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: types.BackboneArea}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}
	// A test decides in microseconds what a router decides over minutes, so the RFC 2328
	// Section 9.5 MinLSInterval rate limit would refuse the second origination of one LSA
	// and make the assertion read the first one's body.
	eng.lsdb.SetTimers(ospflsdb.TimerConfig{MinLSInterval: time.Nanosecond})
	return eng, rid, nssa
}

// countAreaLSAsByType reports how many non-purged LSAs of ls are held in the area store.
func countAreaLSAsByType(eng *engine, area types.AreaID, ls types.LSType) int {
	n := 0
	for _, h := range eng.lsdb.Summary(area) {
		if h.Type == ls && !h.Age.IsMaxAge() {
			n++
		}
	}
	return n
}

// TestOSPFv3NSSABorderRouterOriginatesDefault drives the reconciler on an OSPFv3 NSSA border
// router and reads the LSA it installs, proving the default reaches the area as an NSSA-LSA.
func TestOSPFv3NSSABorderRouterOriginatesDefault(t *testing.T) {
	eng, rid, nssa := v6DualAreaNSSAABR(t, "")

	eng.applyNSSADefaults()

	// RFC requirement: RFC3101-2.4-5 positive -- an NSSA border router
	// originates a default-destination LSA into every attached regular NSSA,
	// and on OSPFv3 that LSA is the 0x2007 NSSA-LSA (RFC 5340 sec 4.4.3.7).
	body, ok := decodeV6External(t, eng, nssa, v6NSSAKey(rid, v6NSSADefaultLSID))
	require.True(t, ok, "an OSPFv3 NSSA border router originates its default as a 0x2007 NSSA-LSA")
	assert.Equal(t, 0, int(body.Prefix.Length), "the default destination is a zero-length prefix")
	assert.Empty(t, body.Prefix.Address, "a zero PrefixLength carries no Address Prefix words")
	assert.Equal(t, uint32(1), body.Metric, "the default carries the area default-cost")
	assert.False(t, body.ExternalType2, "the border-router default is a Type-1 external metric")
	// RFC requirement: RFC3101-2.4-4 positive -- an NSSA border router
	// clears the P-bit on its Type-7 default; on OSPFv3 that bit rides in
	// PrefixOptions rather than the LSA header Options.
	assert.Zero(t, body.Prefix.Options&ospfv3types.OptPrefixP, "the border-router default is P-clear")
}

// TestOSPFv3NSSADefaultUsesV6Producer is the address-family wiring test: it asserts the
// negative space the LS type leaves behind. An OSPFv2-keyed Type 7 in an OSPFv3 area store
// is what a conforming peer reads as function code 7 at link-local flooding scope.
func TestOSPFv3NSSADefaultUsesV6Producer(t *testing.T) {
	eng, _, nssa := v6DualAreaNSSAABR(t, "")

	eng.applyNSSADefaults()

	assert.Equal(t, 1, countAreaLSAsByType(eng, nssa, types.LSType(ospfv3types.LSTypeNSSA)),
		"exactly one OSPFv3 NSSA-LSA, the default")
	assert.Equal(t, 0, countAreaLSAsByType(eng, nssa, types.LSTypeNSSA),
		"no OSPFv2-keyed Type 7 may reach an OSPFv3 area store")
}

// TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA proves the no-summary NSSA takes its default
// from the summary path instead, and that a regular NSSA does not, so the two producers do
// not both fire in one area.
func TestOSPFv3NSSANoSummaryDefaultUsesSummaryLSA(t *testing.T) {
	eng, _, nssa := v6DualAreaNSSAABR(t, `,"no-summary":"true"`)

	eng.applyNSSADefaults()

	// RFC requirement: RFC3101-2.7-2 negative -- a no-summary NSSA does not
	// take its border-router default as a Type-7.
	assert.Equal(t, 0, countAreaLSAsByType(eng, nssa, types.LSType(ospfv3types.LSTypeNSSA)),
		"a no-summary NSSA takes its default from the summary path, not as an NSSA-LSA")

	defaultPrefix := netip.PrefixFrom(netip.IPv6Unspecified(), 0)
	// RFC requirement: RFC3101-2.7-2 positive -- when summary import is
	// suppressed, the border-router default is a summary-LSA; the OSPFv3
	// equivalent is the Inter-Area-Prefix-LSA (RFC 5340 sec 4.4).
	nets, routers := v6ApplyAreaTypePolicy(nil, []v6SummaryRouter{{}},
		ospfspf.AreaSummaryPolicy{Type: ospfspf.AreaTypeNSSA, NoSummary: true, DefaultCost: 17})
	require.Len(t, nets, 1, "a no-summary NSSA receives exactly one inter-area prefix, the default")
	assert.Equal(t, defaultPrefix, nets[0].Prefix)
	assert.Equal(t, uint32(17), nets[0].Metric, "the default carries the area default-cost")
	assert.Empty(t, routers, "a stub or NSSA area never receives an Inter-Area-Router-LSA")

	regular, _ := v6ApplyAreaTypePolicy(nil, nil,
		ospfspf.AreaSummaryPolicy{Type: ospfspf.AreaTypeNSSA, DefaultCost: 17})
	assert.Empty(t, regular, "a regular NSSA takes its default as an NSSA-LSA, not an inter-area prefix")
}

// TestOSPFv3NSSADefaultSurvivesUnrelatedWithdrawal covers the keep-set: the withdrawal of a
// redistributed prefix sweeps stale self externals, and the per-area default is not one.
func TestOSPFv3NSSADefaultSurvivesUnrelatedWithdrawal(t *testing.T) {
	eng, rid, nssa := v6DualAreaNSSAABR(t, "")
	prefix := netip.MustParsePrefix("2001:db8:9::/64")
	require.NoError(t, eng.InjectExternal(prefix, "connected"))
	eng.applyNSSADefaults()
	require.Equal(t, 2, countAreaLSAsByType(eng, nssa, types.LSType(ospfv3types.LSTypeNSSA)),
		"the redistributed prefix and the default are both NSSA-LSAs")

	_, err := eng.WithdrawExternal(prefix)
	require.NoError(t, err)

	_, ok := decodeV6External(t, eng, nssa, v6NSSAKey(rid, v6NSSADefaultLSID))
	require.True(t, ok, "the default LSA is still installed")
	assert.Equal(t, 1, countAreaLSAsByType(eng, nssa, types.LSType(ospfv3types.LSTypeNSSA)),
		"withdrawing an unrelated redistributed prefix leaves the default alone")
}

// TestOSPFv3NSSADefaultLSIDDoesNotCollide pins the reservation the default relies on:
// redistribution pre-increments its allocator, so it never hands out the default's Link
// State ID. A collision would make one LSA overwrite the other.
func TestOSPFv3NSSADefaultLSIDDoesNotCollide(t *testing.T) {
	eng, _, _ := v6DualAreaNSSAABR(t, "")

	for _, s := range []string{"2001:db8:1::/64", "2001:db8:2::/64", "2001:db8:3::/64"} {
		require.NoError(t, eng.InjectExternal(netip.MustParsePrefix(s), "connected"))
	}

	for prefix, lsid := range eng.redistV6 {
		assert.NotEqual(t, v6NSSADefaultLSID, lsid, "redistribution must not take the default's LSID (%s)", prefix)
	}
}

// TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress covers the internal-router half
// on OSPFv3. A single-area NSSA router is not a border router, so its only route to a
// default is the operator's `default-originate`, and RFC 3101 makes a usable forwarding
// address a precondition for the P-set LSA that leaf asks for.
func TestOSPFv3NSSAInternalRouterDefaultNeedsForwardingAddress(t *testing.T) {
	eng, rid := newV6RedistEngine(t, `{"ospf":{"router-id":"10.0.10.9","areas":{"area":{`+
		`"0.0.0.6":{"area-id":"0.0.0.6","area-type":"nssa","nssa":{"default-originate":"true"}}}},`+
		`"interfaces":{"interface":{"eth1":{"area":"0.0.0.6"}}}}}`)
	nssa := types.AreaID{0, 0, 0, 6}
	eng.running["eth1"] = interfaceConfig{Name: "eth1", AreaID: nssa}

	eng.applyNSSADefaults()

	// RFC requirement: RFC3101-2.4-2 negative -- a P-set Type-7 default is
	// not originated without a valid non-zero forwarding address.
	// RFC requirement: RFC3101-2.4-5 negative -- a router that is not an
	// NSSA border router originates no border-router default.
	assert.Equal(t, 0, countAreaLSAsByType(eng, nssa, types.LSType(ospfv3types.LSTypeNSSA)),
		"no usable forwarding address on eth1, so no P-set default")
	_, ok := decodeV6External(t, eng, nssa, v6NSSAKey(rid, v6NSSADefaultLSID))
	assert.False(t, ok)
}

// TestOSPFv3NSSADefaultPBitFollowsForwardingAddress drives the origination boundary itself
// with an explicit forwarding address, which a unit test cannot get from a real interface.
func TestOSPFv3NSSADefaultPBitFollowsForwardingAddress(t *testing.T) {
	eng, rid, nssa := v6DualAreaNSSAABR(t, "")
	fa := netip.MustParseAddr("2001:db8:6::1").As16()

	require.True(t, eng.v6OriginateNSSADefault(nssa, rid, 42, fa, true, true))

	body, ok := decodeV6External(t, eng, nssa, v6NSSAKey(rid, v6NSSADefaultLSID))
	require.True(t, ok)
	// RFC requirement: RFC3101-2.4-4 negative -- an internal NSSA router's
	// Type-7 default may carry the P-bit set, so the border-router rule is
	// not a blanket clear.
	assert.NotZero(t, body.Prefix.Options&ospfv3types.OptPrefixP, "a P-set default keeps the bit")
	assert.True(t, body.HasForwardingAddr)
	assert.Equal(t, fa, body.ForwardingAddr)
	assert.Equal(t, uint32(42), body.Metric)

	require.True(t, eng.v6OriginateNSSADefault(nssa, rid, 42, [16]byte{}, false, true))
	body, ok = decodeV6External(t, eng, nssa, v6NSSAKey(rid, v6NSSADefaultLSID))
	require.True(t, ok)
	assert.Zero(t, body.Prefix.Options&ospfv3types.OptPrefixP,
		"a zero forwarding address clears the P-bit at the origination boundary")
	assert.False(t, body.HasForwardingAddr)
}
