// Design: docs/guide/rpki.md -- an Invalid route stays in Adj-RIB-In, ineligible for selection
// Related: validate_runtime.go -- validateRPKI, which retries this check against the live daemon

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
)

// The validation states bgp-adj-rib-in reports in its `validation-state`
// field. They copy ValidationValid, ValidationNotFound and ValidationInvalid in
// internal/component/bgp/plugins/adj_rib_in/rib_validation.go, because le does
// not import a plugin package; TestRPKIDemoStatesAgreeWithAdjRIBIn compares them.
const (
	rpkiDemoStateValid    = 1
	rpkiDemoStateNotFound = 2
	rpkiDemoStateInvalid  = 3
)

// rpkiDemoRoute is one Adj-RIB-In row of the rpki demo: the route key, the
// RFC 6811 state bgp-rpki gave it, and whether that state made the path
// ineligible for the decision process.
type rpkiDemoRoute struct {
	Key             string `json:"key"`
	ValidationState int    `json:"validation-state"`
	Ineligible      bool   `json:"ineligible"`
}

// rpkiDemoRoutesExpected is what the demo's RTR cache and its policy
// (`invalid reject`, `not-found accept`) produce for the three prefixes the
// peer announces in demos/terminal/rpki/routes.msg. `reject` keeps the Invalid
// route in Adj-RIB-In and marks it ineligible; it does not drop it.
var rpkiDemoRoutesExpected = []rpkiDemoRoute{
	{Key: "ipv4/unicast:9.43.0.0/24", ValidationState: rpkiDemoStateValid, Ineligible: false},
	{Key: "ipv4/unicast:10.43.0.0/24", ValidationState: rpkiDemoStateInvalid, Ineligible: true},
	{Key: "ipv4/unicast:11.43.0.0/24", ValidationState: rpkiDemoStateNotFound, Ineligible: false},
}

// checkRPKIDemoAdjRIBIn answers nil only when the JSON answer of
// `show bgp adj-rib-in` holds exactly the expected rows, each with its expected
// state and eligibility. It names the first row that differs otherwise.
func checkRPKIDemoAdjRIBIn(document string) error {
	var answer struct {
		AdjRIBIn map[string][]rpkiDemoRoute `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return fmt.Errorf("validation failed: adj-rib-in answer is not JSON: %w\n%s", err, document)
	}
	held := make(map[string]rpkiDemoRoute)
	for _, routes := range answer.AdjRIBIn {
		for _, route := range routes {
			held[route.Key] = route
		}
	}
	for _, expected := range rpkiDemoRoutesExpected {
		route, found := held[expected.Key]
		if !found {
			return fmt.Errorf("validation failed: Adj-RIB-In holds no %s\n%s", expected.Key, document)
		}
		if route != expected {
			return fmt.Errorf("validation failed: %s has validation-state %d and ineligible %t, expected %d and %t\n%s",
				expected.Key, route.ValidationState, route.Ineligible,
				expected.ValidationState, expected.Ineligible, document)
		}
	}
	if len(held) != len(rpkiDemoRoutesExpected) {
		return fmt.Errorf("validation failed: Adj-RIB-In holds %d routes, expected %d\n%s",
			len(held), len(rpkiDemoRoutesExpected), document)
	}
	return nil
}
