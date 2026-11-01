package filter_aspath

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// TestMatchConditionNamingNoASPathListIsRefused proves a match instance whose
// as-path condition names something that is not a as-path-list instance is refused
// when its chain compiles, with the resolver's message naming the reference.
// It replaces the fail-closed arm of the removed filter-update handler (a
// request naming an unknown list was rejected): a reference no definition
// backs never reaches a running chain.
// Method: compile a one-member chain naming match "m" whose condition names
// (a) "MISSING", which no instance holds, and (b) "m" itself, an instance of
// type match; each compile must fail with the exact refusal.
//
// MUTATION: dropping either check in registryResolver
// (internal/component/bgp/policychain/chain.go) turns the case red.
func TestMatchConditionNamingNoASPathListIsRefused(t *testing.T) {
	for name, tc := range map[string]struct{ ref, want string }{
		"undefined": {"MISSING",
			`"MISSING" is not a as-path-list defined under the policy root; define policy as-path-list MISSING, or name an existing one`},
		"another type": {"m", `"m" is a match, not a as-path-list; name a as-path-list defined under the policy root`},
	} {
		t.Run(name, func(t *testing.T) {
			reg, err := policy.NewRegistry(policy.Instance{Name: "m", Type: policy.MatchFilterType,
				Config: map[string]any{policy.FieldASPath.String(): tc.ref}})
			if err != nil {
				t.Fatalf("registry: %v", err)
			}
			_, err = policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
			if err == nil {
				t.Fatalf("compile accepted the as-path condition naming %q", tc.ref)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("compile error = %q, want it to hold %q", err, tc.want)
			}
		})
	}
}
