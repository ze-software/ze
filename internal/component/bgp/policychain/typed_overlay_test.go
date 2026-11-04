package policychain

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestChainTypedWireOverlay proves ORIGIN and AIGP retain their typed numeric
// answer beside overlaid wire bytes: a later AIGP increment starts at the
// preceding set, and both a subsequent reader and final apply observe 110.
func TestChainTypedWireOverlay(t *testing.T) {
	prior := func(_ *policyview.View, sets *Sets) (Decision, error) {
		if err := sets.Origin(attribute.OriginEGP); err != nil {
			return DecisionUnspecified, err
		}
		return DecisionPass, sets.AIGP(NumericActionSet, 100)
	}
	increment := func(view *policyview.View, sets *Sets) (Decision, error) {
		for _, check := range []struct {
			field policy.Field
			want  uint64
		}{{policy.FieldOrigin, 1}, {policy.FieldAIGP, 100}} {
			if number, ok := atomicRead(t, view, check.field).Number(); !ok || number != check.want {
				t.Errorf("preceding %s set reads (%d, %v), want (%d, true)", check.field, number, ok, check.want)
			}
		}
		return DecisionPass, sets.AIGP(NumericActionIncrement, 10)
	}
	gate := func(view *policyview.View, _ *Sets) (Decision, error) {
		if number, ok := atomicRead(t, view, policy.FieldAIGP).Number(); !ok || number != 110 {
			t.Errorf("incremented AIGP reads (%d, %v), want (110, true)", number, ok)
		}
		return DecisionPass, nil
	}
	chain := atomicChain(t, prior, increment, gate)
	outcome, sets := run(t, chain, update(t))
	if outcome.Action != OutcomeAccept || len(sets.Failures()) != 0 {
		t.Fatalf("outcome=%+v failures=%v", outcome, sets.Failures())
	}
	var want [attribute.AIGPWireLen]byte
	attribute.WriteAIGPMetric(want[:], 0, 110)
	ops := applied(t, sets.Folded(), Current{})
	if len(ops) != 2 {
		t.Fatalf("operations=%+v, want ORIGIN and incremented AIGP", ops)
	}
	for _, op := range ops {
		if op.Code == uint8(attribute.AttrAIGP) && !bytes.Equal(op.Buf, want[:]) {
			t.Errorf("applied AIGP=%x, want %x", op.Buf, want)
		}
	}
}
