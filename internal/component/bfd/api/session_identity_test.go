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
			VRF:  DefaultVRF,
			Addrs: []LinkAddress{
				{Addr: netip.MustParseAddr("172.30.0.2"), Prefix: netip.MustParsePrefix("172.30.0.0/24")},
				{Addr: netip.MustParseAddr("fe80::a"), Prefix: netip.MustParsePrefix("fe80::/64")},
			},
		},
		{
			Name: "eth1",
			VRF:  DefaultVRF,
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
		if got := req.Canonical(Topology{Links: linkTable()}).Key(); got != want {
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
	got := linkLocal.Canonical(Topology{Links: linkTable()})
	if got.Interface != "" || got.Local.IsValid() {
		t.Errorf("link-local peer on two candidate links derived interface=%q local=%v, want both untouched", got.Interface, got.Local)
	}

	// A peer on no connected prefix is the other refusal: nothing to derive
	// from, so the request keeps the identity its client gave it.
	offLink := SessionRequest{Peer: netip.MustParseAddr("198.51.100.7"), Mode: SingleHop}
	if got := offLink.Canonical(Topology{Links: linkTable()}); got.Interface != "" || got.Local.IsValid() {
		t.Errorf("off-link peer derived interface=%q local=%v, want both untouched", got.Interface, got.Local)
	}

	// A named interface the peer is not on: the request names eth1 while the
	// peer sits on eth0's subnet, so no link is consistent with it.
	wrongLink := SessionRequest{Peer: netip.MustParseAddr("172.30.0.10"), Interface: "eth1", Mode: SingleHop}
	if got := wrongLink.Canonical(Topology{Links: linkTable()}); got.Local.IsValid() {
		t.Errorf("peer off the named interface derived local=%v, want no local address invented", got.Local)
	}
}

// TestCanonicalDefaultsTheVRFForEveryMode is the second half of the identity
// reduction: an empty VRF and "default" name one routing instance, so they must
// not produce two keys. Multi-hop has no link to derive from, so the VRF is the
// only field Canonical touches there.
func TestCanonicalDefaultsTheVRFForEveryMode(t *testing.T) {
	peer := netip.MustParseAddr("203.0.113.9")
	unset := SessionRequest{Peer: peer, Mode: MultiHop}.Canonical(Topology{Links: linkTable()})
	named := SessionRequest{Peer: peer, VRF: DefaultVRF, Mode: MultiHop}.Canonical(Topology{Links: linkTable()})
	if unset.Key() != named.Key() {
		t.Errorf("empty VRF key %+v != %q VRF key %+v", unset.Key(), DefaultVRF, named.Key())
	}
	if unset.Interface != "" || unset.Local.IsValid() {
		t.Errorf("multi-hop request gained interface=%q local=%v, want no link derivation", unset.Interface, unset.Local)
	}
}

// multiHopLinks is the link set behind a multi-hop derivation: lo carries the
// loopback source a multi-hop session usually runs from, eth0 is the interface
// the route to a remote system leaves by, and eth9 is in another VRF.
func multiHopLinks() []Link {
	return []Link{
		{
			Name: "eth0",
			VRF:  DefaultVRF,
			Addrs: []LinkAddress{
				{Addr: netip.MustParseAddr("172.30.0.2"), Prefix: netip.MustParsePrefix("172.30.0.0/24")},
				{Addr: netip.MustParseAddr("fe80::a"), Prefix: netip.MustParsePrefix("fe80::/64")},
			},
		},
		{
			Name: "eth9",
			VRF:  "red",
			Addrs: []LinkAddress{
				{Addr: netip.MustParseAddr("172.30.0.2"), Prefix: netip.MustParsePrefix("172.30.0.0/24")},
			},
		},
	}
}

// RFC requirement: RFC5882-4.4-1 positive -- "If multiple control protocols
// wish to establish BFD sessions with the same remote system for the same data
// protocol, all MUST share a single BFD session" (RFC 5882 sec 4.4). The
// requirement does not stop at single-hop. A pinned multi-hop-session names
// its local address, a BGP peer with `connection local ip` names the same one,
// and a client that wrongly named an interface names one too. Canonical clears
// the interface, which plays no part in a routed session, so all three reach
// one key. A client that names no local address is joined in the engine
// (TestMultiHopClientsInAVRFShareOneSession).
//
// MUTATION: stop canonicalMultiHop clearing Interface and the third shape
// keeps eth0 in its key, so this test goes red.
func TestCanonicalCollapsesMultiHopShapesOntoOneKey(t *testing.T) {
	peer := netip.MustParseAddr("203.0.113.9")
	topology := Topology{Links: multiHopLinks()}
	want := Key{
		Peer:  peer,
		Local: netip.MustParseAddr("172.30.0.2"),
		VRF:   DefaultVRF,
		Mode:  MultiHop,
	}
	shapes := map[string]SessionRequest{
		"the pinned entry names its local address":  {Peer: peer, Local: netip.MustParseAddr("172.30.0.2"), Mode: MultiHop},
		"the bgp peer names the same local address": {Peer: peer, Local: netip.MustParseAddr("172.30.0.2"), VRF: DefaultVRF, Mode: MultiHop},
		"a client that wrongly named an interface":  {Peer: peer, Local: netip.MustParseAddr("172.30.0.2"), Interface: "eth0", Mode: MultiHop},
	}
	for name, req := range shapes {
		if got := req.Canonical(topology).Key(); got != want {
			t.Errorf("%s: key = %+v, want %+v", name, got, want)
		}
	}
}

// TestCanonicalRefusesAMultiHopDerivationItCannotMake: Canonical never derives
// a multi-hop local address, over link tables where one address, none, or two
// of the peer's family would be candidates. A source address a client did not
// pin is not part of the session's identity (RFC 5883 sec 4.1 counts only the
// endpoints a session names), and inventing one would split an unpinned client
// from the session it shares.
func TestCanonicalRefusesAMultiHopDerivationItCannotMake(t *testing.T) {
	peer := netip.MustParseAddr("203.0.113.9")
	multiHomed := []Link{{
		Name: "eth0",
		VRF:  DefaultVRF,
		Addrs: []LinkAddress{
			{Addr: netip.MustParseAddr("172.30.0.2"), Prefix: netip.MustParsePrefix("172.30.0.0/24")},
			{Addr: netip.MustParseAddr("10.1.0.2"), Prefix: netip.MustParsePrefix("10.1.0.0/24")},
		},
	}}
	for name, tc := range map[string]struct {
		req   SessionRequest
		links []Link
	}{
		"one v4 source on eth0":    {SessionRequest{Peer: peer, Mode: MultiHop}, multiHopLinks()},
		"no global v6 source":      {SessionRequest{Peer: netip.MustParseAddr("2001:db8::9"), Mode: MultiHop}, multiHopLinks()},
		"two v4 sources on eth0":   {SessionRequest{Peer: peer, Mode: MultiHop}, multiHomed},
		"one v4 source in vrf red": {SessionRequest{Peer: peer, VRF: "red", Mode: MultiHop}, multiHopLinks()},
	} {
		if got := tc.req.Canonical(Topology{Links: tc.links}); got.Local.IsValid() {
			t.Errorf("%s: local = %v, want none derived", name, got.Local)
		}
	}
}

// TestCanonicalWillNotCrossVRFs is the third round-6 finding. The same prefix
// can be configured in two VRFs and reach two different systems, so a link in
// a VRF the request does not name is not a candidate for it: the derivation
// refuses rather than handing back another VRF's interface and address.
func TestCanonicalWillNotCrossVRFs(t *testing.T) {
	peer := netip.MustParseAddr("172.30.0.10")
	inRed := SessionRequest{Peer: peer, VRF: "red", Mode: SingleHop}
	got := inRed.Canonical(Topology{Links: linkTable()})
	if got.Interface != "" || got.Local.IsValid() {
		t.Errorf("a request in VRF red was given interface=%q local=%v from the default VRF", got.Interface, got.Local)
	}

	// The same request with the link actually in red derives from it.
	red := linkTable()
	red[0].VRF = "red"
	if got := inRed.Canonical(Topology{Links: red}); got.Interface != "eth0" {
		t.Errorf("a request in VRF red with a red link: interface = %q, want eth0", got.Interface)
	}

	// And a multi-hop request in a VRF: no link, of either VRF, gives it a
	// local address or an interface.
	multi := SessionRequest{Peer: netip.MustParseAddr("203.0.113.9"), VRF: "red", Interface: "eth9", Mode: MultiHop}
	if got := multi.Canonical(Topology{Links: multiHopLinks()}); got.Local.IsValid() || got.Interface != "" {
		t.Errorf("multi-hop in VRF red: interface=%q local=%v, want neither", got.Interface, got.Local)
	}
}
