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
	if err := checkRPKIDemoAdjRIBIn(rpkiDemoAnswerPassing); err != nil {
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
		err := checkRPKIDemoAdjRIBIn(tc.answer)
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
