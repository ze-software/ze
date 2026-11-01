//go:build !race

package filter_path_asn

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
)

// TestBGPMatchRejectASNAllocatesNothing proves the reject-asn condition
// judges an UPDATE, both arms included, with no heap allocation once the
// view's text buffer is warm. It is built without the race detector, whose
// sync.Pool drops the regexp matchers the pattern arm reuses.
func TestBGPMatchRejectASNAllocatesNothing(t *testing.T) {
	chain := rejectASNChain(t)
	update := pathUpdate(t, 65001, 65002, 65003)
	session := policyview.Session{EBGP: true, Sender: 65001, SenderKnown: true}
	var view policyview.View
	sets := new(policychain.RunSets)
	allocs := testing.AllocsPerRun(100, func() {
		view.Reset(update, session)
		if outcome := chain.Run(&view, sets); outcome.Action != policychain.OutcomeAccept {
			t.Fatalf("outcome %+v, want accept", outcome)
		}
		view.Clear()
	})
	if allocs != 0 {
		t.Fatalf("%v allocations per UPDATE, want 0", allocs)
	}
}
