// Design: docs/architecture/fib/fib-depth-4-srv6.md -- SRv6 SID extraction from PrefixSID attribute
// RFC: rfc/short/rfc9252.md -- SRv6 SID Information Sub-TLV and transposition scheme

package pool

import (
	"net/netip"
)

// SRv6 Prefix-SID TLV types (RFC 9252 Section 3.1).
const (
	tlvTypeSRv6L3Service = 5
	tlvTypeSRv6L2Service = 6
)

// SRv6 Sub-TLV types within L3/L2 Service TLV.
const (
	subTLVTypeSRv6SIDInfo = 1
)

// SRv6 Sub-Sub-TLV types within SID Information Sub-TLV.
const (
	subSubTLVTypeSIDStructure = 1
)

// SRv6SIDResult carries the extracted SRv6 SID along with transposition
// parameters from the SID Structure Sub-Sub-TLV (RFC 9252 Section 3.2.1).
type SRv6SIDResult struct {
	SID            netip.Addr
	TransposOffset uint8
	TransposLen    uint8
	HasTranspos    bool
}

// ExtractSRv6SID parses the BGP Prefix-SID attribute (code 40) value bytes
// and extracts the first SRv6 SID (IPv6 address) from an L3-service or
// L2-service TLV. Returns an invalid Addr if no SRv6 SID is found or the
// attribute is SR-MPLS only (label index TLV type 1).
func ExtractSRv6SID(prefixSIDValue []byte) netip.Addr {
	r := ExtractSRv6SIDFull(prefixSIDValue)
	return r.SID
}

// ExtractSRv6SIDFull parses the BGP Prefix-SID attribute and returns the
// SRv6 SID along with transposition parameters. Use this when the caller
// needs to apply transposition (VPN/EVPN SAFIs where label carries SID bits).
//
// Only the first SRv6 L3 Service TLV and the first SRv6 L2 Service TLV are
// read. A later instance of a type already met is skipped whatever it holds,
// so a first instance with no valid SID leaves that type with no SID rather
// than handing the question to the second. When the first L3 instance holds
// no valid SID, the first L2 instance is still consulted: the sentences below
// each bind one type.
//
// The SID Structure Sub-Sub-TLV is judged here, family-free: a SID whose
// structure fails a Section 3.2.1 condition is answered as no SID. The
// family-bound checks (label width, Transposition Scheme applicability) are
// the caller's, because the attribute does not carry the route family.
func ExtractSRv6SIDFull(prefixSIDValue []byte) SRv6SIDResult {
	var seenL3, seenL2 bool
	off := 0
	for off+3 <= len(prefixSIDValue) {
		tlvType := prefixSIDValue[off]
		tlvLen := int(prefixSIDValue[off+1])<<8 | int(prefixSIDValue[off+2])
		off += 3
		if off+tlvLen > len(prefixSIDValue) {
			return SRv6SIDResult{}
		}
		value := prefixSIDValue[off : off+tlvLen]
		off += tlvLen
		switch tlvType {
		case tlvTypeSRv6L3Service:
			// RFC 9252 Section 7: "If multiple instances of the SRv6 L3 Service
			// TLV are encountered, all but the first instance MUST be ignored."
			if seenL3 {
				continue
			}
			seenL3 = true
		case tlvTypeSRv6L2Service:
			// RFC 9252 Section 7: "If multiple instances of the SRv6 L2 Service
			// TLV are encountered, all but the first instance MUST be ignored."
			if seenL2 {
				continue
			}
			seenL2 = true
		default:
			continue
		}
		if r := extractSIDFromServiceTLV(value); r.SID.IsValid() {
			return r
		}
	}
	return SRv6SIDResult{}
}

// extractSIDFromServiceTLV parses Sub-TLVs within an SRv6 L3/L2 Service TLV
// and returns the first SRv6 SID with its SID Structure parameters.
// RFC 9252 Section 2: Service TLV value = Reserved(1) + Sub-TLVs.
// RFC 9252 Section 3.1: SID Info Sub-TLV value = Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + Sub-Sub-TLVs.
//
// A SID whose SID Structure Sub-Sub-TLV fails Section 3.2.1 is answered as no
// SID (sidStructureValid).
func extractSIDFromServiceTLV(data []byte) SRv6SIDResult {
	if len(data) < 1 {
		return SRv6SIDResult{}
	}
	// Skip 1-byte Reserved at start of Service TLV value.
	off := 1
	for off+3 <= len(data) {
		subType := data[off]
		subLen := int(data[off+1])<<8 | int(data[off+2])
		off += 3
		if off+subLen > len(data) {
			return SRv6SIDResult{}
		}
		// RFC 9252 Section 3.1: min length 21 = Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1)
		if subType == subTLVTypeSRv6SIDInfo && subLen >= 21 {
			var ip6 [16]byte
			// Value: Reserved(1) + SRv6 SID(16) + ...
			copy(ip6[:], data[off+1:off+17])

			result := SRv6SIDResult{SID: netip.AddrFrom16(ip6)}
			// Sub-Sub-TLVs follow the 21 fixed octets.
			if subLen > 21 {
				valid, offset, length := parseSIDStructure(data[off+21 : off+subLen])
				// RFC 9252 Section 7: "The SRv6 SID value in the SRv6 SID
				// Information Sub-TLV is invalid when ... any of the conditions
				// for the fields of the Sub-Sub-TLV, as specified in Section
				// 3.2.1, is not met."
				if !valid {
					return SRv6SIDResult{}
				}
				result.TransposOffset, result.TransposLen, result.HasTranspos = offset, length, length > 0
			}
			return result
		}
		off += subLen
	}
	return SRv6SIDResult{}
}

// sidStructureLength is the value length of the SID Structure Sub-Sub-TLV.
//
// RFC 9252 Section 3.2.1: "This field contains a total length of 6 octets."
// A structure of any other length makes the SID invalid.
const sidStructureLength = 6

// parseSIDStructure finds the SID Structure Sub-Sub-TLV (type 1) among the
// Sub-Sub-TLVs of one SID Information Sub-TLV and judges it against Section
// 3.2.1. It answers valid=false when the structure makes the SID invalid,
// and valid=true with the transposition (zero length when nothing is
// transposed) otherwise. No SID Structure at all is valid with no
// transposition. A Sub-Sub-TLV overrunning its Sub-TLV is malformed, which
// the RFC 7606 walk turns into treat-as-withdraw before the RIB stores the
// route; it answers invalid here so this reader never trusts what follows.
//
// RFC 9252 Section 3.2.1 SID Structure Sub-Sub-TLV value:
//
//	0        1        2        3        4        5
//	+--------+--------+--------+--------+--------+--------+
//	|  LBL   |  LNL   |   FL   |   AL   | T.Len  | T.Off  |
//	+--------+--------+--------+--------+--------+--------+
func parseSIDStructure(subSubTLVs []byte) (valid bool, transposOffset, transposLen uint8) {
	off := 0
	for off+3 <= len(subSubTLVs) {
		ssType := subSubTLVs[off]
		ssLen := int(subSubTLVs[off+1])<<8 | int(subSubTLVs[off+2])
		off += 3
		if off+ssLen > len(subSubTLVs) {
			return false, 0, 0
		}
		if ssType != subSubTLVTypeSIDStructure {
			off += ssLen
			continue
		}
		// A length other than 6 fails the Section 3.2.1 field definition: the
		// SID is invalid (Section 7 lists only an inconsistent length as
		// malformed, so it is not a malformation).
		if ssLen != sidStructureLength {
			return false, 0, 0
		}
		v := subSubTLVs[off : off+sidStructureLength]
		lbl, lnl, fl, al, length, offset := v[0], v[1], v[2], v[3], v[4], v[5]

		// RFC 9252 Section 3.2.1: "A Transposition Length of 0 indicates nothing
		// is transposed and that the entire SRv6 SID value is encoded in the SID
		// Information Sub-TLV. In this case, the Transposition Offset MUST be
		// set to 0."
		if length == 0 && offset != 0 {
			return false, 0, 0
		}
		// RFC 9252 Section 3.2.1: "the sum of the Locator Block Length (LBL),
		// Locator Node Length (LNL), Function Length (FL), and Argument Length
		// (AL) fields MUST be less than or equal to 128 and greater than the
		// sum of Transposition Offset and Transposition Length." Erratum 7817
		// corrects "greater than" to "greater than or equal to".
		sum := uint16(lbl) + uint16(lnl) + uint16(fl) + uint16(al)
		if sum > 128 || sum < uint16(offset)+uint16(length) {
			return false, 0, 0
		}
		// RFC 9252 Section 3.2.1: "A receiver is unable to validate the
		// applicability of arguments for SRv6 Endpoint Behaviors that are
		// unknown to it and hence MUST ignore SRv6 SIDs with arguments
		// (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors."
		// Ze knows no Endpoint Behavior, so every behavior is unknown to it.
		if al != 0 {
			return false, 0, 0
		}
		return true, offset, length
	}
	return true, 0, 0
}

// ApplyTransposition reconstructs the full SRv6 SID by merging transposed bits
// from the NLRI label field into the partial SID at the specified bit offset.
// RFC 9252 Section 3.2.1 + errata 7652: the label carries transposLen bits of
// the SID starting at transposOffset. The bits occupy the high-order positions
// of the label field (labelWidth: 20 for VPN, 24 for EVPN).
func ApplyTransposition(sid netip.Addr, label uint32, transposOffset, transposLen, labelWidth uint8) netip.Addr {
	if transposLen == 0 || !sid.Is6() {
		return sid
	}
	ip6 := sid.As16()
	for i := range int(transposLen) {
		bitPos := int(transposOffset) + i
		if bitPos >= 128 {
			break
		}
		byteIdx := bitPos / 8
		bitIdx := uint(7 - bitPos%8)
		// Extract bit from label: MSB-first from high-order position of labelWidth field.
		labelBit := (label >> (uint(labelWidth) - 1 - uint(i))) & 1
		if labelBit != 0 {
			ip6[byteIdx] |= 1 << bitIdx
		}
	}
	return netip.AddrFrom16(ip6)
}
