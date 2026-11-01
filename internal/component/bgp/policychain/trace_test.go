package policychain

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
)

// TestChainTraceIsDisplayOnly proves the display-only run show policy test
// takes (D-11): it reports each member's step in order, a passing member with
// the sets it asked, and a failure with its error, while it neither counts
// the failure in Refused nor changes the outcome a live run reaches.
//
// Method: the same chain and route through Trace, then Run; only Run counts.
func TestChainTraceIsDisplayOnly(t *testing.T) {
	prefixGateBody, prefixGateMP = nlriV4Second, nil
	chain := prefixChain(t, typeDropFirstAnnounce, typeDropStray, typePrefixGate)
	wire := prefixUpdate(t, joined(nlriV4First, nlriV4Second), nil)

	var view policyview.View
	var sets RunSets
	type step struct {
		name, sets string
		step       TraceStep
		failed     bool
	}
	var steps []step
	view.Reset(wire, policyview.Session{})
	outcome := chain.Trace(&view, &sets, func(m MemberTrace) {
		s := step{name: m.Name, step: m.Step, failed: m.Err != nil}
		if m.Sets != nil {
			s.sets = string(m.Sets.AppendText(nil))
		}
		if m.Step == TraceFailClosed && !errors.Is(m.Err, errStrayDrop) {
			t.Errorf("member %s error = %v, want the stray drop", m.Name, m.Err)
		}
		steps = append(steps, s)
	})
	view.Clear()
	want := []step{
		{name: "a", sets: "nlri drop announce offset 0", step: TracePass},
		{name: "b", step: TraceFailClosed, failed: true},
	}
	if len(steps) != len(want) {
		t.Fatalf("steps = %+v, want %+v", steps, want)
	}
	for i := range want {
		if steps[i] != want[i] {
			t.Fatalf("step %d = %+v, want %+v", i, steps[i], want[i])
		}
	}
	if outcome.Action != OutcomeReject || !outcome.Failed || outcome.Member != "b" {
		t.Fatalf("trace outcome = %+v, want member b failed", outcome)
	}
	if chain.Refused() != 0 {
		t.Fatalf("refused after trace = %d, want 0: a display-only run refused no route", chain.Refused())
	}
	live, _ := run(t, chain, wire)
	if live != outcome {
		t.Fatalf("live outcome = %+v, trace outcome %+v: the trace must reach the live decision", live, outcome)
	}
	if chain.Refused() != 1 {
		t.Fatalf("refused after run = %d, want 1", chain.Refused())
	}
}

// TestSetsAppendText proves the display line names each set a member asked,
// in the text answer's directive words.
func TestSetsAppendText(t *testing.T) {
	var s Sets
	if err := s.LocalPreference(NumericActionSet, 200); err != nil {
		t.Fatal(err)
	}
	if err := s.MED(NumericActionIncrement, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.AddCommunity(0xfde90064); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveCommunity(0xfde90001); err != nil {
		t.Fatal(err)
	}
	if err := s.PrependASPath(2); err != nil {
		t.Fatal(err)
	}
	if err := s.RemovePrivateAS(RemovePrivatePeerAS); err != nil {
		t.Fatal(err)
	}
	want := "local-preference 200 med increment 5 community remove 65001:1 community add 65001:100 " +
		"remove-private-as replace-peer as-path-prepend 2"
	if got := string(s.AppendText(nil)); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}
