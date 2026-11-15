// Design: docs/architecture/site-facts.md -- committed scenario inventory boundaries.
package sitefacts

import "testing"

// Every tracked scenario-bearing suite contributes once; hidden directories,
// root files, image-only suites and similarly named sibling trees do not.
func TestInteropInventoryIncludesAllTrackedSuites(t *testing.T) {
	got := countInterop([]string{
		"test/interop/Dockerfile.bird", "test/interop/Dockerfile.ze",
		"test/interop/scenarios/bgp-routes/ze.conf",
		"test/interop/scenarios/bgp-routes/frr.conf",
		"test/interop-ipsec/scenarios/ike/ze.conf",
		"test/interop-l2tp/scenarios/session/ze.conf",
		"test/interop-pppoe/scenarios/session/ze.conf",
		"test/interop-radius/scenarios/login/ze.conf",
		"test/interop-radius-subscriber/scenarios/login/ze.conf",
		"test/interop-rsvpte/scenarios/path/ze.conf",
		"test/interop-topology/scenarios/access/ze.conf",
		"test/interop-future/scenarios/new-case/ze.conf",
		"test/interop-future/scenarios/.draft/ze.conf",
		"test/interop-future/scenarios/README.md",
		"test/interop-images/Dockerfile.peer",
		"test/interopphony/scenarios/not-a-case/ze.conf",
		"test/functional/scenarios/not-a-case/ze.conf",
	})
	if got.scenarios != 9 || got.scenarioDirs != 10 || len(got.suites) != 9 {
		t.Fatalf("inventory = %d visible/%d raw/%d suites, want 9/10/9", got.scenarios, got.scenarioDirs, len(got.suites))
	}
	if got.targets != 2 {
		t.Errorf("legacy image counter = %d, want BIRD plus FRR only", got.targets)
	}
	if got.protocols["radius"] != 2 || got.protocols["other"] != 1 {
		t.Errorf("RADIUS/other = %d/%d, want 2/1", got.protocols["radius"], got.protocols["other"])
	}
	for suite, count := range got.suites {
		if count != 1 {
			t.Errorf("suite %s = %d, want one distinct scenario", suite, count)
		}
	}
	for label, partition := range map[string]map[string]int{"suites": got.suites, "protocols": got.protocols} {
		total := 0
		for _, count := range partition {
			total += count
		}
		if total != got.scenarios {
			t.Errorf("%s sum = %d, want %d", label, total, got.scenarios)
		}
	}
}

// Reviewed prefix aliases and four historically named OSPFv3 cases are assigned
// once by primary area, even when the scenario name mentions another protocol.
func TestInteropProtocolPartitionUsesReviewedPrimaryAreas(t *testing.T) {
	for _, test := range []struct{ scenario, protocol string }{
		{"bgp-ospf-cross-protocol", "bgp"}, {"addpath-frr", "bgp"},
		{"as-override-frr", "bgp"}, {"frr-session", "bgp"},
		{"local-as-frr", "bgp"}, {"no-prepend-frr", "bgp"},
		{"peer-as-frr", "bgp"}, {"show-routes", "bgp"}, {"shutdown-frr", "bgp"},
		{"ospf-bgp-cross-protocol", "ospfv2"}, {"ospfv3-frr", "ospfv3"},
		{"ospf-ipsec-ah-frr", "ospfv3"}, {"ospf-ipsec-frr", "ospfv3"},
		{"ospf-multiaf-frr", "ospfv3"}, {"ospf-multiaf-v4-frr", "ospfv3"},
		{"isis-frr", "isis"}, {"bfd-frr", "bfd"}, {"bmp-pmacct", "bmp"},
		{"rpki-origin", "rpki"}, {"rtr-stayrtr", "rpki"}, {"vrrp-peer", "vrrp"},
		{"dns-resolver", "dns"}, {"dhcpv4-server", "dhcpv4"},
		{"dhcpv6-server", "dhcpv6"}, {"ldp-frr", "ldp"},
		{"as112-service", "as112"}, {"gtsm-frr", "gtsm"},
		{"bgphony-case", "other"},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			got := countInterop([]string{"test/interop/scenarios/" + test.scenario + "/ze.conf"})
			total := 0
			for _, count := range got.protocols {
				total += count
			}
			if got.protocols[test.protocol] != 1 || total != 1 {
				t.Errorf("partition = %v, want exactly one %s", got.protocols, test.protocol)
			}
		})
	}
}

// A suite with no main-suite files cannot acquire the separately supplied FRR
// target, and an empty checkout still publishes explicit zero protocol counts.
func TestInteropLegacyImagesStayScopedToMainSuite(t *testing.T) {
	for _, paths := range [][]string{nil, {"test/interop-ipsec/Dockerfile.strongswan"}} {
		got := countInterop(paths)
		if got.targets != 0 || got.scenarios != 0 || len(got.suites) != 0 {
			t.Errorf("empty scenario inventory = %+v", got)
		}
		if count, stated := got.protocols["bgp"]; !stated || count != 0 {
			t.Error("empty inventory does not explicitly state zero BGP cases")
		}
	}
}

// The registered derivation reads tracked sibling suites too, while an
// untracked case beside them cannot move the committed scenario claim.
func TestInteropDerivationIncludesTrackedSiblingSuitesOnly(t *testing.T) {
	root := newCheckout(t, map[string]string{
		"a/a.go": "package a\n",
		"test/interop/scenarios/bgp-routes/ze.conf":   "\n",
		"test/interop-ipsec/scenarios/ike/ze.conf":    "\n",
		"test/interop-ipsec/scenarios/.draft/ze.conf": "\n",
	})
	writeFixtureFile(t, root, "test/interop-ipsec/scenarios/untracked/ze.conf", "\n")
	if _, code := update(root); code != 0 {
		t.Fatalf("update answered %d, want 0", code)
	}
	written := readFacts(t, root)
	for key, want := range map[string]int{
		"interop.scenarios": 2, "interop.scenario_dirs_raw": 3, "interop.suite_count": 2,
		"interop.suites.interop": 1, "interop.suites.interop-ipsec": 1,
		"interop.protocols.bgp": 1, "interop.protocols.ipsec": 1, "interop.protocols.other": 0,
	} {
		entry, stated := written.Facts[key]
		if !stated || entry.Value != want {
			t.Errorf("%s = %d (stated %t), want %d", key, entry.Value, stated, want)
		}
	}
}
