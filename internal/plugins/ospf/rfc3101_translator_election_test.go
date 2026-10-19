// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA Type-7 translator election.
// Related: nssa.go -- electNSSATranslator, nssaTranslatorEnabled.
// Related: rfc3101_translator_list_test.go -- the NSSA-reachability half of list membership.
//
// VALIDATES: RFC 3101 Section 3.1: the list holds the border routers "reachable both over
// the NSSA and as ASBRs over the AS's transit topology"; a listed border router "whose
// router-LSA has bit Nt set or who has a higher router ID" disables the candidate; and only
// a border router whose NSSATranslatorState is enabled (role Always) sets the Nt-bit.
// PREVENTS: a border router the transit topology does not reach switching translation off,
// a candidate translating beside an Always translator, and a candidate advertising Nt.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// transitElectionCase builds the candidate border router 10.0.6.1 with a higher-Router-ID
// border router 10.0.6.9 (B set, Nt clear) in the NSSA, reached over the NSSA, and holding an
// ASBR entry in each area of asbrIn. It runs the translation of one Type-7 and returns how
// many Type-5s the candidate originated.
func transitElectionCase(t *testing.T, asbrIn ...types.AreaID) int {
	t.Helper()
	eng, nssa := nssaTransEngine(t, "10.0.6.1")
	self, higher := ridOf("10.0.6.1"), ridOf("10.0.6.9")
	view := fixedReachability{
		reached: map[types.RouterID]bool{self: true, higher: true},
		asbr:    map[types.AreaID][]types.RouterID{},
	}
	for _, area := range asbrIn {
		view.asbr[area] = append(view.asbr[area], higher)
	}
	eng.nssaReachabilityFn = func() routerReachability { return view }

	eng.lsdb.OriginateRouter(ospflsdb.OriginInput{AreaID: nssa, RouterID: higher, ABR: true})
	eng.lsdb.OriginateNSSA(nssa, ridOf("10.0.6.2"), ip4Of("10.28.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)
	eng.translateNSSA(transTime)
	return eng.lsdb.SelfExternalCount(self)
}

func TestRFC3101TransitASBRReachabilityDecidesListing(t *testing.T) {
	// Goal: the second list condition, reachability as an ASBR over the transit topology.
	// Method: the higher-Router-ID border router is reached over the NSSA in every case; only
	// where an ASBR entry for it exists changes.
	nssa := types.AreaID{0, 0, 0, 5}
	// RFC requirement: RFC3101-3.1-5 positive -- a higher-Router-ID border router the NSSA's
	// SPF reaches but no area holds an ASBR entry for, or only the NSSA itself does, is not on
	// the list, so the candidate is elected and translates.
	assert.Equal(t, 1, transitElectionCase(t), "a border router no area reaches as an ASBR is not listed")
	assert.Equal(t, 1, transitElectionCase(t, nssa), "an ASBR entry in the NSSA is not reachability over the transit topology")
	// RFC requirement: RFC3101-3.1-5 negative -- the same router with an ASBR entry in the
	// backbone is on the list, and its higher Router ID leaves the candidate translating nothing.
	assert.Equal(t, 0, transitElectionCase(t, types.BackboneArea), "a border router reached as an ASBR in the backbone is listed")
}

func TestRFC3101ElectionRuleDisablesCandidate(t *testing.T) {
	// Goal: both halves of the election rule, Nt set or a higher Router ID. Method: the
	// candidate 10.0.6.5 shares the NSSA with one other border router, reached over the NSSA
	// and as an ASBR: lower Router ID with the Nt-bit set, lower with it clear, and higher
	// with it clear (such as a router configured `translate never`).
	translated := func(other string, nt bool) int {
		eng, nssa := nssaTransEngine(t, "10.0.6.5")
		self := ridOf("10.0.6.5")
		eng.lsdb.OriginateRouter(ospflsdb.OriginInput{AreaID: nssa, RouterID: ridOf(other), ABR: true, NSSATranslator: nt})
		eng.lsdb.OriginateNSSA(nssa, ridOf("10.0.6.2"), ip4Of("10.27.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)
		eng.translateNSSA(transTime)
		return eng.lsdb.SelfExternalCount(self)
	}
	// RFC requirement: RFC3101-3.1-4 positive -- a listed lower-Router-ID border router whose
	// router-LSA has the Nt-bit set disables the candidate, which translates nothing.
	assert.Equal(t, 0, translated("10.0.6.1", true), "a listed border router with the Nt-bit set disables the candidate")
	// RFC requirement: RFC3101-3.1-4 positive -- a listed higher-Router-ID border router whose
	// router-LSA has the Nt-bit clear disables the candidate, which translates nothing.
	assert.Equal(t, 0, translated("10.0.6.9", false), "a listed higher-Router-ID border router disables the candidate")
	// RFC requirement: RFC3101-3.1-4 negative -- when the only other listed border router has a
	// lower Router ID and the Nt-bit clear, the candidate is elected and translates.
	assert.Equal(t, 1, translated("10.0.6.1", false), "a lower-Router-ID border router with the Nt-bit clear leaves the candidate elected")
}

func TestRFC3101OnlyAlwaysRoleSetsNtBit(t *testing.T) {
	// Goal: the Nt-bit means NSSATranslatorState enabled, which only the Always role reaches.
	// Method: ask the decision for an NSSA with each role, and for a regular area.
	nssa := func(role string) areaConfig {
		return areaConfig{AreaID: types.AreaID{0, 0, 0, 5}, AreaType: types.AreaTypeNSSA, NSSATranslateRole: role}
	}
	assert.True(t, nssaTranslatorEnabled(nssa(translateRoleAlways)), "an Always border router sets the Nt-bit")
	assert.False(t, nssaTranslatorEnabled(nssa(translateRoleCandidate)), "a candidate leaves the Nt-bit clear, elected or not")
	assert.False(t, nssaTranslatorEnabled(nssa(translateRoleNever)), "a never border router leaves the Nt-bit clear")
	assert.False(t, nssaTranslatorEnabled(areaConfig{AreaID: types.BackboneArea, AreaType: types.AreaTypeNormal, NSSATranslateRole: translateRoleAlways}),
		"a regular area carries no Nt-bit")
}
