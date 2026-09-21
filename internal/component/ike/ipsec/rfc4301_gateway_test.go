// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: config.go -- parseSiteToSitePeer, and traffic_selector.go -- parseTrafficSelectors, the producers
// RFC: rfc/short/rfc4301.md -- configuring a security gateway for destination ranges (Section 4.5.3)
package ipsec

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// rfc4301GatewayTree builds vpn { ipsec { site-to-site { peer <name> { remote-address
// <gateway>; traffic-selector <n> { remote { prefix <range> } } } } } } with one selector
// entry per range given, so the peer IS the administrator's statement that the ranges
// are reached through that gateway.
func rfc4301GatewayTree(name, gateway string, ranges ...string) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	peer := config.NewTree()
	peer.Set("remote-address", gateway)
	for i, r := range ranges {
		ts := config.NewTree()
		ts.GetOrCreateContainer("local").Set("prefix", "10.0.0.0/8")
		ts.GetOrCreateContainer("remote").Set("prefix", r)
		peer.AddListEntry("traffic-selector", strconv.Itoa(i+1), ts)
	}
	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", name, peer)
	return tree
}

// VALIDATES: RFC4301-4.5.3-1. The administrator configures the address of a security
// gateway and the ranges of destination addresses that require it, as one site-to-site
// peer carrying a remote address and a list of remote prefixes, and the parsed peer
// holds the gateway address and every range as written.
// PREVENTS: a gateway address or a destination range that the interface accepts and the
// model loses, which sends the range's traffic in the clear.
// RFC requirement: RFC4301-4.5.3-1 positive -- a peer's remote address and remote prefixes are read as the gateway and the destination ranges that require it.
func TestRFC4301GatewayIsConfiguredForDestinationRanges(t *testing.T) {
	cfg, err := ParseIPsecConfig(rfc4301GatewayTree("branch", "198.51.100.1", "10.2.0.0/16", "10.3.0.0/24"))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	peer, ok := cfg.Peers["branch"]
	if !ok {
		t.Fatalf("no peer named branch was parsed; got %v", cfg.Peers)
	}
	if peer.RemoteAddress != "198.51.100.1" {
		t.Fatalf("gateway address = %q, want 198.51.100.1", peer.RemoteAddress)
	}
	if len(peer.TrafficSelectors) != 2 {
		t.Fatalf("destination ranges = %d, want 2", len(peer.TrafficSelectors))
	}
	for i, want := range []string{"10.2.0.0/16", "10.3.0.0/24"} {
		if got := peer.TrafficSelectors[i].RemotePrefix; got == nil || got.String() != want {
			t.Errorf("range[%d] = %v, want %s", i, got, want)
		}
	}
}

// VALIDATES: RFC4301-4.5.3-1. A destination range the interface cannot represent, a
// malformed prefix, is refused by name rather than widened or dropped.
// PREVENTS: an administrator's range being silently replaced by a different one.
// RFC requirement: RFC4301-4.5.3-1 negative -- a malformed destination range is refused naming the peer and the selector.
func TestRFC4301GatewayRefusesAMalformedDestinationRange(t *testing.T) {
	for _, bad := range []string{"10.2.0.0/33", "10.2.0.0", "not-a-prefix"} {
		t.Run(bad, func(t *testing.T) {
			_, err := ParseIPsecConfig(rfc4301GatewayTree("branch", "198.51.100.1", bad))
			if err == nil {
				t.Fatalf("the range %q was accepted", bad)
			}
			if !strings.Contains(err.Error(), `peer "branch"`) || !strings.Contains(err.Error(), bad) {
				t.Fatalf("error = %q, want it to name the peer and the range", err)
			}
		})
	}
}
