// Design: docs/architecture/update-building.md — EVPN UPDATE builders
// RFC: rfc/short/rfc7432.md — EVPN NLRI route types
// Overview: update_build.go — core UpdateBuilder struct and unicast builders
// Related: update_build_grouped.go — grouped and size-aware UPDATE builders
package message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// ErrEVPNOrigination identifies a refused local EVPN advertisement.
var ErrEVPNOrigination = errors.New("invalid EVPN origination")

// EVPNParams contains parameters for building EVPN route UPDATEs.
//
// RFC 7432 - BGP MPLS-Based Ethernet VPN.
// NLRI bytes must be pre-built by the caller (same pattern as FlowSpecParams/MUPParams).
type EVPNParams struct {
	// NLRI is the pre-built EVPN NLRI bytes.
	// Caller constructs these using evpn.NewEVPNType1-5() then Bytes().
	NLRI []byte

	// NextHop is the next-hop address (PE address).
	NextHop netip.Addr

	// Path attributes
	Origin            attribute.Origin
	ASPath            []uint32
	MED               uint32
	LocalPreference   uint32
	Communities       []uint32
	LargeCommunities  [][3]uint32
	ExtCommunityBytes []byte // Pre-packed (RT, etc.)

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456).
	ClusterList []uint32
}

// BuildEVPN builds an UPDATE message for EVPN routes (AFI=25, SAFI=70).
//
// RFC 7432 - BGP MPLS-Based Ethernet VPN.
func (ub *UpdateBuilder) BuildEVPN(p EVPNParams) (*Update, error) {
	if len(p.NLRI) == 0 {
		return nil, fmt.Errorf("%w: missing NLRI", ErrEVPNOrigination)
	}
	if !p.NextHop.IsValid() {
		return nil, fmt.Errorf("%w: missing next hop", ErrEVPNOrigination)
	}
	if p.NextHop.IsUnspecified() {
		return nil, fmt.Errorf("%w: unspecified next hop", ErrEVPNOrigination)
	}
	if p.NextHop.IsMulticast() {
		return nil, fmt.Errorf("%w: multicast next hop", ErrEVPNOrigination)
	}
	if err := ValidateEVPNOrigination(p.NLRI, p.ExtCommunityBytes, false); err != nil {
		return nil, err
	}
	ub.resetScratch()

	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	attrs = ub.appendASPath(attrs, p.ASPath)

	// 3. NEXT_HOP - for IPv4 next-hop compatibility
	if p.NextHop.Is4() {
		attrs = append(attrs, &attribute.NextHop{Addr: p.NextHop})
	}

	// 4. MED
	if p.MED > 0 {
		attrs = append(attrs, attribute.MED(p.MED))
	}

	// 5. LOCAL_PREF
	if ub.IsIBGP {
		lp := p.LocalPreference
		if lp == 0 {
			lp = 100
		}
		attrs = append(attrs, attribute.LocalPref(lp))
	}

	// 8. COMMUNITIES
	if len(p.Communities) > 0 {
		sorted := make([]uint32, len(p.Communities))
		copy(sorted, p.Communities)
		slices.Sort(sorted)

		comms := make(attribute.Communities, len(sorted))
		for i, c := range sorted {
			comms[i] = attribute.Community(c)
		}
		attrs = append(attrs, comms)
	}

	// 9. ORIGINATOR_ID
	if p.OriginatorID != 0 {
		origIP := netip.AddrFrom4([4]byte{
			byte(p.OriginatorID >> 24), byte(p.OriginatorID >> 16),
			byte(p.OriginatorID >> 8), byte(p.OriginatorID),
		})
		attrs = append(attrs, attribute.OriginatorID(origIP))
	}

	// 10. CLUSTER_LIST
	if len(p.ClusterList) > 0 {
		cl := make(attribute.ClusterList, len(p.ClusterList))
		copy(cl, p.ClusterList)
		attrs = append(attrs, cl)
	}

	// 14. MP_REACH_NLRI for EVPN
	mpReach := ub.buildMPReachEVPN(p)
	attrs = append(attrs, mpReach)

	// 16. EXTENDED_COMMUNITIES
	if len(p.ExtCommunityBytes) > 0 {
		attrs = append(attrs, &rawAttribute{
			flags: attribute.FlagOptional | attribute.FlagTransitive,
			code:  attribute.AttrExtCommunity,
			data:  p.ExtCommunityBytes,
		})
	}

	// 32. LARGE_COMMUNITIES
	if len(p.LargeCommunities) > 0 {
		lcs := make(attribute.LargeCommunities, len(p.LargeCommunities))
		for i, lc := range p.LargeCommunities {
			lcs[i] = attribute.LargeCommunity{
				GlobalAdmin: lc[0],
				LocalData1:  lc[1],
				LocalData2:  lc[2],
			}
		}
		attrs = append(attrs, lcs)
	}

	// Order attributes: MP_UNREACH first, regular attrs by code, MP_REACH last.
	// Matches the wire-byte order used by ExaBGP fixture round-trip tests.
	attrBytes := ub.packAttributesOrderedInto(attrs, nil)

	return &Update{
		PathAttributes: attrBytes,
	}, nil
}

// buildMPReachEVPN builds MP_REACH_NLRI for EVPN routes.
// NLRI bytes must be pre-built by the caller in p.NLRI.
func (ub *UpdateBuilder) buildMPReachEVPN(p EVPNParams) *rawAttribute {

	// BuildEVPN checked that the caller supplied a usable next hop.
	nhBytes := p.NextHop.AsSlice()
	nhLen := len(nhBytes)

	// MP_REACH_NLRI format:
	// AFI (2) + SAFI (1) + NH Len (1) + NH + Reserved (1) + NLRI
	value := ub.alloc(2 + 1 + 1 + nhLen + 1 + len(p.NLRI))
	value[0] = 0x00
	value[1] = byte(family.AFIL2VPN) // AFI 25
	value[2] = byte(family.SAFIEVPN) // SAFI 70
	value[3] = byte(nhLen)
	copy(value[4:4+nhLen], nhBytes)
	value[4+nhLen] = 0 // reserved
	copy(value[5+nhLen:], p.NLRI)

	return &rawAttribute{
		flags: attribute.FlagOptional,
		code:  attribute.AttrMPReachNLRI,
		data:  value,
	}
}

// ValidateEVPNOrigination checks requirements on locally originated EVPN routes.
// It does not apply sender constraints to received routes or withdrawals.
func ValidateEVPNOrigination(nlris, extCommunities []byte, addPath bool) error {
	if len(extCommunities)%8 != 0 {
		return fmt.Errorf("%w: incomplete extended community", ErrEVPNOrigination)
	}
	hasRT, hasESILabel, hasESImport := false, false, false
	for off := 0; off+8 <= len(extCommunities); off += 8 {
		ec := extCommunities[off : off+8]
		switch ec[1] {
		case 1:
			if ec[0] == 6 {
				hasESILabel = true
			}
		case 2:
			switch ec[0] {
			case 0, 1, 2:
				hasRT = true
			case 6:
				hasESImport = true
			}
		}
	}
	for len(nlris) != 0 {
		if addPath {
			if len(nlris) < 4 {
				return fmt.Errorf("%w: invalid path identifier length", ErrEVPNOrigination)
			}
			nlris = nlris[4:]
		}
		if len(nlris) < 2 {
			return fmt.Errorf("%w: invalid NLRI length", ErrEVPNOrigination)
		}
		if int(nlris[1])+2 > len(nlris) {
			return fmt.Errorf("%w: invalid NLRI length", ErrEVPNOrigination)
		}
		size := int(nlris[1]) + 2
		switch nlris[0] {
		case 1:
			if size != 27 {
				return fmt.Errorf("%w: Ethernet A-D requires one three-octet label field", ErrEVPNOrigination)
			}
			// RFC 7432 Section 8.2.1: MAX-ET identifies an A-D per ES route,
			// whose Route Distinguisher must be the IPv4-address type.
			if binary.BigEndian.Uint32(nlris[20:24]) == 0xffffffff {
				if binary.BigEndian.Uint16(nlris[2:4]) != 1 {
					return fmt.Errorf("%w: Ethernet A-D per ES requires a type 1 route distinguisher", ErrEVPNOrigination)
				}
				if nlris[24]|nlris[25]|nlris[26] != 0 {
					return fmt.Errorf("%w: Ethernet A-D per ES requires a zero NLRI label", ErrEVPNOrigination)
				}
				if !hasESILabel {
					return fmt.Errorf("%w: Ethernet A-D per ES requires an ESI Label extended community", ErrEVPNOrigination)
				}
				if !hasRT {
					return fmt.Errorf("%w: Ethernet A-D per ES requires a route target", ErrEVPNOrigination)
				}
			}
		case 3:
			// RFC 7432 Section 11.1: IMET advertisements carry one or more RTs.
			if !hasRT {
				return fmt.Errorf("%w: inclusive multicast Ethernet Tag route requires a route target", ErrEVPNOrigination)
			}
		case 4:
			if size != 25 && size != 37 {
				return fmt.Errorf("%w: invalid Ethernet Segment route length", ErrEVPNOrigination)
			}
			if int(nlris[20]) != (size-21)*8 {
				return fmt.Errorf("%w: invalid Ethernet Segment originator address length", ErrEVPNOrigination)
			}
			// RFC 7432 Section 8.1.1: the Ethernet Segment RD MUST be Type 1,
			// and its advertisement MUST carry the Section 7.6 ES-Import RT.
			if binary.BigEndian.Uint16(nlris[2:4]) != 1 {
				return fmt.Errorf("%w: Ethernet Segment requires a type 1 route distinguisher", ErrEVPNOrigination)
			}
			if !hasESImport {
				return fmt.Errorf("%w: Ethernet Segment requires an ES-Import route target", ErrEVPNOrigination)
			}
		}
		nlris = nlris[size:]
	}
	return nil
}
