// Design: docs/architecture/testing/interop.md -- pure parser and predicate tests for spec-fib-depth interop checkers.
// Related: check_fib_depth.go -- parsers and predicates exercised here.
// Related: register_fib_depth.go -- scenario constants and registrations.
package bgp

import (
	"net/netip"
	"slices"
	"testing"
)

// TestParseFRRInstalledNextHops verifies FRR zebra `show ip route <p> json` parsing.
//
// VALIDATES: single-path, ECMP multipath, and recursive FRR zebra JSON entries
// extract exact sorted active+fib nexthops, and absence refuses installed entries.
// PREVENTS: inactive, non-FIB, recursive-parent, wrong-protocol, or uninstalled
// FRR routes from passing FIB parity assertions.
func TestParseFRRInstalledNextHops(t *testing.T) {
	hop5 := netip.MustParseAddr("172.30.0.5")
	hop15 := netip.MustParseAddr("172.30.0.15")

	ecmpJSON := `{"10.100.0.0/24":[{"prefix":"10.100.0.0/24","protocol":"bgp","selected":true,"destSelected":true,"installed":true,"nexthops":[` +
		`{"ip":"172.30.0.15","active":true,"fib":true},` +
		`{"ip":"172.30.0.5","active":true,"fib":true},` +
		`{"ip":"172.30.0.99","active":true,"fib":false}]}]}`
	got, err := parseFRRInstalledNextHops(ecmpJSON, ecmpEqualPrefix)
	if err != nil {
		t.Fatalf("ECMP parse failed: %v", err)
	}
	if want := sortedAddrs(hop5, hop15); !slices.Equal(got, want) {
		t.Fatalf("ECMP hops = %v, want %v", got, want)
	}

	// Recursive FRR zebra JSON carries the recursive BGP NH (no fib:true) beside
	// the resolved on-link FIB NH (active:true, fib:true).
	recursiveJSON := `{"10.200.0.0/24":[{"prefix":"10.200.0.0/24","protocol":"bgp","selected":true,"destSelected":true,"installed":true,"nexthops":[` +
		`{"ip":"10.250.0.1","recursive":true,"active":true},` +
		`{"ip":"172.30.0.15","interfaceName":"eth0","active":true,"fib":true}]}]}`
	got, err = parseFRRInstalledNextHops(recursiveJSON, recursiveSubjectPrefix)
	if err != nil {
		t.Fatalf("recursive parse failed: %v", err)
	}
	if want := []netip.Addr{hop15}; !slices.Equal(got, want) {
		t.Fatalf("recursive hops = %v, want %v", got, want)
	}

	for _, bad := range []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"null", "null"},
		{"empty-object", "{}"},
		{"uninstalled", `{"10.100.0.0/24":[{"prefix":"10.100.0.0/24","protocol":"bgp","selected":true,"destSelected":true,"installed":false,"nexthops":[{"ip":"172.30.0.5","active":true,"fib":true}]}]}`},
		{"wrong-protocol", `{"10.100.0.0/24":[{"prefix":"10.100.0.0/24","protocol":"static","selected":true,"destSelected":true,"installed":true,"nexthops":[{"ip":"172.30.0.5","active":true,"fib":true}]}]}`},
		{"no-fib-flag", `{"10.100.0.0/24":[{"prefix":"10.100.0.0/24","protocol":"bgp","selected":true,"destSelected":true,"installed":true,"nexthops":[{"ip":"172.30.0.5","active":true,"fib":false}]}]}`},
		{"wrong-prefix-field", `{"10.100.0.0/24":[{"prefix":"10.100.2.0/24","protocol":"bgp","selected":true,"destSelected":true,"installed":true,"nexthops":[{"ip":"172.30.0.5","active":true,"fib":true}]}]}`},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if _, err := parseFRRInstalledNextHops(bad.body, ecmpEqualPrefix); err == nil {
				t.Fatalf("case %s unexpectedly succeeded", bad.name)
			}
		})
	}

	if err := requireFRRInstalledRouteAbsent("{}", recursiveUnresolvablePrefix); err != nil {
		t.Fatalf("empty FRR JSON rejected as absent: %v", err)
	}
	if err := requireFRRInstalledRouteAbsent("null", recursiveUnresolvablePrefix); err == nil {
		t.Fatal("null FRR JSON accepted as absent")
	}
	if err := requireFRRInstalledRouteAbsent(recursiveJSON, recursiveSubjectPrefix); err == nil {
		t.Fatal("installed FRR route accepted as absent")
	}
}

// TestParseLinuxKernelNextHops verifies iproute2 `ip -4 -j [-N] route show exact` parsing.
//
// VALIDATES: single-path gateway, multipath nexthops[], numeric protocol matching,
// and fail-closed absence checking.
// PREVENTS: foreign-protocol routes, missing gateways, empty arrays, or null JSON
// from passing Linux kernel FIB assertions.
func TestParseLinuxKernelNextHops(t *testing.T) {
	hop5 := netip.MustParseAddr("172.30.0.5")
	hop15 := netip.MustParseAddr("172.30.0.15")

	single := `[{"dst":"10.100.2.0/24","gateway":"172.30.0.5","dev":"eth0","protocol":"250","flags":[]}]`
	got, err := parseLinuxKernelNextHops(single, ecmpUnequalPrefix, fibDepthZeProtocol)
	if err != nil {
		t.Fatalf("single-path kernel parse failed: %v", err)
	}
	if want := []netip.Addr{hop5}; !slices.Equal(got, want) {
		t.Fatalf("single-path hops = %v, want %v", got, want)
	}

	multipath := `[{"dst":"10.100.0.0/24","protocol":"250","flags":[],"nexthops":[{"gateway":"172.30.0.15","dev":"eth0","weight":1},{"gateway":"172.30.0.5","dev":"eth0","weight":1}]}]`
	got, err = parseLinuxKernelNextHops(multipath, ecmpEqualPrefix, fibDepthZeProtocol)
	if err != nil {
		t.Fatalf("multipath kernel parse failed: %v", err)
	}
	if want := sortedAddrs(hop5, hop15); !slices.Equal(got, want) {
		t.Fatalf("multipath hops = %v, want %v", got, want)
	}

	// Empty wantProto accepts FRR's kernel route (e.g., protocol "bgp" or "186").
	frrKernel := `[{"dst":"10.200.0.0/24","gateway":"172.30.0.15","dev":"eth0","protocol":"bgp","nhid":12}]`
	got, err = parseLinuxKernelNextHops(frrKernel, recursiveSubjectPrefix, "")
	if err != nil {
		t.Fatalf("FRR kernel route parse failed: %v", err)
	}
	if want := []netip.Addr{hop15}; !slices.Equal(got, want) {
		t.Fatalf("FRR kernel hops = %v, want %v", got, want)
	}

	for _, bad := range []struct {
		name  string
		body  string
		proto string
	}{
		{"empty-array", "[]", fibDepthZeProtocol},
		{"null", "null", fibDepthZeProtocol},
		{"wrong-proto", `[{"dst":"10.100.2.0/24","gateway":"172.30.0.5","protocol":"kernel"}]`, fibDepthZeProtocol},
		{"wrong-dst", `[{"dst":"10.100.0.0/24","gateway":"172.30.0.5","protocol":"250"}]`, fibDepthZeProtocol},
		{"no-gateway", `[{"dst":"10.100.2.0/24","dev":"eth0","protocol":"250"}]`, fibDepthZeProtocol},
		{"two-routes", `[{"dst":"10.100.2.0/24","gateway":"172.30.0.5","protocol":"250"},{"dst":"10.100.2.0/24","gateway":"172.30.0.15","protocol":"250"}]`, fibDepthZeProtocol},
	} {
		t.Run(bad.name, func(t *testing.T) {
			if _, err := parseLinuxKernelNextHops(bad.body, ecmpUnequalPrefix, bad.proto); err == nil {
				t.Fatalf("case %s unexpectedly succeeded", bad.name)
			}
		})
	}

	if err := requireLinuxKernelRouteAbsent("[]", recursiveUnresolvablePrefix); err != nil {
		t.Fatalf("empty array rejected as absent: %v", err)
	}
	if err := requireLinuxKernelRouteAbsent("null", recursiveUnresolvablePrefix); err == nil {
		t.Fatal("null accepted as absent")
	}
	if err := requireLinuxKernelRouteAbsent(single, ecmpUnequalPrefix); err == nil {
		t.Fatal("present kernel route accepted as absent")
	}
}

// TestZeFIBDepthParsers verifies Ze CLI JSON parsers for show ecmp-groups,
// show nexthop-table, show bgp rib | received, show bgp rib best | reason, and
// show bgp peer detail.
//
// VALIDATES: each parser accepts canonical Ze JSON and rejects missing, wrong,
// single-candidate, or malformed answers.
// PREVENTS: vacuous passes when a CLI command omits a field or returns wrong state.
func TestZeFIBDepthParsers(t *testing.T) {
	hop3 := netip.MustParseAddr("172.30.0.3")
	hop5 := netip.MustParseAddr("172.30.0.5")
	hop15 := netip.MustParseAddr("172.30.0.15")
	nhRec := netip.MustParseAddr(recursiveResolvableNH)
	nhFRR := netip.MustParseAddr(igpCostFRRNextHop)
	nhGoBGP := netip.MustParseAddr(igpCostGoBGPNextHop)

	t.Run("show ecmp-groups", func(t *testing.T) {
		good := `[{"prefix":"10.100.0.0/24","family":"ipv4/unicast","paths":[{"next-hop":"172.30.0.15","weight":1}]}]`
		if err := requireZeECMPGroup(good, ecmpEqualPrefix, []netip.Addr{hop5, hop15}); err != nil {
			t.Fatalf("valid ecmp-groups rejected: %v", err)
		}
		if err := requireZeECMPGroup(`[]`, ecmpEqualPrefix, []netip.Addr{hop5, hop15}); err == nil {
			t.Fatal("empty ecmp-groups accepted")
		}
		wrongHop := `[{"prefix":"10.100.0.0/24","family":"ipv4/unicast","paths":[{"next-hop":"172.30.0.99","weight":1}]}]`
		if err := requireZeECMPGroup(wrongHop, ecmpEqualPrefix, []netip.Addr{hop5, hop15}); err == nil {
			t.Fatal("ecmp-groups with unexpected sibling accepted")
		}
	})

	t.Run("show nexthop-table", func(t *testing.T) {
		good := `[{"next-hop":"10.250.0.1","resolved":true,"direct-nh":"172.30.0.15","dependents":["10.200.0.0/24"]}]`
		if err := requireZeNextHopResolved(good, nhRec, hop15, recursiveSubjectPrefix); err != nil {
			t.Fatalf("valid nexthop-table rejected: %v", err)
		}
		unresolved := `[{"next-hop":"10.250.0.1","resolved":false,"direct-nh":"172.30.0.15","dependents":["10.200.0.0/24"]}]`
		if err := requireZeNextHopResolved(unresolved, nhRec, hop15, recursiveSubjectPrefix); err == nil {
			t.Fatal("unresolved nexthop accepted")
		}
		wrongDirect := `[{"next-hop":"10.250.0.1","resolved":true,"direct-nh":"172.30.0.5","dependents":["10.200.0.0/24"]}]`
		if err := requireZeNextHopResolved(wrongDirect, nhRec, hop15, recursiveSubjectPrefix); err == nil {
			t.Fatal("wrong direct-nh accepted")
		}
		missingDep := `[{"next-hop":"10.250.0.1","resolved":true,"direct-nh":"172.30.0.15","dependents":["10.200.2.0/24"]}]`
		if err := requireZeNextHopResolved(missingDep, nhRec, hop15, recursiveSubjectPrefix); err == nil {
			t.Fatal("missing dependent prefix accepted")
		}
	})

	t.Run("show bgp rib received", func(t *testing.T) {
		good := `{"pipe":[{"op":"received"}],"routes":[` +
			`{"peer":"172.30.0.3","direction":"received","prefix":"10.210.0.0/24","family":"ipv4/unicast","next-hop":"10.221.0.1"},` +
			`{"peer":"172.30.0.5","direction":"received","prefix":"10.210.0.0/24","family":"ipv4/unicast","next-hop":"10.222.0.1"}]}`
		want := map[netip.Addr]netip.Addr{hop3: nhFRR, hop5: nhGoBGP}
		if err := requireZeReceivedCandidates(good, igpCostSubjectPrefix, want); err != nil {
			t.Fatalf("valid received candidates rejected: %v", err)
		}
		oneOnly := `{"pipe":[{"op":"received"}],"routes":[` +
			`{"peer":"172.30.0.3","direction":"received","prefix":"10.210.0.0/24","family":"ipv4/unicast","next-hop":"10.221.0.1"}]}`
		if err := requireZeReceivedCandidates(oneOnly, igpCostSubjectPrefix, want); err == nil {
			t.Fatal("single received candidate accepted when two were required")
		}
	})

	t.Run("show bgp rib best reason", func(t *testing.T) {
		good := `{"best-path-reason":[{"family":"ipv4/unicast","prefix":"10.210.0.0/24","winner-peer":"172.30.0.5",` +
			`"candidates":["172.30.0.5","172.30.0.3"],` +
			`"steps":[{"step":"igp-cost","incumbent":"172.30.0.3","challenger":"172.30.0.5","winner":"172.30.0.5","reason":"igp-cost 20 vs 10"}]}]}`
		decision, err := parseZeBestPathReason(good, igpCostSubjectPrefix)
		if err != nil {
			t.Fatalf("valid best-path reason rejected: %v", err)
		}
		if decision.Winner != hop5 || decision.Step != igpCostStepName || !slices.Equal(decision.Candidates, sortedAddrs(hop3, hop5)) {
			t.Fatalf("unexpected decision = %+v", decision)
		}
		noStep := `{"best-path-reason":[{"family":"ipv4/unicast","prefix":"10.210.0.0/24","winner-peer":"172.30.0.5","candidates":["172.30.0.5"],"steps":[]}]}`
		if _, err := parseZeBestPathReason(noStep, igpCostSubjectPrefix); err == nil {
			t.Fatal("single-candidate reason with no step accepted")
		}
		wrongPrefix := `{"best-path-reason":[{"family":"ipv4/unicast","prefix":"10.99.0.0/24","winner-peer":"172.30.0.5","candidates":["172.30.0.3","172.30.0.5"],"steps":[{"step":"igp-cost","winner":"172.30.0.5"}]}]}`
		if _, err := parseZeBestPathReason(wrongPrefix, igpCostSubjectPrefix); err == nil {
			t.Fatal("reason for wrong prefix accepted")
		}
	})

	t.Run("show bgp peer detail", func(t *testing.T) {
		good := `{"peers":{"172.30.0.5":{"state":"established","connections-established":1,"connections-dropped":0}}}`
		counters, err := parseZePeerSession(good, "172.30.0.5")
		if err != nil {
			t.Fatalf("valid peer detail rejected: %v", err)
		}
		if counters.Established != 1 || counters.Dropped != 0 {
			t.Fatalf("counters = %+v, want {1 0}", counters)
		}
		down := `{"peers":{"172.30.0.5":{"state":"active","connections-established":1,"connections-dropped":1}}}`
		if _, err := parseZePeerSession(down, "172.30.0.5"); err == nil {
			t.Fatal("non-established peer accepted")
		}
	})
}

// TestBIRDRouteWinnerParser verifies BIRD 3.4.0 `show route for <p> all` parsing.
//
// VALIDATES: both candidates present, exact primary selection (*), via gateway,
// and bgp_next_hop bound to the selected block.
// PREVENTS: covering routes, single-candidate tables, tab-indented fake asterisks,
// or cross-block attribute mixing from passing BIRD IGP-cost parity checks.
func TestBIRDRouteWinnerParser(t *testing.T) {
	hop3 := netip.MustParseAddr("172.30.0.3")
	hop5 := netip.MustParseAddr("172.30.0.5")
	nhA := netip.MustParseAddr(igpCostFRRNextHop)
	nhB := netip.MustParseAddr(igpCostGoBGPNextHop)
	both := []string{igpCostBIRDPeerA, igpCostBIRDPeerB}

	phase1 := "BIRD 3.4.0+branch..no.branch..c3669497e394 ready.\nTable master4:\n" +
		"10.210.0.0/24        unicast [peer_b 04:22:31.100 from 172.30.0.5] * (100/10) [i]\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tpreference: 100\n" +
		"\tfrom: 172.30.0.5\n" +
		"\tsource: BGP\n" +
		"\tbgp_origin: IGP\n" +
		"\tbgp_path: \n" +
		"\tbgp_next_hop: 10.222.0.1\n" +
		"\tbgp_local_pref: 100\n" +
		"\tbgp_med: 0\n" +
		"                     unicast [peer_a 04:22:31.200 from 172.30.0.3] (100/20) [i]\n" +
		"\tvia 172.30.0.3 on eth0\n" +
		"\tpreference: 100\n" +
		"\tfrom: 172.30.0.3\n" +
		"\tsource: BGP\n" +
		"\tbgp_origin: IGP\n" +
		"\tbgp_path: \n" +
		"\tbgp_next_hop: 10.221.0.1\n" +
		"\tbgp_local_pref: 100\n" +
		"\tbgp_med: 0\n"

	if err := requireBIRDRouteWinner(phase1, igpCostSubjectPrefix, igpCostBIRDPeerB, both, hop5, nhB); err != nil {
		t.Fatalf("Phase 1 BIRD route winner rejected: %v", err)
	}
	if err := requireBIRDRouteWinner(phase1, igpCostSubjectPrefix, igpCostBIRDPeerA, both, hop3, nhA); err == nil {
		t.Fatal("Phase 1 BIRD output accepted wrong winner peer_a")
	}
	if err := requireBIRDRouteWinner(phase1, igpCostSubjectPrefix, igpCostBIRDPeerB, both, hop3, nhB); err == nil {
		t.Fatal("Phase 1 BIRD output accepted wrong via gateway")
	}
	if err := requireBIRDRouteWinner(phase1, igpCostSubjectPrefix, igpCostBIRDPeerB, both, hop5, nhA); err == nil {
		t.Fatal("Phase 1 BIRD output accepted bgp_next_hop from the non-selected block")
	}

	phase2 := "BIRD 3.4.0+branch..no.branch..c3669497e394 ready.\nTable master4:\n" +
		"10.210.0.0/24        unicast [peer_a 04:22:31.200 from 172.30.0.3] * (100/20) [i]\n" +
		"\tvia 172.30.0.3 on eth0\n" +
		"\tbgp_next_hop: 10.221.0.1\n" +
		"                     unicast [peer_b 04:22:31.100 from 172.30.0.5] (100/30) [i]\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tbgp_next_hop: 10.222.0.1\n"

	if err := requireBIRDRouteWinner(phase2, igpCostSubjectPrefix, igpCostBIRDPeerA, both, hop3, nhA); err != nil {
		t.Fatalf("Phase 2 BIRD route winner rejected: %v", err)
	}

	singleOnly := "BIRD 3.4.0 ready.\nTable master4:\n" +
		"10.210.0.0/24        unicast [peer_b 04:22:31.100 from 172.30.0.5] * (100/10) [i]\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tbgp_next_hop: 10.222.0.1\n"
	if err := requireBIRDRouteWinner(singleOnly, igpCostSubjectPrefix, igpCostBIRDPeerB, both, hop5, nhB); err == nil {
		t.Fatal("single-candidate BIRD output accepted when two candidates were required")
	}

	covering := "BIRD 3.4.0 ready.\nTable master4:\n" +
		"10.0.0.0/8           unicast [peer_b 04:22:31.100 from 172.30.0.5] * (100/10) [i]\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tbgp_next_hop: 10.222.0.1\n"
	if err := requireBIRDRouteWinner(covering, igpCostSubjectPrefix, igpCostBIRDPeerB, []string{igpCostBIRDPeerB}, hop5, nhB); err == nil {
		t.Fatal("covering prefix accepted as exact route match")
	}

	fakeAttrStar := "BIRD 3.4.0 ready.\nTable master4:\n" +
		"10.210.0.0/24        unicast [peer_b 04:22:31.100 from 172.30.0.5] (100/10) [i]\n" +
		"\tbgp_path: [peer_b] *\n" +
		"\tvia 172.30.0.5 on eth0\n" +
		"\tbgp_next_hop: 10.222.0.1\n"
	if err := requireBIRDRouteWinner(fakeAttrStar, igpCostSubjectPrefix, igpCostBIRDPeerB, []string{igpCostBIRDPeerB}, hop5, nhB); err == nil {
		t.Fatal("tab-indented attribute line with asterisk accepted as selected route")
	}
}
