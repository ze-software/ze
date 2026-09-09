// VALIDATES: RFC 5882 sec 4.4 -- a BGP request that names neither its egress
// interface nor its local address reaches the SAME api.Key as the OSPF request
// for the same neighbor on the same link, so the two protocols share one BFD
// session instead of putting two packet streams on the link.
// PREVENTS: the key-shaped half of the single-session requirement going
// unproven. The engine already shares a session per Key
// (engine/rfc5882_shared_session_test.go); that guarantee is worth nothing
// while two clients for one neighbor build two different keys, which is what
// Ze did until Canonical existed.
package api

import (
	"net/netip"
	"testing"
)

// linkTable is the link set the derivation runs over: two links in one system,
// each with an IPv4 subnet and an IPv6 link-local. eth0 holds the subnet the
// neighbor 172.30.0.10 is reachable on.
func linkTable() []Link {
	return []Link{
		{
			Name: "eth0",
			Addrs: []LinkAddress{
				{Addr: netip.MustParseAddr("172.30.0.2"), Prefix: netip.MustParsePrefix("172.30.0.0/24")},
				{Addr: netip.MustParseAddr("fe80::a"), Prefix: netip.MustParsePrefix("fe80::/64")},
			},
		},
		{
			Name: "eth1",
			Addrs: []LinkAddress{
				{Addr: netip.MustParseAddr("10.1.0.2"), Prefix: netip.MustParsePrefix("10.1.0.0/24")},
				{Addr: netip.MustParseAddr("fe80::b"), Prefix: netip.MustParsePrefix("fe80::/64")},
			},
		},
	}
}

// sharedKey is the one key every client for neighbor 172.30.0.10 on eth0 must
// arrive at. Three tests outside this package assert their own real request
// builder reaches it: TestStrictPeerRequestReachesTheSharedKey
// (internal/component/bgp/reactor) drives bfdRequestFor,
// TestOSPFNeighborRequestReachesTheSharedKey (internal/plugins/ospf) drives
// bfdRequestForNeighbor, and TestPinnedSessionReachesTheSharedKey
// (internal/component/bfd) drives the pinned-session parser. No single test
// can drive all three, because each builder is unexported in its own package.
func sharedKey() Key {
	return Key{
		Peer:      netip.MustParseAddr("172.30.0.10"),
		Local:     netip.MustParseAddr("172.30.0.2"),
		Interface: "eth0",
		VRF:       DefaultVRF,
		Mode:      SingleHop,
	}
}

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4). Sharing is
// looked up by api.Key, so the requirement binds the KEY as much as the
// registry that looks it up. Canonical derives the interface and the local
// address a client left out, so the OSPF shape (both named), the BGP shape
// (neither named) and the two half-named shapes in between all reduce to one
// key and therefore to one session.
func TestCanonicalCollapsesEveryClientShapeOntoOneKey(t *testing.T) {
	peer := netip.MustParseAddr("172.30.0.10")
	local := netip.MustParseAddr("172.30.0.2")
	shapes := map[string]SessionRequest{
		"ospf names the interface and the local address": {Peer: peer, Local: local, Interface: "eth0", Mode: SingleHop},
		"bgp names neither":                     {Peer: peer, Mode: SingleHop},
		"bgp names only its local address":      {Peer: peer, Local: local, Mode: SingleHop},
		"bgp names only its bfd interface leaf": {Peer: peer, Interface: "eth0", Mode: SingleHop},
	}
	want := sharedKey()
	for name, req := range shapes {
		if got := req.Canonical(linkTable()).Key(); got != want {
			t.Errorf("%s: key = %+v, want %+v (a second key here is a second session on one link)", name, got, want)
		}
	}
}

// TestCanonicalRefusesToGuessAnAmbiguousLink covers the case the derivation
// must NOT answer. An IPv6 link-local peer is on fe80::/64, which every link
// carries, so the link table cannot say which one the client meant. Inventing
// one would merge a session onto a link it may not be on, so the request is
// returned as written and gets its own key.
func TestCanonicalRefusesToGuessAnAmbiguousLink(t *testing.T) {
	linkLocal := SessionRequest{Peer: netip.MustParseAddr("fe80::c"), Mode: SingleHop}
	got := linkLocal.Canonical(linkTable())
	if got.Interface != "" || got.Local.IsValid() {
		t.Errorf("link-local peer on two candidate links derived interface=%q local=%v, want both untouched", got.Interface, got.Local)
	}

	// A peer on no connected prefix is the other refusal: nothing to derive
	// from, so the request keeps the identity its client gave it.
	offLink := SessionRequest{Peer: netip.MustParseAddr("198.51.100.7"), Mode: SingleHop}
	if got := offLink.Canonical(linkTable()); got.Interface != "" || got.Local.IsValid() {
		t.Errorf("off-link peer derived interface=%q local=%v, want both untouched", got.Interface, got.Local)
	}

	// A named interface the peer is not on: the request names eth1 while the
	// peer sits on eth0's subnet, so no link is consistent with it.
	wrongLink := SessionRequest{Peer: netip.MustParseAddr("172.30.0.10"), Interface: "eth1", Mode: SingleHop}
	if got := wrongLink.Canonical(linkTable()); got.Local.IsValid() {
		t.Errorf("peer off the named interface derived local=%v, want no local address invented", got.Local)
	}
}

// TestCanonicalDefaultsTheVRFForEveryMode is the second half of the identity
// reduction: an empty VRF and "default" name one routing instance, so they must
// not produce two keys. Multi-hop has no link to derive from, so the VRF is the
// only field Canonical touches there.
func TestCanonicalDefaultsTheVRFForEveryMode(t *testing.T) {
	peer := netip.MustParseAddr("203.0.113.9")
	unset := SessionRequest{Peer: peer, Mode: MultiHop}.Canonical(linkTable())
	named := SessionRequest{Peer: peer, VRF: DefaultVRF, Mode: MultiHop}.Canonical(linkTable())
	if unset.Key() != named.Key() {
		t.Errorf("empty VRF key %+v != %q VRF key %+v", unset.Key(), DefaultVRF, named.Key())
	}
	if unset.Interface != "" || unset.Local.IsValid() {
		t.Errorf("multi-hop request gained interface=%q local=%v, want no link derivation", unset.Interface, unset.Local)
	}
}
