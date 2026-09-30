package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref proves the
// sender side of the attributes an MP_REACH_NLRI UPDATE must carry.
//
// VALIDATES: an IPv6 route built by UpdateBuilder.BuildUnicast carries
// MP_REACH_NLRI with ORIGIN and AS_PATH on eBGP and on iBGP, and LOCAL_PREF on
// iBGP, both when the route states its attributes and when it states none
// (no Origin, no AS path, no LOCAL_PREF value).
// PREVENTS: a builder that emits only the attributes the caller filled in, so
// a bare route leaves as an MP_REACH UPDATE without its mandatory attributes.
//
// RFC requirement: RFC4760-3-3 positive -- with ORIGIN IGP stated, the MP_REACH_NLRI UPDATE
// carries ORIGIN and AS_PATH on eBGP and on iBGP.
// RFC requirement: RFC4760-3-3 negative -- a route that states no ORIGIN and no AS path still
// leaves, on eBGP and on iBGP, as an MP_REACH_NLRI UPDATE carrying ORIGIN and AS_PATH.
// RFC requirement: RFC4760-3-4 positive -- the iBGP MP_REACH_NLRI UPDATE of the stated route
// carries LOCAL_PREF.
// RFC requirement: RFC4760-3-4 negative -- a route that states no LOCAL_PREF value still
// leaves on iBGP as an MP_REACH_NLRI UPDATE carrying LOCAL_PREF.
func TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref(t *testing.T) {
	stated := &UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8::/64"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
	}
	bare := &UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/48"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
	}
	for _, params := range []*UnicastParams{stated, bare} {
		for _, isIBGP := range []bool{false, true} {
			upd := NewUpdateBuilder(65001, isIBGP, true, false).BuildUnicast(params)
			codes := attrCodesInBlob(upd.PathAttributes)
			what := params.Prefix.String()
			require.True(t, codes[attrCodeMPReachNLRI], "%s ibgp=%v: MP_REACH_NLRI", what, isIBGP)
			require.True(t, codes[attrCodeOrigin], "%s ibgp=%v: ORIGIN", what, isIBGP)
			require.True(t, codes[attrCodeASPath], "%s ibgp=%v: AS_PATH", what, isIBGP)
			if isIBGP {
				require.True(t, codes[attrCodeLocalPref], "%s ibgp: LOCAL_PREF", what)
			}
		}
	}
}
