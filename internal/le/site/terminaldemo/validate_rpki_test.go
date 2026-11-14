package siteterminaldemo

import (
	"strings"
	"testing"

	adjribin "github.com/ze-software/ze/internal/component/bgp/plugins/adj_rib_in"
)

// rpkiDemoAnswerPassing carries the rows of the `show bgp adj-rib-in | no-more |
// json` answer a live ze daemon gave for the demo config and routes on
// 2026-10-08, keeping only the fields the check reads: the Invalid route is held
// and marked ineligible.
const rpkiDemoAnswerPassing = `{"adj-rib-in": {"127.0.0.2": [
 {"key": "ipv4/unicast:9.43.0.0/24", "validation-state": 1, "ineligible": false},
 {"key": "ipv4/unicast:10.43.0.0/24", "validation-state": 3, "ineligible": true},
 {"key": "ipv4/unicast:11.43.0.0/24", "validation-state": 2, "ineligible": false}]}}`

// TestCheckRPKIDemoAdjRIBIn proves the rpki demo check accepts the answer the
// daemon gives and refuses each way that answer can be wrong. Method: one
// passing answer and one case per defect, each naming the text its error must
// carry.
func TestCheckRPKIDemoAdjRIBIn(t *testing.T) {
	if err := checkRPKIDemoAdjRIBIn(rpkiDemoAnswerPassing, true); err != nil {
		t.Fatalf("the daemon's answer was refused: %v", err)
	}
	cases := []struct {
		name    string
		answer  string
		refusal string
	}{
		{
			// The failed render of 2026-10-08: every UPDATE was treated as a withdraw.
			name: "empty", answer: `{"adj-rib-in": {}}`,
			refusal: "holds no ipv4/unicast:9.43.0.0/24",
		},
		{
			// The demo's former expectation, which the daemon no longer gives.
			name: "invalid dropped",
			answer: `{"adj-rib-in": {"127.0.0.2": [
 {"key": "ipv4/unicast:9.43.0.0/24", "validation-state": 1, "ineligible": false},
 {"key": "ipv4/unicast:11.43.0.0/24", "validation-state": 2, "ineligible": false}]}}`,
			refusal: "holds no ipv4/unicast:10.43.0.0/24",
		},
		{
			name: "invalid eligible",
			answer: strings.Replace(rpkiDemoAnswerPassing,
				`"validation-state": 3, "ineligible": true`, `"validation-state": 3, "ineligible": false`, 1),
			refusal: "10.43.0.0/24 has validation-state 3 and ineligible false",
		},
		{
			name: "extra route",
			answer: strings.Replace(rpkiDemoAnswerPassing, `]}}`,
				`, {"key": "ipv4/unicast:12.43.0.0/24", "validation-state": 2}]}}`, 1),
			refusal: "holds 4 routes, expected 3",
		},
		{name: "not json", answer: "adj-rib-in:", refusal: "is not JSON"},
	}
	for _, tc := range cases {
		err := checkRPKIDemoAdjRIBIn(tc.answer, true)
		if err == nil {
			t.Errorf("%s: accepted, want a refusal naming %q", tc.name, tc.refusal)
			continue
		}
		if !strings.Contains(err.Error(), tc.refusal) {
			t.Errorf("%s: refusal %q does not name %q", tc.name, err, tc.refusal)
		}
	}
}

// TestCheckRPKIDemoAdjRIBInReadsOnePeer proves the check judges the named
// peer's rows alone, as the shared lab needs: its other peers hold routes of
// their own. Method: the expected rows under another peer are refused for
// transit, and another peer's extra rows do not count against transit.
// MUTATION: reading every peer's rows (`for _, routes := range answer.AdjRIBIn`)
// in place of `answer.AdjRIBIn[peer]` turns both assertions red.
func TestCheckRPKIDemoAdjRIBInReadsOnePeer(t *testing.T) {
	elsewhere := strings.Replace(rpkiDemoAnswerPassing, `"127.0.0.2"`, `"127.0.0.4"`, 1)
	err := checkRPKIDemoAdjRIBIn(elsewhere, true)
	if err == nil || !strings.Contains(err.Error(), "of 127.0.0.2 holds no ipv4/unicast:9.43.0.0/24") {
		t.Errorf("another peer's rows stood in for transit: %v", err)
	}
	shared := strings.Replace(rpkiDemoAnswerPassing, `]}}`,
		`], "127.0.0.4": [{"key": "ipv4/unicast:10.0.0.0/24", "validation-state": 0, "ineligible": false}]}}`, 1)
	if err := checkRPKIDemoAdjRIBIn(shared, true); err != nil {
		t.Errorf("the customer's route was counted against transit: %v", err)
	}
}

// TestCheckRPKIDemoAdjRIBInBeforeCommit proves the pre-commit form: transit
// holds its three routes unvalidated, and none may be ineligible before RPKI
// is committed. Method: the daemon's rows with no state pass, a missing row,
// an extra row and an ineligible row are each refused by name.
// MUTATION: dropping the `route.Ineligible` refusal under `!validated` turns
// the "ineligible early" case red.
func TestCheckRPKIDemoAdjRIBInBeforeCommit(t *testing.T) {
	const unvalidated = `{"adj-rib-in": {"127.0.0.2": [
 {"key": "ipv4/unicast:9.43.0.0/24", "validation-state": 0, "ineligible": false},
 {"key": "ipv4/unicast:10.43.0.0/24", "validation-state": 0, "ineligible": false},
 {"key": "ipv4/unicast:11.43.0.0/24", "validation-state": 0, "ineligible": false}]}}`
	if err := checkRPKIDemoAdjRIBIn(unvalidated, false); err != nil {
		t.Fatalf("unvalidated transit rows were refused before the commit: %v", err)
	}
	// The same rows are not the committed answer: the states are still absent.
	if err := checkRPKIDemoAdjRIBIn(unvalidated, true); err == nil {
		t.Error("unvalidated rows were accepted as the committed answer")
	}
	cases := []struct {
		name    string
		answer  string
		refusal string
	}{
		{
			name: "ineligible early",
			answer: strings.Replace(unvalidated,
				`10.43.0.0/24", "validation-state": 0, "ineligible": false`, `10.43.0.0/24", "validation-state": 0, "ineligible": true`, 1),
			refusal: "10.43.0.0/24 is ineligible before RPKI is committed",
		},
		{
			name: "route missing",
			answer: strings.Replace(unvalidated,
				",\n"+` {"key": "ipv4/unicast:11.43.0.0/24", "validation-state": 0, "ineligible": false}`, "", 1),
			refusal: "holds no ipv4/unicast:11.43.0.0/24",
		},
		{
			name: "extra route",
			answer: strings.Replace(unvalidated, `]}}`,
				`, {"key": "ipv4/unicast:12.43.0.0/24", "validation-state": 0}]}}`, 1),
			refusal: "holds 4 routes, expected 3",
		},
	}
	for _, tc := range cases {
		err := checkRPKIDemoAdjRIBIn(tc.answer, false)
		if err == nil {
			t.Errorf("%s: accepted, want a refusal naming %q", tc.name, tc.refusal)
			continue
		}
		if !strings.Contains(err.Error(), tc.refusal) {
			t.Errorf("%s: refusal %q does not name %q", tc.name, err, tc.refusal)
		}
	}
}

// TestRPKIDemoStatesAgreeWithAdjRIBIn keeps the demo's copy of the validation
// state numbers equal to the values bgp-adj-rib-in reports. Method: compare
// each local constant with the plugin's exported constant.
func TestRPKIDemoStatesAgreeWithAdjRIBIn(t *testing.T) {
	pairs := []struct {
		name         string
		demo, plugin int
	}{
		{"valid", rpkiDemoStateValid, int(adjribin.ValidationValid)},
		{"not-found", rpkiDemoStateNotFound, int(adjribin.ValidationNotFound)},
		{"invalid", rpkiDemoStateInvalid, int(adjribin.ValidationInvalid)},
	}
	for _, pair := range pairs {
		if pair.demo != pair.plugin {
			t.Errorf("%s: demo copy is %d, bgp-adj-rib-in reports %d", pair.name, pair.demo, pair.plugin)
		}
	}
}
