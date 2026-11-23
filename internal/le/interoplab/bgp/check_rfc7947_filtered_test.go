// Design: docs/architecture/testing/interop.md -- predicate unit tests for bgp-route-server-filtered-adj-rib-in-frr.
// Related: check_rfc7947_filtered.go -- predicates exercised here.
package bgp

import "testing"

// TestRouteServerFilteredAdjRIBInPredicates verifies the fail-closed JSON and
// BIRD table predicates of checkRouteServerFilteredAdjRIBIn.
//
// VALIDATES: bgp-route-server-filtered-adj-rib-in-frr checker rejects leaked
// denied prefixes, missing filtered marks, wrong filter-step names, and wrong
// status totals.
func TestRouteServerFilteredAdjRIBInPredicates(t *testing.T) {
	const peer = "172.30.0.3"
	validShow := `{"adj-rib-in":{"172.30.0.3":[` +
		`{"family":"ipv4/unicast","nlri-hex":"180a6300","filtered":false},` +
		`{"family":"ipv4/unicast","nlri-hex":"180a6301","filtered":true,"filter-step":"peer-policy-chain"}` +
		`]}}`
	if err := requireRSFilteredAdjRIBIn(validShow, peer); err != nil {
		t.Fatalf("valid show rejected: %v", err)
	}
	missingFiltered := `{"adj-rib-in":{"172.30.0.3":[` +
		`{"family":"ipv4/unicast","nlri-hex":"180a6300","filtered":false},` +
		`{"family":"ipv4/unicast","nlri-hex":"180a6301","filtered":false,"filter-step":"peer-policy-chain"}` +
		`]}}`
	if requireRSFilteredAdjRIBIn(missingFiltered, peer) == nil {
		t.Fatal("unmarked denied prefix was accepted")
	}
	wrongStep := `{"adj-rib-in":{"172.30.0.3":[` +
		`{"family":"ipv4/unicast","nlri-hex":"180a6300","filtered":false},` +
		`{"family":"ipv4/unicast","nlri-hex":"180a6301","filtered":true,"filter-step":"loop"}` +
		`]}}`
	if requireRSFilteredAdjRIBIn(wrongStep, peer) == nil {
		t.Fatal("wrong filter-step was accepted")
	}

	if err := requireRSFilteredStatus(`{"total-routes":2,"filtered-routes":1}`); err != nil {
		t.Fatalf("valid status rejected: %v", err)
	}
	if requireRSFilteredStatus(`{"total-routes":2,"filtered-routes":0}`) == nil {
		t.Fatal("zero filtered-routes was accepted")
	}

	if err := requireBIRDDeniedAbsent("10.99.0.0/24 unicast [ze_peer] * (100)\n"); err != nil {
		t.Fatalf("clean BIRD table rejected: %v", err)
	}
	if requireBIRDDeniedAbsent("10.99.0.0/24 unicast\n10.99.1.0/24 unicast\n") == nil {
		t.Fatal("BIRD table leaking denied prefix was accepted")
	}
	if requireBIRDDeniedAbsent("") == nil {
		t.Fatal("empty BIRD table without accepted control was accepted")
	}
}
