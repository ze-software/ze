package policychain

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Community overlay test types: a gate that passes only a route whose
// COMMUNITY reads exactly communityGateWant, and a member that removes the
// community typeCommunity adds.
const (
	typeCommunityGate   = "policychain-test-community-gate"   // reads community
	typeCommunityRemove = "policychain-test-community-remove" // sets community
)

// communityGateWant is the COMMUNITY value the gate passes; each test sets it.
var communityGateWant []byte

func init() {
	comm := policy.FieldCommunity
	policy.MustRegisterFilterType(typeCommunityGate, []policy.Field{comm}, nil)
	policy.MustRegisterFilterType(typeCommunityRemove, nil, []policy.Field{comm})
	MustRegisterEvaluator(typeCommunityGate, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		reader, err := policyview.ReaderFor(comm)
		if err != nil {
			return nil, err
		}
		return func(view *policyview.View, _ *Sets) (Decision, error) {
			value, err := view.Read(reader)
			if err != nil {
				return DecisionUnspecified, err
			}
			if bytes.Equal(value.Wire(), communityGateWant) {
				return DecisionPass, nil
			}
			return DecisionReject, nil
		}, nil
	})
	MustRegisterEvaluator(typeCommunityRemove, rpc.OnErrorReject, func(string, map[string]any, policy.InstanceResolver) (Evaluator, error) {
		return func(_ *policyview.View, sets *Sets) (Decision, error) {
			return DecisionPass, sets.RemoveCommunity(0xfde90064)
		}, nil
	})
}

func community(values ...byte) []byte {
	return pathAttr(0xc0, attribute.AttrCommunity, values)
}

// TestChainCommunityReadAfterSet proves a later member reads the COMMUNITY an
// earlier member's add and remove made, not the wire value: the route carries
// 65001:1 and 65001:2, typeCommunity removes 65001:1 and adds 65001:100, and
// the gate after it passes only on 65001:2 then 65001:100, the value the
// COMMUNITY handler writes for those operations. A gate that read the wire
// would reject.
func TestChainCommunityReadAfterSet(t *testing.T) {
	communityGateWant = []byte{0xfd, 0xe9, 0, 2, 0xfd, 0xe9, 0, 100}
	chain := compile(t, policy.FinalAccept,
		policy.Instance{Name: "set", Type: typeCommunity},
		policy.Instance{Name: "gate", Type: typeCommunityGate})
	wire := update(t, community(0xfd, 0xe9, 0, 1, 0xfd, 0xe9, 0, 2))
	outcome, _ := run(t, chain, wire)
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept: the gate did not read the set value", outcome)
	}

	// A route that carries no COMMUNITY reads as the additions alone.
	communityGateWant = []byte{0xfd, 0xe9, 0, 100}
	outcome, _ = run(t, chain, update(t))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("absent COMMUNITY: outcome = %+v, want accept", outcome)
	}
}

// TestChainCommunityRemoveCancelsAdd proves a later member's remove of a
// community an earlier member added leaves no add behind: the handler applies
// removes to the route's own values only, so the folded add would otherwise
// put the community on the wire although the later member, and the overlay it
// read, took it off.
func TestChainCommunityRemoveCancelsAdd(t *testing.T) {
	chain := compile(t, policy.FinalAccept,
		policy.Instance{Name: "set", Type: typeCommunity},
		policy.Instance{Name: "unset", Type: typeCommunityRemove})
	outcome, sets := run(t, chain, update(t))
	if outcome.Action != OutcomeAccept {
		t.Fatalf("outcome = %+v, want accept", outcome)
	}
	for _, op := range applied(t, sets.Folded(), Current{}) {
		if op.Action == filterapi.AttrModAdd && bytes.Equal(op.Buf, []byte{0xfd, 0xe9, 0, 100}) {
			t.Fatalf("ops %+v still add 65001:100", applied(t, sets.Folded(), Current{}))
		}
	}
}
