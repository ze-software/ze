// Design: docs/architecture/wire/ospfv3.md -- OSPF Segment Routing IPv6 wire codec
// (RFC 8666). Encodes/decodes the OSPFv3 Extended-LSA SR sub-TLV VALUE bytes. The
// flag semantics match RFC 8665, but the type codes are the OSPFv3 Extended-LSA
// registry values (Prefix-SID 4, Adj-SID 5, LAN-Adj-SID 6, SID/Label 7,
// Ext-Prefix-Range 9) and the field layout drops the OSPFv2 MT-ID octet.
// RFC: rfc/short/rfc8666.md (§3.1 SID/Label, §5 Ext-Prefix-Range, §6 Prefix-SID, §7 Adj-SID)

package sr

import (
	"encoding/binary"
)

// OSPFv2 (RFC 8665) TLV/sub-TLV type codes, kept explicit so the IPv4 and IPv6
// code sets never cross (RFC 8666 §9 warns the numbers differ).
const (
	V4TypeSIDLabel       uint16 = 1
	V4TypeSRAlgorithm    uint16 = 8
	V4TypeSRGB           uint16 = 9
	V4TypeSRLB           uint16 = 14
	V4TypeSRMS           uint16 = 15
	V4TypeExtPrefixRange uint16 = 2
	V4TypePrefixSID      uint16 = 2
	V4TypeAdjSID         uint16 = 2
	V4TypeLANAdjSID      uint16 = 3
)

// OSPFv3 (RFC 8666) Extended-LSA registry type codes.
const (
	V6TypePrefixSID      uint16 = 4
	V6TypeAdjSID         uint16 = 5
	V6TypeLANAdjSID      uint16 = 6
	V6TypeSIDLabel       uint16 = 7
	V6TypeExtPrefixRange uint16 = 9
)

// ---- OSPFv3 Prefix-SID sub-TLV (RFC 8666 §6, type 4) ----
// Layout: Flags(1) Algorithm(1) Reserved(2) SID/Index/Label(3 or 4). No MT-ID.

// EncodePrefixSIDValueV6 returns the OSPFv3 Prefix-SID sub-TLV value.
func EncodePrefixSIDValueV6(p PrefixSID) []byte {
	isLabel := p.Flags.V && p.Flags.L
	val := p.Index
	if isLabel {
		val = p.Label
	}
	sid := sidBytes(isLabel, val)
	b := make([]byte, 4+len(sid))
	b[0] = p.Flags.toByte()
	b[1] = p.Algorithm
	copy(b[4:], sid)
	return b
}

// DecodePrefixSIDValueV6 parses an OSPFv3 Prefix-SID sub-TLV value with V/L
// validation (RFC 8666 §6).
func DecodePrefixSIDValueV6(v []byte) (PrefixSID, error) {
	if len(v) < 4 {
		return PrefixSID{}, ErrMalformed
	}
	flags := sidFlagsFromByte(v[0])
	if !flags.validVL() {
		return PrefixSID{}, ErrMalformed
	}
	isLabel := flags.V && flags.L
	width := 4
	if isLabel {
		width = 3
	}
	if len(v) < 4+width {
		return PrefixSID{}, ErrMalformed
	}
	p := PrefixSID{Flags: flags, Algorithm: v[1], IsLabel: isLabel}
	if isLabel {
		p.Label = read24(v, 4) & 0x0FFFFF
	} else {
		p.Index = binary.BigEndian.Uint32(v[4:])
	}
	return p, nil
}

// ---- OSPFv3 Adj-SID sub-TLV (RFC 8666 §7.1, type 5) ----
// Layout: Flags(1) Weight(1) Reserved(2) SID(3 or 4). No MT-ID.

// EncodeAdjSIDValueV6 returns the OSPFv3 Adj-SID sub-TLV value.
func EncodeAdjSIDValueV6(a AdjSID) []byte {
	isLabel := a.Flags.V && a.Flags.L
	val := a.Index
	if isLabel {
		val = a.Label
	}
	sid := sidBytes(isLabel, val)
	b := make([]byte, 4+len(sid))
	b[0] = a.Flags.toByte()
	b[1] = a.Weight
	copy(b[4:], sid)
	return b
}

// DecodeAdjSIDValueV6 parses an OSPFv3 Adj-SID sub-TLV value.
func DecodeAdjSIDValueV6(v []byte) (AdjSID, error) {
	if len(v) < 4 {
		return AdjSID{}, ErrMalformed
	}
	flags := adjFlagsFromByte(v[0])
	if !flags.validVL() {
		return AdjSID{}, ErrMalformed
	}
	isLabel := flags.V && flags.L
	width := 4
	if isLabel {
		width = 3
	}
	if len(v) < 4+width {
		return AdjSID{}, ErrMalformed
	}
	a := AdjSID{Flags: flags, Weight: v[1], IsLabel: isLabel}
	if isLabel {
		a.Label = read24(v, 4) & 0x0FFFFF
	} else {
		a.Index = binary.BigEndian.Uint32(v[4:])
	}
	return a, nil
}

// ---- OSPFv3 LAN-Adj-SID sub-TLV (RFC 8666 §7.2, type 6) ----
// Layout: Flags(1) Weight(1) Reserved(2) NeighborID(4) SID(3 or 4).

// EncodeLANAdjSIDValueV6 returns the OSPFv3 LAN-Adj-SID sub-TLV value.
func EncodeLANAdjSIDValueV6(a AdjSID) []byte {
	isLabel := a.Flags.V && a.Flags.L
	val := a.Index
	if isLabel {
		val = a.Label
	}
	sid := sidBytes(isLabel, val)
	b := make([]byte, 8+len(sid))
	b[0] = a.Flags.toByte()
	b[1] = a.Weight
	copy(b[4:8], a.NeighborID[:])
	copy(b[8:], sid)
	return b
}

// decodeLANAdjSIDValueV6 parses an OSPFv3 LAN-Adj-SID sub-TLV value.
func decodeLANAdjSIDValueV6(v []byte) (AdjSID, error) {
	if len(v) < 8 {
		return AdjSID{}, ErrMalformed
	}
	flags := adjFlagsFromByte(v[0])
	if !flags.validVL() {
		return AdjSID{}, ErrMalformed
	}
	isLabel := flags.V && flags.L
	width := 4
	if isLabel {
		width = 3
	}
	if len(v) < 8+width {
		return AdjSID{}, ErrMalformed
	}
	a := AdjSID{Flags: flags, Weight: v[1], IsLabel: isLabel, IsLAN: true}
	copy(a.NeighborID[:], v[4:8])
	if isLabel {
		a.Label = read24(v, 8) & 0x0FFFFF
	} else {
		a.Index = binary.BigEndian.Uint32(v[8:])
	}
	return a, nil
}

// ---- OSPFv3 SID/Label sub-TLV (RFC 8666 §3.1, type 7) ----
//
// No encoder, because RFC 8666 gives this sub-TLV no parent. §3.1 says only
// that it "appears in multiple TLVs or sub-TLVs defined later in this
// document", and it names none. Every TLV the document defines carries its SID
// inline: Prefix-SID (§6), Adj-SID (§7.1), LAN-Adj-SID (§7.2) and Extended
// Prefix Range (§5). Two TLVs do nest a SID/Label sub-TLV, the SID/Label Range
// and the SR Local Block. §4 takes both from RFC 8665 unmodified, so both use
// the OSPFv2 type-1 code (EncodeRangeValue, codec.go). V6TypeSIDLabel stays to
// record the value RFC 8666 §9 assigns.

// ---- OSPFv3 Extended Prefix Range TLV (RFC 8666 §5, type 9) ----
// Layout: PrefixLength(1) AF(1) RangeSize(2) Flags(1) Reserved(3)
//         AddressPrefix(((PrefixLength+31)/32) words, zero padded) Sub-TLVs.

// v6PrefixWordBytes returns the number of address-prefix bytes for a prefix
// length: ((PrefixLength+31)/32) 32-bit words (RFC 8666 §5).
func v6PrefixWordBytes(prefixLen uint8) int { return ((int(prefixLen) + 31) / 32) * 4 }

// EncodeExtPrefixRangeValueV6 returns the value of an IPv6 Extended Prefix Range
// TLV carrying one Prefix-SID sub-TLV. addr supplies the significant prefix bytes;
// it is zero-padded/truncated to the RFC 8666 §5 word count.
func EncodeExtPrefixRangeValueV6(prefixLen uint8, addr []byte, rangeSize uint16, sid PrefixSID) []byte {
	words := v6PrefixWordBytes(prefixLen)
	sub := writeSubTLV(V6TypePrefixSID, EncodePrefixSIDValueV6(sid))
	b := make([]byte, 8+words+len(sub))
	b[0] = prefixLen
	b[1] = 1 // AF = IPv6 unicast
	binary.BigEndian.PutUint16(b[2:], rangeSize)
	n := min(len(addr), words)
	copy(b[8:8+n], addr[:n])
	copy(b[8+words:], sub)
	return b
}

// DecodeExtPrefixRangeValueV6 parses an IPv6 Extended Prefix Range TLV value.
func DecodeExtPrefixRangeValueV6(v []byte) (ExtPrefixRange, error) {
	if len(v) < 8 {
		return ExtPrefixRange{}, ErrMalformed
	}
	r := ExtPrefixRange{
		PrefixLength: v[0],
		AF:           v[1],
		RangeSize:    binary.BigEndian.Uint16(v[2:]),
		IAFlag:       v[4]&0x80 != 0,
	}
	words := v6PrefixWordBytes(r.PrefixLength)
	if len(v) < 8+words {
		return ExtPrefixRange{}, ErrMalformed
	}
	if words > 0 {
		r.AddressV6 = make([]byte, words)
		copy(r.AddressV6, v[8:8+words])
	}
	it := newSubTLVIter(v[8+words:])
	for it.Next() {
		if it.Type() != V6TypePrefixSID {
			continue
		}
		p, err := DecodePrefixSIDValueV6(it.Value())
		if err != nil {
			return ExtPrefixRange{}, err
		}
		r.PrefixSIDs = append(r.PrefixSIDs, p)
	}
	if it.Err() != nil {
		return ExtPrefixRange{}, it.Err()
	}
	return r, nil
}
