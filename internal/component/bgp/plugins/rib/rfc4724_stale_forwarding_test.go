// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// RFC: rfc/short/rfc4724.md -- Section 4.2, stale routes are not differentiated in forwarding
// Overview: bestpath.go -- SelectBest and comparePair, the best path the forwarding plane is given
// Related: bestpath_test.go -- TestComparePair_GRStaleCompetesNormally, the positive

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone puts a route retained as stale by
// Graceful Restart (stale level 1) against a fresh route in the two shapes where a selection
// that looked at staleness would answer differently from one that did not.
//
// VALIDATES: RFC 4724 Section 4.2 -- "The router MUST NOT differentiate between stale and
// other routing information during forwarding." The best path SelectBest returns is the
// route the forwarding plane uses, and staleness neither favors a route nor costs it a
// tie-break: a fresh route with a higher LOCAL_PREF wins, and with every attribute equal the
// stale route from the lower peer address wins exactly as a fresh one would.
// PREVENTS: a selection that keeps the stale route installed because it was there first, and
// one that drops a stale route on a tie it would win when fresh.
//
// RFC requirement: RFC4724-4.2-5 negative -- SelectBest given a level-1 stale route and a fresh route picks the fresh one when it has the higher LOCAL_PREF, and picks the stale one from 10.0.0.1 over the fresh one from 10.0.0.2 when every other attribute is equal, in both argument orders.
func TestRFC4724GRStaleRouteIsDecidedByItsAttributesAlone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		stale *Candidate
		fresh *Candidate
		want  string
	}{
		{
			"fresh route has the higher LOCAL_PREF",
			&Candidate{PeerAddr: "10.0.0.1", PeerIP: netip.MustParseAddr("10.0.0.1"), LocalPref: 100, StaleLevel: 1},
			&Candidate{PeerAddr: "10.0.0.2", PeerIP: netip.MustParseAddr("10.0.0.2"), LocalPref: 200},
			"10.0.0.2",
		},
		{
			"every attribute equal, stale route from the lower peer address",
			&Candidate{PeerAddr: "10.0.0.1", PeerIP: netip.MustParseAddr("10.0.0.1"), LocalPref: 100, StaleLevel: 1},
			&Candidate{PeerAddr: "10.0.0.2", PeerIP: netip.MustParseAddr("10.0.0.2"), LocalPref: 100},
			"10.0.0.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, SelectBest([]*Candidate{tc.stale, tc.fresh}).PeerAddr,
				"the attributes decide, stale first")
			assert.Equal(t, tc.want, SelectBest([]*Candidate{tc.fresh, tc.stale}).PeerAddr,
				"the attributes decide, fresh first")
		})
	}
}
