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
// peerOnLink settles the session-only one-hop condition. Pair inclusion is
// stronger: one connected prefix must contain both peer and effective Global,
// rather than each address belonging to a different prefix in the same snapshot.
type linkScope struct {
	// connected holds the subnets this host is directly attached to.
	connected []netip.Prefix
	// peer is the recipient address captured with this interface snapshot.
	peer netip.Addr
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
		peer:       peerAddr.Unmap(),
		peerOnLink: network.SharesSubnet(connected, peerAddr),
	}
}

// sharesNextHopSubnet answers RFC 2545 Section 3's joint condition without
// allocation or a kernel read: "The link-local address shall be included in the
// Next Hop field if and only if the BGP speaker shares a common subnet with the
// entity identified by the global IPv6 address carried in the Network Address
// of Next Hop field and the peer the route is being advertised to."
// The loop is bounded by the interface-prefix snapshot captured at setup.
func (ls *linkScope) sharesNextHopSubnet(global netip.Addr) bool {
	if ls == nil {
		return false
	}
	for _, prefix := range ls.connected {
		if prefix.Contains(ls.peer) && prefix.Contains(global) {
			return true
		}
	}
	return false
}

// linkLocalNextHop returns the link-local address to write after globalNextHop in
// the MP_REACH_NLRI Network Address of Next Hop field, or the zero Addr when the
// field carries the global address alone.
//
// A zero Addr means no Link-Local was selected, not proof of Section 3's
// "In all other cases" condition. It also covers an unavailable configured
// address or a third-party Link-Local Ze does not learn. Those gaps cannot
// turn a common subnet into a non-common one.
//
// A nil scope has read no interface table and appends nothing; it does not
// establish the RFC's topology condition.
//
// configured is the operator's link-local address for this session. A configured
// address that is not link-local unicast is not appended either: Section 3 names
// the second address "the link-local IPv6 address of the next hop", so writing
// anything else there would break the same sentence it is meant to satisfy.
//
// router names whose address globalNextHop is (nextHopOwners.classify). The
// configured address is the SPEAKER's own Link-Local, so it is appended only
// where that is the right second address; see the comment at the check below.
func (ls *linkScope) linkLocalNextHop(configured, globalNextHop netip.Addr, router nextHopRouter) netip.Addr {
	if ls == nil || !ls.peerOnLink {
		return netip.Addr{}
	}
	// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the
	// Network Address of Next Hop field the global IPv6 address of the next hop,
	// potentially followed by the link-local IPv6 address of the next hop."
	//
	// draft-ietf-idr-linklocal-capability Section 4: "If the route is directly
	// connected to the speaker, [...] the next hop MUST include its own
	// Link-Local IPv6 address."
	//
	// The second address is the next hop's own. When the global is the
	// speaker's address, the speaker's Link-Local is that next hop's. For any
	// other router the speaker does not learn that router's Link-Local, so its
	// own would send the peer's traffic to the wrong router: the global goes
	// alone (a recorded gap on RFC 2545 Section 3, rfc2545.md).
	//
	// The draft's second condition, a global that is the internal peer's own
	// address, needs no case here. RFC 4271 Section 5.1.3: "A route originated
	// by a BGP speaker SHALL NOT be advertised to a peer using an address of
	// that peer as NEXT_HOP." Ze withholds such a route on every rail
	// (originatedNextHopIsPeerOwn, egressNextHopIsPeerOwn), so it classifies as
	// a third party and nothing it would carry reaches the wire.
	switch router {
	case nextHopRouterSpeaker:
		// The speaker's own Link-Local is the right second address.
	case nextHopRouterUnspecified, nextHopRouterThirdParty:
		return netip.Addr{}
	default:
		panic("BUG: invalid next-hop router")
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
	// RFC 2545 Section 3: both entities must share ONE connected subnet with
	// the speaker. Separate interfaces cannot establish that joint condition.
	if !ls.sharesNextHopSubnet(global) {
		return netip.Addr{}
	}
	return configured
}

// applyLinkLocalNextHop upgrades a single-address IPv6 next-hop wire form to the
// 32-octet global-plus-link-local form, and only when RFC 2545 Section 3's
// condition holds for the global address that form already carries.
//
// It runs after precomputeNextHop (peer_forward_facts.go), which settles the
// global address: for next hop self the session's connected endpoint
// (connectedLocalAddress) or else the configured local address, and for an
// explicit next hop the configured one. Section 3 is decided against the host
// interface table rather than against config, so the two steps read different
// inputs and stay separate.
func applyLinkLocalNextHop(s *PeerSettings, f *peerForwardFacts, scope *linkScope) {
	switch f.nhMode {
	case nhModeSelfV6, nhModeExplicitV6:
		// The single-address IPv6 forms, the only ones Section 3 can raise.
	default:
		return
	}
	// draft-ietf-idr-linklocal-capability Section 4: "If the route is directly
	// connected to the speaker, [...] the next hop MUST include its own
	// Link-Local IPv6 address."
	//
	// The global is read back from the form precomputeNextHop wrote, never from
	// config again: under `local ip auto` no local address is configured, and
	// the connected endpoint is the speaker's own address. Reading the config
	// here dropped the speaker's own Link-Local address in that configuration.
	global := netip.AddrFrom16(f.nhGlobal).Unmap()

	owners := nextHopOwners{endpoint: f.connectedLocal, configured: s.LocalAddress}
	if f.localScope != nil {
		if held := f.localScope.Load(); held != nil {
			owners.held = held.addresses
		}
	}
	linkLocal := scope.linkLocalNextHop(s.LinkLocal, global, owners.classify(global))
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
// globalNextHop for this peer over session, or the zero Addr when Section 3's
// condition does not hold. session MAY be nil: the speaker's address is then
// known from config and the interface table alone.
func (p *Peer) linkLocalNextHopFor(session *Session, globalNextHop netip.Addr) netip.Addr {
	owners := nextHopOwners{endpoint: connectedLocalAddress(session), configured: p.settings.LocalAddress}
	if session != nil {
		if held := session.nextHopScope.Load(); held != nil {
			owners.held = held.addresses
		}
	}
	return p.llScope.Load().linkLocalNextHop(p.settings.LinkLocal, globalNextHop, owners.classify(globalNextHop))
}

// nextHopRouter names the router a global next hop belongs to, which decides
// whose Link-Local may follow it (linkScope.linkLocalNextHop).
type nextHopRouter uint8

const (
	nextHopRouterUnspecified nextHopRouter = iota
	// nextHopRouterSpeaker: the global is one of this speaker's own addresses.
	nextHopRouterSpeaker
	// nextHopRouterThirdParty: any other router, whose Link-Local Ze never learns.
	// The peer's own address lands here too: RFC 4271 Section 5.1.3 keeps a
	// route with that NEXT_HOP off the wire (linkScope.linkLocalNextHop).
	nextHopRouterThirdParty
)

// nextHopOwners holds the addresses a global next hop is classified against for
// one session. Safe for concurrent use: it is a value, built per call.
type nextHopOwners struct {
	// endpoint is the session's TCP local endpoint (connectedLocalAddress).
	endpoint netip.Addr
	// configured is the peer's configured local address.
	configured netip.Addr
	// held is every interface address of this host, host bits kept
	// (receiveNextHopScope.addresses), so an address on another interface counts.
	held []netip.Prefix
}

// nextHopOwners captures established-session ownership without acquiring Peer.mu
// or reading the kernel. Queued writers can already hold that mutex.
func (f *peerForwardFacts) nextHopOwners() nextHopOwners {
	owners := nextHopOwners{endpoint: f.connectedLocal, configured: f.localAddr}
	if f.localScope != nil {
		if held := f.localScope.Load(); held != nil {
			owners.held = held.addresses
		}
	}
	return owners
}

// classify answers which router global names: one of the speaker's own
// addresses, or a third party.
func (o nextHopOwners) classify(global netip.Addr) nextHopRouter {
	if !global.IsValid() {
		return nextHopRouterUnspecified
	}
	global = global.Unmap()
	if global == o.endpoint.Unmap() {
		return nextHopRouterSpeaker
	}
	if global == o.configured.Unmap() {
		return nextHopRouterSpeaker
	}
	if holdsAddress(o.held, global) {
		return nextHopRouterSpeaker
	}
	return nextHopRouterThirdParty
}

// sameLinkLayerSegment reports whether two peer addresses sit on ONE link-layer
// segment this speaker is attached to.
//
// draft-ietf-idr-linklocal-capability Section 4: "A Route Reflector (RR)
// reflecting a route with a link-local-only next hop MUST NOT advertise that
// route to a client unless the client shares the same link-layer segment as the
// original advertiser." A link-local next hop names a host on one segment and
// nothing else, so a client on another segment is told to forward through an
// address it cannot resolve.
//
// The answer is derived from the subnets this host is attached to, which is the
// same evidence RFC 2545 Section 3's "shares a common subnet" is decided on
// (linkScope above). Two peers inside ONE connected prefix are on one segment as
// far as this speaker can see.
//
// A LINK-LOCAL PREFIX IS NOT EVIDENCE, and the exclusion is what makes the
// answer safe rather than merely usual. Every IPv6 interface holds fe80::/64, so
// ConnectedPrefixes reports that same prefix once per interface and two peers on
// DIFFERENT segments both fall inside it. The interface each one came from is not
// in that list, so a link-local prefix cannot separate them and is refused as
// evidence. The requirement it serves is a MUST NOT, and a false "they share a
// segment" would publish the route this function exists to withhold.
//
// An unreadable interface table is an empty list and answers false, so a failed
// read withholds rather than advertises.
func sameLinkLayerSegment(connected []netip.Prefix, advertiser, client netip.Addr) bool {
	if !advertiser.IsValid() || !client.IsValid() {
		return false
	}
	advertiser, client = advertiser.Unmap(), client.Unmap()
	for _, prefix := range connected {
		if prefix.Addr().IsLinkLocalUnicast() {
			continue
		}
		if prefix.Contains(advertiser) && prefix.Contains(client) {
			return true
		}
	}
	return false
}

// connectedPrefixes returns the subnets this snapshot was settled against.
//
// A nil scope has read no interface table and answers the empty list, which
// every reader treats as "no shared subnet" rather than as "every subnet".
func (ls *linkScope) connectedPrefixes() []netip.Prefix {
	if ls == nil {
		return nil
	}
	return ls.connected
}
