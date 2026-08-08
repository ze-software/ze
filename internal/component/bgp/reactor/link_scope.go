// Design: docs/architecture/core-design.md — RFC 2545 Section 3 link-local next-hop condition
// RFC: rfc/short/rfc2545.md
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/network"
)

// linkScope carries the host facts that decide RFC 2545 Section 3's inclusion
// condition, read as one snapshot so every route of one session answers against
// the same view of the interface table.
//
// RFC 2545 Section 3: "The link-local address shall be included in the Next Hop
// field if and only if the BGP speaker shares a common subnet with the entity
// identified by the global IPv6 address carried in the Network Address of Next
// Hop field and the peer the route is being advertised to."
//
// The sentence gives "shares a common subnet with" two objects: the next-hop
// entity, and the peer. peerOnLink answers the peer half, which depends on the
// session alone and so is settled once per snapshot. The next-hop half depends
// on the route being advertised, so linkLocalNextHop answers it per next hop.
type linkScope struct {
	// connected holds the subnets this host is directly attached to.
	connected []netip.Prefix
	// peerOnLink reports whether the speaker shares a common subnet with the
	// peer the route is being advertised to.
	peerOnLink bool
}

// newLinkScope reads the host interface table and settles the peer half of the
// Section 3 condition for peerAddr.
//
// It reads the kernel, so it runs at session establishment and at a config
// reload, never per UPDATE.
func newLinkScope(peerAddr netip.Addr) *linkScope {
	return newLinkScopeFrom(network.ConnectedPrefixes(), peerAddr)
}

// newLinkScopeFrom settles the peer half against an interface table the caller
// has already read, so a fan-out over many peers pays one kernel read for all of
// them (refreshPeerLinkScopes, reactor_iface.go). The slice is treated as
// immutable by every reader, so sharing it is safe.
func newLinkScopeFrom(connected []netip.Prefix, peerAddr netip.Addr) *linkScope {
	return &linkScope{
		connected:  connected,
		peerOnLink: network.SharesSubnet(connected, peerAddr),
	}
}

// linkLocalNextHop returns the link-local address to write after globalNextHop in
// the MP_REACH_NLRI Network Address of Next Hop field, or the zero Addr when the
// field carries the global address alone.
//
// The zero Addr is Section 3's else-branch: "In all other cases a BGP speaker
// shall advertise to its peer in the Network Address field only the global IPv6
// address of the next hop (the value of the Length of Network Address of Next Hop
// field shall be set to 16)."
//
// A nil scope has read no interface table, so it knows of no shared subnet and
// appends nothing. An "if and only if" makes an unproven condition a false one,
// and the alternative would turn a failed read into a permissive answer.
//
// configured is the operator's link-local address for this session. A configured
// address that is not link-local unicast is not appended either: Section 3 names
// the second address "the link-local IPv6 address of the next hop", so writing
// anything else there would break the same sentence it is meant to satisfy.
func (ls *linkScope) linkLocalNextHop(configured, globalNextHop netip.Addr) netip.Addr {
	if ls == nil || !ls.peerOnLink {
		return netip.Addr{}
	}
	if !configured.Is6() || !configured.IsLinkLocalUnicast() {
		return netip.Addr{}
	}
	// Section 3 names the first address "the global IPv6 address of the next hop".
	// An IPv4 address, or a link-local one, in that slot breaks the sentence
	// whatever the second slot holds, so nothing is appended to it: a 32-octet
	// field would make the length octet look right over a first address the
	// section forbids. This guard is independent of the ones on the config leaves
	// that feed the address (parsePeerFromTree in config.go,
	// buildDynamicGroupSettings in ../config/peers.go), because a config leaf is
	// not the only way a value reaches this field.
	global := globalNextHop.Unmap()
	if !global.Is6() || attribute.ValidateGlobalNextHop(global) != nil {
		return netip.Addr{}
	}
	// SharesSubnet asks whether SOME local interface holds a subnet containing the
	// address, not whether it is the SAME interface that faces the peer. A next hop
	// on eth1 and a peer on eth0 therefore both pass, and the peer receives a
	// link-local it cannot resolve on its own link. That is Section 3's literal
	// text: it binds "the BGP speaker", one speaker sharing a common subnet with
	// each of the two entities, and it names no single interface. Narrowing to one
	// interface would refuse a case the section permits.
	if !network.SharesSubnet(ls.connected, global) {
		return netip.Addr{}
	}
	return configured
}

// applyLinkLocalNextHop upgrades a single-address IPv6 next-hop wire form to the
// 32-octet global-plus-link-local form, and only when RFC 2545 Section 3's
// condition holds for the global address that form already carries.
//
// It runs after precomputeNextHop (peer_forward_facts.go), which settles the
// global address from config. Section 3 is decided against the host interface
// table rather than against config, so the two steps read different inputs and
// stay separate.
func applyLinkLocalNextHop(s *PeerSettings, f *peerForwardFacts, scope *linkScope) {
	var global netip.Addr
	switch f.nhMode {
	case nhModeSelfV6:
		global = s.LocalAddress.Unmap()
	case nhModeExplicitV6:
		global = s.NextHopAddress.Unmap()
	default:
		return
	}

	linkLocal := scope.linkLocalNextHop(s.LinkLocal, global)
	if !linkLocal.IsValid() {
		return
	}

	f.nhMode = nhModeSelfV6LL
	ll := linkLocal.As16()
	copy(f.nhGlobalLL[:16], f.nhGlobal[:])
	copy(f.nhGlobalLL[16:], ll[:])
}

// refreshLinkScope re-reads the host interface table for this peer.
//
// It runs with the forwarding-facts refresh, so a session that establishes after
// an interface comes up answers Section 3 against the table as it stands then,
// rather than against the one that existed when the config was loaded.
func (p *Peer) refreshLinkScope() {
	p.refreshLinkScopeFrom(network.ConnectedPrefixes())
}

// refreshLinkScopeFrom re-settles this peer's snapshot against an interface table
// the caller has already read.
//
// The reactor refreshes every established peer when one address is added to or
// removed from ANY interface (refreshPeerLinkScopes, reactor_iface.go), on the
// event-bus goroutine that delivers the event. Sharing one read across that
// fan-out costs one syscall instead of one per peer.
func (p *Peer) refreshLinkScopeFrom(connected []netip.Prefix) {
	p.llScope.Store(newLinkScopeFrom(connected, p.settings.Address))
}

// linkLocalNextHopFor returns the link-local address to append after
// globalNextHop for this peer, or the zero Addr when Section 3's condition does
// not hold.
func (p *Peer) linkLocalNextHopFor(globalNextHop netip.Addr) netip.Addr {
	return p.llScope.Load().linkLocalNextHop(p.settings.LinkLocal, globalNextHop)
}
