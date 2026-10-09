// Design: docs/architecture/core-design.md -- egress route decisions on the forward rails
// RFC: rfc/short/rfc4271.md -- a route is not advertised to a peer using that peer's own address as NEXT_HOP (Section 5.1.3)
// Related: forward_med.go -- the Section 5.1.4 sibling, which shares this once-per-UPDATE / once-per-destination shape
// Related: peer_forward_facts.go -- applyFactsNextHop, which records ze's own next-hop rewrite
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
package reactor

import (
	"encoding/binary"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
)

// nextHopValue names every NEXT_HOP address one UPDATE offers a destination.
//
// There are three because one UPDATE can carry more than one. The legacy
// NEXT_HOP attribute (code 3) governs IPv4 unicast; MP_REACH_NLRI (code 14)
// governs every other family and carries a SECOND address in the RFC 2545
// Section 3 two-address form. A single address would have to pick one and would
// then answer about a route the destination is not being sent.
//
// The addresses are held unmapped, so an IPv4-mapped 16-byte form and the
// 4-byte form of the same address compare equal.
type nextHopValue struct {
	legacy netip.Addr // NEXT_HOP attribute, code 3
	mp     netip.Addr // MP_REACH_NLRI global next hop, code 14
	mpLL   netip.Addr // MP_REACH_NLRI link-local next hop, RFC 2545 Section 3

	// mpFamily is the AFI/SAFI of the MP_REACH_NLRI that carries mp: the pair
	// RFC 8950 Section 4 licenses an IPv6 next hop for. Zero when the UPDATE
	// carries no MP_REACH_NLRI. A next-hop rewrite never changes it.
	mpFamily family.Family

	// mpIPv6 records the wire address family before nextHopAddr unmaps it.
	// The 16/24/32/48-octet forms are IPv6; 4/12 are IPv4. The NLRI AFI
	// cannot supply this fact (RFC 9830 Section 2.1), and every MP rewrite
	// replaces it. False with no MP address means no wire family was read.
	mpIPv6 bool

	// mpWireLen retains the received field's length before address extraction.
	// Ordinary origination must validate it against its NLRI family: decoding
	// a VPN-shaped field into a global address does not license that wire form.
	mpWireLen uint8

	// Canonical RFC 4659 Section 3.2.1.1 absent-global VPN pair: zero RDs,
	// unspecified Global, and a valid Link-Local second address. Permission
	// still depends on the family and this destination's actual peering.
	mpVPNUnspecifiedPair bool

	// A received field that the family forbids must leave on advertisement,
	// but its bytes never identify a forwarding address or a withholding gate.
	mpIgnored bool
}

// has reports whether any address this UPDATE offers is addr.
func (n nextHopValue) has(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	return n.legacy == addr || n.mp == addr || n.mpLL == addr
}

// valid reports whether any address was found at all.
func (n nextHopValue) valid() bool {
	return n.legacy.IsValid() || n.mp.IsValid() || n.mpLL.IsValid()
}

// nextHopAddr turns a wire next-hop field of any documented length into an
// address pair. The second address is the RFC 2545 Section 3 link-local half and
// is invalid for every other length.
//
// The VPN forms (RFC 4364 Section 4.3.2) prefix the address with an 8-octet
// Route Distinguisher that is always zero on a next hop, so the address sits at
// offset 8 and the length is 8 longer. Reading the RD as the address is how a
// VPN next hop would silently compare against nothing.
func nextHopAddr(nh []byte) (netip.Addr, netip.Addr) {
	switch len(nh) {
	case 4:
		return netip.AddrFrom4([4]byte(nh)), netip.Addr{}
	case 16:
		return netip.AddrFrom16([16]byte(nh)).Unmap(), netip.Addr{}
	case 32:
		return netip.AddrFrom16([16]byte(nh[:16])).Unmap(), netip.AddrFrom16([16]byte(nh[16:])).Unmap()
	case 12: // RD + IPv4
		return netip.AddrFrom4([4]byte(nh[8:])), netip.Addr{}
	case 24: // RD + IPv6
		return netip.AddrFrom16([16]byte(nh[8:])).Unmap(), netip.Addr{}
	case 48: // RD + IPv6 global, RD + IPv6 link-local
		return netip.AddrFrom16([16]byte(nh[8:24])).Unmap(), netip.AddrFrom16([16]byte(nh[32:])).Unmap()
	}
	return netip.Addr{}, netip.Addr{}
}

// payloadNextHop reports every NEXT_HOP an UPDATE payload advertises.
//
// It reads the attribute SECTION rather than the payload bytes, so a prefix in
// the NLRI holding the byte 0x03 is not mistaken for the attribute. A malformed
// payload answers nothing: the RFC 7606 handling of a bad payload is not this
// function's decision (validateNextHopAttr and validateMPReachNextHop,
// message/rfc7606.go, own the lengths).
//
// Allocation-free: ParseUpdateSections computes offsets and AttrFind walks the
// attribute headers in place.
func payloadNextHop(payload []byte) nextHopValue {
	var out nextHopValue
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return out
	}
	attrs := sections.Attrs(payload)
	if attrs == nil {
		return out
	}
	if _, _, value, found := attribute.AttrFind(attrs, attribute.AttrNextHop); found && len(value) == 4 {
		out.legacy = netip.AddrFrom4([4]byte(value))
	}
	if _, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI); found {
		// AFI(2) + SAFI(1) + next-hop length(1) + next hop.
		if len(value) >= 4 {
			out.mpFamily = family.Family{AFI: family.AFI(binary.BigEndian.Uint16(value)), SAFI: family.SAFI(value[2])}
			out.mpWireLen = value[3]
			nhLen := int(value[3])
			if 4+nhLen <= len(value) {
				if out.mpFamily.NeedsNextHop() {
					out.mp, out.mpLL = nextHopAddr(value[4 : 4+nhLen])
					out.mpIPv6 = nhLen == 16 || nhLen == 24 || nhLen == 32 || nhLen == 48
					out.mpVPNUnspecifiedPair = vpnUnspecifiedNextHopPair(value[4 : 4+nhLen])
				} else {
					out.mpIgnored = nhLen != 0
				}
			}
		}
	}
	return out
}

// applyNextHopFamily enforces the same family contract as origination before
// any egress next-hop decision. RFC 8955 Section 4: "When advertising Flow
// Specifications, the Length of the Next-Hop Network Address MUST be set to 0.
// The Network Address of the Next-Hop field MUST be ignored."
//
// Clear only MP operations, retaining a genuine legacy sibling's NEXT_HOP.
// Reuse existing operation slots rather than growing the accumulator. Received
// bytes remain immutable; the registered handler materializes the empty field
// in the destination-owned output. An already empty, unchanged field costs no
// edit and keeps the forwarding identity path.
func applyNextHopFamily(mods *filterapi.ModAccumulator, base nextHopValue) {
	if base.mpFamily.NeedsNextHop() {
		return
	}
	found := false
	ops := mods.Ops()
	for i := range ops {
		if ops[i].Code == uint8(attribute.AttrMPReachNLRI) && ops[i].Action == filterapi.AttrModSet {
			ops[i] = filterapi.AttrOp{Code: uint8(attribute.AttrMPReachNLRI), Action: filterapi.AttrModSet}
			found = true
		}
	}
	if base.mpIgnored && !found {
		mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, nil)
	}
}

// modsNextHop reports the NEXT_HOP this destination's accumulated operations
// write, and whether any operation writes one at all.
//
// Both writers of a next hop on the egress rails record an AttrModSet here:
// applyFactsNextHop (peer_forward_facts.go) for a configured next-hop mode, and
// an egress filter for a policy rewrite. Later Sets win, which is what
// genericAttrSetHandler and mpReachNextHopHandler (filter_delta_handlers.go)
// both do with the operation list, so a scan that keeps the LAST match answers
// about the bytes the rebuild will emit.
//
// Codes 3 and 14 are collected independently because one destination can carry
// both: applyFactsNextHop records the legacy address and the family-permitted
// MP_REACH form together for an IPv4 next-hop mode.
func modsNextHop(mods *filterapi.ModAccumulator) (nextHopValue, bool) {
	var out nextHopValue
	set := false
	for _, op := range mods.Ops() {
		if op.Action != filterapi.AttrModSet {
			continue
		}
		switch op.Code {
		case uint8(attribute.AttrNextHop):
			if a, _ := nextHopAddr(op.Buf); a.IsValid() {
				out.legacy = a
				set = true
			}
		case uint8(attribute.AttrMPReachNLRI):
			if a, ll := nextHopAddr(op.Buf); a.IsValid() {
				out.mp, out.mpLL = a, ll
				out.mpWireLen = uint8(len(op.Buf)) // nextHopAddr accepts at most 48 octets.
				out.mpIPv6 = len(op.Buf) == 16 || len(op.Buf) == 24 || len(op.Buf) == 32 || len(op.Buf) == 48
				out.mpVPNUnspecifiedPair = vpnUnspecifiedNextHopPair(op.Buf)
				set = true
			}
		}
	}
	return out, set
}

// egressNextHopIsPeerOwn answers, for ONE destination, whether the NEXT_HOP it
// is about to be sent is that destination's OWN address.
//
// RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
// advertised to a peer using an address of that peer as NEXT_HOP."
//
// Telling a peer to reach a destination through itself is a blackhole: the peer
// resolves the next hop to one of its own interfaces and the traffic never
// leaves it. The hazard has one shape and three producers, so the question is
// asked about the ADDRESS that will be on the wire rather than about any one
// producer:
//
//   - a configured next-hop mode, recorded by applyFactsNextHop. This is the
//     operator pointing every route at a fixed address, so it hits every route
//     to that destination rather than one.
//   - an egress filter's next-hop rewrite, which reaches the rails as the same
//     operation and is read the same way. A policy may not grant what the RFC
//     refuses, and this is why the question is asked AFTER the filter pass.
//   - the received NEXT_HOP carried through unchanged, which is the third-party
//     next hop Section 5.1.3 case 2 permits. A route server relaying one client's
//     third-party next hop to the client that OWNS that address is the everyday
//     way this arises, and it is why the check is on both rails.
//
// mods is read first because a Set replaces the payload's address. base is the
// payload the rebuild runs over, read once per UPDATE by the caller and re-read
// only for a destination whose base differs (the shape applyFactsMED uses).
//
// Allocation-free: netip.Addr comparisons over a fixed-size operation list.
func egressNextHopIsPeerOwn(f *peerForwardFacts, mods *filterapi.ModAccumulator, base nextHopValue) bool {
	if !f.addr.IsValid() {
		return false
	}
	if nh, set := modsNextHop(mods); set {
		return nh.has(f.addr)
	}
	return base.has(f.addr)
}

// originatedNextHopIsPeerOwn asks the SAME question of a route Ze ORIGINATES,
// which is the case RFC 4271 Section 5.1.3 names in so many words:
//
//	"A route originated by a BGP speaker SHALL NOT be advertised to a peer
//	 using an address of that peer as NEXT_HOP."
//
// egressNextHopIsPeerOwn above covers the RELAYED route, where the hazard
// arrives as a third-party next hop Section 5.1.3 case 2 permits. This function
// covers the originated one, where the address comes from Ze's own configuration
// or from a plugin. Between them the two cover every route Ze advertises.
//
// IT READS THE BUILT BODY RATHER THAN ANY PRODUCER'S TYPED VALUE, and that is
// the whole point of asking it here. Five rails originate a NEXT_HOP today:
// configured static routes and default-originate (peer_initial_sync.go), the RIB
// op-queue drain (peer_rib_routes.go), the announce batch and the RFC 9494 stale
// re-advertise (reactor_api_batch.go). Each resolves its address differently and
// each already refuses a next hop it cannot ENCODE, so a per-rail copy of this
// question would be five copies to keep in step and a sixth rail would be born
// without it. One question at the write boundary cannot be forgotten.
//
// A withdrawal and an End-of-RIB marker reach this with no NEXT_HOP and no
// MP_REACH_NLRI, so they answer false without a special case.
//
// NO EXEMPTION FOR AN ADDRESS THAT IS ALSO ZE'S OWN, and the temptation is real.
// `next-hop self` resolves to Ze's local address on the session
// (precomputeNextHop, peer_forward_facts.go), so on a fixture whose two ends
// carry ONE address every originated route is withheld here and the fixture goes
// red. That reads like a false positive and is not one. Section 5.1.3's
// prohibition is SHALL NOT and its "use its own IP address" is only SHOULD, so
// the prohibition governs wherever the two name one value. BIRD lands the same
// way: bgp_update_next_hop_ip (proto/bgp/packets.c) tests the final next hop
// against remote_ip AFTER its own next-hop-self substitution and withdraws the
// route. FRR compares nothing on that path and emits the violating address. A
// session whose two ends hold one address is a topology no hardware reaches, and
// a fixture that reaches it is the thing to fix. Owner decision, 2026-08-15.
//
// Allocation-free: payloadNextHop walks the attribute headers in place.
func originatedNextHopIsPeerOwn(body []byte, peer netip.Addr) bool {
	if !peer.IsValid() {
		return false
	}
	return payloadNextHop(body).has(peer)
}

// linkLocalOnly reports whether the MP_REACH next hop this UPDATE offers is a
// Link-Local-only Next Hop, the 16-octet form of
// draft-ietf-idr-linklocal-capability Section 3.
//
// The length half of the classification is already settled by the time the pair
// is built: nextHopAddr fills mpLL for the 32-octet and 48-octet forms only, so
// an invalid mpLL beside a link-local mp is the 16-octet field. The address half
// is attribute.IsLinkLocalOnlyNextHop's own test, over the address rather than
// over the bytes it came from.
func (n nextHopValue) linkLocalOnly() bool {
	if n.mpLL.IsValid() {
		return false
	}
	return n.mp.Is6() && n.mp.IsLinkLocalUnicast()
}

// egressNextHopIsLinkLocalOnly answers, for ONE destination, whether the
// MP_REACH next hop it is about to be sent is a Link-Local-only Next Hop.
//
// It resolves the address the same way egressNextHopIsPeerOwn does, and for the
// same reason: a next-hop rewrite recorded in mods replaces the payload's
// address, so the question is asked about the bytes the rebuild will emit. That
// is what lets ONE test answer both halves of the route-reflector rule
// (reactor_api_forward.go): a client the operator configured next-hop-self for
// carries ze's own address here and is no longer link-local-only, which is the
// rewrite Section 4 offers as the first of its two answers.
func egressNextHopIsLinkLocalOnly(mods *filterapi.ModAccumulator, base nextHopValue) bool {
	if nh, set := modsNextHop(mods); set {
		return nh.linkLocalOnly()
	}
	return base.linkLocalOnly()
}

// egressNextHopLinkLocalOnlyRefused answers, for ONE destination, whether the
// MP_REACH_NLRI next hop it is about to be sent is a Link-Local-only Next Hop
// the destination's session may not carry.
//
// draft-ietf-idr-linklocal-capability Section 2: "When the capability has not
// been negotiated, the procedures in this document do not apply." Section 5
// adds the Extended Next Hop half for IPv4 NLRI. Peer.linkLocalOnlyNextHopRefused
// (peer.go) holds both, and the announce rail (Peer.resolveNextHop) asks the same
// method, so the rails cannot disagree.
//
// The address asked is the one the rebuild will emit, whoever chose it: one
// recorded in mods by a configured next-hop mode or a filter, or the received
// one relayed unchanged. Next hop self is built from the session's connected
// endpoint (precomputeNextHop, peer_forward_facts.go), and on a session that
// runs over a link-local address that endpoint is link-local. A received
// Link-Local-only next hop is refused too, because RFC 2545 Section 3 makes the
// Global address mandatory in the field: "A BGP speaker shall advertise to its
// peer in the Network Address of Next Hop field the global IPv6 address of the
// next hop, potentially followed by the link-local IPv6 address of the next
// hop." The draft's Section 3 says the Link-Local-only form "received without
// the Link-Local Next Hop Capability having been negotiated is not conformant
// with [RFC2545]".
//
// A destination base with no MP_REACH_NLRI carries no field to write the
// address into, so it answers false.
func egressNextHopLinkLocalOnlyRefused(dest *Peer, mods *filterapi.ModAccumulator, base nextHopValue) bool {
	if base.mpFamily == (family.Family{}) {
		return false
	}
	emitted := base
	if written, set := modsNextHop(mods); set {
		emitted = written
	}
	// A valid second address is the 32-octet RFC 2545 Section 3 pair, whose
	// first address is the Global one: never the Link-Local-only form.
	if !emitted.linkLocalOnly() {
		return false
	}
	return dest.linkLocalOnlyNextHopRefused(emitted.mp, base.mpFamily)
}

// applyEgressNextHopScope normalizes the effective MP next-hop field for ONE
// destination, after policy and the configured next-hop rewrite.
//
// draft-ietf-idr-linklocal-capability Section 4: "When sending a message to an
// external peer X, and the peer is multiple IP hops away from the speaker (aka
// "multihop EBGP"): * Link-Local IPv6 next hops MUST NOT be included." The same
// Section says it of an internal peer: "If the internal peer is more than one
// IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop."
// RFC 2545 Section 3 binds every session, negotiated or not: "The link-local
// address shall be included in the Next Hop field if and only if the BGP
// speaker shares a common subnet with the entity identified by the global IPv6
// address carried in the Network Address of Next Hop field and the peer the
// route is being advertised to."
//
// Under next hop unchanged or auto, applyFactsNextHop records nothing. A
// speaker-owned global introduced by policy still needs the configured own
// Link-Local on a common subnet, and a received pair must lose its Link-Local
// outside that subnet. The last MP_REACH_NLRI Set wins over basePayload.
//
// The destination and effective Global must belong to ONE connected prefix.
// Source-peer membership cannot substitute for either. A nil scope has read no
// interface table and proves no common subnet, so the Link-Local is removed.
//
// A pair with an invalid address in either slot MUST remain intact here:
// trimming it would hide the pair's invalid form from egressNextHopWithheld,
// which MUST refuse it through the existing per-section withdrawal path.
//
// Plain IPv6 next-hop bytes (RFC 2545 Section 3; MP_REACH value offset +4):
//
//	+0                    +16                    +32
//	| Global IPv6 (16 B)  | Link-Local IPv6 (16 B) |
//
// The single-address form ends at +16; the paired form ends at +32.
//
// A trimmed field aliases its original operation or payload. A newly constructed
// pair is copied into mods' owned storage; no stack-backed slice is retained.
func applyEgressNextHopScope(dest *Peer, facts *peerForwardFacts, mods *filterapi.ModAccumulator, basePayload []byte, baseFamily family.Family) {
	field := payloadMPNextHopField(basePayload)
	for _, op := range mods.Ops() {
		if op.Code != uint8(attribute.AttrMPReachNLRI) {
			continue
		}
		if op.Action == filterapi.AttrModSet {
			field = op.Buf
		}
	}
	profile := attribute.MPNextHopProfile(attribute.AFI(baseFamily.AFI), attribute.SAFI(baseFamily.SAFI))
	if profile.IPv6Roles && !slices.Contains(profile.Lengths, len(field)) {
		// RFC 2545 Section 3; RFC 8950 Section 3; RFC 9830 Section 2.1.
		// Preserve a wrong-width field instead of hiding it by RD stripping.
		return
	}
	var globalOctets int
	switch len(field) {
	case 16:
		if !profile.IPv6Roles {
			return
		}
		global := netip.AddrFrom16([16]byte(field)).Unmap()
		if !global.Is6() {
			return
		}
		if !global.IsGlobalUnicast() {
			return
		}
		configured := dest.settings.LinkLocal.Unmap()
		if !configured.Is6() {
			return
		}
		if !configured.IsLinkLocalUnicast() {
			return
		}
		scope := dest.llScope.Load()
		if scope == nil {
			return
		}
		if !scope.peerOnLink {
			return
		}
		owners := facts.nextHopOwners()
		// RFC 2545 Section 3: append only this entity's own Link-Local on
		// the common subnet. Third-party discovery remains an absent feature.
		ll := scope.linkLocalNextHop(configured, global, owners.classify(global))
		if !ll.IsValid() {
			return
		}
		var pair [32]byte
		copy(pair[:16], field)
		address := ll.As16()
		copy(pair[16:], address[:])
		mods.OpCopy(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, pair[:])
		return
	case 32: // Global(16) + Link-Local(16)
		globalOctets = 16
	case 48: // RD(8) + Global(16), RD(8) + Link-Local(16)
		globalOctets = 24
	default:
		return
	}
	second := netip.AddrFrom16([16]byte(field[len(field)-16:])).Unmap()
	if !second.Is6() || !second.IsLinkLocalUnicast() {
		return
	}
	global := netip.AddrFrom16([16]byte(field[globalOctets-16 : globalOctets])).Unmap()
	vpnPair := false
	if !global.Is6() || !global.IsGlobalUnicast() {
		vpnPair = vpnUnspecifiedNextHopPair(field)
		if !vpnPair {
			return
		}
	}
	if scope := dest.llScope.Load(); scope != nil {
		if scope.peerOnLink {
			// RFC 4659 Section 3.2.1.1 explicitly uses an unspecified Global
			// for VPN-IPv6 speakers peering only over link-local addresses.
			if vpnPair {
				if vpnIPv6LinkLocalPeering(facts, baseFamily) {
					return
				}
			}
			// RFC 2545 Section 3: require the joint common-subnet condition.
			if scope.sharesNextHopSubnet(global) {
				return
			}
		}
	}
	mods.Op(uint8(attribute.AttrMPReachNLRI), filterapi.AttrModSet, field[:globalOctets])
}

// vpnUnspecifiedNextHopPair recognizes the exact 48-octet RFC 4659
// Section 3.2.1.1 form. The first 32 octets hold the zero RD, unspecified
// Global, and the second zero RD. A single unspecified address is not this form.
func vpnUnspecifiedNextHopPair(field []byte) bool {
	if len(field) != 48 {
		return false
	}
	if [32]byte(field[:32]) != ([32]byte{}) {
		return false
	}
	return netip.AddrFrom16([16]byte(field[32:])).IsLinkLocalUnicast()
}

// vpnIPv6LinkLocalPeering identifies RFC 4659 Section 3.2.1.1's peering
// exception using the established session's captured local endpoint. Configured
// LocalAddress cannot substitute for the address the peer actually connects to.
func vpnIPv6LinkLocalPeering(f *peerForwardFacts, fam family.Family) bool {
	if fam != (family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}) {
		return false
	}
	if f == nil {
		return false
	}
	if !f.connectedLocal.Is6() {
		return false
	}
	if !f.connectedLocal.IsLinkLocalUnicast() {
		return false
	}
	return f.addr.Is6() && f.addr.IsLinkLocalUnicast()
}

// destOnLink reports whether dest is one IP hop away: a connected subnet of
// this speaker holds its address (Peer.llScope, linkScope.peerOnLink).
//
// A nil scope has read no interface table and proves no shared subnet, so it
// answers false, for the reason linkScope.linkLocalNextHop gives. Pair trimming
// reads the same snapshot's peerOnLink field before checking the Global entity.
func destOnLink(dest *Peer) bool {
	scope := dest.llScope.Load()
	if scope == nil {
		return false
	}
	return scope.peerOnLink
}

// egressNextHopLinkLocalOnlyOffLink answers, for ONE destination, whether the
// MP_REACH_NLRI next hop it is about to be sent is Link-Local-only while the
// destination is more than one IP hop away.
//
// draft-ietf-idr-linklocal-capability Section 4: "If the internal peer is more
// than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next
// hop." Of a multihop external peer: "Link-Local IPv6 next hops MUST NOT be
// included." Removing the only address leaves none, and Section 4 then says:
// "If, after completing these procedures, there are no IPv6 next hop addresses
// included in the next hop, the BGP route MUST not be advertised to its peer.
// Instead, treat-as-withdraw (Section 2 of [RFC7606]) is used." Of the external
// peer it adds: "If a Global IPv6 next hop is not included, the route MUST NOT
// be advertised to the external peer (treat-as-withdraw)."
// RFC 2545 Section 3 binds every session the same way: "The link-local address
// shall be included in the Next Hop field if and only if the BGP speaker shares
// a common subnet with the entity identified by the global IPv6 address carried
// in the Network Address of Next Hop field and the peer the route is being
// advertised to."
//
// applyEgressNextHopScope removes the Link-Local half of a pair, which leaves a
// Global. This is the case with no Global to leave: a received 16-octet
// Link-Local-only field (or the 24-octet RD plus Link-Local VPN form) relayed
// under next hop unchanged or auto. The question is mode-independent, because
// it is asked of the address about to be written: a next-hop rewrite recorded
// in mods replaces the received one and this answers false. An on-link
// destination keeps the received Link-Local ("the speaker can use the received
// Link-Local IPv6 address, provided that peer X is directly attached").
//
// Unspecified and multicast Global addresses are refused independently by
// egressNextHopWithheld, regardless of the destination's subnet.
func egressNextHopLinkLocalOnlyOffLink(dest *Peer, mods *filterapi.ModAccumulator, base nextHopValue) bool {
	if destOnLink(dest) {
		return false
	}
	if nh, set := modsNextHop(mods); set {
		return nh.linkLocalOnly()
	}
	return base.linkLocalOnly()
}

// withholdGate names the egress next-hop gate that refused ONE destination the
// announcement (egressNextHopWithheld). The zero value is never returned.
type withholdGate uint8

const (
	withholdUnspecified            withholdGate = iota
	withholdNone                                // no gate refused the announcement
	withholdNextHopSelfAbsent                   // next-hop self with no local address
	withholdPeerOwn                             // the next hop is the destination's own address
	withholdReflectedLinkLocalOnly              // RR: Link-Local-only, client off the advertiser's segment
	withholdLinkLocalOnlyOffLink                // Link-Local-only, destination more than one hop away
	withholdNoExtendedNextHop                   // IPv6 next hop for IPv4 NLRI without RFC 8950
	withholdLinkLocalOnlyRefused                // Link-Local-only to a session that may not carry it
	withholdUnusableGlobal                      // unusable address in the IPv6 Global slot
	withholdInvalidPair                         // second address is not IPv6 link-local unicast
	withholdInvalidWidth                        // wire field length is not valid for IPv6 unicast
)

// withholdGateText is the operator log line and the governing document of each
// gate, indexed by the gate.
var withholdGateText = [...]struct{ message, rfc string }{
	withholdNextHopSelfAbsent:      {"withholding route: next-hop self is configured and the session has no local address", "RFC 4271 Section 5.1.3"},
	withholdPeerOwn:                {"withholding route: its next hop is this peer's own address", "RFC 4271 Section 5.1.3"},
	withholdReflectedLinkLocalOnly: {"withholding route: its next hop is link-local-only and this client is not on the advertiser's link-layer segment", "draft-ietf-idr-linklocal-capability Section 4"},
	withholdLinkLocalOnlyOffLink:   {"withholding route: its next hop is link-local-only and this peer is more than one IP hop away", "draft-ietf-idr-linklocal-capability Section 4"},
	withholdNoExtendedNextHop:      {"withholding route: its IPv6 next hop for IPv4 NLRI needs the Extended Next Hop capability this peer did not negotiate", "RFC 8950 Section 4"},
	withholdLinkLocalOnlyRefused:   {"withholding route: its next hop is link-local-only and this peer did not negotiate the Link-Local Next Hop capability", "draft-ietf-idr-linklocal-capability Section 2"},
	withholdUnusableGlobal:         {"withholding route: its IPv6 global next-hop slot has no usable global address", "RFC 2545 Section 3"},
	withholdInvalidPair:            {"withholding route: its IPv6 next-hop pair has a non-link-local second address", "RFC 2545 Section 3"},
	withholdInvalidWidth:           {"withholding route: its next-hop wire length is invalid for IPv6 unicast", "RFC 2545 Section 3"},
}

// warn logs the suppression. draft-ietf-idr-linklocal-capability Section 4 asks
// for it: "implementations SHOULD log this suppression, or otherwise expose it
// through operator notification ... so that unexpected reachability gaps can be
// detected." The other gates log for the same reason.
//
// The next hop logged is the one the gate judged: the last rewrite recorded in
// mods, else the received one. Its family is the MP_REACH_NLRI's, or IPv4
// unicast for the legacy NEXT_HOP of an UPDATE that carries no MP_REACH_NLRI
// (RFC 4271 Section 5.1.3).
func (g withholdGate) warn(f *peerForwardFacts, advertiser netip.Addr, mods *filterapi.ModAccumulator, base nextHopValue) {
	text := withholdGateText[g]
	judged := base
	if written, set := modsNextHop(mods); set {
		judged = written
	}
	nextHop := judged.mp
	fam := base.mpFamily
	if fam == (family.Family{}) {
		nextHop = judged.legacy
		fam = family.IPv4Unicast
	}
	fwdLogger().Warn(text.message,
		"peer", f.addrStr, "advertiser", advertiser, "next-hop", nextHop, "family", fam,
		"rfc", text.rfc,
		"action", "announcement not sent to this peer; it is sent a withdrawal of the routes instead")
}

// egressNextHopWithheld runs, for ONE destination, every egress gate that
// refuses an announcement because of the next hop about to be written, and
// returns the first that refuses, or withholdNone. Both forward rails
// (forwardUpdateCore, reactorForwardRS) call it after every next-hop rewrite is
// recorded in mods, so a policy may not grant what the RFC refuses, and the two
// rails cannot answer differently. A refused destination is sent a withdrawal
// of the routes (mods.SetWithdraw, buildWithdrawalPayload), because it may hold
// the previous generation of the route.
//
// The order only decides which reason is logged: every gate refuses alike.
//
//   - RFC 4271 Section 5.1.3: "A BGP speaker MUST be able to support the
//     disabling advertisement of third party NEXT_HOP attributes in order to
//     handle imperfectly bridged media." Next-hop self with no address of this
//     speaker to write (precomputeNextHop set nhSelfWithheld) would pass the
//     third-party next hop the operator disabled.
//   - RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be
//     advertised to a peer using an address of that peer as NEXT_HOP."
//     (egressNextHopIsPeerOwn). Withheld rather than rewritten, which keeps the
//     transparency RFC 7947 Section 2.2.2 requires of a route server.
//   - draft-ietf-idr-linklocal-capability Section 4: "A Route Reflector (RR)
//     reflecting a route with a link-local-only next hop MUST NOT advertise that
//     route to a client unless the client shares the same link-layer segment as
//     the original advertiser." A client the operator configured a next-hop
//     mode for carries this speaker's address in mods and passes. Asked only
//     when reflected says the destination receives the route by reflection.
//   - draft-ietf-idr-linklocal-capability Section 4, the Link-Local-only next
//     hop towards a peer more than one hop away
//     (egressNextHopLinkLocalOnlyOffLink). It is a different question from the
//     reflection gate, which judges the client against the ADVERTISER's
//     segment, so neither subsumes the other.
//   - RFC 8950 Section 4: "A BGP speaker MUST only advertise the IPv4 or
//     VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has
//     first ascertained via the BGP Capability Advertisement that the BGP peer
//     supports the Extended Next Hop Encoding capability for the relevant
//     AFI/SAFI pair." (egressNextHopLacksExtendedNextHop).
//   - draft-ietf-idr-linklocal-capability Section 2: "When the capability has
//     not been negotiated, the procedures in this document do not apply."
//     (egressNextHopLinkLocalOnlyRefused).
func egressNextHopWithheld(dest *Peer, f *peerForwardFacts, mods *filterapi.ModAccumulator, base nextHopValue, reflected bool, advertiser netip.Addr) withholdGate {
	// RFC 2545 Section 3 requires a global IPv6 next-hop entity. RFC 4659
	// Section 3.2.1.1 permits the canonical absent-global VPN pair only for
	// link-local peering. This is separate from capability 77's single-address
	// form. Judge the field after every policy rewrite and trim.
	emitted := base
	// mpReachNextHopHandler cannot create MP_REACH without a source attribute.
	// applyFactsNextHop records both legacy and MP rewrites for IPv4 modes;
	// its unused MP operation must not impose IPv6 rules on a legacy route.
	if base.mpFamily != (family.Family{}) {
		if written, set := modsNextHop(mods); set && written.mpWireLen != 0 {
			emitted = written
		}
	}
	// RFC 2545 Section 3; RFC 8950 Section 3; RFC 9830 Section 2.1.
	// The effective replacement wins over obsolete input, before normalization.
	profile := attribute.MPNextHopProfile(attribute.AFI(base.mpFamily.AFI), attribute.SAFI(base.mpFamily.SAFI))
	if profile.IPv6Roles && !slices.Contains(profile.Lengths, int(emitted.mpWireLen)) {
		return withholdInvalidWidth
	}
	// An addressless MP_REACH is still an announcement: its family-specific
	// width must be checked before the no-address exit. A later policy repair
	// must also reach the self, role, identity, capability and scope gates.
	// Withdrawals and FlowSpec still have no effective forwarding address;
	// applyNextHopFamily clears the latter's ignored MP rewrites.
	if !emitted.valid() {
		return withholdNone
	}
	if f.nhSelfWithheld {
		return withholdNextHopSelfAbsent
	}
	// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the
	// Network Address of Next Hop field the global IPv6 address of the next
	// hop, potentially followed by the link-local IPv6 address of the next hop."
	// The trimming boundary keeps an invalid pair intact so this gate sees
	// both slots even when either prefix-membership predicate fails.
	// Capability 77 licenses a single address, never a malformed pair.
	if emitted.mpLL.IsValid() && (!emitted.mpLL.Is6() || !emitted.mpLL.IsLinkLocalUnicast()) {
		return withholdInvalidPair
	}
	// RFC 9830 Section 2.1: "The Length field of the next-hop address specifies
	// the next-hop address family."
	// The next-hop length determines its wire family independently of NLRI
	// AFI. Keep that fact after unmapping so a mapped IPv6 wire address cannot
	// pass as native IPv4. A real single link-local is decided by the
	// capability and scope gates below; a paired first address is not exempt.
	// RFC 4798 Section 2 and RFC 4659 Section 3.2.1.2 define single mapped
	// fields for IPv6 labeled and VPN NLRI. They do not license a mapped
	// global in ordinary plain IPv6 or SR Policy, or a mapped/global pair.
	mapped := profile.MappedIPv4 && emitted.mp.Is4() && emitted.mpIPv6 &&
		!emitted.mpLL.IsValid() && slices.Contains(profile.Lengths, int(emitted.mpWireLen))
	if emitted.mp.IsValid() && emitted.mpIPv6 && !emitted.linkLocalOnly() && !mapped {
		if !emitted.mp.Is6() || !emitted.mp.IsGlobalUnicast() {
			if !emitted.mpVPNUnspecifiedPair || !destOnLink(dest) || !vpnIPv6LinkLocalPeering(f, base.mpFamily) {
				return withholdUnusableGlobal
			}
		}
	}
	if egressNextHopIsPeerOwn(f, mods, base) {
		return withholdPeerOwn
	}
	if reflected && egressNextHopIsLinkLocalOnly(mods, base) &&
		!sameLinkLayerSegment(dest.llScope.Load().connectedPrefixes(), advertiser, f.addr) {
		return withholdReflectedLinkLocalOnly
	}
	if egressNextHopLinkLocalOnlyOffLink(dest, mods, base) {
		return withholdLinkLocalOnlyOffLink
	}
	if egressNextHopLacksExtendedNextHop(dest, mods, base) {
		return withholdNoExtendedNextHop
	}
	if egressNextHopLinkLocalOnlyRefused(dest, mods, base) {
		return withholdLinkLocalOnlyRefused
	}
	return withholdNone
}

// payloadMPNextHopField returns the Network Address of Next Hop field of an
// UPDATE payload's MP_REACH_NLRI, or nil when it carries none or the field
// runs past the attribute. It reads the attribute section the way
// payloadNextHop does. Allocation-free.
func payloadMPNextHopField(payload []byte) []byte {
	sections, err := wire.ParseUpdateSections(payload)
	if err != nil {
		return nil
	}
	attrs := sections.Attrs(payload)
	if attrs == nil {
		return nil
	}
	_, _, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	if !found {
		return nil
	}
	// AFI(2) + SAFI(1) + next-hop length(1) + next hop.
	if len(value) < 4 {
		return nil
	}
	end := 4 + int(value[3])
	if end > len(value) {
		return nil
	}
	return value[4:end]
}

// egressNextHopLacksExtendedNextHop answers, for ONE destination, whether the
// MP_REACH_NLRI it is about to be sent carries IPv4 NLRI with an IPv6 next hop
// that the destination never licensed.
//
// RFC 8950 Section 4: "A BGP speaker MUST only advertise the IPv4 or VPN-IPv4
// NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has first
// ascertained via the BGP Capability Advertisement that the BGP peer supports
// the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair."
//
// The address is resolved the way egressNextHopIsPeerOwn resolves it: an
// MP_REACH_NLRI next hop recorded in mods (a configured next-hop mode or a
// filter rewrite) replaces the payload's, so the question is asked about the
// bytes the rebuild will emit. A received IPv6 next hop passed along unchanged
// and an explicit IPv6 next hop are refused alike. The pair is judged by
// Peer.canUseNextHopFor against the destination's own send context, the same
// producer the announce rails ask (Peer.resolveNextHop), so the rails cannot
// disagree about which pair licenses what.
//
// nextHopAddr unmaps an IPv4-mapped address, so such an address is not IPv6 here.
func egressNextHopLacksExtendedNextHop(dest *Peer, mods *filterapi.ModAccumulator, base nextHopValue) bool {
	// RFC 8950 Section 3: only the families it extends are gated.
	if !rfc8950Family(base.mpFamily) {
		return false
	}
	nextHop := base.mp
	if rewritten, set := modsNextHop(mods); set && rewritten.mp.IsValid() {
		nextHop = rewritten.mp
	}
	if !nextHop.Is6() {
		return false
	}
	return !dest.canUseNextHopFor(nextHop, base.mpFamily)
}
