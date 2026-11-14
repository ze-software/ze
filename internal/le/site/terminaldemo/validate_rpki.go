// Design: docs/guide/rpki.md -- an Invalid route stays in Adj-RIB-In, ineligible for selection
// Related: scenarios_routing.go -- requireRPKITransit reads the live answer this file judges.

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
// peer announces in demos/terminal/showcase-lab/transit.msg. `reject` keeps the Invalid
// route in Adj-RIB-In and marks it ineligible; it does not drop it.
var rpkiDemoRoutesExpected = []rpkiDemoRoute{
	{Key: "ipv4/unicast:9.43.0.0/24", ValidationState: rpkiDemoStateValid, Ineligible: false},
	{Key: "ipv4/unicast:10.43.0.0/24", ValidationState: rpkiDemoStateInvalid, Ineligible: true},
	{Key: "ipv4/unicast:11.43.0.0/24", ValidationState: rpkiDemoStateNotFound, Ineligible: false},
}

// checkRPKIDemoAdjRIBIn judges the JSON answer of `show bgp adj-rib-in` for
// the transit peer (loopbackPeerAddress) of the shared lab, whose other peers
// hold routes of their own. It reads that peer's rows alone and
// answers nil only when that peer holds exactly the expected routes. Once
// RPKI is committed (validated), each route also carries its expected state
// and eligibility; before the commit no route may be ineligible yet. It names
// the first route that differs otherwise.
func checkRPKIDemoAdjRIBIn(document string, validated bool) error {
	const peer = loopbackPeerAddress
	var answer struct {
		AdjRIBIn map[string][]rpkiDemoRoute `json:"adj-rib-in"`
	}
	if err := json.Unmarshal([]byte(document), &answer); err != nil {
		return fmt.Errorf("validation failed: adj-rib-in answer is not JSON: %w\n%s", err, document)
	}
	routes := answer.AdjRIBIn[peer]
	held := make(map[string]rpkiDemoRoute, len(routes))
	for _, route := range routes {
		held[route.Key] = route
	}
	for _, expected := range rpkiDemoRoutesExpected {
		route, found := held[expected.Key]
		if !found {
			return fmt.Errorf("validation failed: Adj-RIB-In of %s holds no %s\n%s", peer, expected.Key, document)
		}
		if !validated {
			if route.Ineligible {
				return fmt.Errorf("validation failed: %s is ineligible before RPKI is committed\n%s", expected.Key, document)
			}
			continue
		}
		if route != expected {
			return fmt.Errorf("validation failed: %s has validation-state %d and ineligible %t, expected %d and %t\n%s",
				expected.Key, route.ValidationState, route.Ineligible,
				expected.ValidationState, expected.Ineligible, document)
		}
	}
	// The row count, not the key count: a key announced twice is one route too many.
	if len(routes) != len(rpkiDemoRoutesExpected) {
		return fmt.Errorf("validation failed: Adj-RIB-In of %s holds %d routes, expected %d\n%s",
			peer, len(routes), len(rpkiDemoRoutesExpected), document)
	}
	return nil
}
