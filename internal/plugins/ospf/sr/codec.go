// Design: docs/architecture/wire/ospf.md -- OSPF Segment Routing IPv4 wire codec
// (RFC 8665). Encodes/decodes the TLV and sub-TLV VALUE bytes carried by the
// RFC 7770 RI LSA and the RFC 7684 Extended Prefix/Link Opaque LSAs; the opaque
// carrier frames the outer Type/Length and 4-octet padding. Bound-checked decode
// never panics; a malformed length yields an error so the caller ignores the LSA.
// RFC: rfc/short/rfc8665.md (§2.1 SID/Label, §3 RI TLVs, §4 Ext-Prefix-Range, §5 Prefix-SID, §6 Adj-SID)

package sr

import (
	"encoding/binary"
	"errors"
	"slices"
)

// ErrMalformed marks a length- or field-inconsistent SR TLV/sub-TLV. The caller
// treats the carrying LSA as malformed and does not install anything from it
// (RFC 8665 §9/§10; RFC 8666 §10/§11).
var ErrMalformed = errors.New("sr: malformed TLV")

// RFC 8665 flag bit masks. Bit 0 is the most significant bit of the flags octet.
// Prefix-SID (§5): NP bit1, M bit2, E bit3, V bit4, L bit5.
const (
	flagNP byte = 0x40
	flagM  byte = 0x20
	flagE  byte = 0x10
	flagV  byte = 0x08
	flagL  byte = 0x04
)

// Adj-SID (§6.1) flag bit masks: B bit0, V bit1, L bit2, G bit3, P bit4.
const (
	adjFlagB byte = 0x80
	adjFlagV byte = 0x40
	adjFlagL byte = 0x20
	adjFlagG byte = 0x10
	adjFlagP byte = 0x08
)

// AdjSIDFlags carries the Adj-SID / LAN-Adj-SID flags (RFC 8665 §6.1/§6.2,
// RFC 8666 §7.1/§7.2 use the same bit positions).
type AdjSIDFlags struct {
	B bool // Backup: adjacency eligible for protection
	V bool // Value/Index
	L bool // Local/Global
	G bool // Group
	P bool // Persistent
}

// PrefixSID is a decoded Prefix-SID sub-TLV. IsLabel selects Label (V=1/L=1,
// 3-octet local label) over Index (V=0/L=0, 4-octet index).
type PrefixSID struct {
	Flags     SIDFlags
	MTID      uint8
	Algorithm uint8
	Index     uint32
	Label     uint32
	IsLabel   bool
}

// AdjSID is a decoded Adj-SID or LAN-Adj-SID sub-TLV. IsLAN marks the LAN form
// (NeighborID present).
type AdjSID struct {
	Flags      AdjSIDFlags
	MTID       uint8
	Weight     uint8
	NeighborID [4]byte
	Index      uint32
	Label      uint32
	IsLabel    bool
	IsLAN      bool
}

// ExtPrefixRange is a decoded OSPF Extended Prefix Range TLV (RFC 8665 §4 IPv4,
// RFC 8666 §5 IPv6). Address holds the IPv4 32-bit prefix; AddressV6 holds the
// IPv6 prefix padded to ((PrefixLength+31)/32) 32-bit words. AF selects which.
type ExtPrefixRange struct {
	PrefixLength uint8
	AF           uint8
	RangeSize    uint16
	IAFlag       bool
	Address      [4]byte
	AddressV6    []byte
	PrefixSIDs   []PrefixSID
}

func (f SIDFlags) toByte() byte {
	var b byte
	if f.NP {
		b |= flagNP
	}
	if f.M {
		b |= flagM
	}
	if f.E {
		b |= flagE
	}
	if f.V {
		b |= flagV
	}
	if f.L {
		b |= flagL
	}
	return b
}

func sidFlagsFromByte(b byte) SIDFlags {
	return SIDFlags{NP: b&flagNP != 0, M: b&flagM != 0, E: b&flagE != 0, V: b&flagV != 0, L: b&flagL != 0}
}

// validVL reports whether the V/L pair is one of the only two legal combinations
// (RFC 8665 §5): V=0/L=0 (index) or V=1/L=1 (local label).
func (f SIDFlags) validVL() bool { return f.V == f.L }

func (f AdjSIDFlags) toByte() byte {
	var b byte
	if f.B {
		b |= adjFlagB
	}
	if f.V {
		b |= adjFlagV
	}
	if f.L {
		b |= adjFlagL
	}
	if f.G {
		b |= adjFlagG
	}
	if f.P {
		b |= adjFlagP
	}
	return b
}

func adjFlagsFromByte(b byte) AdjSIDFlags {
	return AdjSIDFlags{B: b&adjFlagB != 0, V: b&adjFlagV != 0, L: b&adjFlagL != 0, G: b&adjFlagG != 0, P: b&adjFlagP != 0}
}

func (f AdjSIDFlags) validVL() bool { return f.V == f.L }

func align4(n int) int { return (n + 3) &^ 3 }

// put24 writes the low 24 bits of v big-endian at buf[0:3].
func put24(buf []byte, v uint32) {
	buf[0] = byte(v >> 16)
	buf[1] = byte(v >> 8)
	buf[2] = byte(v)
}

func read24(b []byte, off int) uint32 {
	return uint32(b[off])<<16 | uint32(b[off+1])<<8 | uint32(b[off+2])
}

// sidBytes encodes a SID/Index/Label field: 3 octets (20-bit label) if isLabel,
// else 4 octets (32-bit index).
func sidBytes(isLabel bool, val uint32) []byte {
	if isLabel {
		return []byte{byte(val >> 16 & 0x0F), byte(val >> 8), byte(val)}
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, val)
	return b
}

// writeSubTLV frames one 4-octet-aligned sub-TLV (Type, Length, value, zero pad),
// per the RFC 5250 / RFC 7770 opaque TLV convention.
func writeSubTLV(typ uint16, value []byte) []byte {
	b := make([]byte, 4+align4(len(value)))
	binary.BigEndian.PutUint16(b, typ)
	binary.BigEndian.PutUint16(b[2:], uint16(len(value)))
	copy(b[4:], value)
	return b
}

// subTLVIter is a bound-checked iterator over consecutive 4-octet-aligned
// sub-TLVs. It never panics; Err reports truncation.
type subTLVIter struct {
	data []byte
	off  int
	typ  uint16
	val  []byte
	err  error
}

func newSubTLVIter(b []byte) *subTLVIter { return &subTLVIter{data: b} }

func (it *subTLVIter) Next() bool {
	if it.err != nil || it.off >= len(it.data) {
		return false
	}
	if len(it.data)-it.off < 4 {
		it.err = ErrMalformed
		return false
	}
	it.typ = binary.BigEndian.Uint16(it.data[it.off:])
	l := int(binary.BigEndian.Uint16(it.data[it.off+2:]))
	if it.off+4+l > len(it.data) {
		it.err = ErrMalformed
		return false
	}
	it.val = it.data[it.off+4 : it.off+4+l]
	it.off += 4 + align4(l)
	if it.off > len(it.data) {
		// Padding runs past the region: tolerate an unpadded final sub-TLV.
		it.off = len(it.data)
	}
	return true
}

func (it *subTLVIter) Type() uint16  { return it.typ }
func (it *subTLVIter) Value() []byte { return it.val }
func (it *subTLVIter) Err() error    { return it.err }

// ---- SID/Label sub-TLV (RFC 8665 §2.1, type 1) ----

func encodeSIDLabelSubTLV(isLabel bool, val uint32) []byte {
	return writeSubTLV(1, sidBytes(isLabel, val))
}

// decodeSIDLabelSubTLV parses exactly the first SID/Label sub-TLV in v and
// returns whether it is a label (Length 3) or index (Length 4).
func decodeSIDLabelSubTLV(v []byte) (bool, uint32, error) {
	it := newSubTLVIter(v)
	if !it.Next() {
		if it.Err() != nil {
			return false, 0, it.Err()
		}
		return false, 0, ErrMalformed
	}
	if it.Type() != 1 {
		return false, 0, ErrMalformed
	}
	val := it.Value()
	switch len(val) {
	case 3:
		return true, read24(val, 0) & 0x0FFFFF, nil
	case 4:
		return false, binary.BigEndian.Uint32(val), nil
	default:
		return false, 0, ErrMalformed
	}
}

// ---- SR-Algorithm TLV (RFC 8665 §3.1, type 8) ----

// EncodeAlgorithmValue returns the SR-Algorithm TLV value (a list of algorithm
// octets). Algorithm 0 (SPF) MUST be present (RFC 8665 §3.1).
func EncodeAlgorithmValue(algos []uint8) []byte {
	b := make([]byte, len(algos))
	copy(b, algos)
	return b
}

// DecodeAlgorithmValue returns the advertised algorithm list.
func DecodeAlgorithmValue(v []byte) ([]uint8, error) {
	out := make([]uint8, len(v))
	copy(out, v)
	return out, nil
}

// HasAlgorithm reports whether a is in the advertised algorithm list.
func HasAlgorithm(algos []uint8, a uint8) bool {
	return slices.Contains(algos, a)
}

// ---- SID/Label Range (SRGB, type 9) and SR Local Block (SRLB, type 14) ----

// EncodeRangeValue returns the value of a SID/Label Range or SRLB TLV: a 3-octet
// Range Size, a reserved octet, and exactly one SID/Label sub-TLV carrying the
// first label of the range (RFC 8665 §3.2/§3.3).
func EncodeRangeValue(r LabelRange) []byte {
	sub := encodeSIDLabelSubTLV(true, r.Base)
	b := make([]byte, 4+len(sub))
	put24(b, r.Size)
	copy(b[4:], sub)
	return b
}

// DecodeRangeValue parses a SID/Label Range or SRLB TLV value into a LabelRange.
// It enforces Range Size > 0 and exactly one SID/Label sub-TLV (RFC 8665 §3.2), and
// hardens the reception (RFC 8665 §10 / RFC 8666 §11): a SID/Label Range lives in the
// 20-bit MPLS label space whose first 16 labels (0..15) are reserved (RFC 3032). A
// range whose base is reserved, whose base falls outside the 20-bit label space, or
// that extends past the largest 20-bit label is malformed and rejected, so a received
// range can never source a reserved or out-of-space label. The 3-octet label sub-TLV
// form is already masked to 20 bits by the read below; a 4-octet (index-form) base
// beyond the label space is caught by the bounds check.
func DecodeRangeValue(v []byte) (LabelRange, error) {
	if len(v) < 8 {
		return LabelRange{}, ErrMalformed
	}
	size := read24(v, 0)
	if size == 0 {
		return LabelRange{}, ErrMalformed
	}
	it := newSubTLVIter(v[4:])
	var base uint32
	count := 0
	for it.Next() {
		if it.Type() != 1 {
			continue
		}
		count++
		val := it.Value()
		switch len(val) {
		case 3:
			base = read24(val, 0) & 0x0FFFFF
		case 4:
			base = binary.BigEndian.Uint32(val)
		default:
			return LabelRange{}, ErrMalformed
		}
	}
	if it.Err() != nil {
		return LabelRange{}, it.Err()
	}
	if count != 1 {
		return LabelRange{}, ErrMalformed
	}
	// Reject a reserved base (0..15), a base outside the 20-bit label space, or a range
	// that runs past MaxLabel (uint64 arithmetic so base+size cannot overflow).
	if base < MinLabel || base > MaxLabel || uint64(base)+uint64(size)-1 > uint64(MaxLabel) {
		return LabelRange{}, ErrMalformed
	}
	return LabelRange{Base: base, Size: size}, nil
}

// ---- SRMS Preference TLV (RFC 8665 §3.4, type 15) ----

// EncodeSRMSValue returns the SRMS Preference TLV value (1-octet preference plus
// 3 reserved octets).
func EncodeSRMSValue(pref uint8) []byte { return []byte{pref, 0, 0, 0} }

// DecodeSRMSValue reads the SRMS preference.
func DecodeSRMSValue(v []byte) (uint8, error) {
	if len(v) < 1 {
		return 0, ErrMalformed
	}
	return v[0], nil
}

// ---- Prefix-SID sub-TLV (RFC 8665 §5, type 2) ----

// EncodePrefixSIDValue returns the Prefix-SID sub-TLV value: Flags, Reserved,
// MT-ID, Algorithm, then the SID/Index/Label field sized by the V/L flags.
func EncodePrefixSIDValue(p PrefixSID) []byte {
	isLabel := p.Flags.V && p.Flags.L
	val := p.Index
	if isLabel {
		val = p.Label
	}
	sid := sidBytes(isLabel, val)
	b := make([]byte, 4+len(sid))
	b[0] = p.Flags.toByte()
	b[2] = p.MTID
	b[3] = p.Algorithm
	copy(b[4:], sid)
	return b
}

// DecodePrefixSIDValue parses a Prefix-SID sub-TLV value with V/L validation
// (RFC 8665 §5: only V=0/L=0 or V=1/L=1 are valid, else ignore).
func DecodePrefixSIDValue(v []byte) (PrefixSID, error) {
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
	p := PrefixSID{Flags: flags, MTID: v[2], Algorithm: v[3], IsLabel: isLabel}
	if isLabel {
		p.Label = read24(v, 4) & 0x0FFFFF
	} else {
		p.Index = binary.BigEndian.Uint32(v[4:])
	}
	return p, nil
}

// ---- Adj-SID (RFC 8665 §6.1, type 2) and LAN-Adj-SID (§6.2, type 3) ----

// EncodeAdjSIDValue returns the Adj-SID sub-TLV value.
func EncodeAdjSIDValue(a AdjSID) []byte {
	isLabel := a.Flags.V && a.Flags.L
	val := a.Index
	if isLabel {
		val = a.Label
	}
	sid := sidBytes(isLabel, val)
	b := make([]byte, 4+len(sid))
	b[0] = a.Flags.toByte()
	b[2] = a.MTID
	b[3] = a.Weight
	copy(b[4:], sid)
	return b
}

// DecodeAdjSIDValue parses an Adj-SID sub-TLV value with V/L validation.
func DecodeAdjSIDValue(v []byte) (AdjSID, error) {
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
	a := AdjSID{Flags: flags, MTID: v[2], Weight: v[3], IsLabel: isLabel}
	if isLabel {
		a.Label = read24(v, 4) & 0x0FFFFF
	} else {
		a.Index = binary.BigEndian.Uint32(v[4:])
	}
	return a, nil
}

// EncodeLANAdjSIDValue returns the LAN-Adj-SID sub-TLV value (adds the 4-octet
// Neighbor ID before the SID field).
func EncodeLANAdjSIDValue(a AdjSID) []byte {
	isLabel := a.Flags.V && a.Flags.L
	val := a.Index
	if isLabel {
		val = a.Label
	}
	sid := sidBytes(isLabel, val)
	b := make([]byte, 8+len(sid))
	b[0] = a.Flags.toByte()
	b[2] = a.MTID
	b[3] = a.Weight
	copy(b[4:8], a.NeighborID[:])
	copy(b[8:], sid)
	return b
}

// DecodeLANAdjSIDValue parses a LAN-Adj-SID sub-TLV value.
func DecodeLANAdjSIDValue(v []byte) (AdjSID, error) {
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
	a := AdjSID{Flags: flags, MTID: v[2], Weight: v[3], IsLabel: isLabel, IsLAN: true}
	copy(a.NeighborID[:], v[4:8])
	if isLabel {
		a.Label = read24(v, 8) & 0x0FFFFF
	} else {
		a.Index = binary.BigEndian.Uint32(v[8:])
	}
	return a, nil
}

// ---- OSPF Extended Prefix Range TLV (RFC 8665 §4, type 2, IPv4) ----

// EncodeExtPrefixRangeValueV4 returns the value of an IPv4 Extended Prefix Range
// TLV carrying one Prefix-SID sub-TLV. iaFlag sets the IA-Flag (RFC 8665 §4: an
// ABR propagating the range between areas MUST set it).
func EncodeExtPrefixRangeValueV4(prefixLen uint8, addr [4]byte, rangeSize uint16, iaFlag bool, sid PrefixSID) []byte {
	sub := writeSubTLV(2, EncodePrefixSIDValue(sid))
	b := make([]byte, 12+len(sub))
	b[0] = prefixLen
	b[1] = 0 // AF = IPv4 unicast
	binary.BigEndian.PutUint16(b[2:], rangeSize)
	if iaFlag {
		b[4] = 0x80 // IA-Flag is bit 0 (MSB)
	}
	copy(b[8:12], addr[:])
	copy(b[12:], sub)
	return b
}

// DecodeExtPrefixRangeValueV4 parses an IPv4 Extended Prefix Range TLV value.
func DecodeExtPrefixRangeValueV4(v []byte) (ExtPrefixRange, error) {
	if len(v) < 12 {
		return ExtPrefixRange{}, ErrMalformed
	}
	r := ExtPrefixRange{
		PrefixLength: v[0],
		AF:           v[1],
		RangeSize:    binary.BigEndian.Uint16(v[2:]),
		IAFlag:       v[4]&0x80 != 0,
	}
	copy(r.Address[:], v[8:12])
	it := newSubTLVIter(v[12:])
	for it.Next() {
		if it.Type() != 2 {
			continue
		}
		p, err := DecodePrefixSIDValue(it.Value())
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
