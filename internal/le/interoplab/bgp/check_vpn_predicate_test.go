// Design: docs/architecture/testing/interop.md -- pure decisions over foreign daemon output.
// Related: check_vpn_predicate.go -- the VPN-IPv6 next-hop readers under test.
package bgp

import (
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: the readers checkVPNFRR decides with, in both polarities, over the
// text FRR 10.3.1 and BIRD 3.4.0 print, with no container.
// PREVENTS: the RD-keyed FRR document read as a route with no path, a BIRD
// route without its link-local read as one with it, and a missing route read
// as an empty next-hop list.
func TestVPNNextHopReaders(t *testing.T) {
	t.Run("FRR document nested under the RD", func(t *testing.T) {
		output := `{"65001:100":{"prefix":"2001:db8:465a::/48","paths":[{"nexthops":[{"ip":"2001:db8:ffff::1","scope":"global"}]}]}}`
		entries, err := parseFRRVPNNextHops(output, "65001:100")
		if err != nil {
			t.Fatalf("parseFRRVPNNextHops: %v", err)
		}
		if err := requireNextHopShape(entries, netip.MustParseAddr("2001:db8:ffff::1"), netip.Addr{}); err != nil {
			t.Fatalf("requireNextHopShape: %v", err)
		}
	})
	t.Run("FRR document without the queried RD", func(t *testing.T) {
		output := `{"65001:100":{"paths":[{"nexthops":[{"ip":"2001:db8:ffff::1","scope":"global"}]}]}}`
		if _, err := parseFRRVPNNextHops(output, "65001:200"); err == nil {
			t.Fatal("an answer about another RD was accepted")
		}
	})
	t.Run("FRR unwrapped document is refused", func(t *testing.T) {
		output := `{"prefix":"2001:db8:465a::/48","paths":[{"nexthops":[{"ip":"2001:db8:ffff::1","scope":"global"}]}]}`
		if _, err := parseFRRVPNNextHops(output, "65001:100"); err == nil {
			t.Fatal("a document with no RD key was accepted")
		}
	})

	global := netip.MustParseAddr("fd00:1e::2")
	linkLocal := netip.MustParseAddr("fe80::fd00:1e:0:2")
	table := strings.Join([]string{
		"Table vpntab6:",
		"65001:100 2001:db8:4659::/48 unreachable [ze_vpn6 10:00:00.000] * (100) [AS65001i]",
		"\tpreference: 100",
		"\tfrom: fd00:1e::2",
		"\tsource: BGP",
		"\tbgp_origin: IGP",
		"\tbgp_path: 65001",
		"\tbgp_next_hop: fd00:1e::2 fe80::fd00:1e:0:2",
		"\tbgp_local_pref: 100",
		"\tInternal route handling values: 0L 3G 0S id 1",
		"65001:100 2001:db8:465a::/48 unreachable [ze_vpn6 10:00:00.000] * (100) [AS65001i]",
		"\tpreference: 100",
		"\tsource: BGP",
		"\tbgp_next_hop: 2001:db8:ffff::1",
		"\tInternal route handling values: 0L 4G 0S id 2",
		"65001:100 2001:db8:9999::/48 unreachable [ze_vpn6 10:00:00.000] * (100) [AS65001i]",
		"\tpreference: 100",
		"\tsource: BGP",
		"\tInternal route handling values: 0L 5G 0S id 3",
		"",
	}, "\n")
	t.Run("BIRD on-link route carries both addresses", func(t *testing.T) {
		decoded, err := birdVPNNextHops(table, "65001:100", "2001:db8:4659::/48")
		if err != nil {
			t.Fatalf("birdVPNNextHops: %v", err)
		}
		if err := requireBIRDNextHops(decoded, global, linkLocal); err != nil {
			t.Fatalf("requireBIRDNextHops: %v", err)
		}
		if err := requireBIRDNextHops(decoded, global, netip.Addr{}); err == nil {
			t.Fatal("a route carrying a link-local passed as one without")
		}
	})
	t.Run("BIRD off-link route carries the global alone", func(t *testing.T) {
		decoded, err := birdVPNNextHops(table, "65001:100", "2001:db8:465a::/48")
		if err != nil {
			t.Fatalf("birdVPNNextHops: %v", err)
		}
		if err := requireBIRDNextHops(decoded, netip.MustParseAddr("2001:db8:ffff::1"), netip.Addr{}); err != nil {
			t.Fatalf("requireBIRDNextHops: %v", err)
		}
		if err := requireBIRDNextHops(decoded, netip.MustParseAddr("2001:db8:ffff::1"), linkLocal); err == nil {
			t.Fatal("a route without a link-local passed as one with it")
		}
	})
	t.Run("BIRD route without a next-hop line", func(t *testing.T) {
		if _, err := birdVPNNextHops(table, "65001:100", "2001:db8:9999::/48"); err == nil {
			t.Fatal("a route with no bgp_next_hop line was accepted")
		}
	})
	t.Run("BIRD without the route", func(t *testing.T) {
		if _, err := birdVPNNextHops(table, "65001:200", "2001:db8:4659::/48"); err == nil {
			t.Fatal("a route under another RD was accepted")
		}
	})
}
