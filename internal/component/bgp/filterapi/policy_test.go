// Design: docs/architecture/core-design.md — validated policy snapshots.

package filterapi

import "testing"

// TestPolicyConstruction proves only named finals create configured policies,
// while zero/internal values retain absence and the IBGP acceptance contract.
func TestPolicyConstruction(t *testing.T) {
	for _, final := range []FinalDisposition{0, 3, 255} {
		policy, err := NewPolicy(nil, final)
		if err == nil || policy.Configured() {
			t.Fatalf("NewPolicy(%d) = %+v, %v; want refusal and absent value", final, policy, err)
		}
	}
	for _, final := range []FinalDisposition{FinalAccept, FinalReject} {
		policy, err := NewPolicy(nil, final)
		if err != nil {
			t.Fatal(err)
		}
		if !policy.Configured() || policy.Final() != final || policy.AcceptsAtEnd(true) != (final == FinalAccept) {
			t.Fatalf("explicit empty policy lost final %d: %+v", final, policy)
		}
		refs := []FilterRef{{Name: "test:one", Inactive: true}}
		replaced := policy.WithFilters(refs)
		if len(policy.Filters()) != 0 || replaced.Final() != final || &replaced.Filters()[0] != &refs[0] {
			t.Fatal("WithFilters changed the original, lost final, or copied borrowed refs")
		}
	}
	for _, policy := range []Policy{{}, InternalPolicy([]FilterRef{{Name: "automatic"}})} {
		if policy.Configured() || policy.Final() != 0 || !policy.AcceptsAtEnd(false) || policy.AcceptsAtEnd(true) {
			t.Fatalf("absent operator policy changed behavior: %+v", policy)
		}
	}
}

// TestPolicyVerdictAtExhaustion checks every operator final against both known
// session types, including automatic and inactive references that imply no policy.
func TestPolicyVerdictAtExhaustion(t *testing.T) {
	for _, refs := range [][]FilterRef{nil, {{Name: "automatic"}}, {{Name: "off", Inactive: true}}} {
		for _, final := range []FinalDisposition{0, FinalAccept, FinalReject} {
			policy := InternalPolicy(refs)
			if final != 0 {
				var err error
				policy, err = NewPolicy(refs, final)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, external := range []bool{false, true} {
				want := final == FinalAccept || final == 0 && !external
				if got := policy.AcceptsAtEnd(external); got != want {
					t.Errorf("final=%d external=%v refs=%v: accept=%v want=%v", final, external, refs, got, want)
				}
			}
		}
	}
}
