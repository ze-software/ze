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

// TestOrderedStepsAreTheEngineSteps verifies the built pass is the engine's
// own steps and nothing else, every plugin filter being a chain member: on ingress the loop step and the policy-chain
// step, on egress the policy-chain step (a no-op accept at runtime when a peer
// has no configured filters), so the pipeline is never empty and needs no nil
// guard.
func TestOrderedStepsAreTheEngineSteps(t *testing.T) {
	ingress := buildOrderedIngressSteps()
	if len(ingress) != 2 || ingress[0].name != "loop" || ingress[0].policyChain || !ingress[1].policyChain {
		t.Fatalf("ingress steps = %v, want the loop step then the policy-chain step", ingressStepNames(ingress))
	}
	egress := buildOrderedEgressSteps()
	if len(egress) != 1 || egress[0].name != policyChainStepName || egress[0].stage != filterapi.FilterStagePeerChain {
		t.Fatalf("egress steps = %v, want single policy-chain step", egressStepNames(egress))
	}
}

// TestOrderedIngressCarriesTheEngineLoopStep proves loop detection is engine
// code: the built pass holds exactly one loop step, at the protocol stage, running LoopIngress,
// ahead of the policy chain (RFC 4271 Section 9.1.2, RFC 4456 Section 8).
func TestOrderedIngressCarriesTheEngineLoopStep(t *testing.T) {
	steps := buildOrderedIngressSteps()
	assertOrder(t, ingressStepNames(steps), []string{"loop", policyChainStepName})
	if steps[0].stage != filterapi.FilterStageProtocol || steps[0].inproc == nil || steps[0].policyChain {
		t.Fatalf("loop step = %+v, want the protocol-stage engine step", steps[0])
	}
}
