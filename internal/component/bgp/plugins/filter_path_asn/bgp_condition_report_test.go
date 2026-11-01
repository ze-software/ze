package filter_path_asn

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
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

// testPrefixCondition is a match condition that reads prefix and holds for
// every entry, registered by this test binary only, so a match naming it is
// judged per NLRI entry as a prefix-list condition makes it.
var testPrefixCondition = policy.MustRegisterMatchCondition("test-every-prefix", []policy.Field{policy.FieldPrefix}, false)

func init() {
	policychain.MatchConditions.MustAdd(testPrefixCondition,
		func(any, policy.InstanceResolver) (policy.ConditionCheck[policychain.MatchRoute], error) {
			return func(r *policychain.MatchRoute) (bool, error) { return r.Prefix.IsValid(), nil }, nil
		})
}

// TestBGPMatchRejectASNCountsOncePerUpdate proves a reject-asn refusal is
// counted once per UPDATE when the match judges each NLRI entry, as the
// filter-update handler counted it: one filter-update call, one verdict,
// one count per UPDATE (filter_path_asn.go). Method: a match naming a
// per-prefix condition and the reject-asn condition judges an UPDATE of two
// prefixes over a refused AS_PATH, and the counter reads 1.
func TestBGPMatchRejectASNCountsOncePerUpdate(t *testing.T) {
	reg := installMetrics(t)
	registry, err := policy.NewRegistry(
		policy.Instance{Name: "guard", Type: filterTypeName, Config: conditionListConfig()},
		policy.Instance{Name: "m", Type: policy.MatchFilterType,
			Config: map[string]any{filterTypeName: "guard", testPrefixCondition.Key(): "any"}},
	)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	chain, err := policychain.Compile(testAttachmentPrefix, registry, []policychain.Member{{Name: "m"}}, policy.FinalAccept, policychain.Bases{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	update := pathUpdate(t, 65001, 3356, 64999)
	payload := append(append([]byte(nil), update.Payload()...), 24, 10, 2, 0) // a second prefix, 10.2.0.0/24
	update = wireu.NewWireUpdate(payload, update.SourceCtxID())
	var view policyview.View
	view.Reset(update, policyview.Session{EBGP: true, Sender: 65001, SenderKnown: true})
	defer view.Clear()
	if outcome := chain.Run(&view, new(policychain.RunSets)); outcome.Action != policychain.OutcomeReject {
		t.Fatalf("outcome %+v, want reject", outcome)
	}
	if got := rejectTotal(t, scrapeRejects(t, reg)); got != 1 {
		t.Fatalf("counted %d rejects for one UPDATE, want 1", got)
	}
}
