// Design: docs/architecture/testing/interop.md -- the branch tests a bespoke checker owes.
// RFC: rfc/short/rfc4456.md -- Section 9, the CLUSTER_LIST length tie-break.
// Related: check_rfc4456.go -- the predicates under test.
//
// VALIDATES: the two readers checkClusterListLengthTieBreak turns peer output
// into a verdict with: which BIRD route line is the selected one, and which
// peer and which decision step ze's narrated best-path answer names. Boundary:
// zero narrated comparisons (one candidate) and one narrated comparison (two
// candidates) are the two sides the wait predicate settles on.
// PREVENTS: a verdict read out of an answer that carries none. An attribute
// value with brackets and an asterisk in it passing as the selected route, a
// failed or empty query passing as a decision, and a single-candidate answer
// with no comparison in it passing as a decided one. Each would let the
// scenario report agreement between ze and BIRD that neither speaker stated.
package bgp

import (
	"strings"
	"testing"
)

// birdRouteListing writes the two-route answer BIRD gives for one net, with the
// asterisk on the protocol named. The shape is rt_show_rte's format string
// (nest/rt-show.c): "<net> <dest> [<protocol> <time>]<flag><info>", the second
// route indented under a blank net column, and the "from" clause present only
// when the route's neighbor differs from its gateway.
func birdRouteListing(primary string) string {
	direct := "unicast [" + clusterListDirectProtocol + " 04:22:31.100]"
	reflected := "unicast [" + clusterListReflectedProtocol + " 04:22:31.200 from 172.30.0.3]"
	if primary == clusterListDirectProtocol {
		direct += " *"
	}
	if primary == clusterListReflectedProtocol {
		reflected += " *"
	}
	return "BIRD 2.15.1 ready.\nTable master4:\n" +
		clusterListPrefix + "        " + direct + " (100) [i]\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tType: BGP univ\n" +
		"\tBGP.origin: IGP\n" +
		"\tBGP.next_hop: 172.30.0.5\n" +
		"\tBGP.local_pref: 100\n" +
		"                    " + reflected + " (100) [i]\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tType: BGP univ\n" +
		"\tBGP.origin: IGP\n" +
		"\tBGP.next_hop: 172.30.0.5\n" +
		"\tBGP.local_pref: 100\n" +
		"\tBGP.originator_id: 172.30.0.5\n" +
		"\t" + clusterListAttributeLabel + "172.30.0.3\n"
}

// TestBIRDPrimaryProtocol pins which BIRD route line counts as the verdict.
func TestBIRDPrimaryProtocol(t *testing.T) {
	name, err := birdPrimaryProtocol(birdRouteListing(clusterListDirectProtocol))
	if err != nil || name != clusterListDirectProtocol {
		t.Fatalf("primary = %q, %v; want %q", name, err, clusterListDirectProtocol)
	}
	name, err = birdPrimaryProtocol(birdRouteListing(clusterListReflectedProtocol))
	if err != nil || name != clusterListReflectedProtocol {
		t.Fatalf("primary = %q, %v; want %q", name, err, clusterListReflectedProtocol)
	}
	if _, err := birdPrimaryProtocol(birdRouteListing("")); err == nil {
		t.Fatal("a listing with no selected route answered a verdict")
	}
	if _, err := birdPrimaryProtocol(""); err == nil {
		t.Fatal("an empty BIRD answer answered a verdict")
	}
	// The info suffix carries brackets of its own, and an attribute line can
	// too. Neither is a route line, and neither may be read as one.
	if _, err := birdPrimaryProtocol("\tBGP.as_path: [65000] *\n"); err == nil {
		t.Fatal("an attribute line passed as the selected route")
	}
}

// TestClusterListDecision pins the two fields ze's narrated answer must carry.
func TestClusterListDecision(t *testing.T) {
	answer := `{"best-path-reason":[{"family":"ipv4/unicast","prefix":"` + clusterListPrefix +
		`","winner-peer":"172.30.0.5","candidates":["172.30.0.3","172.30.0.5"],` +
		`"steps":[{"step":"cluster-list-length","incumbent":"172.30.0.3","challenger":"172.30.0.5",` +
		`"winner":"172.30.0.5","reason":"cluster-list-length 0 vs 1"}]}]}`
	winner, step, err := clusterListDecision(answer)
	if err != nil || winner != "172.30.0.5" || step != clusterListDecidingStep {
		t.Fatalf("decision = %q %q, %v", winner, step, err)
	}

	peerAddress := strings.ReplaceAll(answer, clusterListDecidingStep, "peer-address")
	_, step, err = clusterListDecision(peerAddress)
	if err != nil || step != "peer-address" {
		t.Fatalf("a peer-address decision was not reported as one: %q, %v", step, err)
	}

	// One candidate produces an answer with no narrated comparison. Reporting a
	// blank step would let the wait predicate settle on a decision that has not
	// happened yet.
	single := `{"best-path-reason":[{"family":"ipv4/unicast","prefix":"` + clusterListPrefix +
		`","winner-peer":"172.30.0.5","candidates":["172.30.0.5"],"steps":[]}]}`
	if _, _, err := clusterListDecision(single); err == nil {
		t.Fatal("a single-candidate answer passed as a decided comparison")
	}
	if _, _, err := clusterListDecision(`{"best-path-reason":[]}`); err == nil {
		t.Fatal("an empty best-path answer passed as a decision")
	}
	if _, _, err := clusterListDecision("connection refused"); err == nil {
		t.Fatal("a failed query passed as a decision")
	}
	if _, _, err := clusterListDecision(""); err == nil {
		t.Fatal("an empty answer passed as a decision")
	}
}
