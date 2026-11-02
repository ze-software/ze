package reactor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestFlowSpecSectionAsksForkedRIBInBatches proves a stored FlowSpec relay asks
// a selecting RIB in another process about a whole section in batches, not
// once per rule. Method: three rules (one kept, one whose redirect the RIB
// refused, one no generation of which is eligible) go through flowSpecSection
// while the FlowSpec gate is announced with a command; the test dispatcher
// answers from canned verdicts and counts commands. Per-rule asking sends six
// commands; the batched section sends one eligibility batch and one redirect
// batch, and still sorts each rule into the kept, refused or denied region.
func TestFlowSpecSectionAsksForkedRIBInBatches(t *testing.T) {
	ruleKept := []byte{5, 1, 24, 10, 0, 0}
	ruleRefused := []byte{5, 1, 24, 10, 0, 1}
	ruleDenied := []byte{5, 1, 24, 10, 0, 2}
	const msgID = 77
	eligible := map[string]bool{
		ribevents.FlowSpecKey(ruleKept):    true,
		ribevents.FlowSpecKey(ruleRefused): true,
	}
	redirectRefused := map[string]bool{ribevents.FlowSpecKey(ruleRefused): true}

	const command = "request test flowspec-gate"
	calls := 0
	release := ribevents.RegisterGateDispatch(func(_ context.Context, cmd string, args ...string) (string, json.RawMessage, error) {
		calls++
		var request ribevents.FlowSpecRequest
		if err := json.Unmarshal([]byte(args[0]), &request); err != nil {
			return "", nil, err
		}
		answer := ribevents.FlowSpecAnswer{Results: make([]ribevents.FlowSpecResult, len(request.Queries))}
		for i, query := range request.Queries {
			switch query.Operation {
			case ribevents.FlowSpecEligible:
				answer.Results[i].Verdict = eligible[query.Route.NLRI] && (query.MsgID == 0 || query.MsgID == msgID)
			case ribevents.FlowSpecRedirect:
				answer.Results[i].Verdict = redirectRefused[query.Route.NLRI]
			case ribevents.FlowSpecPresent, ribevents.FlowSpecPathOf:
				t.Errorf("unexpected FlowSpec operation %q", query.Operation)
			}
		}
		data, err := json.Marshal(answer)
		return "done", data, err
	})
	t.Cleanup(release)
	ribevents.RegisterValidationLookup(nil, nil)
	ribevents.RegisterFlowSpecRedirectLookup(nil)
	ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil)
	ribevents.NoteFlowSpecGate(ribevents.FlowSpecGateState{Command: command})
	t.Cleanup(func() { ribevents.NoteFlowSpecGate(ribevents.FlowSpecGateState{}) })

	data := append(append(bytes.Clone(ruleKept), ruleRefused...), ruleDenied...)
	s := validationSelection{
		peer:    netip.MustParseAddr("192.0.2.1"),
		msgID:   msgID,
		scratch: make([]byte, len(data)),
		replay:  true,
	}
	kept, denied, err := s.flowSpecSection(data, family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(kept, ruleKept) {
		t.Errorf("kept = %x, want %x", kept, ruleKept)
	}
	if !bytes.Equal(s.refused, ruleRefused) {
		t.Errorf("redirect refused = %x, want %x", s.refused, ruleRefused)
	}
	if !bytes.Equal(denied, ruleDenied) {
		t.Errorf("denied = %x, want %x", denied, ruleDenied)
	}
	if calls != 2 {
		t.Fatalf("RIB asked %d times, want 2: one eligibility batch and one redirect batch per section", calls)
	}
}
