// Design: docs/architecture/testing/interop.md -- the L2TP subscriber of Ze's LAN DNS resolver.
// Related: dns_resolver.go -- the predicates under test.
package l2tp

import (
	"net/netip"
	"strings"
	"testing"
)

var testResolver = netip.MustParseAddr(localPPPAddress)

// busyboxAnswer is busybox 1.37 nslookup's reply to `-type=a` from a
// resolver that does not set AA.
const busyboxAnswer = "Server:\t\t10.100.0.1\nAddress:\t10.100.0.1:53\n\n" +
	"Non-authoritative answer:\nName:\twww.lab.example\nAddress: 192.0.2.10\n\n" +
	"Name:\twww.lab.example\nAddress: 192.0.2.11\n\n"

// VALIDATES: peerDNSIsResolver accepts the resolv.conf pppd writes when IPCP
// gave the resolver as both DNS servers, and refuses each way IPCP can have
// given the subscriber something else.
// PREVENTS: assertion 2 passing when the pool's dns-primary never reached
// the subscriber (an empty file), or when it reached it as another address.
func TestPeerDNSIsResolver(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		refuse string
	}{
		{name: "primary and secondary are the resolver", file: "nameserver 10.100.0.1\nnameserver 10.100.0.1\n"},
		{name: "primary alone is the resolver", file: "nameserver 10.100.0.1\n"},
		{name: "no server negotiated", file: "", refuse: "names no DNS server"},
		{name: "primary is another server", file: "nameserver 8.8.8.8\nnameserver 10.100.0.1\n", refuse: "DNS server 1 is 8.8.8.8"},
		{name: "secondary is another server", file: "nameserver 10.100.0.1\nnameserver 8.8.4.4\n", refuse: "DNS server 2 is 8.8.4.4"},
		{name: "server is not an address", file: "nameserver resolver\n", refuse: "not an address"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := peerDNSIsResolver(test.file, testResolver)
			requireRefusal(t, err, test.refuse)
		})
	}
}

// VALIDATES: routeLeavesPPP accepts the route `ip route get` prints through
// the PPP interface, and refuses the lab interface and an unreadable route.
// PREVENTS: assertion 4 resolving from the LAC's lab address rather than as
// the subscriber, which would prove nothing about allow-from or the link.
func TestRouteLeavesPPP(t *testing.T) {
	cases := []struct {
		name   string
		route  string
		refuse string
	}{
		{name: "over the PPP link", route: "10.100.0.1 dev ppp0 src 10.100.0.2 uid 0 \n    cache \n"},
		{name: "over the lab interface", route: "10.100.0.1 via 172.29.0.1 dev eth0 src 172.29.0.3\n", refuse: "leaves eth0"},
		{name: "no device", route: "RTNETLINK answers: Network is unreachable\n", refuse: "names no device"},
		{name: "device word last", route: "10.100.0.1 dev", refuse: "names no device"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			requireRefusal(t, routeLeavesPPP(test.route), test.refuse)
		})
	}
}

// VALIDATES: nslookupAnswer returns the A records busybox prints for the
// name, and refuses a failed lookup, a reply from another server, a reply
// with no record, and output with no reply at all.
// PREVENTS: assertion 4 passing on a REFUSED or SERVFAIL answer, on an
// answer from a server other than Ze's, or on a lookup that timed out.
func TestNslookupAnswer(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   int
		refuse string
	}{
		{name: "busybox answer", output: busyboxAnswer, want: 2},
		{name: "hash port form", output: strings.Replace(busyboxAnswer, "10.100.0.1:53", "10.100.0.1#53", 1), want: 2},
		{name: "refused", output: "Server:\t\t10.100.0.1\nAddress:\t10.100.0.1:53\n\n** server can't find www.lab.example: REFUSED\n\n",
			refuse: "did not resolve www.lab.example: REFUSED"},
		{name: "servfail", output: "** server can't find www.lab.example: SERVFAIL\n", refuse: "SERVFAIL"},
		{name: "another server", output: strings.Replace(busyboxAnswer, "10.100.0.1:53", "8.8.8.8:53", 1), refuse: "answer came from 8.8.8.8:53"},
		{name: "another port", output: strings.Replace(busyboxAnswer, "10.100.0.1:53", "10.100.0.1:5353", 1), refuse: "answer came from"},
		{name: "no record", output: "Server:\t\t10.100.0.1\nAddress:\t10.100.0.1:53\n\n", refuse: "answered no address"},
		{name: "record for another name", output: strings.ReplaceAll(busyboxAnswer, "Name:\twww.lab.example", "Name:\tother.lab.example"),
			refuse: "answered no address"},
		{name: "timed out", output: ";; connection timed out; no servers could be reached\n", refuse: "printed no server"},
		{name: "record not an address", output: strings.Replace(busyboxAnswer, "192.0.2.10", "www", 1), refuse: "not an address"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			addresses, err := nslookupAnswer(test.output, testResolver, subscriberQueryName)
			requireRefusal(t, err, test.refuse)
			if len(addresses) != test.want {
				t.Fatalf("addresses = %v, want %d", addresses, test.want)
			}
		})
	}
}

// VALIDATES: requireAddresses accepts Knot's RRset in either order and
// refuses a missing, an extra and a repeated record.
// PREVENTS: assertion 4 passing on half the RRset or on a record Knot does
// not serve.
func TestRequireAddresses(t *testing.T) {
	ten := netip.MustParseAddr("192.0.2.10")
	eleven := netip.MustParseAddr("192.0.2.11")
	other := netip.MustParseAddr("192.0.2.99")
	cases := []struct {
		name   string
		got    []netip.Addr
		refuse string
	}{
		{name: "exact", got: []netip.Addr{ten, eleven}},
		{name: "reversed", got: []netip.Addr{eleven, ten}},
		{name: "missing", got: []netip.Addr{ten}, refuse: "want Knot's RRset"},
		{name: "extra", got: []netip.Addr{ten, eleven, other}, refuse: "want Knot's RRset"},
		{name: "repeated", got: []netip.Addr{ten, ten}, refuse: "want Knot's RRset"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			requireRefusal(t, requireAddresses(test.got, subscriberAddresses), test.refuse)
		})
	}
}

// requireRefusal holds err to nil when refuse is empty, and otherwise to an
// error whose text carries refuse.
func requireRefusal(t *testing.T, err error, refuse string) {
	t.Helper()
	if refuse == "" {
		if err != nil {
			t.Fatalf("refused: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("accepted, want a refusal containing %q", refuse)
	}
	if !strings.Contains(err.Error(), refuse) {
		t.Fatalf("refusal %q does not contain %q", err, refuse)
	}
}
