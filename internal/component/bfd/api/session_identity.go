// Design: rfc/short/rfc5882.md -- one session per remote system, whatever asks for it
// Detail: events.go -- SessionRequest.Key, the identity this file canonicalizes

package api

import "net/netip"

// LinkAddress is one address configured on a link, with the connected prefix
// it belongs to.
type LinkAddress struct {
	Addr   netip.Addr
	Prefix netip.Prefix
}

// Link is one candidate egress link: its kernel device name, the VRF the link
// is in, and the addresses it carries. The BFD component binds a client's
// logical interface before passing the backend's link table to Canonical;
// a client never assembles one.
type Link struct {
	Name string
	// VRF is the routing instance this link is in, DefaultVRF when it is in
	// none. A link in another VRF is not a candidate: the same prefix can be
	// configured in two VRFs and reach two different systems, so a request
	// MUST NOT be given a link from a VRF it does not name.
	VRF   string
	Addrs []LinkAddress
}

// Topology is what Canonical reads to complete a request: the links this
// system has.
type Topology struct {
	Links []Link
}

// Canonical returns the request with the five fields of Key reduced to the
// form every client of the BFD service must arrive at.
//
// RFC 5882 Section 4.4: "If multiple control protocols wish to establish BFD
// sessions with the same remote system for the same data protocol, all MUST
// share a single BFD session."
//
// Key is peer, local, interface, VRF and hop mode, so two clients share one
// session only when they produce the same five fields, and the clients do not
// produce them the same way. OSPF (internal/plugins/ospf/bfd_client.go,
// bfdRequestForNeighbor) always names the interface it runs on and the address
// it runs from. BGP (internal/component/bgp/reactor/peer_bfd.go,
// bfdRequestFor) names an interface only when the operator wrote the optional
// `bfd interface` leaf, and a local address only when the peer has one.
// Without this canonicalization, OSPF and BGP to one neighbor on one link
// build two keys, so the engine opens two sessions and puts two packet streams
// on the link, which is what Section 4.4 forbids.
//
// The fields a client left out are derived from the link the peer sits on, so
// a request that names less arrives at the key a request that names more
// already has:
//
//   - The interface is the link that holds the request's local address, or,
//     when the request has no local address, the one link whose connected
//     prefix contains the peer.
//   - The local address is that link's address inside the prefix that contains
//     the peer.
//
// The derivation is refused when it is ambiguous: no link matches, or more
// than one does. An IPv6 link-local peer is the standing example, because
// every link carries fe80::/64 and only the client knows which one it meant. A
// refusal leaves the request as the client wrote it, and the key half is then
// finished by the engine: engine.Loop.EnsureSession joins a request with unset
// fields to the ONE live session it cannot be told apart from, and gives it
// its own session only when it could mean more than one. Both OSPF families,
// and a BGP peer carrying the `bfd interface` leaf, name the interface
// themselves and never reach the derivation.
//
// Multi-hop takes a different reduction, because Section 4.4 binds it too:
// canonicalMultiHop below.
func (r SessionRequest) Canonical(t Topology) SessionRequest {
	if r.VRF == "" {
		r.VRF = DefaultVRF
	}
	if r.Mode != SingleHop {
		return r.canonicalMultiHop()
	}
	if r.Interface != "" && r.Local.IsValid() {
		return r
	}
	name, local, ok := r.deriveLink(t.Links)
	if !ok {
		return r
	}
	r.Interface = name
	r.Local = local
	return r
}

// canonicalMultiHop is the reduction for a session that is not on a link.
//
// RFC 5882 Section 4.4 binds multi-hop exactly as it binds single-hop, and the
// clients disagree there too: parseMultiHopSession requires the operator to
// write `local`, while bfdRequestFor leaves Local invalid for a peer with no
// `connection local ip`.
//
// The interface is cleared, because a multi-hop session is routed rather than
// put on one link: SessionRequest.Interface documents itself as single-hop
// only, the engine's first-packet index keys on it, and a received multi-hop
// packet names no ingress link, so a session that kept an interface could not
// be demultiplexed by it either.
//
// The local address is NOT derived. A multi-hop session is keyed on the peer,
// the VRF and the mode, plus the local address only where a client pinned one,
// because RFC 5883 Section 4.1 makes a pinned address part of the session's
// identity: "Multiple sessions between the same pair of systems must have at
// least one endpoint address distinct from one another." A client that pinned
// none joins the session to that peer in the engine
// (engine.Loop.EnsureSession), which needs no route lookup and so answers in
// every VRF and with no interface backend.
func (r SessionRequest) canonicalMultiHop() SessionRequest {
	r.Interface = ""
	return r
}

// deriveLink returns the one link consistent with what the request already
// names, and the local address to use on it. ok is false when no link matches
// or more than one does.
func (r SessionRequest) deriveLink(links []Link) (string, netip.Addr, bool) {
	var (
		name  string
		local netip.Addr
		found int
	)
	for _, l := range links {
		if l.VRF != r.VRF {
			continue
		}
		if r.Interface != "" && l.Name != r.Interface {
			continue
		}
		addr, ok := r.addressOn(l)
		if !ok {
			continue
		}
		found++
		if found > 1 {
			return "", netip.Addr{}, false
		}
		name, local = l.Name, addr
	}
	return name, local, found == 1
}

// addressOn reports whether the link can carry the request, and with which
// local address. A request that names its local address selects the link that
// holds that address and keeps it. A request that does not selects the link
// whose connected prefix contains the peer, and takes that prefix's address.
func (r SessionRequest) addressOn(l Link) (netip.Addr, bool) {
	for _, a := range l.Addrs {
		if r.Local.IsValid() {
			if a.Addr == r.Local {
				return r.Local, true
			}
			continue
		}
		if a.Prefix.Contains(r.Peer) {
			return a.Addr, true
		}
	}
	return netip.Addr{}, false
}
