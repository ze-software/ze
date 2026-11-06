// Design: docs/architecture/core-design.md -- the in-process BGP chain
// Related: chain.go -- Chain.SameDecisions

package policychain

import (
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	typeSameResolves = "test-same-resolves"
	typeSameList     = "test-same-list"
)

func init() {
	policy.MustRegisterFilterType(typeSameResolves, nil, nil)
	policy.MustRegisterFilterType(typeSameList, nil, nil)
	MustRegisterEvaluator(typeSameResolves, rpc.OnErrorReject,
		func(_ string, _ map[string]any, resolve policy.InstanceResolver) (Evaluator, error) {
			if _, err := resolve("list", typeSameList); err != nil {
				return nil, err
			}
			return func(*policyview.View, *Sets) (Decision, error) { return DecisionPass, nil }, nil
		})
}

// sameResolvesChain compiles one member that resolves "list" under listConfig.
func sameResolvesChain(t *testing.T, listConfig map[string]any) *Chain {
	t.Helper()
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "member", Type: typeSameResolves},
		policy.Instance{Name: "list", Type: typeSameList, Config: listConfig})
	if err != nil {
		t.Fatal(err)
	}
	chain, err := Compile(testAttachment, reg, []Member{{Name: "member"}}, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

// externalChain compiles one member a plugin runs.
func externalChain(t *testing.T, final policy.Final, bases Bases, filter string) *Chain {
	t.Helper()
	reg, err := policy.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	chain, err := Compile(testAttachment, reg, []Member{{Name: filter, Plugin: "p"}}, final, bases)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

// TestChainSameDecisions proves SameDecisions answers what a reload needs to know:
// whether two chains judge alike. Each chain below is compiled from its own
// registry, as a config parse does, so no two share an evaluator.
//
// VALIDATES: a rebuilt chain from the same config is the same; a change in a member's
// config, in what it resolved, in the final or in the bases is not.
// PREVENTS: reflect.DeepEqual's answer, false for every chain holding an evaluator.
func TestChainSameDecisions(t *testing.T) {
	setLP := func(value uint32) *Chain {
		return compile(t, policy.FinalAccept, policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, value)})
	}
	list := func(value string) map[string]any { return map[string]any{"value": value} }
	cases := []struct {
		name string
		a, b *Chain
		want bool
	}{
		{name: "rebuilt-evaluator", a: setLP(1), b: setLP(1), want: true},
		{name: "member-config-changed", a: setLP(1), b: setLP(2), want: false},
		{name: "rebuilt-resolved", a: sameResolvesChain(t, list("1")), b: sameResolvesChain(t, list("1")), want: true},
		{name: "resolved-config-changed", a: sameResolvesChain(t, list("1")), b: sameResolvesChain(t, list("2")), want: false},
		{name: "rebuilt-external", a: externalChain(t, policy.FinalAccept, Bases{}, "f"), b: externalChain(t, policy.FinalAccept, Bases{}, "f"), want: true},
		{name: "external-renamed", a: externalChain(t, policy.FinalAccept, Bases{}, "f"), b: externalChain(t, policy.FinalAccept, Bases{}, "g"), want: false},
		{name: "final-changed", a: externalChain(t, policy.FinalAccept, Bases{}, "f"), b: externalChain(t, policy.FinalReject, Bases{}, "f"), want: false},
		{name: "bases-changed", a: externalChain(t, policy.FinalAccept, Bases{}, "f"), b: externalChain(t, policy.FinalAccept, Bases{MED: 5}, "f"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.SameDecisions(tc.b); got != tc.want {
				t.Fatalf("SameDecisions=%v, want %v", got, tc.want)
			}
		})
	}
}

// TestChainSameDecisionsIgnoresRefused proves the count of failed routes plays no
// part: a chain under load is the same chain, and comparing it reads no counter a
// run is writing.
func TestChainSameDecisionsIgnoresRefused(t *testing.T) {
	a, b := externalChain(t, policy.FinalAccept, Bases{}, "f"), externalChain(t, policy.FinalAccept, Bases{}, "f")
	a.refused.Add(3)
	if !a.SameDecisions(b) {
		t.Fatal("a failure count made two identical chains differ")
	}
}

// TestRebuiltChainIsNotDeepEqual pins why SameDecisions exists: if this ever passes
// DeepEqual, the comparison it replaced was right and this code is not needed.
func TestRebuiltChainIsNotDeepEqual(t *testing.T) {
	build := func() *Chain {
		return compile(t, policy.FinalAccept, policy.Instance{Name: "lp", Type: typeSetLocal, Config: numericConfig(NumericActionSet, 1)})
	}
	if reflect.DeepEqual(build(), build()) {
		t.Fatal("two rebuilt chains compare DeepEqual")
	}
}
