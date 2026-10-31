// Design: docs/architecture/core-design.md — RFC 2545 Section 3 link-local next-hop condition
// RFC: rfc/short/rfc2545.md
// Overview: peer.go — Peer struct and FSM state machine

package reactor

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/network"
)

// linkLocalCondition names the common-subnet condition an address family's
// RFC sets for including the speaker's link-local after the global next hop.
// The zero value is unspecified, never a condition.
type linkLocalCondition uint8

const (
	linkLocalConditionUnspecified linkLocalCondition = iota
	// linkLocalConditionNextHopAndPeer is RFC 2545 Section 3's joint
	// condition: ONE connected subnet holds the global next hop and the peer.
	linkLocalConditionNextHopAndPeer
	// linkLocalConditionPeer is RFC 4659 Section 3.2.1.1's condition: the
	// speaker shares a connected subnet with the peer, wherever the global is.
	linkLocalConditionPeer
)

// linkLocalConditionFor returns the condition fam's RFC sets.
//
// RFC 4659 Section 3.2.1.1: "The link-local address shall be included in the
// Next Hop field if and only if the advertising BGP speaker shares a common
// subnet with the peer the route is being advertised to [BGP-IPv6]." The
// sentence names the peer alone, so VPN-IPv6 includes the link-local behind a
// global the peer does not share a subnet with, such as a loopback (owner
// decision 2026-10-19, plan/rfc-gap-difficulty-2026-10-18.md row 22). Every
// other family carrying an IPv6 next hop keeps RFC 2545 Section 3's wording,
// which names the next-hop entity too.
func linkLocalConditionFor(fam family.Family) linkLocalCondition {
	if fam == (family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}) {
		return linkLocalConditionPeer
	}
	return linkLocalConditionNextHopAndPeer
}

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
	// neighbors is the neighbor table read with connected. It names the
	// Link-Local of another router whose global is the next hop.
	neighbors neighborView
	// peer is the recipient address captured with this interface snapshot.
	peer netip.Addr
	// peerOnLink reports whether the speaker shares a common subnet with the
	// peer the route is being advertised to.
	peerOnLink bool
	// clock times the rate limit on the warning a third-party next hop sent
	// without its router's Link-Local logs. A peer's scope carries the
	// peer's clock (refreshLinkScopeWith), so a test on a fake clock drives it.
	clock clock.Clock
	// unreadWarned says whether the warning for an unreadable neighbor table
	// was given. A peer's scope carries the peer's own flag
	// (Peer.neighborTableUnreadWarned), so the warning is once per peer.
	unreadWarned *atomic.Bool
}

// newLinkScope reads the host interface and neighbor tables and settles the
// peer half of the Section 3 condition for peerAddr.
//
// It reads the kernel, so it runs at session establishment and at a config
// reload, never per UPDATE. It has no reactor to ask, so its neighbor view
// says no watch was started; a peer builds its scope with the reactor's watch
// state instead (refreshLinkScope).
func newLinkScope(peerAddr netip.Addr) *linkScope {
	return newLinkScopeWith(network.ConnectedPrefixes(), readNeighborView(neighborWatchState{}), peerAddr)
}

// newLinkScopeWith settles the peer half against interface and neighbor tables
// the caller has already read, so a fan-out over many peers pays one kernel
// read of each for all of them (refreshPeerLinkScopes, reactor_iface.go). Both
// are treated as immutable by every reader, so sharing them is safe.
func newLinkScopeWith(connected []netip.Prefix, neighbors neighborView, peerAddr netip.Addr) *linkScope {
	return &linkScope{
		connected:    connected,
		neighbors:    neighbors,
		peer:         peerAddr.Unmap(),
		peerOnLink:   network.SharesSubnet(connected, peerAddr),
		clock:        clock.RealClock{},
		unreadWarned: new(atomic.Bool),
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

// holdsCommonSubnet answers condition for global toward this scope's peer.
// A nil scope has read no interface table and proves no common subnet.
func (ls *linkScope) holdsCommonSubnet(global netip.Addr, condition linkLocalCondition) bool {
	if ls == nil {
		return false
	}
	if !ls.peerOnLink {
		return false
	}
	switch condition {
	case linkLocalConditionPeer:
		// RFC 4659 Section 3.2.1.1: "shares a common subnet with the peer the
		// route is being advertised to"; peerOnLink settled exactly that.
		return true
	case linkLocalConditionNextHopAndPeer:
		// RFC 2545 Section 3: both entities must share ONE connected subnet
		// with the speaker. Separate interfaces cannot establish that.
		return ls.sharesNextHopSubnet(global)
	case linkLocalConditionUnspecified:
		panic("BUG: link-local condition unspecified")
	default:
		panic("BUG: invalid link-local condition")
	}
}

// linkLocalNextHop returns the link-local address to write after globalNextHop in
// the MP_REACH_NLRI Network Address of Next Hop field, or the zero Addr when the
// field carries the global address alone.
//
// A zero Addr means no Link-Local was selected, not proof of Section 3's
// "In all other cases" condition. It also covers an unavailable configured
// address, or a third-party router whose Link-Local the neighbor table does not
// name. Those gaps cannot turn a common subnet into a non-common one.
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
// after the speaker's own global; another router's comes from the neighbor
// table (see the switch below).
//
// condition is the family's common-subnet condition (linkLocalConditionFor).
func (ls *linkScope) linkLocalNextHop(configured, globalNextHop netip.Addr, router nextHopRouter, condition linkLocalCondition) netip.Addr {
	if ls == nil || !ls.peerOnLink {
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
	if !global.Is6() {
		return netip.Addr{}
	}
	if attribute.ValidateGlobalNextHop(global) != nil {
		return netip.Addr{}
	}
	// RFC 2545 Section 3 asks for ONE subnet holding the global and the peer;
	// RFC 4659 Section 3.2.1.1 asks for a subnet shared with the peer.
	if !ls.holdsCommonSubnet(global, condition) {
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
	// other router it is that router's Link-Local, which only the kernel
	// neighbor table names; the speaker's own would send the peer's traffic to
	// the wrong router, so it is never substituted.
	//
	// The draft's second condition, a global that is the internal peer's own
	// address, needs no case here. RFC 4271 Section 5.1.3: "A route originated
	// by a BGP speaker SHALL NOT be advertised to a peer using an address of
	// that peer as NEXT_HOP." Ze withholds such a route on every rail
	// (originatedNextHopIsPeerOwn, egressNextHopIsPeerOwn), so whatever it
	// classifies as, nothing it would carry reaches the wire.
	switch router {
	case nextHopRouterSpeaker:
		if !configured.Is6() {
			return netip.Addr{}
		}
		if !configured.IsLinkLocalUnicast() {
			return netip.Addr{}
		}
		return configured
	case nextHopRouterThirdParty:
		// RFC 4659 Section 3.2.1.1 keeps its own VPN-IPv6 contract, which
		// names the advertising speaker's Link-Local; only RFC 2545's
		// sentence names the next-hop entity's.
		if condition != linkLocalConditionNextHopAndPeer {
			return netip.Addr{}
		}
		// RFC 2545 Section 3: "The link-local address shall be included in
		// the Next Hop field if and only if the BGP speaker shares a common
		// subnet with the entity identified by the global IPv6 address
		// carried in the Network Address of Next Hop field and the peer the
		// route is being advertised to."
		return ls.thirdPartyLinkLocal(global)
	case nextHopRouterUnspecified:
		return netip.Addr{}
	default:
		panic("BUG: invalid next-hop router")
	}
}

// thirdPartyLinkLocal returns the Link-Local of the router holding global, as
// the neighbor table read with this scope names it, or the zero Addr with a
// warning that names the next hop and why none is known: once per peer for a
// table that cannot be read, rate-limited for a table that names no router. The
// caller has already established the common subnet, so a miss here is a field
// sent with the global address alone where Section 3 asks for both.
func (ls *linkScope) thirdPartyLinkLocal(global netip.Addr) netip.Addr {
	linkLocal, outcome, err := ls.neighbors.linkLocalOf(global)
	if err != nil {
		warnNeighborTableUnread(ls.unreadWarned, ls.peer, global, err)
		return netip.Addr{}
	}
	switch outcome {
	case network.LinkLocalFound:
		return linkLocal
	case network.LinkLocalNoGlobalEntry, network.LinkLocalGlobalUnbound,
		network.LinkLocalNoCandidate, network.LinkLocalAmbiguous:
		warnThirdPartyLinkLocalUnknown(ls.clock.Now().UnixNano(), ls.peer, global, outcome, ls.neighbors.watch)
		return netip.Addr{}
	case network.LinkLocalOutcomeUnspecified:
		panic("BUG: neighbor table lookup answered no outcome")
	default:
		panic("BUG: invalid neighbor table outcome")
	}
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
	router := owners.classify(global)
	// RFC 4659 Section 3.2.1.1 decides the VPN-IPv6 pair on its own condition,
	// which can hold where RFC 2545 Section 3's does not (a loopback global).
	// precomputeVPNNextHop fills the global half of nhVPNPair.
	if vpnLinkLocal := scope.linkLocalNextHop(s.LinkLocal, global, router, linkLocalConditionPeer); vpnLinkLocal.IsValid() {
		f.nhVPNLinkLocal = true
		ll := vpnLinkLocal.As16()
		copy(f.nhVPNPair[32:], ll[:])
	}
	linkLocal := scope.linkLocalNextHop(s.LinkLocal, global, router, linkLocalConditionNextHopAndPeer)
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
	p.refreshLinkScopeWith(network.ConnectedPrefixes(), readNeighborView(p.neighborWatchState()))
}

// neighborWatchState answers the state of the neighbor table watch of this
// peer's reactor, or not started for a peer built outside one (tests).
func (p *Peer) neighborWatchState() neighborWatchState {
	if p.reactor == nil {
		return neighborWatchState{}
	}
	return p.reactor.neighborWatchState()
}

// refreshLinkScopeWith re-settles this peer's snapshot against interface and
// neighbor tables the caller has already read.
//
// The reactor refreshes every established peer when one address is added to or
// removed from ANY interface, on the event-bus goroutine that delivers the
// event, and when the neighbor table watch signals a change, on
// neighborWatchWorker (refreshPeerLinkScopes, reactor_iface.go). Sharing one
// read of each table across that fan-out costs one syscall each instead of one
// per peer.
func (p *Peer) refreshLinkScopeWith(connected []netip.Prefix, neighbors neighborView) {
	scope := newLinkScopeWith(connected, neighbors, p.settings.Address)
	scope.clock = p.clock
	scope.unreadWarned = &p.neighborTableUnreadWarned
	if neighbors.links != nil {
		// The table was read, so the cause of an earlier warning is gone; a
		// later failure is a new one and warns again.
		p.neighborTableUnreadWarned.Store(false)
	}
	p.llScope.Store(scope)
}

// linkLocalNextHopFor returns the link-local address to append after
// globalNextHop for this peer over session, or the zero Addr when fam's
// common-subnet condition does not hold (linkLocalConditionFor). session MAY be
// nil: the speaker's address is then known from config and the interface table
// alone.
func (p *Peer) linkLocalNextHopFor(session *Session, globalNextHop netip.Addr, fam family.Family) netip.Addr {
	owners := nextHopOwners{endpoint: connectedLocalAddress(session), configured: p.settings.LocalAddress}
	if session != nil {
		if held := session.nextHopScope.Load(); held != nil {
			owners.held = held.addresses
		}
	}
	return p.llScope.Load().linkLocalNextHop(p.settings.LinkLocal, globalNextHop, owners.classify(globalNextHop), linkLocalConditionFor(fam))
}

// nextHopRouter names the router a global next hop belongs to, which decides
// whose Link-Local may follow it (linkScope.linkLocalNextHop).
type nextHopRouter uint8

const (
	nextHopRouterUnspecified nextHopRouter = iota
	// nextHopRouterSpeaker: the global is one of this speaker's own addresses.
	nextHopRouterSpeaker
	// nextHopRouterThirdParty: any other router, whose Link-Local only the
	// kernel neighbor table names (linkScope.thirdPartyLinkLocal). The peer's
	// own address lands here too: RFC 4271 Section 5.1.3 keeps a route with
	// that NEXT_HOP off the wire (linkScope.linkLocalNextHop).
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
