package filter_path_asn

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/policy"
)

// rejectTotal sums every series of the reject counter.
func rejectTotal(t *testing.T, lines []string) int {
	t.Helper()
	total := 0
	for _, line := range lines {
		value := line[strings.LastIndexByte(line, ' ')+1:]
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			t.Fatalf("series %q: %v", line, err)
		}
		total += int(n)
	}
	return total
}

// actionChain compiles a chain of one match instance whose reject-asn
// condition names the guard list, under the given action ("" leaves it out).
func actionChain(t *testing.T, action string) *policychain.Chain {
	t.Helper()
	cfg := map[string]any{filterTypeName: "guard"}
	if action != "" {
		cfg["action"] = action
	}
	reg, err := policy.NewRegistry(
		policy.Instance{Name: "guard", Type: filterTypeName, Config: conditionListConfig()},
		policy.Instance{Name: "m", Type: policy.MatchFilterType, Config: cfg},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(testAttachmentBGP, reg, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return chain
}

// TestBGPMatchRejectASNCountsOnlyRefusals proves the reject-asn condition
// counts a reject exactly when the old filter did: when the list refuses the
// route and that refusal drops it on a live run. Method: each case runs one
// chain over one route and reads the counter after. A display-only trace
// (show policy test) refused no route, and under `action reject` the list
// refusing a route lets the route pass, so neither counts; the live
// accept-action reject counts once.
func TestBGPMatchRejectASNCountsOnlyRefusals(t *testing.T) {
	session := policyview.Session{EBGP: true, Sender: 65001, SenderKnown: true}
	transit := []uint32{65001, 3356, 64999}
	cases := []struct {
		name    string
		action  string
		path    []uint32
		trace   bool
		outcome policychain.OutcomeAction
		counted int
	}{
		{name: "live reject under accept", path: transit, outcome: policychain.OutcomeReject, counted: 1},
		{name: "live pattern reject", path: []uint32{65001, 64512}, outcome: policychain.OutcomeReject, counted: 1},
		{name: "display-only trace", path: transit, trace: true, outcome: policychain.OutcomeReject},
		{name: "list refuses under action reject", action: "reject", path: transit, outcome: policychain.OutcomeAccept},
		{name: "clean path under action reject", action: "reject", path: []uint32{65001, 65002}, outcome: policychain.OutcomeReject},
		{name: "clean path under accept", path: []uint32{65001, 65002}, outcome: policychain.OutcomeAccept},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := installMetrics(t)
			chain := actionChain(t, tc.action)
			var view policyview.View
			view.Reset(pathUpdate(t, tc.path...), session)
			defer view.Clear()
			var outcome policychain.Outcome
			if tc.trace {
				outcome = chain.Trace(&view, new(policychain.RunSets), func(policychain.MemberTrace) {})
			} else {
				outcome = chain.Run(&view, new(policychain.RunSets))
			}
			if outcome.Action != tc.outcome {
				t.Fatalf("outcome %+v, want %v", outcome, tc.outcome)
			}
			if got := rejectTotal(t, scrapeRejects(t, reg)); got != tc.counted {
				t.Fatalf("counted %d rejects, want %d", got, tc.counted)
			}
		})
	}
}
