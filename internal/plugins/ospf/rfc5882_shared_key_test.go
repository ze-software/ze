// VALIDATES: RFC 5882 sec 4.4 -- the request OSPF builds for a Full neighbor is
// already the canonical key, so a BGP peer to the same neighbor joins OSPF's
// session rather than opening a second one.
// PREVENTS: a change to bfdRequestForNeighbor that stops naming the interface
// or the interface address, which would leave BGP's derived key with nothing to
// meet and put two BFD packet streams on one link.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4). This is
// the OSPF half of the shared key. bfdRequestForNeighbor names both the link
// and the address it runs from, so api.SessionRequest.Canonical has nothing to
// derive and returns the key unchanged, with or without a link table. The BGP
// half, which does need the derivation, is
// TestStrictPeerRequestReachesTheSharedKey
// (internal/component/bgp/reactor/config_bfd_strict_test.go), and it asserts
// this same key.
func TestOSPFNeighborRequestReachesTheSharedKey(t *testing.T) {
	want := api.Key{
		Peer:      netip.MustParseAddr("172.30.0.10"),
		Local:     netip.MustParseAddr("172.30.0.2"),
		Interface: "eth0",
		VRF:       api.DefaultVRF,
		Mode:      api.SingleHop,
	}
	req := bfdRequestForNeighbor(
		netip.MustParseAddr("172.30.0.10"),
		netip.MustParseAddr("172.30.0.2"),
		"eth0",
		enabledBFD(),
	)
	if got := req.Canonical(nil).Key(); got != want {
		t.Fatalf("ospf key = %+v, want %+v (BGP derives this key for the same neighbor)", got, want)
	}
}
