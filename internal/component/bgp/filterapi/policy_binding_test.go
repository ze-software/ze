// Design: docs/architecture/core-design.md -- registered stage ownership of policy references.

package filterapi

import (
	"testing"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestPolicyIngressOwnership covers every policy constructor and reference
// replacement, including zero and inactive values that must clear the invariant.
func TestPolicyIngressOwnership(t *testing.T) {
	// The bound ref the engine's loop binding answers (reactor/filter
	// BindLoopDetection, tested there); this test owns what a policy does
	// with a binding, not how one is granted.
	ref := routepolicy.Ref{Name: "loop:CHECK"}
	inactive := ref
	inactive.Inactive = true
	var absent Policy
	if absent.RequiresIngress() {
		t.Fatal("zero policy requires ingress")
	}
	for _, final := range []routepolicy.Final{0, routepolicy.FinalAccept, routepolicy.FinalReject} {
		refs := []routepolicy.Ref{{Name: "external:first"}, ref}
		bindings := []bool{false, true}
		policy := InternalPolicy(nil).WithBoundFilters(refs, bindings)
		if final != 0 {
			configured, err := NewPolicy(nil, final)
			if err != nil {
				t.Fatal(err)
			}
			policy = configured.WithBoundFilters(refs, bindings)
		}
		if !policy.RequiresIngress() || policy.Final() != final || &policy.Filters()[0] != &refs[0] {
			t.Fatalf("constructor lost ownership, final or borrowed refs: %+v", policy)
		}
		for _, replacement := range []struct {
			refs     []routepolicy.Ref
			bindings []bool
		}{{nil, nil}, {[]routepolicy.Ref{inactive}, []bool{true}}, {[]routepolicy.Ref{{Name: "external:only"}}, nil}} {
			replaced := policy.WithBoundFilters(replacement.refs, replacement.bindings)
			if replaced.RequiresIngress() || replaced.Final() != final || replaced.AcceptsAtEnd(true) != policy.AcceptsAtEnd(true) {
				t.Fatalf("replacement retained ownership or changed disposition: %+v", replaced)
			}
			if !policy.RequiresIngress() || len(policy.Filters()) != len(refs) {
				t.Fatalf("replacement mutated its source: %+v", policy)
			}
			restored := replaced.WithBoundFilters(refs, bindings)
			if !restored.RequiresIngress() || restored.Final() != final {
				t.Fatalf("replacement did not restore ownership: %+v", restored)
			}
		}
	}
}
