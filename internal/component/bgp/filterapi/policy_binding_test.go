// Design: docs/architecture/core-design.md -- registered stage ownership of policy references.

package filterapi

import "testing"

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
			ref := BindInProcess(FilterRef{Name: name, Inactive: inactive}, "protocol-policy")
			if ref.Name != "protocol-owner:CHECK" || !ref.InProcess() || ref.ActiveExternal() || ref.Inactive != inactive {
				t.Fatalf("binding %q: %+v", name, ref)
			}
			policy := InternalPolicy([]FilterRef{ref}).WithFilters([]FilterRef{ref})
			if policy.Final() != 0 || policy.Configured() || !policy.Filters()[0].InProcess() {
				t.Fatalf("policy copy lost absence or ownership: %+v", policy)
			}
		}
	}
	ref := BindInProcess(FilterRef{Name: "protocol-owner"}, "protocol-policy")
	if ref.Name != "protocol-owner:protocol-owner" || !ref.InProcess() {
		t.Fatalf("plain instance matching its owner's name = %+v", ref)
	}
	for _, tc := range []struct{ name, policyType string }{
		{"external:CHECK", "protocol-policy"},
		{"protocol-owner:CHECK", "other-policy"},
		{"CHECK", ""},
	} {
		ref := BindInProcess(FilterRef{Name: tc.name}, tc.policyType)
		if ref.InProcess() || !ref.ActiveExternal() || ref.Name != tc.name {
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
	ref := BindInProcess(FilterRef{Name: "CHECK"}, "check")
	if !ref.InProcess() {
		t.Fatal("registered ingress owner did not bind")
	}
	inactive := ref
	inactive.Inactive = true
	var absent Policy
	if absent.RequiresIngress() {
		t.Fatal("zero policy requires ingress")
	}
	for _, final := range []FinalDisposition{0, FinalAccept, FinalReject} {
		refs := []FilterRef{{Name: "external:first"}, ref}
		policy := InternalPolicy(refs)
		if final != 0 {
			var err error
			policy, err = NewPolicy(refs, final)
			if err != nil {
				t.Fatal(err)
			}
		}
		if !policy.RequiresIngress() || policy.Final() != final || &policy.Filters()[0] != &refs[0] {
			t.Fatalf("constructor lost ownership, final or borrowed refs: %+v", policy)
		}
		for _, replacement := range [][]FilterRef{nil, {inactive}, {{Name: "external:only"}}} {
			replaced := policy.WithFilters(replacement)
			if replaced.RequiresIngress() || replaced.Final() != final || replaced.AcceptsAtEnd(true) != policy.AcceptsAtEnd(true) {
				t.Fatalf("replacement retained ownership or changed disposition: %+v", replaced)
			}
			if !policy.RequiresIngress() || len(policy.Filters()) != len(refs) {
				t.Fatalf("replacement mutated its source: %+v", policy)
			}
			restored := replaced.WithFilters(refs)
			if !restored.RequiresIngress() || restored.Final() != final {
				t.Fatalf("replacement did not restore ownership: %+v", restored)
			}
		}
	}
}
