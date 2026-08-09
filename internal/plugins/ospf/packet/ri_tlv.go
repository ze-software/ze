// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- RFC 7770 Router Information TLV codec.
// RFC: rfc/short/rfc7770.md -- the RI LSA TLV stream (sec 2.3), the Router Informational
// Capabilities TLV (sec 2.4), the capability bit assignments (sec 2.5), and the Router
// Functional Capabilities TLV (sec 2.6).
//
// The RI LSA body is a sequence of Type/Length/Value triplets in the SAME 4-byte-aligned
// format as RFC 3630 TE (sec 2.3). This file codes RI TLVs ON TOP of the generic
// opaque_tlv.go builder/iterator (spec-ospf-ext-1); it never re-implements TLV framing or
// the 4-octet alignment. It exposes an RI-specific surface (the exported RITLV type, the
// builder/iterator, and the Informational Capabilities bitfield) for the RI consumer in
// package ospf, which owns the RI LSA carriage and the capability derivation.

package packet

// RIOpaqueType is the RFC 7770 sec 2.1 OSPFv2 Router Information Opaque LSA type: an Opaque
// LSA (LS type 9/10/11 by scope) with Opaque type 4 and the RI LSA Instance ID as its Opaque
// ID. The OSPFv3 RI LSA is instead a native LSA (v3/types.RIFunctionCode); only OSPFv2
// carries RI as an opaque LSA.
const RIOpaqueType uint8 = 4

// RFC 7770 sec 2.3 / sec 5.3: the assigned RI LSA TLV type codes carried by this
// implementation. Higher type codes are registered by downstream consumers (Segment
// Routing / ext-5) through the in-process RI-TLV registry in package ospf.
const (
	// RITLVInformationalCapabilities is the Router Informational Capabilities TLV (sec 2.4).
	RITLVInformationalCapabilities uint16 = 1
	// RITLVFunctionalCapabilities is the Router Functional Capabilities TLV (sec 2.6).
	RITLVFunctionalCapabilities uint16 = 2
)

// RFC 7770 sec 2.5: the Router Informational Capability bit indices. Bits are numbered
// left to right with the most significant bit being bit 0 (sec 2.4), so bit index i maps
// to mask 1<<(31-i) in the 32-bit informational-capabilities word.
const (
	RIInfoBitGracefulRestart       uint = 0 // RFC 3623: OSPF graceful restart capable
	RIInfoBitGracefulRestartHelper uint = 1 // RFC 3623: OSPF graceful restart helper
	RIInfoBitStubRouter            uint = 2 // RFC 6987: OSPF Stub Router support
	RIInfoBitTrafficEngineering    uint = 3 // RFC 3630: OSPF Traffic Engineering support
)

// RICapabilitiesMinLen is the initial length (sec 2.4/2.6) of the Informational and
// Functional Capabilities TLV values: 4 octets (32 bits), a multiple of 4.
const RICapabilitiesMinLen = 4

// RITLV is one Type/Length/Value triple in an RI LSA body (RFC 7770 sec 2.3). Value is the
// raw value bytes without the 4-byte alignment padding; the builder pads to a 4-octet
// boundary and the Length field on the wire excludes that padding.
type RITLV struct {
	Type  uint16
	Value []byte
}

// toOpaque adapts an RI TLV to the generic opaque TLV so the shared 4-byte-aligned builder
// (opaque_tlv.go) carries it: RI reuses the framing rather than re-implementing it.
func (t RITLV) toOpaque() opaqueTLV { return opaqueTLV(t) }

// RITLVsEncodedLen returns the total on-wire length of a TLV set (each 4-byte aligned),
// so the RI originator can size an instance and detect overflow (RFC 7770 sec 3).
func RITLVsEncodedLen(tlvs []RITLV) int {
	n := 0
	for _, t := range tlvs {
		n += t.toOpaque().EncodedLen()
	}
	return n
}

// EncodeRITLVs renders an RI LSA body (the bytes after the 20-byte LSA header) from an
// ordered TLV set using the ext-1 4-byte-aligned builder. The caller is responsible for
// ordering (RFC 7770 sec 2.4: the Informational Capabilities TLV MUST be first in
// Instance 0). The result is handed to the opaque carrier (OSPFv2) or the native
// self-LSA encoder (OSPFv3) verbatim.
func EncodeRITLVs(tlvs []RITLV) []byte {
	ops := make([]opaqueTLV, len(tlvs))
	for i, t := range tlvs {
		ops[i] = t.toOpaque()
	}
	b := make([]byte, opaqueTLVsLen(ops))
	writeOpaqueTLVs(b, ops)
	return b
}

// RIInfoBitMask returns the informational-capabilities word mask for bit index i (0..31),
// encoding the RFC 7770 sec 2.4 "most significant bit is bit 0" numbering.
func RIInfoBitMask(bit uint) uint32 {
	if bit > 31 {
		return 0
	}
	return uint32(1) << (31 - bit)
}

// RICapabilitiesValue encodes a 32-bit capability word as the 4-octet, big-endian TLV
// value used by both the Informational (sec 2.4) and Functional (sec 2.6) Capabilities
// TLVs (initial length 4, a multiple of 4).
func RICapabilitiesValue(field uint32) []byte {
	b := make([]byte, RICapabilitiesMinLen)
	writeUint32(b, 0, field)
	return b
}

// RIReadCapabilities decodes the leading 32 capability bits from a Capabilities TLV value.
// RFC 7770 sec 2.6: bits beyond the received Length are treated as "not supported", so a
// value shorter than 4 octets contributes only the octets present and the rest read 0; a
// longer value (a multiple of 4) is truncated to the first 32 assigned bits this
// implementation understands.
func RIReadCapabilities(value []byte) uint32 {
	var field uint32
	for i := 0; i < RICapabilitiesMinLen && i < len(value); i++ {
		field |= uint32(value[i]) << (24 - 8*i)
	}
	return field
}

// DecodeRITLVStream walks an RI LSA body's TLV stream (the bytes after the 20-byte LSA
// header) over the ext-1 bound-checked iterator and returns the decoded TLVs. Each Value is a
// zero-copy view into body (the caller must copy to retain it past body's lifetime). It NEVER
// panics on malformed input (AC-14): on a truncated or over-long TLV it returns the TLVs
// decoded so far plus a non-nil error, so a renderer shows what it can.
func DecodeRITLVStream(body []byte) ([]RITLV, error) {
	it := newOpaqueTLVIterator(body)
	var out []RITLV
	for it.Next() {
		out = append(out, RITLV{Type: it.Type(), Value: it.Value()})
	}
	return out, it.Err()
}
