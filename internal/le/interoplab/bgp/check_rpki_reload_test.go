// Design: docs/architecture/testing/interop.md -- live RPKI policy reload observations.
package bgp

import (
	"strings"
	"testing"
)

// rpkiReloadCheckerBranches rejects incomplete retention and cross-route evidence.
func rpkiReloadCheckerBranches(t *testing.T) {
	const control = "10.58.0.0/16 unicast [ze_peer 12:00:00] * (100)\n\tBGP.as_path: 65000 65001\n"
	const invalid = "9.58.0.0/24 unicast [ze_peer 12:00:00] * (100)\n\tBGP.as_path: 65000 65001\n"
	for _, test := range []struct {
		name     string
		output   string
		accepted bool
		valid    bool
	}{
		{"rejected", control, false, true},
		{"accepted", control + invalid, true, true},
		{"invalid leaked", control + invalid, false, false},
		{"invalid not exported", control, true, false},
		{"unanswered query", "", false, false},
		{"control lost", invalid, true, false},
		{"wrong invalid origin", control + strings.Replace(invalid, "65000 65001", "65000 65003", 1), true, false},
		{"prefix substring", control + strings.Replace(invalid, "9.58.0.0/24", "19.58.0.0/24", 1), true, false},
		{"covering route", control + strings.Replace(invalid, "9.58.0.0/24", "9.58.0.0/16", 1), true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := requireRPKIReloadExport(test.output, test.accepted)
			if (err == nil) != test.valid {
				t.Fatalf("accepted=%t: export error=%v, want valid=%t", test.accepted, err, test.valid)
			}
		})
	}

	valid, invalidState := uint8(1), uint8(3)
	eligible, ineligible := false, true
	received := []rpkiReloadRoute{
		{Key: "ipv4/unicast:" + rpkiReloadControl, Family: "ipv4/unicast", Attributes: "40010100", NextHop: "ac1e0003", NLRI: "100a3a", State: &valid, Ineligible: &eligible},
		{Key: "ipv4/unicast:" + rpkiReloadInvalid, Family: "ipv4/unicast", Attributes: "40010100", NextHop: "ac1e0003", NLRI: "18093a00", State: &invalidState, Ineligible: &ineligible},
	}
	original, err := requireRPKIReloadRoutes(received, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	received[1].Ineligible = &eligible
	if _, err := requireRPKIReloadRoutes(received, true, original); err != nil {
		t.Fatalf("accepting the retained route failed: %v", err)
	}
	received[1].Ineligible = &ineligible
	if _, err := requireRPKIReloadRoutes(received, false, original); err != nil {
		t.Fatalf("restoring rejection failed: %v", err)
	}
	for _, test := range []struct {
		name   string
		mutate func([]rpkiReloadRoute) []rpkiReloadRoute
	}{
		{"route removed", func(routes []rpkiReloadRoute) []rpkiReloadRoute { return routes[:1] }},
		{"duplicate path", func(routes []rpkiReloadRoute) []rpkiReloadRoute { return append(routes, routes[1]) }},
		{"eligible invalid", func(routes []rpkiReloadRoute) []rpkiReloadRoute { routes[1].Ineligible = &eligible; return routes }},
		{"missing eligibility", func(routes []rpkiReloadRoute) []rpkiReloadRoute { routes[1].Ineligible = nil; return routes }},
		{"validation state lost", func(routes []rpkiReloadRoute) []rpkiReloadRoute { routes[1].State = nil; return routes }},
		{"attributes rewritten", func(routes []rpkiReloadRoute) []rpkiReloadRoute { routes[1].Attributes = "40010101"; return routes }},
		{"next hop rewritten", func(routes []rpkiReloadRoute) []rpkiReloadRoute { routes[1].NextHop = "ac1e0002"; return routes }},
		{"NLRI rewritten", func(routes []rpkiReloadRoute) []rpkiReloadRoute { routes[1].NLRI = "18093a01"; return routes }},
	} {
		t.Run(test.name, func(t *testing.T) {
			routes := test.mutate(append([]rpkiReloadRoute(nil), received...))
			if _, err := requireRPKIReloadRoutes(routes, false, original); err == nil {
				t.Fatal("incorrect retained-route observation accepted")
			}
		})
	}
}
