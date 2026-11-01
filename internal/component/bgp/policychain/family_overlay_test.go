package policychain

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Family-strip overlay test types: members that strip IPv4 unicast or IPv6
// unicast, under an attachment that carries prefix and lets a member set both
// prefix and family.
const (
	typeStripIPv4 = "policychain-test-strip-ipv4" // sets family
	typeStripIPv6 = "policychain-test-strip-ipv6" // sets family

	testAttachmentStrip = "policychain-test strip" // reads prefix, sets prefix and family
)

func init() {
	prefix, fam := policy.FieldPrefix, policy.FieldFamily
	policy.MustRegisterAttachment("bgp", testAttachmentStrip, []policy.Field{prefix, fam}, []policy.Field{prefix, fam})
	policy.MustRegisterFilterType(typeStripIPv4, nil, []policy.Field{fam})
	policy.MustRegisterFilterType(typeStripIPv6, nil, []policy.Field{fam})
	strip := func(stripped family.Family) EvaluatorCompiler {
		return func(string, map[string]any) (Evaluator, error) {
			return func(_ *policyview.View, sets *Sets) (Decision, error) {
				return DecisionPass, sets.StripFamily(stripped)
			}, nil
		}
	}
	MustRegisterEvaluator(typeStripIPv4, rpc.OnErrorReject, strip(family.IPv4Unicast))
	MustRegisterEvaluator(typeStripIPv6, rpc.OnErrorReject, strip(family.IPv6Unicast))
}

// stripChain compiles types, in order, on the strip attachment.
func stripChain(t *testing.T, types ...string) *Chain {
	t.Helper()
	instances := make([]policy.Instance, len(types))
	members := make([]Member, len(types))
	for i, typ := range types {
		name := string(rune('a' + i))
		instances[i] = policy.Instance{Name: name, Type: typ}
		members[i] = Member{Name: name}
	}
	reg, err := policy.NewRegistry(instances...)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := Compile(testAttachmentStrip, reg, members, policy.FinalAccept, Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

// TestChainPrefixReadsNoStrippedFamily proves a later member's prefix no
// longer carries a family an earlier member stripped, as a later text filter
// reads no NLRI of a family an earlier text filter's delta removed
// (applyFilterDelta replaces the nlri slot, reactor/filter_chain.go). The
// strip of one family leaves the other family's section as received.
//
// Method: the gate passes only on the expected sections; a chain that
// answered the received NLRI after the strip makes it reject.
func TestChainPrefixReadsNoStrippedFamily(t *testing.T) {
	body := joined(nlriV4First, nlriV4Second)
	mp := joined(mpReachHead, nlriV6First, nlriV6Second)
	tests := []struct {
		name     string
		types    []string
		wantBody []byte
		wantMP   []byte
	}{
		{name: "strip-ipv4-keeps-mp", types: []string{typeStripIPv4, typePrefixGate}, wantMP: mp},
		{name: "strip-ipv6-keeps-body", types: []string{typeStripIPv6, typePrefixGate}, wantBody: body},
		{name: "strip-both", types: []string{typeStripIPv4, typeStripIPv6, typePrefixGate}},
		{name: "drop-then-strip-ipv6", types: []string{typeDropFirstAnnounce, typeStripIPv6, typePrefixGate},
			wantBody: nlriV4Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prefixGateBody, prefixGateMP = tc.wantBody, tc.wantMP
			chain := stripChain(t, tc.types...)
			outcome, sets := run(t, chain, prefixUpdate(t, body, mp))
			if outcome.Action != OutcomeAccept {
				t.Fatalf("outcome = %+v, want accept: the gate read a stripped family", outcome)
			}
			if !sets.Folded().StripsFamily(family.IPv4Unicast) && !sets.Folded().StripsFamily(family.IPv6Unicast) {
				t.Fatalf("folded sets strip no family")
			}
		})
	}
}

// TestChainPrefixDropAfterStripFails proves a later member that drops from a
// section an earlier member stripped fails under its on-error: the view no
// longer carries the entry it names.
func TestChainPrefixDropAfterStripFails(t *testing.T) {
	chain := stripChain(t, typeStripIPv4, typeDropFirstAnnounce)
	outcome, _ := run(t, chain, prefixUpdate(t, joined(nlriV4First, nlriV4Second), nil))
	if outcome.Action != OutcomeReject || !outcome.Failed || outcome.Member != "b" {
		t.Fatalf("outcome = %+v, want member b failed and the route rejected", outcome)
	}
}
