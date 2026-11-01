// Design: docs/architecture/core-design.md -- registered stage ownership of policy references.

package filter

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestBindLoopDetection confines the engine's ownership of a chain member to
// the loop-detection type and its plain, type-qualified or canonical name.
// Method: bind each spelling, then read the binding back through the policy
// the reactor runs, so a binding the policy does not treat as in-process
// fails here. Other owners and other types stay external and unchanged.
func TestBindLoopDetection(t *testing.T) {
	for _, name := range []string{"CHECK", "loop-detection:CHECK", "loop:CHECK"} {
		for _, inactive := range []bool{false, true} {
			ref, bound := BindLoopDetection(routepolicy.Ref{Name: name, Inactive: inactive}, loopFilterType)
			bindings := []bool{bound}
			owned := filterapi.InternalPolicy(nil).WithBoundFilters([]routepolicy.Ref{ref}, bindings)
			if ref.Name != "loop:CHECK" || !owned.InProcess(0) || owned.ActiveExternal(0) || ref.Inactive != inactive {
				t.Fatalf("binding %q: %+v", name, ref)
			}
		}
	}
	ref, bound := BindLoopDetection(routepolicy.Ref{Name: "loop"}, loopFilterType)
	if ref.Name != "loop:loop" || !bound {
		t.Fatalf("plain instance matching its owner's name = %+v", ref)
	}
	for _, tc := range []struct{ name, policyType string }{
		{"external:CHECK", loopFilterType},
		{"loop:CHECK", "other-policy"},
		{"CHECK", ""},
	} {
		ref, bound := BindLoopDetection(routepolicy.Ref{Name: tc.name}, tc.policyType)
		unowned := filterapi.InternalPolicy(nil).WithBoundFilters([]routepolicy.Ref{ref}, []bool{bound})
		if unowned.InProcess(0) || !unowned.ActiveExternal(0) || ref.Name != tc.name {
			t.Fatalf("unowned reference bound: %+v", ref)
		}
	}
}

// TestOwnsChainMembers answers config load's question "does this compiled-in
// plugin serve a chain member": the loop engine does, and no other name.
func TestOwnsChainMembers(t *testing.T) {
	if !OwnsChainMembers(LoopName) {
		t.Fatal("loop does not own its loop-detection chain members")
	}
	for _, name := range []string{"", "bgp-rs", "loop-detection"} {
		if OwnsChainMembers(name) {
			t.Fatalf("%q owns loop-detection chain members", name)
		}
	}
}
