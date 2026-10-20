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
	Origin          attribute.Origin
	ASPath          []uint32
	MED             uint32
	HasMED          bool // MED is emitted only when set; 0 is a legal metric.
	LocalPreference uint32
	// HasLocalPreference says LocalPreference is configured; without it an
	// iBGP UPDATE carries the default 100. 0 is a legal preference.
	HasLocalPreference bool
	Communities        []uint32
	LargeCommunities   [][3]uint32
	ExtCommunityBytes  []byte // Pre-packed (RT, etc.)

	// ORIGINATOR_ID (RFC 4456) - 0 means not set.
	OriginatorID uint32

	// CLUSTER_LIST (RFC 4456).
	ClusterList []uint32

	// RedundancyMode is the multihoming mode the operator asked an Ethernet
	// A-D per ES route to signal. Unspecified leaves the ESI Label's own
	// Single-Active flag as the statement of the mode.
	RedundancyMode EVPNRedundancyMode
}

// EVPNRedundancyMode is the multihoming redundancy mode an originated
// Ethernet A-D per ES route signals in its ESI Label extended community.
type EVPNRedundancyMode uint8

const (
	// EVPNRedundancyUnspecified means the operator named no mode, so the
	// Single-Active flag of the configured ESI Label states it.
	EVPNRedundancyUnspecified EVPNRedundancyMode = iota
	// EVPNRedundancyAllActive is RFC 7432 All-Active redundancy mode.
	EVPNRedundancyAllActive
	// EVPNRedundancySingleActive is RFC 7432 Single-Active redundancy mode.
	EVPNRedundancySingleActive
)

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
	if err := ValidateEVPNOriginationMode(p.NLRI, p.ExtCommunityBytes, false, p.RedundancyMode); err != nil {
		return nil, err
	}
	ub.resetScratch()

	var attrs []attribute.Attribute

	// 1. ORIGIN
	attrs = append(attrs, p.Origin)

	// 2. AS_PATH
	attrs = ub.appendASPath(attrs, p.ASPath)

	// No NEXT_HOP.
	// RFC 4760 Section 3: "An UPDATE message that carries no NLRI, other than the
	// one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the NEXT_HOP
	// attribute." The next hop rides in MP_REACH_NLRI alone.

	// 4. MED
	attrs = appendMED(attrs, p.MED, p.HasMED)

	// 5. LOCAL_PREF
	attrs = ub.appendLocalPref(attrs, p.LocalPreference, p.HasLocalPreference)

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

	// Order attributes as attribute.OrderAttributes does: the MP attribute
	// first (RFC 7606 Section 5.1), then every other attribute by code.
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

// ValidateEVPNOrigination checks requirements on locally originated EVPN routes
// whose producer carries no redundancy mode: the ESI Label's Single-Active
// flag then states the mode. It does not apply sender constraints to received
// routes or withdrawals.
func ValidateEVPNOrigination(nlris, extCommunities []byte, addPath bool) error {
	return ValidateEVPNOriginationMode(nlris, extCommunities, addPath, EVPNRedundancyUnspecified)
}

// ValidateEVPNOriginationMode checks requirements on locally originated EVPN
// routes, and checks every Ethernet A-D per ES route's ESI Label against the
// redundancy mode the operator asked for.
func ValidateEVPNOriginationMode(nlris, extCommunities []byte, addPath bool, mode EVPNRedundancyMode) error {
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
				// RFC 7432 Section 8.2.1
				if err := checkESILabelRedundancy(extCommunities, mode); err != nil {
					return err
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

// esiLabelSingleActive is the Single-Active bit of the ESI Label Flags octet,
// at bit 40 of the community read as one big-endian 64-bit value.
const esiLabelSingleActive = uint64(1) << 40

// esiLabelReservedMax is the highest reserved MPLS label value.
//
// RFC 3032 Section 6: "Label values 0-15 inclusive have special meaning, as
// specified in this document, or as further assigned by IANA." A valid ESI
// label is therefore above this value.
const esiLabelReservedMax = 15

// checkESILabelRedundancy checks each ESI Label extended community of an
// Ethernet A-D per ES route against the redundancy mode it signals.
//
// RFC 7432 Section 7.5, ESI Label extended community (8 octets):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Type=0x06     | Sub-Type=0x01 | Flags(1 octet)|  Reserved=0   |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|  Reserved=0   |          ESI Label                            |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
//	octet 0 type, 1 sub-type, 2 flags (bit 0 Single-Active), 3-4 reserved,
//	5-7 ESI Label: the label value is the high-order 20 bits, as RFC 7432
//	Section 7.2 encodes its three-octet MPLS label fields.
//
// RFC 7432 Section 7.5: "The low-order bit of the Flags octet is defined as
// the "Single-Active" bit.  A value of 0 means that the multihomed site is
// operating in All-Active redundancy mode, and a value of 1 means that the
// multihomed site is operating in Single-Active redundancy mode."
//
// RFC 7432 Section 8.2.1: "If All-Active redundancy mode is desired, then the
// "Single-Active" bit in the flags of the ESI Label extended community MUST be
// set to 0 and the MPLS label in that Extended Community MUST be set to a valid
// MPLS label value."
//
// RFC 7432 Section 8.2.1: "If Single-Active redundancy mode is desired, then
// the "Single-Active" bit in the flags of the ESI Label extended community MUST
// be set to 1 and the ESI label SHOULD be set to a valid MPLS label value."
// The SHOULD is not enforced: a Single-Active ESI Label keeps any label.
func checkESILabelRedundancy(extCommunities []byte, mode EVPNRedundancyMode) error {
	for off := 0; off+8 <= len(extCommunities); off += 8 {
		ec := extCommunities[off : off+8]
		if ec[0] != 6 {
			continue
		}
		if ec[1] != 1 {
			continue
		}
		value := binary.BigEndian.Uint64(ec)
		singleActive := value&esiLabelSingleActive != 0
		switch mode {
		case EVPNRedundancyUnspecified:
			// The flag states the mode the operator wants.
		case EVPNRedundancyAllActive:
			if singleActive {
				return fmt.Errorf("%w: redundancy-mode all-active needs the ESI Label Single-Active flag at 0, "+
					"but extended community 0x%016x sets it to 1 (RFC 7432 Section 8.2.1); "+
					"use extended community 0x%016x, or set redundancy-mode single-active",
					ErrEVPNOrigination, value, value&^esiLabelSingleActive)
			}
		case EVPNRedundancySingleActive:
			if !singleActive {
				return fmt.Errorf("%w: redundancy-mode single-active needs the ESI Label Single-Active flag at 1, "+
					"but extended community 0x%016x sets it to 0 (RFC 7432 Section 8.2.1); "+
					"use extended community 0x%016x, or set redundancy-mode all-active",
					ErrEVPNOrigination, value, value|esiLabelSingleActive)
			}
		default:
			return fmt.Errorf("%w: unknown redundancy mode %d; use all-active or single-active",
				ErrEVPNOrigination, mode)
		}
		if singleActive {
			continue
		}
		label := uint32(ec[5])<<12 | uint32(ec[6])<<4 | uint32(ec[7])>>4
		if label <= esiLabelReservedMax {
			return fmt.Errorf("%w: All-Active mode needs a valid MPLS label in the ESI Label, "+
				"but extended community 0x%016x carries label %d, a reserved value from 0 to 15 (RFC 7432 Section 8.2.1); "+
				"put a label from 16 to 1048575 in the high-order 20 bits of its last three octets, "+
				"for example extended community 0x%016x for label 16",
				ErrEVPNOrigination, value, label, value&^0xffffff|16<<4)
		}
	}
	return nil
}
