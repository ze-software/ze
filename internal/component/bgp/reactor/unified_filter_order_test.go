// Design: docs/architecture/core-design.md -- Ingress Filter Pipeline (section 9): unified stage-ordered filter pipeline
// Characterization + order tests for the merged ingress/egress passes. The
// cross-system behavioral outcome (policy chain runs after OTC on a live UPDATE)
// is guarded end-to-end by the filter .ci suite; these tests lock the ORDER of
// the built step lists, which the per-UPDATE passes iterate.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

func probeIngress(_ filterapi.PeerFilterInfo, _ []byte, _ map[string]any) (bool, []byte) {
	return true, nil
}

func probeEgress(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
	return true
}

func mustRegisterIngress(t *testing.T, name string, stage, priority int) {
	t.Helper()
	if err := filterapi.Register(filterapi.Filter{Name: name, Stage: stage, Priority: priority, Ingress: probeIngress}); err != nil {
		t.Fatal(err)
	}
}

func mustRegisterEgress(t *testing.T, name string, stage, priority int) {
	t.Helper()
	if err := filterapi.Register(filterapi.Filter{Name: name, Stage: stage, Priority: priority, Egress: probeEgress}); err != nil {
		t.Fatal(err)
	}
}

// orderedEgressStepsFromFuncs wraps in-process egress funcs as ordered steps for
// tests that build a Reactor by hand (bypassing startAPIServer, which normally
// builds orderedEgressSteps). It mirrors the in-process portion of
// buildOrderedEgressSteps; tests exercising forwarding mods (not the export policy
// chain) need only these steps.
func orderedEgressStepsFromFuncs(funcs ...filterapi.EgressFilterFunc) []orderedEgressStep {
	steps := make([]orderedEgressStep, 0, len(funcs))
	for i, f := range funcs {
		steps = append(steps, orderedEgressStep{
			name:     "test-egress",
			stage:    filterapi.FilterStagePolicy,
			priority: i,
			inproc:   f,
		})
	}
	return steps
}

func ingressStepNames(steps []orderedIngressStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.name
	}
	return out
}

func egressStepNames(steps []orderedEgressStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.name
	}
	return out
}

func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("order length = %d %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestUnifiedIngressReproducesLegacyOrder locks the corrected pre-refactor order:
// the whole in-process pass (loop -> community -> redistribute -> OTC) runs, and
// the external per-peer policy chain runs LAST -- after the Annotation-stage OTC
// filter, not before it. A regression that placed the policy chain at Policy stage
// (the original, wrong design) would move it ahead of OTC and fail here.
//
// VALIDATES: spec-unify-filters R-1 / AC-2.
// PREVENTS: reordering the external chain relative to OTC.
func TestUnifiedIngressReproducesLegacyOrder(t *testing.T) {
	snap := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snap) })
	filterapi.ResetForTest()

	// Mirror the real registered ingress filters' names and stages.
	mustRegisterIngress(t, "loop", filterapi.FilterStageProtocol, 0)
	mustRegisterIngress(t, "bgp-filter-community", filterapi.FilterStagePolicy, 0)
	mustRegisterIngress(t, "bgp-redistribute", filterapi.FilterStagePolicy, 0)
	mustRegisterIngress(t, "bgp-role", filterapi.FilterStageAnnotation, 0)

	steps := buildOrderedIngressSteps()
	assertOrder(t, ingressStepNames(steps), []string{
		"loop", "bgp-filter-community", "bgp-redistribute", "bgp-role", policyChainStepName,
	})

	// The last step must be the external policy chain, not an in-process filter.
	last := steps[len(steps)-1]
	if !last.policyChain {
		t.Fatalf("last step %q is not the policy chain", last.name)
	}
	if last.inproc != nil {
		t.Fatal("policy chain step must not carry an in-process func")
	}
	// Exactly one policy-chain step, and it sorts after the Annotation-stage OTC.
	otcIdx, chainIdx, chainCount := -1, -1, 0
	for i, s := range steps {
		if s.name == "bgp-role" {
			otcIdx = i
		}
		if s.policyChain {
			chainIdx = i
			chainCount++
		}
	}
	if chainCount != 1 {
		t.Fatalf("expected exactly one policy-chain step, got %d", chainCount)
	}
	if otcIdx < 0 || chainIdx <= otcIdx {
		t.Fatalf("policy chain (idx %d) must run after OTC (idx %d)", chainIdx, otcIdx)
	}
}

// TestUnifiedIngressOrder verifies the merged pass honors Stage, then Priority,
// then name, and that a filter registered at any stage below FilterStagePeerChain
// still interleaves before the policy chain (AC-1).
func TestUnifiedIngressOrder(t *testing.T) {
	snap := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snap) })
	filterapi.ResetForTest()

	// Two Policy-stage filters differing only by priority, plus a late stage
	// between Annotation and PeerChain.
	mustRegisterIngress(t, "community", filterapi.FilterStagePolicy, 0)
	mustRegisterIngress(t, "prefix", filterapi.FilterStagePolicy, 10)
	mustRegisterIngress(t, "loop", filterapi.FilterStageProtocol, 0)
	mustRegisterIngress(t, "late", filterapi.FilterStageAnnotation+50, 0)

	steps := buildOrderedIngressSteps()
	assertOrder(t, ingressStepNames(steps), []string{
		"loop", "community", "prefix", "late", policyChainStepName,
	})
}

// TestUnifiedEgressOrder verifies the egress builder mirrors the registered egress
// order (community -> gr -> role) and appends the export policy chain last (AC-5).
func TestUnifiedEgressOrder(t *testing.T) {
	snap := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snap) })
	filterapi.ResetForTest()

	mustRegisterEgress(t, "bgp-filter-community", filterapi.FilterStagePolicy, 0)
	mustRegisterEgress(t, "bgp-gr", filterapi.FilterStageAnnotation, 0)
	mustRegisterEgress(t, "bgp-role", filterapi.FilterStageAnnotation, 0)

	steps := buildOrderedEgressSteps()
	assertOrder(t, egressStepNames(steps), []string{
		"bgp-filter-community", "bgp-gr", "bgp-role", policyChainStepName,
	})
	last := steps[len(steps)-1]
	if !last.policyChain || last.inproc != nil {
		t.Fatalf("last egress step must be the policy chain, got name=%q policyChain=%v", last.name, last.policyChain)
	}
}

// TestOrderedStepsEmptyRegistry verifies that with no in-process filters the pass
// is just the policy-chain step (a no-op accept at runtime when a peer has no
// configured filters), so the pipeline is never empty and needs no nil guard.
func TestOrderedStepsEmptyRegistry(t *testing.T) {
	snap := filterapi.Snapshot()
	t.Cleanup(func() { filterapi.Restore(snap) })
	filterapi.ResetForTest()

	ingress := buildOrderedIngressSteps()
	if len(ingress) != 1 || !ingress[0].policyChain {
		t.Fatalf("empty registry: ingress steps = %v, want single policy-chain step", ingressStepNames(ingress))
	}
	egress := buildOrderedEgressSteps()
	if len(egress) != 1 || !egress[0].policyChain {
		t.Fatalf("empty registry: egress steps = %v, want single policy-chain step", egressStepNames(egress))
	}
}
