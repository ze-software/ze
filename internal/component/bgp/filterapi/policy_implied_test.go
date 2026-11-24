// Design: docs/architecture/core-design.md -- Policy.Append and the implied-filter provenance seam.
// Related: policy.go -- immutable Policy snapshots and stage bindings.

package filterapi

import (
	"slices"
	"testing"

	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

type stubLibraryChain struct{ id int }

func (s stubLibraryChain) Refused() uint64 { return 0 }
func (s stubLibraryChain) SameDecisions(other LibraryChain) bool {
	o, ok := other.(stubLibraryChain)
	return ok && s.id == o.id
}

type stubTemplatedChain struct{ id int }

func (s stubTemplatedChain) CompileResolved(resolved Policy, _ PeerFilterInfo) (Policy, error) {
	return resolved, nil
}

// TestPolicyAppendPreservesDispositionAndBindings verifies Policy.Append mirrors
// Policy.Prepend at the tail of the chain.
//
// VALIDATES: Append keeps Configured() == false on unconfigured policies (RFC
// 8212 default-deny), keeps Final() on configured policies, preserves existing
// stage bindings and RequiresIngress, and never mutates the receiver's slices.
// PREVENTS: auto-appending next-hop:auto marking an absent export policy as
// configured or dropping existing loop-detection bindings.
func TestPolicyAppendPreservesDispositionAndBindings(t *testing.T) {
	autoRef := routepolicy.Ref{Name: "next-hop:auto"}

	var absent Policy
	appended := absent.Append(autoRef, false)
	if appended.Configured() {
		t.Fatal("Append on absent Policy set Configured() == true")
	}
	if appended.Final() != 0 {
		t.Fatalf("Append on absent Policy set Final() = %d, want 0", appended.Final())
	}
	if appended.AcceptsAtEnd(true) {
		t.Fatal("Append on absent Policy accepted eBGP route; RFC 8212 requires reject")
	}
	if !appended.AcceptsAtEnd(false) {
		t.Fatal("Append on absent Policy rejected internal route")
	}
	if appended.Bindings() != nil {
		t.Fatalf("Append(false) on unbound policy allocated bindings %v, want nil", appended.Bindings())
	}
	if !slices.Equal(appended.Filters(), []routepolicy.Ref{autoRef}) {
		t.Fatalf("Filters = %v, want [%v]", appended.Filters(), autoRef)
	}
	if len(absent.Filters()) != 0 {
		t.Fatal("Append mutated receiver Filters")
	}

	configured, err := NewPolicy([]routepolicy.Ref{{Name: "modify-1"}}, routepolicy.FinalAccept)
	if err != nil {
		t.Fatal(err)
	}
	bound := configured.Prepend(routepolicy.Ref{Name: "loop:default"}, true)
	tail := bound.Append(autoRef, false)
	if !tail.Configured() || tail.Final() != routepolicy.FinalAccept {
		t.Fatalf("Append changed configured disposition: configured=%v final=%d", tail.Configured(), tail.Final())
	}
	if !tail.RequiresIngress() {
		t.Fatal("Append lost RequiresIngress from bound head filter")
	}
	wantRefs := []routepolicy.Ref{{Name: "loop:default"}, {Name: "modify-1"}, autoRef}
	if !slices.Equal(tail.Filters(), wantRefs) {
		t.Fatalf("Filters = %v, want %v", tail.Filters(), wantRefs)
	}
	wantBindings := []bool{true, false, false}
	if !slices.Equal(tail.Bindings(), wantBindings) {
		t.Fatalf("Bindings = %v, want %v", tail.Bindings(), wantBindings)
	}
	if len(bound.Filters()) != 2 || len(bound.Bindings()) != 2 {
		t.Fatal("Append mutated receiver slices")
	}

	inProcTail := configured.Append(routepolicy.Ref{Name: "loop:tail"}, true)
	if !inProcTail.RequiresIngress() || !slices.Equal(inProcTail.Bindings(), []bool{false, true}) {
		t.Fatalf("Append(true) bindings=%v requiresIngress=%v, want [false true], true",
			inProcTail.Bindings(), inProcTail.RequiresIngress())
	}
}

// TestPolicyImpliedSeamImmutabilityAndPreservation checks the pure-value
// implied-filter provenance seam across every Policy builder method.
//
// VALIDATES: WithImplied and WithImpliedList copy storage without mutating
// callers or receivers, and Implied entries survive WithFilters,
// WithBoundFilters, WithLibrary, WithTemplated, Prepend, and Append.
// PREVENTS: losing implied-filter metadata when bgp/config attaches bindings,
// libraries or templates to a peer's policy.
func TestPolicyImpliedSeamImmutabilityAndPreservation(t *testing.T) {
	head := ImpliedFilter{
		Direction: "import",
		Position:  ImpliedPositionHead,
		Ref:       "loop:default",
		ImpliedBy: "default BGP loop detection",
	}
	tail := ImpliedFilter{
		Direction: "export",
		Position:  ImpliedPositionTail,
		Ref:       "next-hop:auto",
		ImpliedBy: "RFC 4271 Section 5.1.3 default next-hop",
	}

	for _, tc := range []struct {
		pos  ImpliedPosition
		want string
	}{
		{ImpliedPositionUnspecified, "unspecified"},
		{ImpliedPositionHead, "head"},
		{ImpliedPositionTail, "tail"},
		{ImpliedPosition(99), "unknown"},
	} {
		if got := tc.pos.String(); got != tc.want {
			t.Errorf("ImpliedPosition(%d).String() = %q, want %q", tc.pos, got, tc.want)
		}
	}

	var base Policy
	if base.Implied() != nil {
		t.Fatalf("zero Policy Implied = %v, want nil", base.Implied())
	}
	one := base.WithImplied(head)
	two := one.WithImplied(tail)
	if len(base.Implied()) != 0 {
		t.Fatal("WithImplied mutated zero receiver")
	}
	if !slices.Equal(one.Implied(), []ImpliedFilter{head}) {
		t.Fatalf("one.Implied() = %v, want [%v]", one.Implied(), head)
	}
	if !slices.Equal(two.Implied(), []ImpliedFilter{head, tail}) {
		t.Fatalf("two.Implied() = %v, want [%v %v]", two.Implied(), head, tail)
	}

	// WithImpliedList clones the caller slice and normalizes empty to nil.
	src := []ImpliedFilter{head, tail}
	fromList := base.WithImpliedList(src)
	src[0].Ref = "mutated"
	if fromList.Implied()[0].Ref != head.Ref {
		t.Fatal("WithImpliedList aliased caller slice")
	}
	if cleared := fromList.WithImpliedList(nil); cleared.Implied() != nil {
		t.Fatalf("WithImpliedList(nil) = %v, want nil", cleared.Implied())
	}
	if cleared := fromList.WithImpliedList([]ImpliedFilter{}); cleared.Implied() != nil {
		t.Fatalf("WithImpliedList(empty) = %v, want nil", cleared.Implied())
	}

	// Every Policy derivation preserves Implied entries.
	want := []ImpliedFilter{head, tail}
	derived := fromList.
		WithFilters([]routepolicy.Ref{{Name: "a"}}).
		WithBoundFilters([]routepolicy.Ref{{Name: "a"}}, []bool{false}).
		WithLibrary(stubLibraryChain{id: 1}).
		WithTemplated(stubTemplatedChain{id: 2}).
		Prepend(routepolicy.Ref{Name: "loop:default"}, true).
		Append(routepolicy.Ref{Name: "next-hop:auto"}, false)
	if !slices.Equal(derived.Implied(), want) {
		t.Fatalf("derived.Implied() = %v, want %v", derived.Implied(), want)
	}
}
