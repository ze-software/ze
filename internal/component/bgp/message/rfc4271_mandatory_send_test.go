package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes proves the sender side
// of the well-known mandatory attributes an UPDATE carrying NLRI must include.
//
// VALIDATES: every UPDATE UpdateBuilder.BuildUnicast emits whose NLRI field is
// not empty carries ORIGIN, AS_PATH and NEXT_HOP, on eBGP and on iBGP, both
// when the route states its attributes and when it states none (no Origin, no
// AS path), and an IPv4 route with an Extended Next Hop IPv6 next hop leaves
// its NLRI in MP_REACH_NLRI rather than in the body.
// PREVENTS: a builder that writes only the attributes the caller filled in, so
// a bare route leaves as an UPDATE with NLRI and no ORIGIN or AS_PATH.
//
// RFC requirement: RFC4271-5-2 positive -- an IPv4 route stating ORIGIN IGP and an AS path
// leaves, on eBGP and on iBGP, as an UPDATE whose NLRI field carries it and whose
// attributes include ORIGIN, AS_PATH and NEXT_HOP.
// RFC requirement: RFC4271-5-2 negative -- a route stating no ORIGIN and no AS path still
// leaves, on eBGP and on iBGP, as an UPDATE with NLRI carrying ORIGIN, AS_PATH and NEXT_HOP.
func TestRFC4271UpdateWithNLRICarriesTheMandatoryAttributes(t *testing.T) {
	stated := &UnicastParams{
		Prefix:  netip.MustParsePrefix("192.0.2.0/24"),
		NextHop: netip.MustParseAddr("198.51.100.1"),
		Origin:  attribute.OriginIGP,
		ASPath:  []uint32{65002, 65003},
	}
	bare := &UnicastParams{
		Prefix:  netip.MustParsePrefix("203.0.113.0/24"),
		NextHop: netip.MustParseAddr("198.51.100.1"),
	}
	extendedNextHop := &UnicastParams{
		Prefix:             netip.MustParsePrefix("198.18.0.0/16"),
		NextHop:            netip.MustParseAddr("2001:db8::1"),
		UseExtendedNextHop: true,
	}

	// The stated and the bare route carry their NLRI in the UPDATE body.
	for _, params := range []*UnicastParams{stated, bare} {
		for _, isIBGP := range []bool{false, true} {
			upd := mustBuildUnicast(t, NewUpdateBuilder(65001, isIBGP, true, false), params)
			what := params.Prefix.String()
			require.NotEmpty(t, upd.NLRI, "%s ibgp=%v: NLRI field", what, isIBGP)
			requireMandatoryWithNLRI(t, upd, what, isIBGP)
		}
	}

	// RFC 8950: the IPv4 route with an IPv6 next hop carries no body NLRI.
	for _, isIBGP := range []bool{false, true} {
		upd := mustBuildUnicast(t, NewUpdateBuilder(65001, isIBGP, true, false), extendedNextHop)
		require.Empty(t, upd.NLRI, "extended next hop ibgp=%v: NLRI field", isIBGP)
		require.True(t, attrCodesInBlob(upd.PathAttributes)[attrCodeMPReachNLRI],
			"extended next hop ibgp=%v: MP_REACH_NLRI", isIBGP)
	}
}

// requireMandatoryWithNLRI asserts the RFC 4271 Section 5 well-known mandatory
// attributes of an UPDATE whose NLRI field is not empty.
func requireMandatoryWithNLRI(t *testing.T, upd *Update, what string, isIBGP bool) {
	t.Helper()
	codes := attrCodesInBlob(upd.PathAttributes)
	require.True(t, codes[attrCodeOrigin], "%s ibgp=%v: ORIGIN", what, isIBGP)
	require.True(t, codes[attrCodeASPath], "%s ibgp=%v: AS_PATH", what, isIBGP)
	require.True(t, codes[attrCodeNextHop], "%s ibgp=%v: NEXT_HOP", what, isIBGP)
}
