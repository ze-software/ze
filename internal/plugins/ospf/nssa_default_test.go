// VALIDATES: RFC 3101 Section 1.3 -- an NSSA internal router's `default-originate` and the
// no-summary option are mutually exclusive, in both address families, so a totally-NSSA
// area's only default is the border router's summary-LSA.
// PREVENTS: an internal ASBR injecting a Type-7 default into a no-summary NSSA, where the
// area's inter-area traffic would leave the AS through it instead of following the Type-3
// default the border router originated for exactly that purpose.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// nssaInternalOriginatorConfig is a single-area NSSA router with `default-originate` set.
// One area and no backbone makes it an NSSA internal router rather than a border router,
// so the Type-7 default the operator asked for is the only default it can produce.
func nssaInternalOriginatorConfig(noSummary bool) string {
	leaves := ""
	if noSummary {
		leaves = `,"no-summary":"true"`
	}
	return `{"ospf":{"router-id":"10.0.5.1","areas":{"area":{"0.0.0.5":{"area-id":"0.0.0.5",` +
		`"area-type":"nssa","nssa":{"default-originate":"true"}` + leaves + `}}},` +
		`"interfaces":{"interface":{"eth0":{"area":"0.0.0.5"}}}}}`
}

// TestOSPFNSSAInternalDefaultExcludedByNoSummary drives the reconciler on an NSSA internal
// router that has a usable forwarding address, so the only difference between the two runs
// is the no-summary leaf. Each family carries its own control run, because an assertion
// that the LSA is absent proves nothing until the same router has been seen to produce it.
func TestOSPFNSSAInternalDefaultExcludedByNoSummary(t *testing.T) {
	nssa := types.AreaID{0, 0, 0, 5}

	t.Run("OSPFv2", func(t *testing.T) {
		count := func(noSummary bool) int {
			eng, rid := newRedistEngine(t, nssaInternalOriginatorConfig(noSummary))
			eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}
			eng.forwardingAddress = func(string) (netip.Addr, bool) {
				return netip.MustParseAddr("192.0.2.1"), true
			}
			eng.applyNSSADefaults()
			return selfNSSACount(eng, nssa, rid)
		}

		require.Equal(t, 1, count(false),
			"control: with summary import enabled the operator's default-originate produces a Type-7 default")
		// RFC 3101 Section 1.3: "The Type-7 default LSAs originated by NSSA
		// internal routers and the no-summary option are mutually exclusive
		// features."
		assert.Equal(t, 0, count(true), "no-summary makes default-originate inert on an internal router")
	})

	t.Run("OSPFv3", func(t *testing.T) {
		count := func(noSummary bool) int {
			eng, _ := newV6RedistEngine(t, nssaInternalOriginatorConfig(noSummary))
			eng.running["eth0"] = interfaceConfig{Name: "eth0", AreaID: nssa}
			eng.forwardingAddress = func(string) (netip.Addr, bool) {
				return netip.MustParseAddr("2001:db8:5::1"), true
			}
			eng.applyNSSADefaults()
			return countAreaLSAsByType(eng, nssa, types.LSType(ospfv3types.LSTypeNSSA))
		}

		require.Equal(t, 1, count(false),
			"control: with summary import enabled the operator's default-originate produces an NSSA-LSA default")
		// RFC 3101 Section 1.3: the same exclusion, on the address family whose
		// producer RFC 5340 Section 4.4.3.7 maps onto the same procedure.
		assert.Equal(t, 0, count(true), "no-summary makes default-originate inert on an internal router")
	})
}
