package filter_path_asn

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/policy"
)

// TestMatchConditionNamingNoRejectASNIsRefused proves a match instance whose
// reject-asn condition names something that is not a reject-asn instance is refused
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
func TestMatchConditionNamingNoRejectASNIsRefused(t *testing.T) {
	for name, tc := range map[string]struct{ ref, want string }{
		"undefined": {"MISSING",
			`"MISSING" is not a reject-asn defined under the policy root; define policy reject-asn MISSING, or name an existing one`},
		"another type": {"m", `"m" is a match, not a reject-asn; name a reject-asn defined under the policy root`},
	} {
		t.Run(name, func(t *testing.T) {
			reg, err := policy.NewRegistry(policy.Instance{Name: "m", Type: policy.MatchFilterType,
				Config: map[string]any{filterTypeName: tc.ref}})
			if err != nil {
				t.Fatalf("registry: %v", err)
			}
			_, err = policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
			if err == nil {
				t.Fatalf("compile accepted the reject-asn condition naming %q", tc.ref)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("compile error = %q, want it to hold %q", err, tc.want)
			}
		})
	}
}
