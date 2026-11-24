// Design: docs/architecture/core-design.md — peer forwarding facts precomputation
// RFC: rfc/short/rfc2545.md
// Related: peer.go — Peer struct, lifecycle methods
// Related: reactor_api_forward.go — ForwardUpdate egress pipeline
// Related: forward_rs.go — reactorForwardRS egress pipeline
package reactor

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	nhModeNone   uint8 = iota // No precomputed next-hop rewrite
	nhModeSelf4               // Self IPv4: legacy NEXT_HOP + family-permitted MP_REACH
	nhModeSelfV6              // Self IPv6: MP_REACH global only
	// nhModeSelfV6LL is the RFC 2545 Section 3 two-address form: MP_REACH carries
	// the global address then the link-local one. applyLinkLocalNextHop
	// (link_scope.go) raises nhModeSelfV6 to it when the common-subnet condition holds.
	nhModeSelfV6LL
)

type sendCommunityMask uint8

const (
	scSuppressStandard sendCommunityMask = 1 << iota
	scSuppressExtended
	scSuppressLarge
)

// peerForwardFacts holds precomputed per-peer forwarding decisions.
// Built at session lifecycle boundaries, read on every UPDATE iteration.
// Stored via atomic.Pointer on Peer; nil means not established.
type peerForwardFacts struct {
	dissemination *disseminationSnapshot
	addr          netip.Addr
	peerKey       netip.AddrPort
	addrStr       string

	localAS       uint32
	globalLocalAS uint32
	peerAS        uint32
	isEBGP        bool
	localAddr     netip.Addr
	localScope    *atomic.Pointer[receiveNextHopScope]
	aigpEnabled   bool

	rsClient   bool
	rrClient   bool
	asOverride bool
	// rsRelaySend is the route-server fast path's send permission: this peer's
	// attach block grants `send [ update ]` to a process whose plugin declares
	// registry.RelaysOnRSFastPath. The fast path forwards in place of that
	// plugin's cached forward, which filterPermittedPeers refuses a peer
	// without the grant, so reactorForwardRSSection selects no peer whose
	// rsRelaySend is false (rsRelaySendProcessName). rsRelaySendProcess names the
	// granting process so AIGP metric re-advertisement rechecks the same grant.
	rsRelaySend        bool
	rsRelaySendProcess string
	// rsRelayReceive is the same declaration read from the source side: this
	// peer's attach block grants a process whose plugin declares
	// registry.RelaysOnRSFastPath the UPDATEs the peer sends
	// (`receive [ update-received ]`, or `update` or `*`). Without it the
	// delivery graph feeds that plugin none of this peer's UPDATEs, so its
	// cached forward never relays them, and the fast path, forwarding in its
	// place, declines this source too (notifyMessageReceiver,
	// rsRelayReceiveGranted).
	rsRelayReceive bool
	// propagatePrefixSID is the operator's statement that this EBGP neighbor is
	// inside ze's SR domain (RFC 8669 Section 8, forward_prefix_sid.go).
	propagatePrefixSID bool
	name               string
	groupName          string
	exportPolicy       filterapi.Policy
	exportGeneration   uint64 // Read with exportPolicy; a decision stamps its write with it (fwdItem.exportGeneration).
	// exportChainActive records that the export chain must run
	// (hasActiveFilter): the route-server fast path runs it per destination,
	// through the one implementation the general rail runs
	// (runEgressPolicyChain), whoever placed its members.
	exportChainActive bool

	clusterID      uint32
	clusterIDBytes [4]byte

	sendCtxID   bgpctx.ContextID
	sendASN4    bool
	extendedMsg bool
	maxMsgSize  int

	filterInfo filterapi.PeerFilterInfo

	secondaryAS uint32

	// connectedLocal is the local endpoint of the established session's TCP
	// connection, the address the peer reaches this speaker on, or the zero
	// Addr when the session holds none a next hop can carry
	// (connectedLocalAddress). precomputeNextHop builds next-hop self from it.
	connectedLocal netip.Addr
	// staticNextHop is the folded intent when exportPolicy holds only built-in
	// next-hop:* steps or no active filters (hasStaticNextHop == true), letting
	// both forward rails keep their zero-allocation static fast path.
	staticNextHop    policychain.NextHopIntent
	hasStaticNextHop bool
	// selfMode and selfWithheld hold this session's precomputed next-hop-self
	// wire form unconditionally, so both static next-hop:self/auto and dynamic
	// export-chain self intents reuse the same immutable buffers with zero
	// per-UPDATE allocations.
	selfMode     uint8
	selfWithheld bool
	// nhSelfWithheld is a GUARD for the static fast path: self is selected by
	// the static export policy (next-hop:self, or next-hop:auto on ordinary
	// eBGP) and no local address exists to put in the received next hop's
	// place, so both forward rails withhold the announcement toward this peer
	// rather than pass the third-party NEXT_HOP (RFC 4271 Section 5.1.3).
	nhSelfWithheld bool

	nhMode     uint8
	nhLegacy   [4]byte
	nhMapped   [16]byte
	nhGlobal   [16]byte
	nhGlobalLL [32]byte
	// The RD-framed forms of the same self addresses (precomputeVPNNextHop).
	// applyNextHopWire selects framing from the profile, independently of scope.
	nhVPNv4     [12]byte                   // RD + IPv4
	nhVPNMapped [24]byte                   // RD + IPv4-mapped IPv6
	nhVPNPair   [vpnNextHopPairOctets]byte // RD + global, RD + link-local
	// nhVPNLinkLocal records that applyLinkLocalNextHop wrote the speaker's
	// link-local into nhVPNPair under RFC 4659 Section 3.2.1.1's condition,
	// which holds wherever RFC 2545 Section 3's does and also where only the
	// peer shares the subnet. False sends the 24-octet RD + global form.
	nhVPNLinkLocal bool

	scMask sendCommunityMask

	// redirectEnabled reports whether FlowSpec 0x0800 redirect-to-IP next-hop
	// carriage is opted in and extended communities are not suppressed.
	redirectEnabled bool
	// redirectSelf holds this session's precomputed self addresses for FlowSpec
	// redirect-to-IP rewriting under dynamic export-chain intents.
	redirectSelf redirectNextHop
	// redirect is the precomputed FlowSpec redirect-to-IP decision for static
	// export policies (precomputeRedirectNextHop, applyNextHopFamily).
	redirect redirectNextHop
}

// redirectAction is what a destination does with the next hop of a received
// FlowSpec route whose action is redirect/mirror-to-IP-next-hop.
type redirectAction uint8

const (
	redirectUnspecified redirectAction = iota
	redirectKeep                       // the received address leaves unchanged
	redirectRewrite                    // the address becomes redirectNextHop's
	redirectStrip                      // the RFC 8955 Section 4 zero length
)

// redirectNextHop is one destination's precomputed redirect-to-IP decision.
// The addresses are held in their wire form, so the forward rails copy nothing
// per UPDATE. A rewrite holds at most one address of each family.
type redirectNextHop struct {
	action redirectAction
	has4   bool
	has6   bool
	legacy [4]byte  // an IPv4 address
	global [16]byte // an IPv6 global address
}

// hop returns the rewrite address for a rule of the given AFI as the next-hop
// field carries it, or nil when this destination holds no address of that
// family. draft-simpson-idr-flowspec-redirect-02 Section 3: "if an IPv4 [or
// IPv6] packet with destination address D that is normally forwarded to a
// next-hop A matches a filter entry of the type described above it MUST
// instead be redirected (C=0) or mirrored (C=1) to next-hop B, where B is
// found by FIB lookup of the IPv4 [or IPv6] address contained in the
// MP_REACH_NLRI next-hop field". The packet family and the address family pair
// up, so an IPv4 rule takes 4 octets, an IPv6 rule 16, and the other family's
// address is never written: the IPv4-mapped form is no usable IPv6 global
// (egressNextHopWithheld refuses it), and an IPv6 address cannot be the FIB
// key for an IPv4 packet.
func (r *redirectNextHop) hop(afi family.AFI) []byte {
	switch afi {
	case family.AFIIPv4:
		if r.has4 {
			return r.legacy[:]
		}
	case family.AFIIPv6:
		if r.has6 {
			return r.global[:]
		}
	case family.AFIL2VPN, family.AFIBGPLS:
		// No IP packet family, so no FIB key and no rewrite address.
	default:
		// The AFI set is open: a code no family names carries no rewrite address.
	}
	return nil
}

// add records addr as a rewrite address when it can be one: a valid address
// whose family slot is still empty, and for IPv6 not link-local. A link-local
// address names no interface off the session's link, so the FIB lookup the
// draft requires has no answer for it at the recipient's other interfaces and
// the egress link-local gates would refuse the whole rule.
func (r *redirectNextHop) add(addr netip.Addr) {
	if !addr.IsValid() {
		return
	}
	addr = addr.Unmap()
	if addr.Is4() {
		if !r.has4 {
			r.has4 = true
			r.legacy = addr.As4()
		}
		return
	}
	if addr.IsLinkLocalUnicast() {
		return
	}
	if !r.has6 {
		r.has6 = true
		r.global = addr.As16()
	}
}

// rsRelaySendProcessName returns the first process name in the compiled
// delivery graph that is permitted to send bgp:update to p, whose plugin
// declares registry.RelaysOnRSFastPath, and whose ClaimSessionScope matches p,
// or "" when none does. It runs once per snapshot, never once per UPDATE.
func rsRelaySendProcessName(p *Peer, s *PeerSettings, peerAS uint32) string {
	g := p.reactor.DeliveryGraph()
	if g == nil {
		return ""
	}
	for _, name := range g.SendersForPeer(p.addrString, bgpevents.SendUpdate) {
		regName := resolveRegistrationName(p.reactor, name)
		if !registry.RelaysOnRSFastPath(regName) && !registry.RelaysOnRSFastPath(name) {
			continue
		}
		if !peerSettingsMatchesClaimScope(s, peerAS, registry.ClaimSessionScopeFor(regName)) {
			continue
		}
		return name
	}
	return ""
}

// rsRelayReceiveGranted reports whether the compiled delivery graph feeds p's
// received UPDATEs to a process whose plugin declares
// registry.RelaysOnRSFastPath and whose ClaimSessionScope matches p. Like
// rsRelaySendProcessName it runs once per snapshot.
func rsRelayReceiveGranted(p *Peer, s *PeerSettings, peerAS uint32) bool {
	g := p.reactor.DeliveryGraph()
	if g == nil {
		return false
	}
	bgpNS := events.LookupNamespaceID(bgpevents.Namespace)
	updateET := events.LookupEventTypeID(bgpevents.EventUpdate)
	for _, name := range g.ReceiversForPeer(p.addrString, bgpNS, updateET, events.DirReceived) {
		regName := resolveRegistrationName(p.reactor, name)
		if !registry.RelaysOnRSFastPath(regName) && !registry.RelaysOnRSFastPath(name) {
			continue
		}
		if !peerSettingsMatchesClaimScope(s, peerAS, registry.ClaimSessionScopeFor(regName)) {
			continue
		}
		return true
	}
	return false
}

func (p *Peer) forwardFacts() *peerForwardFacts {
	return p.fwdFacts.Load()
}

// refreshForwardFacts builds and stores a new forwarding facts snapshot.
// Called at: setEncodingContexts (after unlock), resolveDynamicPeerSettings.
//
// The link scope is re-read first, because buildForwardFacts precomputes the
// next-hop wire form and RFC 2545 Section 3 decides that form against the host
// interface table (link_scope.go).
func (p *Peer) refreshForwardFacts() {
	p.refreshLinkScope()
	next := p.buildForwardFacts()
	previous := p.fwdFacts.Swap(next)
	p.reportRSRelayGrants(previous, next)
}

// reportRSRelayGrants tells the operator, once, that an rs-fast-path peer lacks
// a route-server grant the fast path obeys (rsRelayReceive, rsRelaySend). The
// fast path declines such a source and passes over such a destination without a
// word per UPDATE, so this is where the decision is made visible: when a
// snapshot first lacks a grant, after a session comes up (previous is nil) or
// when a reload removes it. A rebuild that changes nothing (an interface event,
// refreshPeerLinkScopes) says nothing again.
func (p *Peer) reportRSRelayGrants(previous, next *peerForwardFacts) {
	if !p.settings.RSFastPath {
		return
	}
	missingReceive := !next.rsRelayReceive && (previous == nil || previous.rsRelayReceive)
	missingSend := !next.rsRelaySend && (previous == nil || previous.rsRelaySend)
	if !missingReceive && !missingSend {
		return
	}

	relay := rsRelayProcessName(p)

	var missing, effect, action textbuf.Buffer
	action.Str("permit /bus/endpoint/").Str(relay).Str(" in this peer's bus block:")
	if missingReceive {
		missing.Str("send bgp:update (direction received)")
		effect.Str("the routes this peer sends are relayed to no other peer")
		action.Str(" send { allow [ bgp:update ]; to [ /bus/endpoint/").Str(relay).Str(" ]; }")
	}
	if missingSend {
		if missingReceive {
			missing.Str(" and ")
			effect.Str("; ")
		}
		missing.Str("receive bgp:update")
		effect.Str("this peer is sent no route the route server relays")
		action.Str(" receive { allow [ bgp:update ]; from [ /bus/endpoint/").Str(relay).Str(" ]; }")
	}
	fwdLogger().Warn("rs-fast-path: this peer does not grant the route server what the fast path needs",
		"peer", p.settings.Address,
		"process", relay,
		"missing", missing.String(),
		"effect", effect.String(),
		"action", action.String())
}

// rsRelayProcessName answers the process name an operator's fix names: the
// route-server process already linked to p in the delivery graph, else the
// first registered plugin declaring registry.RelaysOnRSFastPath. It runs only
// when reportRSRelayGrants warns, never per UPDATE.
func rsRelayProcessName(p *Peer) string {
	for _, name := range p.reactor.DeliveryGraph().ProcessListForPeer(p.addrString) {
		regName := resolveRegistrationName(p.reactor, name)
		if registry.RelaysOnRSFastPath(regName) {
			return name
		}
		if registry.RelaysOnRSFastPath(name) {
			return name
		}
	}
	for _, reg := range registry.All() {
		if reg.RelaysOnRSFastPath {
			return reg.Name
		}
	}
	return "<route-server process>"
}

// refreshForwardFactsIfLive rebuilds the snapshot only for a peer that already has
// one, and only if nobody replaced it meanwhile.
//
// A nil snapshot is the "this peer has no session" gate on the forwarding rails
// (reactorForwardRS skips a peer whose forwardFacts() is nil, forward_rs.go), so
// storing one unconditionally from the config-reload goroutine would make a down
// peer look established. The compare-and-swap covers the other direction:
// clearEncodingContexts stores nil on teardown (peer.go), and a plain store racing
// it would resurrect the snapshot for a session that has gone.
func (p *Peer) refreshForwardFactsIfLive() {
	p.refreshForwardFactsIfLiveFrom(network.ConnectedPrefixes(), readNeighborView(p.neighborWatchState()))
}

// refreshForwardFactsIfLiveFrom is refreshForwardFactsIfLive against interface
// and neighbor tables the caller has already read, for a fan-out over many
// peers (refreshPeerLinkScopes, reactor_iface.go).
func (p *Peer) refreshForwardFactsIfLiveFrom(connected []netip.Prefix, neighbors neighborView) {
	for {
		previous := p.fwdFacts.Load()
		if previous == nil {
			return
		}
		p.refreshLinkScopeWith(connected, neighbors)
		next := p.buildForwardFacts()
		if p.fwdFacts.CompareAndSwap(previous, next) {
			p.reportRSRelayGrants(previous, next)
			return
		}
	}
}

// buildForwardFacts computes the snapshot. It reads the peer's settings but stores
// nothing, so the caller decides whether publishing it is correct.
func (p *Peer) buildForwardFacts() *peerForwardFacts {
	s := p.settings

	// ExportPolicy and PeerAS are read under p.mu because both are mutable, and both
	// writers hold this lock: applyHotSwappableSettings (peer_settings_apply.go) writes the
	// filter pair from the config-reload goroutine, and resolveDynamicPeerSettings
	// (reactor_dynamic.go) writes PeerAS and the filter pair on a dynamic peer's
	// establishment.
	//
	// IsEBGP and AIGPEnabled are taken here because both compare LocalAS against
	// PeerAS. Calling either outside the lock would read that mutable field again
	// without synchronization.
	//
	// Every other field read below is set at construction and never mutated (the contract
	// on Peer.Settings, peer.go).
	p.mu.RLock()
	var localScope *atomic.Pointer[receiveNextHopScope]
	session := p.session
	if p.session != nil {
		localScope = &p.session.nextHopScope
	}
	connectedLocal := connectedLocalAddress(p.session)
	sendCtxID := p.sendCtxID
	exportPolicy := s.ExportPolicy
	exportGeneration := p.exportGeneration.Load()
	peerAS := s.PeerAS
	isEBGP := s.IsEBGP()
	isIBGP := s.IsIBGP()
	aigpEnabled := s.AIGPEnabled()
	settingsSnap := *s
	settingsSnap.Bus = s.Bus.Clone()
	p.mu.RUnlock()

	ctx := p.sendCtx.Load()
	nc := p.negotiated.Load()

	sendASN4 := true
	if ctx != nil {
		sendASN4 = ctx.ASN4()
	}

	extendedMsg := nc != nil && nc.ExtendedMessage

	rsRelaySendProc := rsRelaySendProcessName(p, &settingsSnap, peerAS)
	facts := &peerForwardFacts{
		dissemination: p.buildDisseminationSnapshot(session, &settingsSnap),
		addr:          s.Address,
		peerKey:       s.PeerKey(),
		addrStr:       p.addrString,

		localAS:       s.LocalAS,
		globalLocalAS: s.GlobalLocalAS,
		peerAS:        peerAS,
		isEBGP:        isEBGP,
		localAddr:     s.LocalAddress.Unmap(),
		localScope:    localScope,
		aigpEnabled:   aigpEnabled,

		rsClient:           s.RSClient,
		rrClient:           s.RouteReflectorClient,
		asOverride:         s.ASOverride,
		rsRelaySend:        rsRelaySendProc != "",
		rsRelaySendProcess: rsRelaySendProc,
		rsRelayReceive:     rsRelayReceiveGranted(p, &settingsSnap, peerAS),
		propagatePrefixSID: s.PropagateSRv6PrefixSID,
		name:               s.Name,
		groupName:          s.GroupName,
		exportPolicy:       exportPolicy,
		exportGeneration:   exportGeneration,
		exportChainActive:  hasActiveFilter(exportPolicy, directionExport),

		sendCtxID:   sendCtxID,
		sendASN4:    sendASN4,
		extendedMsg: extendedMsg,
		maxMsgSize:  int(message.MaxMessageLength(msgtype.TypeUPDATE, extendedMsg)),

		filterInfo: filterapi.PeerFilterInfo{
			PeerOwner:    p.sourceOwner,
			ConnectionID: connectionIDOf(session),
			Address:      s.Address,
			PeerAS:       peerAS,
			// LocalAS is the effective per-peer local AS (per-peer local-as
			// override when set, otherwise the global local-as). Filling it here
			// lets egress filters read dest.LocalAS instead of re-parsing the raw
			// config JSON: role/OTC stamps it (RFC 9234 R008) and gr/LLGR uses it
			// for iBGP detection (RFC 9494 Section 4.5.3). The readvertise rail
			// already fills the same field from peer.Settings().LocalAS
			// (reactor_api_batch.go), so both egress rails agree.
			LocalAS:   s.LocalAS,
			Name:      s.Name,
			GroupName: s.GroupName,
			RSClient:  s.RSClient,
			IsIBGP:    isIBGP,
		},
	}

	facts.clusterID = s.effectiveClusterID()
	binary.BigEndian.PutUint32(facts.clusterIDBytes[:], facts.clusterID)

	facts.secondaryAS = secondaryPrependAS(s)
	facts.filterInfo.ASOverride = s.ASOverride
	facts.filterInfo.SecondaryAS = facts.secondaryAS

	facts.connectedLocal = connectedLocal
	precomputeNextHop(s, facts)
	applyLinkLocalNextHop(s, facts, p.llScope.Load())
	precomputeVPNNextHop(facts)
	precomputeSendCommunity(s, facts)
	precomputeRedirectNextHop(s, facts)

	return facts
}

// secondaryPrependAS answers the ASN that sits BEHIND the per-peer local-as on
// the egress prepend toward this peer, or zero when one ASN is prepended.
//
// RFC 7705 Section 3.3 states the base behavior toward a peer that carries a
// local-as override: "The BGP router SHOULD first append the globally
// configured ASN to the AS_PATH immediately followed by the 'Local AS' value
// before advertising the UPDATE to an eBGP neighbor." That is the two-ASN form,
// and it is what this returns unless "Replace Old AS" turns it off.
//
// The two local-as options of that section are DIFFERENT rules on DIFFERENT
// rails, and only one of them reaches this rail:
//
//   - "Replace Old AS" is the outbound option, and toward the peer that carries
//     it "the BGP speaker MUST NOT append the globally configured ASN from the
//     AS_PATH attribute. The BGP router MUST append only the configured 'Local
//     AS' ASN value". Zero here is what removes the globally configured ASN.
//   - "No Prepend Inbound" is the inbound option: it governs what is installed
//     and what is advertised to iBGP neighbors, and it "MUST still append the
//     globally configured ASN as normal when advertising the UPDATE to other
//     local eBGP neighbors". It therefore MUST NOT be read here. Reading it
//     here gave the two options one behavior, so an operator asking for either
//     one got the outbound replacement.
func secondaryPrependAS(s *PeerSettings) uint32 {
	if s.GlobalLocalAS == 0 {
		return 0
	}
	// No per-peer override: one AS, prepended once, and RFC 7705 does not apply.
	if s.GlobalLocalAS == s.LocalAS {
		return 0
	}
	if s.LocalASReplaceAS {
		return 0
	}
	return s.GlobalLocalAS
}

// localASPrepend is the AS numbers ONE egress rail prepends toward ONE peer,
// outermost first: the per-peer local AS, then the globally configured AS
// number behind it when RFC 7705 Section 3.3 owes both.
//
// It exists so the two rails cannot disagree. The forward rails read
// secondaryPrependAS once per settings change and cache the answer on the facts
// snapshot (facts.secondaryAS). The announce rail cannot: its queue branch runs
// for a peer whose session is not established (shouldQueue, peer.go), and that
// is exactly when p.fwdFacts holds nil, because clearEncodingContexts stores nil
// on teardown and only setEncodingContexts puts one back. So the announce rail
// asks the same FUNCTION rather than reading the cached value, and this type is
// what carries the pair through its builders without either rail spelling the
// rule a second time.
//
// The zero value is not a valid prepend. Every construction goes through
// localASPrependFor or localASOnly.
type localASPrepend struct {
	// primary is the AS number the peer expects adjacent to itself: the per-peer
	// local-as override where one is configured, otherwise the router's own AS.
	primary uint32
	// secondary is the globally configured AS number, sitting BEHIND primary, or
	// zero when one AS number is prepended.
	secondary uint32
}

// localASPrependFor answers what this peer's settings owe on an egress prepend.
func localASPrependFor(s *PeerSettings) localASPrepend {
	return localASPrepend{primary: s.LocalAS, secondary: secondaryPrependAS(s)}
}

// localASOnly is the single-AS prepend, for a caller that has an AS number and
// no peer settings behind it.
func localASOnly(asn uint32) localASPrepend {
	return localASPrepend{primary: asn}
}

// owed reports whether there is an AS number to prepend at all. A zero primary
// is the "no local AS known" case the announce rail already refused to prepend
// on, and it stays refused.
func (p localASPrepend) owed() bool {
	return p.primary != 0
}

// asns appends this prepend's AS numbers to dst, OUTERMOST first, and returns
// the extended slice. dst is normally a stack array, so the announce path stays
// allocation-free.
//
// Outermost first is this rail's order: every builder in reactor_api_batch.go
// writes asns[0] closest to the peer. The forward rail's wireu.ASPathIntent
// carries the reverse, innermost first, because it prepends one at a time.
func (p localASPrepend) asns(dst []uint32) []uint32 {
	dst = append(dst, p.primary)
	if p.secondary == 0 {
		return dst
	}
	return append(dst, p.secondary)
}

// prependTo inserts this prepend's AS numbers in front of path, leaving the
// outermost one first. It prepends the innermost first because ASPath.Prepend
// pushes onto the front.
func (p localASPrepend) prependTo(path *attribute.ASPath) {
	if p.secondary != 0 {
		path.Prepend(p.secondary)
	}
	path.Prepend(p.primary)
}

// precomputeNextHop fixes the session's immutable next-hop-self wire buffers
// once per facts snapshot from the connected local endpoint (f.connectedLocal)
// or configured LocalAddress, and arms the zero-allocation static fast path
// when f.exportPolicy resolves statically (staticExportNextHop). An IPv6 self
// address lands on the single-address form here; applyLinkLocalNextHop then
// decides whether RFC 2545 Section 3 puts a second address beside it.
func precomputeNextHop(s *PeerSettings, f *peerForwardFacts) {
	// RFC 4271 Section 5.1.3: "A BGP speaker MUST be able to support the
	// disabling advertisement of third party NEXT_HOP attributes in order to
	// handle imperfectly bridged media."
	f.staticNextHop, f.hasStaticNextHop = staticExportNextHop(f.exportPolicy)
	f.nhMode = nhModeNone
	f.nhSelfWithheld = false
	if f.hasStaticNextHop && !intentUsesSelf(f, f.staticNextHop) {
		// Static unchanged / iBGP auto / RS-client auto never rewrites to self;
		// leave selfMode and all self wire buffers zero so downstream scope
		// code never populates speaker link-local addresses for this peer.
		f.selfMode = nhModeNone
		f.selfWithheld = false
		return
	}

	// Next-hop self is built from the address the peer reaches this speaker
	// on: the connected endpoint, which is also what the announce rail sends
	// (resolveNextHop). `local ip auto` leaves LocalAddress unset, and the
	// endpoint is the only answer there. The configured address is read only
	// when the session holds no endpoint; the socket is bound to it, so the
	// two agree whenever both exist.
	local := f.connectedLocal
	if !local.IsValid() {
		local = s.LocalAddress.Unmap()
	}
	switch {
	case !local.IsValid():
		f.selfMode = nhModeNone
		f.selfWithheld = true
	case local.Is4():
		f.selfMode = nhModeSelf4
		f.selfWithheld = false
		f.nhLegacy = local.As4()
		f.nhMapped = local.As16()
	default:
		f.selfMode = nhModeSelfV6
		f.selfWithheld = false
		f.nhGlobal = local.As16()
	}

	if f.hasStaticNextHop {
		f.nhMode = f.selfMode
		f.nhSelfWithheld = f.selfWithheld
	}
}

// intentUsesSelf reports whether intent resolves to next-hop self for
// destination f.
//
// RFC 4271 Section 5.1.3 items 2 and 3: by default (auto) the speaker SHOULD
// use the interface address with which it establishes the eBGP connection.
// RFC 7947 Section 2.2.1: "As the route server does not participate in the
// actual routing of traffic, the NEXT_HOP attribute MUST be passed
// unmodified to the route server clients, similar to the "third-party"
// next-hop feature described in Section 5.1.3. of [RFC4271]."
// Internal peers also retain their received next hop unless self was selected.
func intentUsesSelf(f *peerForwardFacts, intent policychain.NextHopIntent) bool {
	switch intent.Mode {
	case policychain.NextHopModeSelf:
		return true
	case policychain.NextHopModeAuto:
		return f.isEBGP && !f.rsClient
	case policychain.NextHopModeUnspecified, policychain.NextHopModeUnchanged, policychain.NextHopModeExplicit:
		return false
	default:
		panic("BUG: reactor: invalid policychain.NextHopMode")
	}
}

func precomputeSendCommunity(s *PeerSettings, f *peerForwardFacts) {
	f.scMask |= sendCommunitySuppression(s.SendCommunity)
}

// sendCommunitySuppression reads a peer's send-community list into the mask of
// community attributes that MUST NOT be forwarded to it.
//
// It is the ONLY reader of the five keywords in Ze. They are the
// `session/community/send` enumeration of ze-bgp-conf.yang, and
// TestSendCommunityKeywordsMatchTheYANGModel holds the two together. The
// precomputed per-peer path and the API forward path both call this, so the
// keyword an operator writes cannot mean one thing on one path and another on
// the other.
//
// An empty list and "all" both mean send every type, so both answer a zero
// mask. "none" answers all three suppression bits whatever else the list holds.
func sendCommunitySuppression(list []string) sendCommunityMask {
	if len(list) == 0 {
		return 0
	}
	sendStandard, sendLarge, sendExtended := false, false, false
	for _, v := range list {
		// enumeration: gated by TestSendCommunityKeywordsMatchTheYANGModel
		switch v {
		case "all":
			return 0
		case "none":
			return scSuppressStandard | scSuppressExtended | scSuppressLarge
		case "standard":
			sendStandard = true
		case "large":
			sendLarge = true
		case "extended":
			sendExtended = true
		}
	}
	var mask sendCommunityMask
	if !sendStandard {
		mask |= scSuppressStandard
	}
	if !sendExtended {
		mask |= scSuppressExtended
	}
	if !sendLarge {
		mask |= scSuppressLarge
	}
	return mask
}

// precomputeRedirectNextHop decides, once per facts snapshot, what this
// destination does with a FlowSpec redirect-to-IP route's next hop. It runs
// after precomputeNextHop and precomputeSendCommunity, whose results it reads.
func precomputeRedirectNextHop(s *PeerSettings, f *peerForwardFacts) {
	// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of
	// the Next-Hop Network Address MUST be set to 0." Carrying a non-zero next
	// hop for the 0x0800 redirect-to-IP community (draft-simpson-idr-flowspec-
	// redirect-02 Section 3) is an explicit per-session opt-in.
	if !s.FlowSpecRedirectNextHop {
		f.redirectEnabled = false
		f.redirect.action = redirectStrip
		return
	}
	// The 0x0800 community is an extended community. A destination that is
	// not sent extended communities receives no redirect action, so its next
	// hop is the RFC 8955 Section 4 one: "MUST be set to 0".
	if f.scMask&scSuppressExtended != 0 {
		f.redirectEnabled = false
		f.redirect.action = redirectStrip
		return
	}
	f.redirectEnabled = true
	f.redirectSelf = redirectNextHop{action: redirectStrip}
	f.redirectSelf.add(f.connectedLocal)
	f.redirectSelf.add(s.LocalAddress)
	f.redirectSelf.rewriteIfAddressed()

	if f.hasStaticNextHop {
		f.redirect = redirectForIntent(f, f.staticNextHop)
		return
	}
	// Direct callers that invoke precomputeRedirectNextHop without
	// precomputeNextHop still resolve a static export policy.
	if intent, static := staticExportNextHop(f.exportPolicy); static {
		f.redirect = redirectForIntent(f, intent)
		return
	}
	f.redirect.action = redirectStrip
}

// redirectForIntent answers the FlowSpec 0x0800 redirect-to-IP next-hop
// decision for destination f under resolved export-chain intent.
func redirectForIntent(f *peerForwardFacts, intent policychain.NextHopIntent) redirectNextHop {
	if !f.redirectEnabled {
		return redirectNextHop{action: redirectStrip}
	}
	switch intent.Mode {
	case policychain.NextHopModeUnchanged:
		return redirectNextHop{action: redirectKeep}
	case policychain.NextHopModeSelf:
		// With no usable self address, f.redirectSelf.action is redirectStrip:
		// the FlowSpec rule leaves with the RFC 8955 Section 4 zero length.
		return f.redirectSelf
	case policychain.NextHopModeAuto:
		// RFC 7947 Section 2.2.1: "the NEXT_HOP attribute MUST be passed
		// unmodified to the route server clients".
		if f.rsClient {
			return redirectNextHop{action: redirectKeep}
		}
		if !f.isEBGP {
			return redirectNextHop{action: redirectKeep}
		}
		// draft-simpson-idr-flowspec-redirect-02 Section 3: "When a flow-spec
		// route with this community is advertised to an EBGP peer the
		// next-hop address in the MP_REACH_NLRI SHOULD be reset to an address
		// of the advertising router by default, per normal BGP procedures."
		return f.redirectSelf
	case policychain.NextHopModeExplicit:
		r := redirectNextHop{action: redirectStrip}
		r.add(intent.Addr)
		r.rewriteIfAddressed()
		return r
	case policychain.NextHopModeUnspecified:
		return redirectNextHop{action: redirectStrip}
	default:
		panic("BUG: reactor: invalid policychain.NextHopMode")
	}
}

// rewriteIfAddressed turns the decision into a rewrite once an address of
// either family was recorded. A rule of the other family still leaves with the
// zero length (hop returns nil for it).
func (r *redirectNextHop) rewriteIfAddressed() {
	if r.has4 || r.has6 {
		r.action = redirectRewrite
	}
}

func applyFactsNextHop(f *peerForwardFacts, mods *filterapi.ModAccumulator, mpFamily family.Family) {
	applyNextHopWire(f, f.nhMode, mods, mpFamily)
}

func applyNextHopWire(f *peerForwardFacts, mode uint8, mods *filterapi.ModAccumulator, mpFamily family.Family) {
	if mode == nhModeNone {
		return
	}
	applySelfLegacyNextHop(f, mode, mods)
	applySelfMPNextHop(f, mode, mods, mpFamily)
}

// applyNextHopWireForBase records next-hop-self rewrites only for the NLRI
// sections actually carried by baseNextHop. A pure MP_REACH UPDATE carries no
// classic IPv4 NLRI (classicPresence == nextHopClassicAbsent) and no code-3
// NEXT_HOP, so recording AttrModSet(3) would synthesize a spurious legacy
// NEXT_HOP attribute on pure multiprotocol announcements (RFC 4760 Section 3).
func applyNextHopWireForBase(f *peerForwardFacts, mode uint8, baseNextHop nextHopValue, mods *filterapi.ModAccumulator) {
	if mode == nhModeNone {
		return
	}
	if baseNextHop.legacy.IsValid() || baseNextHop.classicPresence != nextHopClassicAbsent {
		applySelfLegacyNextHop(f, mode, mods)
	}
	if baseNextHop.mpFamily != (family.Family{}) && baseNextHop.mpFamily.NeedsNextHop() {
		applySelfMPNextHop(f, mode, mods, baseNextHop.mpFamily)
	}
}

func applySelfLegacyNextHop(f *peerForwardFacts, mode uint8, mods *filterapi.ModAccumulator) {
	if mode == nhModeSelf4 {
		mods.Op(3, filterapi.AttrModSet, f.nhLegacy[:])
	}
}

func applySelfMPNextHop(f *peerForwardFacts, mode uint8, mods *filterapi.ModAccumulator, mpFamily family.Family) {
	switch mode {
	case nhModeNone:
		return
	case nhModeSelf4:
		profile := attribute.MPNextHopProfile(attribute.AFI(mpFamily.AFI), attribute.SAFI(mpFamily.SAFI))
		// This value witnesses the address form only; its bytes never leave.
		// The actual address already has immutable native and mapped buffers.
		// Native width includes the profile's RD, so LS-VPN selects twelve
		// octets instead of requesting mapping because four is absent.
		native := netip.IPv4Unspecified()
		mapV4 := false
		if len(profile.Lengths) != 0 {
			mapV4 = !slices.Contains(profile.Lengths, profile.NextHopOctets(native))
		}
		// RFC 4659 Section 3.2.1.2: the VPN-IPv6 profile requests mapped
		// encoding by default. Other profiles retain their own declared form.
		selected := profile.SelectAddress(native, mapV4)
		if profile.RDPerAddress {
			if selected.Is4() {
				mods.Op(14, filterapi.AttrModSet, f.nhVPNv4[:])
				return
			}
			mods.Op(14, filterapi.AttrModSet, f.nhVPNMapped[:])
			return
		}
		if selected.Is4() {
			mods.Op(14, filterapi.AttrModSet, f.nhLegacy[:])
			return
		}
		mods.Op(14, filterapi.AttrModSet, f.nhMapped[:])
	case nhModeSelfV6, nhModeSelfV6LL:
		profile := attribute.MPNextHopProfile(attribute.AFI(mpFamily.AFI), attribute.SAFI(mpFamily.SAFI))
		if profile.RDPerAddress {
			mods.Op(14, filterapi.AttrModSet, f.rdIPv6NextHop(mpFamily, mode))
			return
		}
		if mode == nhModeSelfV6LL {
			mods.Op(14, filterapi.AttrModSet, f.nhGlobalLL[:])
			return
		}
		mods.Op(14, filterapi.AttrModSet, f.nhGlobal[:])
	}
}

// rdIPv6NextHop selects an RD-framed field without changing its scope condition.
// LS-VPN uses the joint-subnet decision already recorded by nhModeSelfV6LL.
//
// RFC 4659 Section 3.2.1.1: "The value of the Length of the Next Hop Network
// Address field in the MP_REACH_NLRI attribute shall be set to 24 when only a
// global address is present, and to 48 if a link-local address is also
// included in the Next Hop field." Only that family's peer-only condition uses
// nhVPNLinkLocal; RD framing alone does not confer its exception on LS-VPN.
func (f *peerForwardFacts) rdIPv6NextHop(fam family.Family, mode uint8) []byte {
	includeLinkLocal := mode == nhModeSelfV6LL
	if linkLocalConditionFor(fam) == linkLocalConditionPeer {
		includeLinkLocal = f.nhVPNLinkLocal
	}
	if includeLinkLocal {
		return f.nhVPNPair[:]
	}
	return f.nhVPNPair[:24]
}

// precomputeVPNNextHop writes the reusable RD-framed forms once per facts
// snapshot, so the forward rails slice them and copy nothing. The existing VPN
// profiles supply the shared layout; the emitting family's profile admits it.
//
// RFC 4659 Section 3.2.1.1: each VPN-IPv6 address of the Next Hop field has
// its "8-octet RD is set to zero"; the RD slots keep the zero value.
//
//	nhVPNv4:     +0 RD=0 +8 IPv4 +12
//	nhVPNMapped: +0 RD=0 +8 ::ffff:IPv4 +24
//	nhVPNPair:   +0 RD=0 +8 Global +24 RD=0 +32 Link-Local +48
func precomputeVPNNextHop(f *peerForwardFacts) {
	v4Profile := attribute.MPNextHopProfile(attribute.AFIIPv4, attribute.SAFIVPN)
	v6Profile := attribute.MPNextHopProfile(attribute.AFIIPv6, attribute.SAFIVPN)
	// RFC 4364 Section 4.3.4 and RFC 4659 Section 3.2.1.2: each address has
	// a zero RD, including the already-selected mapped IPv4 form.
	v4Profile.WriteNextHops(f.nhVPNv4[:], 0, netip.AddrFrom4(f.nhLegacy), netip.Addr{})
	v6Profile.WriteNextHops(f.nhVPNMapped[:], 0, netip.AddrFrom16(f.nhMapped), netip.Addr{})
	var linkLocal netip.Addr
	if f.selfMode == nhModeSelfV6LL || f.nhMode == nhModeSelfV6LL {
		// The joint condition may select another router's neighbor-table
		// address, which the VPN peer-only decision deliberately does not.
		linkLocal = netip.AddrFrom16([16]byte(f.nhGlobalLL[16:]))
	} else if f.nhVPNLinkLocal {
		linkLocal = netip.AddrFrom16([16]byte(f.nhVPNPair[32:]))
	}
	// RFC 4659 Section 3.2.1.1: the second address gets its own zero RD.
	v6Profile.WriteNextHops(f.nhVPNPair[:], 0, netip.AddrFrom16(f.nhGlobal), linkLocal)
}

func applyFactsSendCommunity(f *peerForwardFacts, mods *filterapi.ModAccumulator) {
	applySendCommunityMask(f.scMask, mods)
}

// applySendCommunityMask writes one suppress op for each community attribute
// the mask names: COMMUNITIES (8), EXTENDED_COMMUNITIES (16), LARGE_COMMUNITIES
// (32).
func applySendCommunityMask(mask sendCommunityMask, mods *filterapi.ModAccumulator) {
	if mask == 0 {
		return
	}
	if mask&scSuppressStandard != 0 {
		mods.Op(8, filterapi.AttrModSuppress, nil)
	}
	if mask&scSuppressExtended != 0 {
		mods.Op(16, filterapi.AttrModSuppress, nil)
	}
	if mask&scSuppressLarge != 0 {
		mods.Op(32, filterapi.AttrModSuppress, nil)
	}
}
