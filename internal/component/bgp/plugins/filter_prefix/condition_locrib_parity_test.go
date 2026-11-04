// Design: docs/guide/route-policy.md -- prefix-list conditions
package filter_prefix

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestPrefixListConditionMatchesLocRIB proves the one prefix-list condition
// gives the same single-prefix verdict as the Loc-RIB evaluator. Independent
// expected decisions cover family, subnet and ge/le boundaries, including an
// unmatched prefix: neither side can pass by copying the other's mistake.
func TestPrefixListConditionMatchesLocRIB(t *testing.T) {
	config := map[string]any{"entry": map[string]any{
		"10.0.0.0/8": map[string]any{"ge": "16", "le": "24", "action": "accept"}}}
	evaluate, err := compileRouteEvaluator("protected", config, nil)
	if err != nil {
		t.Fatal(err)
	}
	chain := matchChain(t)
	for _, tc := range []struct {
		prefix string
		reject bool
	}{
		{"10.0.0.0/8", true},
		{"10.0.0.0/15", true},
		{"10.1.0.0/16", false},
		{"10.1.2.0/24", false},
		{"10.1.2.0/25", true},
		{"192.0.2.0/24", true},
		{"2001:db8::/32", true},
	} {
		t.Run(tc.prefix, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tc.prefix)
			local := evaluate(ribpolicy.Route{Prefix: prefix})
			address := prefix.Addr().AsSlice()
			nlri := append([]byte{byte(prefix.Bits())}, address[:(prefix.Bits()+7)/8]...)
			var attrs []byte
			if prefix.Addr().Is6() {
				nextHop := netip.MustParseAddr("2001:db8::1").As16()
				attrs = mpReach(2, 1, nextHop[:], nlri)
				nlri = nil
			}
			var view policyview.View
			view.Reset(bgpUpdate(t, nil, attrs, nlri), policyview.Session{})
			defer view.Clear()
			var sets policychain.RunSets
			outcome := chain.Run(&view, &sets)
			if outcome.Failed {
				t.Fatalf("BGP condition failed instead of judging %s", prefix)
			}
			bgpReject := outcome.Action == policychain.OutcomeReject
			if local.Reject != tc.reject {
				t.Errorf("Loc-RIB reject = %v, want %v", local.Reject, tc.reject)
			}
			if bgpReject != tc.reject {
				t.Errorf("BGP reject = %v, want %v", bgpReject, tc.reject)
			}
		})
	}
}
