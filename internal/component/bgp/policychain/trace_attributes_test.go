package policychain

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestChainOnlyTraceAttributes proves a single-member display chain retains
// every attribute reader its compiled parent uses, without inheriting the
// parent's live refusal counter. Each selected member must pass and apply
// the requested attribute, rather than fail-open through a zero Reader.
func TestChainOnlyTraceAttributes(t *testing.T) {
	large := attribute.LargeCommunity{GlobalAdmin: 1, LocalData1: 2, LocalData2: 3}
	extended := attribute.ExtendedCommunity{0, 2, 0, 0, 0, 0, 0, 1}
	var aigp [attribute.AIGPWireLen]byte
	attribute.WriteAIGPMetric(aigp[:], 0, 100)
	cases := []struct {
		name string
		code attribute.AttributeCode
		set  func(*Sets) error
		want []byte
	}{
		{"origin", attribute.AttrOrigin, func(s *Sets) error { return s.Origin(attribute.OriginEGP) }, []byte{1}},
		{"next-hop", attribute.AttrNextHop, func(s *Sets) error { return s.NextHop(netip.MustParseAddr("192.0.2.9")) }, []byte{192, 0, 2, 9}},
		{"aigp", attribute.AttrAIGP, func(s *Sets) error { return s.AIGP(NumericActionSet, 100) }, aigp[:]},
		{"large-community", attribute.AttrLargeCommunity, func(s *Sets) error { return s.AddLargeCommunity(large) }, largeAppend(nil, large)},
		{"extended-community", attribute.AttrExtCommunity, func(s *Sets) error { return s.AddExtendedCommunity(extended) }, extended[:]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chain := atomicChain(t, func(_ *policyview.View, sets *Sets) (Decision, error) {
				return DecisionPass, tc.set(sets)
			})
			chain.refused.Store(7)
			only, ok := chain.Only("a")
			if !ok {
				t.Fatal("Only did not find member a")
			}
			if only.Refused() != 0 {
				t.Errorf("Only inherited live refusal counter %d", only.Refused())
			}
			var view policyview.View
			var sets RunSets
			view.Reset(update(t), policyview.Session{})
			defer view.Clear()
			steps := 0
			outcome := only.Trace(&view, &sets, func(step MemberTrace) {
				steps++
				if step.Step != TracePass || step.Err != nil {
					t.Errorf("Only trace step=%+v, want successful attribute set", step)
					return
				}
				var acc filterapi.ModAccumulator
				if _, err := step.Apply(&acc); err != nil {
					t.Fatal(err)
				}
				ops := acc.Ops()
				if len(ops) != 1 || ops[0].Code != uint8(tc.code) || !bytes.Equal(ops[0].Buf, tc.want) {
					t.Errorf("Only applied %+v, want code %d value %x", ops, tc.code, tc.want)
				}
			})
			if outcome.Action != OutcomeAccept || steps != 1 || chain.Refused() != 7 || only.Refused() != 0 {
				t.Errorf("outcome=%+v steps=%d original refusals=%d selected refusals=%d", outcome, steps, chain.Refused(), only.Refused())
			}
		})
	}
}

// TestChainTraceAIGPMetricText proves a passing member's displayed AIGP set
// matches its applied metric, including metrics above uint32. Unresolved
// arithmetic remains action text rather than being rendered as a plain set.
func TestChainTraceAIGPMetricText(t *testing.T) {
	for _, tc := range []struct {
		name    string
		current uint64
		action  NumericAction
		value   uint32
		want    uint64
		text    string
	}{
		{"set", 7, NumericActionSet, 100, 100, "aigp 100"},
		{"wide-increment", 1099511627776, NumericActionIncrement, 10, 1099511627786, "aigp 1099511627786"},
		{"uint64-clamp", 18446744073709551610, NumericActionIncrement, 10, 18446744073709551615, "aigp 18446744073709551615"},
		{"present-zero", 7, NumericActionSet, 0, 0, "aigp 0"},
		{"decrement", 100, NumericActionDecrement, 10, 90, "aigp 90"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain := atomicChain(t, func(_ *policyview.View, sets *Sets) (Decision, error) {
				return DecisionPass, sets.AIGP(tc.action, tc.value)
			})
			var received, want [attribute.AIGPWireLen]byte
			attribute.WriteAIGPMetric(received[:], 0, tc.current)
			attribute.WriteAIGPMetric(want[:], 0, tc.want)
			var view policyview.View
			var sets RunSets
			view.Reset(update(t, pathAttr(0x80, attribute.AttrAIGP, received[:])), policyview.Session{})
			defer view.Clear()
			outcome := chain.Trace(&view, &sets, func(step MemberTrace) {
				if step.Step != TracePass || step.Sets == nil {
					t.Fatalf("trace=%+v, want passing AIGP set", step)
				}
				if text := string(step.Sets.AppendText(nil)); text != tc.text {
					t.Errorf("trace delta=%q, want %q", text, tc.text)
				}
				var acc filterapi.ModAccumulator
				if _, err := step.Apply(&acc); err != nil {
					t.Fatal(err)
				}
				ops := acc.Ops()
				if len(ops) != 1 || ops[0].Code != uint8(attribute.AttrAIGP) || !bytes.Equal(ops[0].Buf, want[:]) {
					t.Errorf("applied AIGP=%+v, want %x", ops, want)
				}
			})
			if outcome.Action != OutcomeAccept {
				t.Errorf("outcome=%+v, want accept", outcome)
			}
		})
	}
	for _, tc := range []struct {
		action NumericAction
		text   string
	}{{NumericActionIncrement, "aigp increment 10"}, {NumericActionDecrement, "aigp decrement 10"}} {
		var sets Sets
		if err := sets.AIGP(tc.action, 10); err != nil {
			t.Fatal(err)
		}
		if text := string(sets.AppendText(nil)); text != tc.text {
			t.Errorf("unresolved delta=%q, want %q", text, tc.text)
		}
	}
}
