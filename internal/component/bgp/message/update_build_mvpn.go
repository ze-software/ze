// Design: docs/architecture/update-building.md — MVPN UPDATE builders
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"net/netip"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
)

// MVPNParams contains parameters for building an MVPN route UPDATE.
//
// RFC 6514 - BGP Encodings and Procedures for Multicast VPNs.
type MVPNParams struct {
	// RouteType is the MVPN route type (5=Source-AD, 6=Shared-Join, 7=Source-Join).
	RouteType uint8

	// IsIPv6 indicates IPv6 MVPN (AFI=2) vs IPv4 MVPN (AFI=1).
	IsIPv6 bool

	// RD is the Route Distinguisher.
	RD [8]byte

	// SourceAS is the source AS for join routes (types 6, 7).
	SourceAS uint32

	// Source is the source IP or RP address.
	Source netip.Addr

	// Group is the multicast group address.
	Group netip.Addr

	// NextHop is the next-hop address.
	NextHop netip.Addr

	// Origin is the origin type.
	Origin attribute.Origin

	// LocalPreference for iBGP.
	LocalPreference uint32

	// MED.
	MED uint32

	// ExtCommunityBytes for route targets.
	ExtCommunityBytes []byte

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456) - cluster IDs traversed.
	ClusterList []uint32
}

// BuildMVPN builds an UPDATE message for MVPN routes (SAFI 5).
//
// RFC 6514 - MVPN routes use MP_REACH_NLRI with SAFI=5.
// Multiple routes can be included in a single UPDATE.
//
// LIMITATION: Shared attributes (Origin, LocalPreference, MED, ExtCommunityBytes,
// OriginatorID, ClusterList) are taken from routes[0] only. If routes in the
// slice have differing values for these attributes, only the first route's
// values are used.
func (ub *UpdateBuilder) BuildMVPN(routes []MVPNParams) *Update {
	ub.resetScratch()

	if len(routes) == 0 {
		return &Update{}
	}

	first := routes[0]
	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, first.Origin)

	// 2. AS_PATH
	// RFC 6793: AS_PATH encoding depends on ASN4 capability negotiation.
	asPath := ub.buildASPath(nil)
	asn4 := ub.ASN4
	asPathBuf := ub.alloc(asPath.LenWithASN4(asn4))
	asPath.WriteToWithASN4(asPathBuf, 0, asn4)
	attrs = append(attrs, &rawAttribute{
		flags: asPath.Flags(),
		code:  asPath.Code(),
		data:  asPathBuf,
	})

	// 3. NEXT_HOP (for IPv4 test compatibility)
	if first.NextHop.Is4() {
		attrs = append(attrs, &attribute.NextHop{Addr: first.NextHop})
	}

	// 4. MED
	if first.MED > 0 {
		attrs = append(attrs, attribute.MED(first.MED))
	}

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		lp := first.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrs = append(attrs, attribute.LocalPref(lp))
	}

	// 9. ORIGINATOR_ID (RFC 4456)
	if first.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(first.OriginatorID >> 24), byte(first.OriginatorID >> 16),
			byte(first.OriginatorID >> 8), byte(first.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	// 10. CLUSTER_LIST (RFC 4456)
	if len(first.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(first.ClusterList))
		copy(cl, first.ClusterList)
		attrs = append(attrs, cl)
	}

	// 14. MP_REACH_NLRI for MVPN
	mpReach := ub.buildMPReachMVPN(routes)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
	if len(first.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  first.ExtCommunityBytes,
		})
	}

	// Sort by type code
	sort.Slice(attrs, func(i, j int) bool {
		return attrs[i].Code() < attrs[j].Code()
	})

	attrBytes := ub.packAttributesOrderedInto(attrs, nil)

	return &Update{
		PathAttributes: attrBytes,
	}
}

// buildMPReachMVPN constructs MP_REACH_NLRI for MVPN routes.
//
// RFC 6514 Section 4 - MVPN NLRI format.
// Sizes the NLRI block via mvpnNLRISize, allocates one scratch-backed value
// buffer, and writes each NLRI directly into it via writeMVPNNLRI.
func (ub *UpdateBuilder) buildMPReachMVPN(routes []MVPNParams) *rawAttribute {
	if len(routes) == 0 {
		return nil
	}

	first := routes[0]

	totalNLRISize := 0
	for i := range routes {
		totalNLRISize += mvpnNLRISize(routes[i])
	}

	// AFI/SAFI
	var afi uint16 = 1 // IPv4
	if first.IsIPv6 {
		afi = 2 // IPv6
	}
	const safiMVPN uint8 = 5

	// Next-hop
	nhBytes := first.NextHop.AsSlice()
	nhLen := len(nhBytes)

	// MP_REACH_NLRI value: AFI(2) + SAFI(1) + NH_Len(1) + NH + Reserved(1) + NLRIs
	valueLen := 2 + 1 + 1 + nhLen + 1 + totalNLRISize
	value := ub.alloc(valueLen)
	value[0] = byte(afi >> 8)
	value[1] = byte(afi)
	value[2] = safiMVPN
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved

	off := 5 + nhLen
	for i := range routes {
		off += writeMVPNNLRI(value, off, routes[i])
	}

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// mvpnNLRISize returns the wire-format length of a single MVPN NLRI.
//
// RFC 6514 Section 4: Route Type (1) + Length (1) + Route-Type-Specific Data.
// Unknown route types produce only the 2-byte header (zero data), matching
// writeMVPNNLRI's behavior.
func mvpnNLRISize(route MVPNParams) int {
	dataLen := 0
	switch route.RouteType {
	case 5:
		// RD (8) + Source (1 + addr) + Group (1 + addr)
		dataLen = 8
		if route.Source.Is4() {
			dataLen += 1 + 4
		} else {
			dataLen += 1 + 16
		}
		if route.Group.Is4() {
			dataLen += 1 + 4
		} else {
			dataLen += 1 + 16
		}
	case 6, 7:
		// RD (8) + Source-AS (4) + Source (1 + addr) + Group (1 + addr)
		dataLen = 8 + 4
		if route.Source.Is4() {
			dataLen += 1 + 4
		} else {
			dataLen += 1 + 16
		}
		if route.Group.Is4() {
			dataLen += 1 + 4
		} else {
			dataLen += 1 + 16
		}
	}
	return 2 + dataLen
}

// writeMVPNNLRI writes one MVPN NLRI into buf at off and returns bytes written.
//
// RFC 6514 Section 4 - NLRI format: Route Type (1) + Length (1) + Data.
// Source/Group prefix length is 32 for IPv4, 128 for IPv6.
// Caller MUST size buf via mvpnNLRISize(route); no capacity checks here.
func writeMVPNNLRI(buf []byte, off int, route MVPNParams) int {
	start := off
	typeOff := off
	lenOff := off + 1
	off += 2
	dataStart := off

	switch route.RouteType {
	case 5: // Source Active A-D: RD + Source + Group
		copy(buf[off:], route.RD[:])
		off += 8
		off += writeMVPNAddr(buf, off, route.Source)
		off += writeMVPNAddr(buf, off, route.Group)

	case 6, 7: // Shared / Source Tree Join: RD + Source-AS + Source + Group
		copy(buf[off:], route.RD[:])
		off += 8
		buf[off] = byte(route.SourceAS >> 24)
		buf[off+1] = byte(route.SourceAS >> 16)
		buf[off+2] = byte(route.SourceAS >> 8)
		buf[off+3] = byte(route.SourceAS)
		off += 4
		off += writeMVPNAddr(buf, off, route.Source)
		off += writeMVPNAddr(buf, off, route.Group)
	}

	dataLen := off - dataStart
	if dataLen > 0xFF {
		// RFC 6514 NLRI Length field is 1 byte. No current route type
		// (5/6/7) produces > 255 bytes of data, but guard against future
		// additions that might silently truncate here.
		panic("BUG: MVPN NLRI data exceeds 255 bytes (RFC 6514 Length field)")
	}
	buf[typeOff] = route.RouteType
	buf[lenOff] = byte(dataLen)
	return off - start
}

// writeMVPNAddr writes a prefix-length + address pair (RFC 6514 Section 4).
func writeMVPNAddr(buf []byte, off int, addr netip.Addr) int {
	if addr.Is4() {
		buf[off] = 32
		a := addr.As4()
		copy(buf[off+1:], a[:])
		return 5
	}
	buf[off] = 128
	a := addr.As16()
	copy(buf[off+1:], a[:])
	return 17
}
