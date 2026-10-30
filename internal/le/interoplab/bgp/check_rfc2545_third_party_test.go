// Design: docs/architecture/testing/interop.md -- RFC 2545 Section 3 for a third-party next hop.
// Related: check_rfc2545_third_party_predicate.go -- the predicates these branches pin.
package bgp

import (
	"net"
	"net/netip"
	"testing"
)

// rfc2545ThirdPartyCheckerBranches drives both polarities of every predicate
// checkRFC2545ThirdPartyLinkLocal decides with, from text iproute2 and
// /proc/net/if_inet6 print, so a lab that cannot discriminate is refused
// rather than read as Ze's verdict. TestBespokeCheckerBranches runs it.
func rfc2545ThirdPartyCheckerBranches(t *testing.T) {
	const ifInet6 = "fd001e0000000000000000000000000f 0b 40 00 80 eth0\n" +
		"fe80000000000000bcd2a1fffe33440f 0b 40 20 80 eth0\n" +
		"00000000000000000000000000000001 01 80 10 80 lo\n"
	want := netip.MustParseAddr("fe80::bcd2:a1ff:fe33:440f")
	got, err := onlyLinkLocal(ifInet6, "eth0")
	if err != nil || got != want {
		t.Fatalf("onlyLinkLocal = %s, %v; want %s", got, err, want)
	}
	if _, err := onlyLinkLocal(ifInet6+"fe800000000000000000000000000099 0b 40 20 80 eth0\n", "eth0"); err == nil {
		t.Fatal("two link-locals on one device were accepted")
	}
	if _, err := onlyLinkLocal(ifInet6, "eth1"); err == nil {
		t.Fatal("a device with no link-local was accepted")
	}

	mac, err := parseContainerMAC("be:d2:a1:33:44:0f\n")
	if err != nil {
		t.Fatalf("a MAC was refused: %v", err)
	}
	if _, err := parseContainerMAC(" \n"); err == nil {
		t.Fatal("an empty MAC answer was accepted")
	}
	global := netip.MustParseAddr("fd00:1e::f")
	other, err := net.ParseMAC("02:42:ac:1e:00:03")
	if err != nil {
		t.Fatal(err)
	}
	entry := func(dst string, hw net.HardwareAddr, state string) string {
		return `{"dst":"` + dst + `","dev":"eth0","lladdr":"` + hw.String() + `","state":["` + state + `"]}`
	}
	bound := "[" + entry("fd00:1e::f", mac, "REACHABLE") + "," + entry(want.String(), mac, "STALE") + "," +
		entry("fd00:1e::3", other, "REACHABLE") + "]"
	if err := requireNeighborPair(bound, global, want, mac); err != nil {
		t.Fatalf("a table binding both addresses to one MAC was refused: %v", err)
	}
	for name, table := range map[string]string{
		"global missing":     "[" + entry(want.String(), mac, "REACHABLE") + "]",
		"link-local missing": "[" + entry("fd00:1e::f", mac, "REACHABLE") + "]",
		"global failed":      "[" + entry("fd00:1e::f", mac, "FAILED") + "," + entry(want.String(), mac, "REACHABLE") + "]",
		"other MAC":          "[" + entry("fd00:1e::f", other, "REACHABLE") + "," + entry(want.String(), other, "REACHABLE") + "]",
		"ambiguous":          "[" + entry("fd00:1e::f", mac, "REACHABLE") + "," + entry(want.String(), mac, "REACHABLE") + "," + entry("fe80::99", mac, "STALE") + "]",
		"not JSON":           "fd00:1e::f dev eth0 lladdr " + mac.String(),
	} {
		if err := requireNeighborPair(table, global, want, mac); err == nil {
			t.Errorf("%s: an unpairable neighbor table was accepted", name)
		}
	}
}
