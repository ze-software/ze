// Design: docs/architecture/core-design.md -- registered stage ownership of policy references.

package filterapi

import (
	"testing"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestBindInProcess confines stage ownership to the registered policy type and
// its plain, type-qualified or canonical name. Other process names stay guards.
func TestBindInProcess(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	ResetForTest()
	if err := Register(Filter{Name: "protocol-owner", PolicyType: "protocol-policy", Ingress: noopIngress, Stage: FilterStageProtocol}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CHECK", "protocol-policy:CHECK", "protocol-owner:CHECK"} {
		for _, inactive := range []bool{false, true} {
			ref, bound := BindInProcess(routepolicy.Ref{Name: name, Inactive: inactive}, "protocol-policy")
			bindings := []bool{bound}
			owned := InternalPolicy(nil).WithBoundFilters([]routepolicy.Ref{ref}, bindings)
			if ref.Name != "protocol-owner:CHECK" || !owned.InProcess(0) || owned.ActiveExternal(0) || ref.Inactive != inactive {
				t.Fatalf("binding %q: %+v", name, ref)
			}
			policy := InternalPolicy([]routepolicy.Ref{ref}).WithBoundFilters([]routepolicy.Ref{ref}, bindings)
			if policy.Final() != 0 || policy.Configured() || !policy.InProcess(0) {
				t.Fatalf("policy copy lost absence or ownership: %+v", policy)
			}
		}
	}
	ref, bound := BindInProcess(routepolicy.Ref{Name: "protocol-owner"}, "protocol-policy")
	if ref.Name != "protocol-owner:protocol-owner" || !bound {
		t.Fatalf("plain instance matching its owner's name = %+v", ref)
	}
	for _, tc := range []struct{ name, policyType string }{
		{"external:CHECK", "protocol-policy"},
		{"protocol-owner:CHECK", "other-policy"},
		{"CHECK", ""},
	} {
		ref, bound := BindInProcess(routepolicy.Ref{Name: tc.name}, tc.policyType)
		unowned := InternalPolicy(nil).WithBoundFilters([]routepolicy.Ref{ref}, []bool{bound})
		if unowned.InProcess(0) || !unowned.ActiveExternal(0) || ref.Name != tc.name {
			t.Fatalf("unowned reference bound: %+v", ref)
		}
	}
}

// TestPolicyTypeRegistrationRefusesAmbiguity ensures map iteration cannot choose
// between two stage owners, or bind an instance to a registration with no hook.
func TestPolicyTypeRegistrationRefusesAmbiguity(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	ResetForTest()
	if err := Register(Filter{Name: "first", PolicyType: "check", Ingress: noopIngress}); err != nil {
		t.Fatal(err)
	}
	if err := Register(Filter{Name: "second", PolicyType: "check", Ingress: noopIngress}); err == nil {
		t.Fatal("duplicate policy owner registered")
	}
	if err := Register(Filter{Name: "no-hook", PolicyType: "other", PeerRemoved: func(PeerFilterInfo) {}}); err == nil {
		t.Fatal("named policy registered without an executable stage")
	}
}

// TestNamedPolicyRequiresIngressHook prevents named ownership from promising
// export execution that the originated-route rails do not run.
func TestNamedPolicyRequiresIngressHook(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	ResetForTest()
	if err := Register(Filter{Name: "egress", PolicyType: "egress-type", Egress: noopEgress}); err == nil {
		t.Fatal("named policy registered without the supported ingress execution")
	}
}

// TestPolicyIngressOwnership covers every policy constructor and reference
// replacement, including zero and inactive values that must clear the invariant.
func TestPolicyIngressOwnership(t *testing.T) {
	snapshot := Snapshot()
	t.Cleanup(func() { Restore(snapshot) })
	ResetForTest()
	if err := Register(Filter{Name: "owner", PolicyType: "check", Ingress: noopIngress}); err != nil {
		t.Fatal(err)
	}
	ref, bound := BindInProcess(routepolicy.Ref{Name: "CHECK"}, "check")
	if !bound {
		t.Fatal("registered ingress owner did not bind")
	}
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
