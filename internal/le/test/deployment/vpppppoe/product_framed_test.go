// Design: docs/labs/pppoe-interop.md -- the framed-route oracle over pinned VPP output.
// Related: product_framed.go -- productFramedRoute, framedUsers, framedClients.
package testvpppppoe

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// productFramedFixture is the shape VPP v25.10 format_fib_entry and
// format_fib_path print for a ze-pppoe route whose one path is attached to
// the session interface at preference 1: the path type name on the path
// header and the interface alone on the next line.
const productFramedFixture = `203.0.113.128/25 fib:0 index:12 locks:2
  ze-pppoe refs:1 entry-flags:attached, src-flags:added,contributing,active,
    path-list:[15] locks:1 flags:shared, uPRF-list:14 len:1 itfs:[3, ]
      path:[17] pl-index:15 ip4 weight=1 pref=1 attached:  oper-flags:resolved,
        pppoe_session0

 forwarding:   unicast-ip4-chain
  [@0]: dpo-load-balance: [proto:ip4 index:12 buckets:1 uRPF:14 to:[0:0]]
`

// The framed route must be a device route: one owned attached path naming
// the session, at the route's metric. A path through the RADIUS gateway, any
// other next hop, another preference, another interface or a foreign source
// is the shape Ze installs when it keeps the gateway, and is refused.
func TestProductFramedRouteRequiresAttachedSessionPath(t *testing.T) {
	if err := productFramedRoute(productFramedFixture, "pppoe_session0"); err != nil {
		t.Fatal(err)
	}
	gateway := strings.Replace(strings.Replace(productFramedFixture, "attached:  oper", "attached-nexthop: oper", 1),
		"        pppoe_session0", "        203.0.113.77 pppoe_session0 (p2p)", 1)
	otherHop := strings.Replace(gateway, "203.0.113.77", "10.20.0.2", 1)
	for name, text := range map[string]string{
		"gateway next hop":   gateway,
		"another next hop":   otherHop,
		"preference zero":    strings.Replace(productFramedFixture, "pref=1", "pref=0", 1),
		"another interface":  strings.Replace(productFramedFixture, "        pppoe_session0", "        pppoe_session1", 1),
		"foreign source":     strings.Replace(productFramedFixture, "ze-pppoe", "api", 1),
		"another prefix":     strings.Replace(productFramedFixture, "203.0.113.128/25", "203.0.113.0/24", 1),
		"no owned path":      "203.0.113.128/25 fib:0 index:12\n  ze-pppoe refs:1\n forwarding: pppoe_session0\n",
		"empty table answer": "",
	} {
		if err := productFramedRoute(text, "pppoe_session0"); err == nil {
			t.Fatalf("%s accepted as a device route through the session: %s", name, text)
		}
	}
}

// The subscriber host route keeps its old acceptance: an attached next hop
// through the session at preference 0, which the device-route shape refuses.
func TestProductOwnedPathShapes(t *testing.T) {
	host := netip.MustParsePrefix("10.20.0.2/32")
	if err := productOwnedPath(productOwnedPathFixture, host, "pppoe_session0", 0, productPathAnyAttached); err != nil {
		t.Fatal(err)
	}
	if productOwnedPath(productOwnedPathFixture, host, "pppoe_session0", 0, productPathAttachedOnly) == nil {
		t.Fatal("an attached next hop passed as a device route")
	}
	if productOwnedPath(productOwnedPathFixture, host, "pppoe_session0", 0, productPathUnspecified) == nil {
		t.Fatal("an unspecified path shape was accepted")
	}
}

// FreeRADIUS must return exactly the route the oracle expects, and admit Ze
// on every candidate network the suite may choose.
func TestProductFramedFixtures(t *testing.T) {
	if !strings.Contains(framedUsers(), "\tFramed-Route = \"203.0.113.128/25 203.0.113.77 1\"\n") {
		t.Fatalf("users file lacks the framed route: %s", framedUsers())
	}
	candidates := []interoplab.Subnet{{IPv4: netip.MustParsePrefix("198.19.40.0/24")}, {IPv4: netip.MustParsePrefix("198.19.41.0/24")}}
	clients := framedClients(candidates)
	for _, want := range []string{"ipaddr = 198.19.40.0/24", "ipaddr = 198.19.41.0/24", "secret = " + radiusSecret, "require_message_authenticator = yes"} {
		if !strings.Contains(clients, want) {
			t.Fatalf("clients.conf lacks %q: %s", want, clients)
		}
	}
	auth := framedAuth("198.19.40.6", "198.19.40.5")
	if !strings.Contains(auth, "source-address 198.19.40.6;") || !strings.Contains(auth, "address 198.19.40.5;") {
		t.Fatalf("RADIUS block does not name the NAS and server: %s", auth)
	}
}
