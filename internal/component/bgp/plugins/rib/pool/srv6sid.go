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
func ExtractSRv6SIDFull(prefixSIDValue []byte) SRv6SIDResult {
	off := 0
	for off+3 <= len(prefixSIDValue) {
		tlvType := prefixSIDValue[off]
		tlvLen := int(prefixSIDValue[off+1])<<8 | int(prefixSIDValue[off+2])
		off += 3
		if off+tlvLen > len(prefixSIDValue) {
			return SRv6SIDResult{}
		}
		if tlvType == tlvTypeSRv6L3Service || tlvType == tlvTypeSRv6L2Service {
			if r := extractSIDFromServiceTLV(prefixSIDValue[off : off+tlvLen]); r.SID.IsValid() {
				return r
			}
		}
		off += tlvLen
	}
	return SRv6SIDResult{}
}

// extractSIDFromServiceTLV parses Sub-TLVs within an SRv6 L3/L2 Service TLV
// and returns the first SRv6 SID with its SID Structure parameters.
// RFC 9252 Section 3.1: Service TLV value = Reserved(1) + Sub-TLVs.
// RFC 9252 Section 3.2: SID Info Sub-TLV value = Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1) + Sub-Sub-TLVs.
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
		// RFC 9252 Section 3.2: min length 21 = Reserved(1) + SID(16) + Flags(1) + Behavior(2) + Reserved(1)
		if subType == subTLVTypeSRv6SIDInfo && subLen >= 21 {
			var ip6 [16]byte
			// Value: Reserved(1) + SRv6 SID(16) + ...
			copy(ip6[:], data[off+1:off+17])
			sid := netip.AddrFrom16(ip6)

			result := SRv6SIDResult{SID: sid}

			// Parse Sub-Sub-TLVs if present (after Reserved(1)+SID(16)+Flags(1)+Behavior(2)+Reserved(1) = 21 bytes).
			if subLen > 21 {
				result.HasTranspos, result.TransposOffset, result.TransposLen = parseSIDStructure(data[off+21 : off+subLen])
			}
			return result
		}
		off += subLen
	}
	return SRv6SIDResult{}
}

// parseSIDStructure looks for the SID Structure Sub-Sub-TLV (type 1) and
// extracts transposition parameters. Returns (found, offset, length).
// RFC 9252 Section 3.2.1: Sub-Sub-TLV format = Type(1) + Length(2) + 6 bytes.
func parseSIDStructure(subSubTLVs []byte) (bool, uint8, uint8) {
	off := 0
	for off+3 <= len(subSubTLVs) {
		ssType := subSubTLVs[off]
		ssLen := int(subSubTLVs[off+1])<<8 | int(subSubTLVs[off+2])
		off += 3
		if off+ssLen > len(subSubTLVs) {
			return false, 0, 0
		}
		if ssType == subSubTLVTypeSIDStructure && ssLen >= 6 {
			// Bytes: LBL(1) + LNL(1) + FL(1) + AL(1) + TransposLen(1) + TransposOffset(1)
			lbl := subSubTLVs[off]
			lnl := subSubTLVs[off+1]
			fl := subSubTLVs[off+2]
			al := subSubTLVs[off+3]
			transposLen := subSubTLVs[off+4]
			transposOffset := subSubTLVs[off+5]

			// RFC 9252 Section 3.2.1 validation (errata 7817: >=).
			sum := uint16(lbl) + uint16(lnl) + uint16(fl) + uint16(al)
			if sum > 128 {
				return false, 0, 0
			}
			if transposLen > 0 && sum < uint16(transposOffset)+uint16(transposLen) {
				return false, 0, 0
			}
			if transposLen == 0 {
				return false, 0, 0
			}
			return true, transposOffset, transposLen
		}
		off += ssLen
	}
	return false, 0, 0
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
