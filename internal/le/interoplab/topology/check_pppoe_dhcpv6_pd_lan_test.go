package topology

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// pdTestCapture is what `tcpdump -n -tt -v -r` prints for two RAs, shaped by
// icmp6_print and icmp6_opt_print (print-icmp6.c): the header line at the
// margin, the RA fields and each option on indented lines.
const pdTestCapture = `1700000010.250000 IP6 (flowlabel 0x1, hlim 255, next-header ICMPv6 (58) payload length: 88) fe80::2 > ff02::1: [icmp6 sum ok] ICMP6, router advertisement, length 88
	hop limit 64, Flags [none], pref medium, router lifetime 1800s, reachable time 0ms, retrans timer 0ms
	  source link-address option (1), length 8 (1): 02:42:ac:1f:b5:03
	  prefix info option (3), length 32 (4): 2001:db8:2000:1::/64, Flags [onlink, auto], valid time 109s, pref. time 49s
	  route info option (24), length 24 (3):  2001:db8:2000::/60, pref=medium, lifetime=109s
1700000011.000000 IP6 (flowlabel 0x1, hlim 255, next-header ICMPv6 (58) payload length: 24) fe80::2 > ff02::1: [icmp6 sum ok] ICMP6, router advertisement, length 24
	hop limit 64, Flags [none], pref medium, router lifetime 0s, reachable time 0ms, retrans timer 0ms
	  source link-address option (1), length 8 (1): 02:42:ac:1f:b5:03
`

// TestPPPoEDHCPv6PDLANVerdicts proves the LAN half of the
// pppoe-dhcpv6-pd-ze-client checker refuses what each red walk leaves: an RA
// advertising the lease's whole lifetimes long after the Reply (the lifetime
// bound removed), and no RA with Router Lifetime 0 after the session ended
// (the W input forced true). Method: verdicts over fixed capture, log and
// command output, each accepting the correct state and refusing the broken
// one.
func TestPPPoEDHCPv6PDLANVerdicts(t *testing.T) {
	delegated := netip.MustParsePrefix("2001:db8:2000::/60")
	subnet, err := delegatedSubnet(delegated, pdSubnetID)
	if err != nil {
		t.Fatalf("delegatedSubnet: %v", err)
	}
	if subnet.String() != "2001:db8:2000:1::/64" {
		t.Errorf("subnet-id 1 of %s is %s, want 2001:db8:2000:1::/64", delegated, subnet)
	}
	if host := subnetHost(subnet, 1); host.String() != "2001:db8:2000:1::1" {
		t.Errorf("::1 of %s is %s", subnet, host)
	}
	if _, err := delegatedSubnet(delegated, 16); err == nil {
		t.Error("subnet-id 16 of a /60: accepted")
	}

	advertisements, err := parseAdvertisements(pdTestCapture)
	if err != nil {
		t.Fatalf("parseAdvertisements: %v", err)
	}
	if len(advertisements) != 2 {
		t.Fatalf("parsed %d RAs, want 2", len(advertisements))
	}
	first := advertisements[0]
	if first.routerLifetime != 1800 || len(first.prefixes) != 1 || len(first.routes) != 1 {
		t.Fatalf("first RA parsed as %+v", first)
	}
	if first.prefixes[0].valid != 109 || first.prefixes[0].preferred != 49 || first.routes[0].valid != 109 {
		t.Errorf("first RA lifetimes parsed as %+v %+v", first.prefixes[0], first.routes[0])
	}

	// The Reply left eleven seconds before the RA: 109s and 49s remain.
	reply, err := accelLogTime("[2023-11-14 22:13:19]: info: ppp0: send [DHCPv6 Reply XID=1 <IA-PD {IA-Prefix 2001:db8:2000::/60 pref_lifetime=60 valid_lifetime=120}>]")
	if err != nil {
		t.Fatalf("accelLogTime: %v", err)
	}
	if want := time.Unix(1699999999, 0); !reply.Equal(want) {
		t.Fatalf("accelLogTime read %s, want %s", reply, want)
	}
	replies := []time.Time{reply}
	if err := advertisedLifetimesVerdict(advertisements, replies, subnet, delegated); err != nil {
		t.Errorf("bounded lifetimes: %v", err)
	}
	unbounded := strings.Replace(pdTestCapture, "valid time 109s, pref. time 49s", "valid time 120s, pref. time 60s", 1)
	if err := judgeCapture(t, unbounded, replies, subnet, delegated); err == nil {
		t.Error("the lease's whole lifetimes eleven seconds after the Reply: accepted")
	}
	noRIO := strings.Replace(pdTestCapture, "route info option (24)", "unknown option (99)", 1)
	if err := judgeCapture(t, noRIO, replies, subnet, delegated); err == nil {
		t.Error("an RA with no RIO for the delegated prefix: accepted")
	}
	zeroed := strings.Replace(pdTestCapture, "router lifetime 1800s", "router lifetime 0s", 1)
	if err := judgeCapture(t, zeroed, replies, subnet, delegated); err == nil {
		t.Error("Router Lifetime 0 with the WAN up and the prefix held: accepted")
	}
	early := []time.Time{time.Unix(1700000005, 0)}
	if err := advertisedLifetimesVerdict(advertisements, early, subnet, delegated); !errors.Is(err, errNoLateAdvertisement) {
		t.Errorf("only an RA five seconds after its Reply: %v, want errNoLateAdvertisement", err)
	}
	skewed := []time.Time{time.Unix(1700003600, 0)}
	if err := advertisedLifetimesVerdict(advertisements, skewed, subnet, delegated); err == nil {
		t.Error("a Reply logged after every RA (a clock skew): accepted")
	}

	since := time.Unix(1700000010, 500000000)
	if err := zeroLifetimeVerdict(advertisements, since); err != nil {
		t.Errorf("Router Lifetime 0 after the session ended: %v", err)
	}
	stuck := strings.Replace(pdTestCapture, "router lifetime 0s", "router lifetime 1800s", 1)
	stuckAdverts, err := parseAdvertisements(stuck)
	if err != nil {
		t.Fatalf("parseAdvertisements: %v", err)
	}
	if err := zeroLifetimeVerdict(stuckAdverts, since); !errors.Is(err, errNoZeroLifetime) {
		t.Errorf("Router Lifetime kept after the session ended: %v, want errNoZeroLifetime", err)
	}
	if err := zeroLifetimeVerdict(advertisements, since.Add(-10*time.Second)); err == nil {
		t.Error("a zero lifetime ten seconds after the termination, past the next RA: accepted")
	}

	if err := unitAddressVerdict("9: lan0    inet6 2001:db8:2000:1::1/64 scope global dynamic \\       valid_lft 117sec preferred_lft 57sec\n", netip.MustParsePrefix("2001:db8:2000:1::1/64")); err != nil {
		t.Errorf("lan0's bounded address: %v", err)
	}
	if err := unitAddressVerdict("9: lan0    inet6 2001:db8:2000:1::1/64 scope global \\       valid_lft forever preferred_lft forever\n", netip.MustParsePrefix("2001:db8:2000:1::1/64")); err == nil {
		t.Error("lan0's address with forever lifetimes: accepted")
	}
	router := netip.MustParseAddr("fe80::2")
	if err := rioRouteVerdict("2001:db8:2000::/60 via fe80::2 dev eth1 proto ra metric 1003 expires 108sec pref medium\n", delegated, router, "eth1"); err != nil {
		t.Errorf("RIO route: %v", err)
	}
	if err := rioRouteVerdict("", delegated, router, "eth1"); err == nil {
		t.Error("no RIO route: accepted")
	}
}

// judgeCapture parses capture and runs advertisedLifetimesVerdict on it.
func judgeCapture(t *testing.T, capture string, replies []time.Time, subnet, delegated netip.Prefix) error {
	t.Helper()
	advertisements, err := parseAdvertisements(capture)
	if err != nil {
		t.Fatalf("parseAdvertisements: %v", err)
	}
	return advertisedLifetimesVerdict(advertisements, replies, subnet, delegated)
}
