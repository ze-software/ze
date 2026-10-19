// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- NSSA Type-7 translator election.
// Related: nssa.go -- electNSSATranslator, the election over the reachable list.
// Related: rfc3101_nssa_test.go -- the election tests that build the list from bare router-LSAs.
//
// VALIDATES: RFC 3101 Section 3.1 list membership: a candidate translator elects over the
// NSSA's border routers "that are reachable both over the NSSA", so a router-LSA in the
// NSSA whose originator the NSSA's SPF does not reach is not a member and cannot disable
// the candidate, while the same router reached by the SPF does.
// PREVENTS: a stale or partitioned higher-Router-ID border router's router-LSA switching
// translation off for the whole NSSA.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// translatorListCase builds a candidate border router 10.0.6.1 whose NSSA holds a
// higher-Router-ID border router 10.0.6.9 (B and Nt set) and one translatable Type-7, lets
// the SPF view reach the routers in reached, runs the translation, and returns how many
// Type-5s the candidate originated.
func translatorListCase(t *testing.T, reached ...types.RouterID) int {
	t.Helper()
	eng, nssa := nssaTransEngine(t, "10.0.6.1")
	self := ridOf("10.0.6.1")
	view := fixedReachability{reached: map[types.RouterID]bool{self: true}}
	for _, r := range reached {
		view.reached[r] = true
	}
	eng.nssaReachabilityFn = func() routerReachability { return view }

	eng.lsdb.OriginateRouter(ospflsdb.OriginInput{AreaID: nssa, RouterID: ridOf("10.0.6.9"), ABR: true, NSSATranslator: true})
	eng.lsdb.OriginateNSSA(nssa, ridOf("10.0.6.2"), ip4Of("10.25.0.0"), ip4Of("255.255.0.0"), false, 10, ip4Of("10.5.0.2"), 0, true)
	eng.translateNSSA(transTime)
	return eng.lsdb.SelfExternalCount(self)
}

func TestRFC3101UnreachableBorderRouterIsNotListed(t *testing.T) {
	// Goal: list membership needs reachability over the NSSA. Method: the higher-Router-ID
	// border router's router-LSA is in the NSSA, but the SPF view does not reach it.
	// RFC requirement: RFC3101-3.1-2 positive -- a border router the NSSA's SPF does not
	// reach is not on the list, so the candidate is elected and translates.
	assert.Equal(t, 1, translatorListCase(t),
		"an unreachable border router is not on the section 3.1 list, so the candidate is elected and translates")
}

func TestRFC3101ReachableBorderRouterIsListed(t *testing.T) {
	// Goal: the same router, reached by the NSSA's SPF, is a list member. Method: identical
	// topology with the SPF view reaching 10.0.6.9.
	// RFC requirement: RFC3101-3.1-2 negative -- a reachable higher-Router-ID border router
	// is on the list, so the candidate is disabled and translates nothing.
	assert.Equal(t, 0, translatorListCase(t, ridOf("10.0.6.9")),
		"a reachable higher-Router-ID border router is on the list and disables the candidate")
}
