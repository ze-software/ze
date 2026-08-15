// Design: docs/architecture/core-design.md -- egress route decisions on the forward rails
// RFC: rfc/short/rfc4271.md -- a route is not advertised to a peer using that peer's own address as NEXT_HOP (Section 5.1.3)
// Related: forward_med.go -- the Section 5.1.4 sibling, which shares this once-per-UPDATE / once-per-destination shape
// Related: peer_forward_facts.go -- applyFactsNextHop, which records ze's own next-hop rewrite
// Related: reactor_api_forward.go -- forwardUpdateCore, the general forward rail
// Related: forward_rs.go -- reactorForwardRS, the route-server forward rail
package reactor

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/wire"
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
			nhLen := int(value[3])
			if 4+nhLen <= len(value) {
				out.mp, out.mpLL = nextHopAddr(value[4 : 4+nhLen])
			}
		}
	}
	return out
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
// both: applyFactsNextHop records the legacy address and the IPv4-mapped
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
