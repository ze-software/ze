// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md -- RFC 7684 Extended Prefix Opaque
// LSA (Opaque Type 7) body codec.
// RFC: rfc/short/rfc7684.md -- sec 2 (Extended Prefix Opaque LSA), sec 2.1 (Extended Prefix
// TLV), sec 5 (malformed-LSA rules). RFC 8665 sec 4 (Extended Prefix Range TLV, carried here
// as a container only, no range semantics).
//
// The Extended Prefix Opaque LSA is an Opaque LSA (Opaque type 7, LS type 10 area or 11 AS)
// whose body is one or more Extended Prefix TLVs (top-level type 1) plus an optional Extended
// Prefix Range TLV (type 2, RFC 8665). Each TLV nests sub-TLVs in the RFC-3630-identical
// 4-byte-aligned generic format. This file codes that body ON TOP of the generic TLV
// iterator/builder from opaque_tlv.go (spec-ospf-ext-1); it never re-implements TLV framing
// or the 4-octet alignment. It defines only the containers (fixed fields + a sub-TLV slot);
// the sub-TLV VALUES (Prefix-SID, etc.) live in RFC 8665 (spec-ospf-ext-5).

package packet

const (
	// ExtPrefixOpaqueType is the RFC 7684 sec 2 Extended Prefix Opaque LSA Opaque type.
	ExtPrefixOpaqueType uint8 = 7

	// ExtPrefixTLVType is the RFC 7684 sec 2.1 top-level Extended Prefix TLV type.
	ExtPrefixTLVType uint16 = 1
	// ExtPrefixRangeTLVType is the RFC 8665 sec 4 Extended Prefix Range TLV type. RFC 7684
	// does NOT define it; it is carried here as an opaque forward-compatible container only.
	ExtPrefixRangeTLVType uint16 = 2

	// RFC 7684 sec 2.1: Route Type values map to the OSPFv2 LS Type registry.
	ExtRouteTypeUnspecified  uint8 = 0
	ExtRouteTypeIntraArea    uint8 = 1
	ExtRouteTypeInterArea    uint8 = 3
	ExtRouteTypeASExternal   uint8 = 5
	ExtRouteTypeNSSAExternal uint8 = 7

	// ExtPrefixAFIPv4Unicast is the only Address Family RFC 7684 sec 2.1 defines; its prefix
	// is a fixed 32-bit value regardless of Prefix Length.
	ExtPrefixAFIPv4Unicast uint8 = 0

	// RFC 7684 sec 2.1 Extended Prefix TLV Flags: A (Attach) and N (Node); others unassigned.
	ExtPrefixFlagA uint8 = 0x80
	ExtPrefixFlagN uint8 = 0x40

	// extPrefixTLVFixedLen is the Extended Prefix TLV value bytes before the sub-TLV region:
	// Route Type(1) + Prefix Length(1) + AF(1) + Flags(1) + Address Prefix(4, AF=0). It is a
	// multiple of 4, so an aligned sub-TLV region needs no extra top-level padding.
	extPrefixTLVFixedLen = 8
)

// ExtSubTLV is one nested sub-TLV (RFC 7684 sec 2) inside an Extended Prefix or Extended Link
// TLV. Value is the raw value bytes without the 4-byte alignment padding; on decode it is a
// zero-copy view into the caller's body (copy to retain past the body's lifetime).
type ExtSubTLV struct {
	Type  uint16
	Value []byte
}

// ExtPrefixTLV is one decoded RFC 7684 sec 2.1 Extended Prefix TLV: the fixed Route Type /
// Prefix Length / AF / Flags / Address Prefix fields plus the nested sub-TLV region. The
// value types are plain so the TLV crosses the plugin boundary by value.
type ExtPrefixTLV struct {
	RouteType     uint8
	PrefixLength  uint8
	AF            uint8
	Flags         uint8
	AddressPrefix [4]byte
	SubTLVs       []ExtSubTLV
}

// HasFlag reports whether the given RFC 7684 sec 2.1 flag bit (A/N) is set.
func (t ExtPrefixTLV) HasFlag(f uint8) bool { return t.Flags&f != 0 }

// ExtPrefixRangeTLV is the RFC 8665 sec 4 Extended Prefix Range TLV. RFC 7684 does not define
// it, so this spec carries it as an opaque forward-compatible container with NO range
// semantics: Value is the whole TLV value preserved byte-for-byte for a downstream consumer
// (spec-ospf-ext-5) that owns RFC 8665. No range field is invented here (ai/rules/no-fabrication).
type ExtPrefixRangeTLV struct {
	Value []byte
}

// ExtPrefixLSA is one decoded Extended Prefix Opaque LSA body: the Extended Prefix TLVs and
// any Extended Prefix Range containers, in wire order.
type ExtPrefixLSA struct {
	Prefixes []ExtPrefixTLV
	Ranges   []ExtPrefixRangeTLV
}

// extSubTLVsToOpaque adapts sub-TLVs to the generic 4-byte-aligned builder (opaque_tlv.go).
func extSubTLVsToOpaque(subs []ExtSubTLV) []opaqueTLV {
	out := make([]opaqueTLV, len(subs))
	for i, s := range subs {
		out[i] = opaqueTLV(s)
	}
	return out
}

// decodeExtSubTLVs walks a sub-TLV region with the bound-checked ext-1 iterator. It NEVER
// panics on malformed input (RFC 7684 sec 5): a sub-TLV overrunning the region or a truncated
// trailing header returns an error so the caller fails the whole LSA. Values are zero-copy
// views into region.
func decodeExtSubTLVs(region []byte) ([]ExtSubTLV, error) {
	var out []ExtSubTLV
	it := newOpaqueTLVIterator(region)
	for it.Next() {
		out = append(out, ExtSubTLV{Type: it.Type(), Value: it.Value()})
	}
	return out, it.Err()
}

// encodeValue renders the Extended Prefix TLV value (fixed fields + aligned sub-TLVs). The
// 32-bit Address Prefix is written for AF=0 regardless of Prefix Length (RFC 7684 sec 2.1).
func (t ExtPrefixTLV) encodeValue() []byte {
	sub := extSubTLVsToOpaque(t.SubTLVs)
	v := make([]byte, extPrefixTLVFixedLen+opaqueTLVsLen(sub))
	v[0] = t.RouteType
	v[1] = t.PrefixLength
	v[2] = t.AF
	v[3] = t.Flags
	writeIPv4(v, 4, t.AddressPrefix)
	off := extPrefixTLVFixedLen
	for _, s := range sub {
		off = s.WriteTo(v, off)
	}
	return v
}

// EncodeExtPrefixLSA renders an Extended Prefix Opaque LSA body (the bytes after the 20-byte
// LSA header) using the ext-1 4-byte-aligned TLV builder. The result is handed to the opaque
// carrier verbatim.
//
// Cold-path exception to ai/rules/buffer-first: origination builds the TLV set with
// make/append and allocates the body slice. It runs only on Extended Prefix LSA origination
// and refresh (a topology/prefix change, rate-limited to MinLSInterval), never on packet
// forwarding, mirroring packet.TELSA.Encode.
func EncodeExtPrefixLSA(lsa ExtPrefixLSA) []byte {
	tlvs := make([]opaqueTLV, 0, len(lsa.Prefixes)+len(lsa.Ranges))
	for i := range lsa.Prefixes {
		tlvs = append(tlvs, opaqueTLV{Type: ExtPrefixTLVType, Value: lsa.Prefixes[i].encodeValue()})
	}
	for i := range lsa.Ranges {
		tlvs = append(tlvs, opaqueTLV{Type: ExtPrefixRangeTLVType, Value: lsa.Ranges[i].Value})
	}
	b := make([]byte, opaqueTLVsLen(tlvs))
	writeOpaqueTLVs(b, tlvs)
	return b
}

// DecodeExtPrefixLSA parses an Extended Prefix Opaque LSA body. It walks the top-level TLVs
// with the bound-checked ext-1 iterator and NEVER panics on malformed input (RFC 7684 sec 5 /
// AC-7): a TLV or sub-TLV overrunning the subsuming LSA/TLV, an Extended Prefix TLV shorter
// than its fixed fields, or trailing data smaller than a TLV header returns an error so the
// caller treats the whole LSA as malformed and does not apply it. An unrecognized top-level
// TLV is skipped via its Length (the iterator advances past it, RFC 7684 forward-compatibility).
// Sub-TLV Values are zero-copy views into body.
func DecodeExtPrefixLSA(body []byte) (ExtPrefixLSA, error) {
	var out ExtPrefixLSA
	it := newOpaqueTLVIterator(body)
	for it.Next() {
		v := it.Value()
		switch it.Type() {
		case ExtPrefixTLVType:
			tlv, err := decodeExtPrefixTLV(v)
			if err != nil {
				return ExtPrefixLSA{}, err
			}
			out.Prefixes = append(out.Prefixes, tlv)
		case ExtPrefixRangeTLVType:
			// RFC 7684 does not define this TLV (RFC 8665 sec 4). Carry the value verbatim.
			out.Ranges = append(out.Ranges, ExtPrefixRangeTLV{Value: v})
		}
		// An unrecognized top-level TLV is skipped: the iterator already advanced past its
		// Length + padding (RFC 7684 forward-compatibility), so no explicit handling is needed.
	}
	if it.Err() != nil {
		return ExtPrefixLSA{}, it.Err()
	}
	return out, nil
}

// decodeExtPrefixTLV parses an Extended Prefix TLV value: the 8-octet fixed header (Route
// Type / Prefix Length / AF / Flags / 32-bit Address Prefix) then the sub-TLV region. A value
// shorter than the fixed header is malformed (RFC 7684 sec 5).
func decodeExtPrefixTLV(v []byte) (ExtPrefixTLV, error) {
	if len(v) < extPrefixTLVFixedLen {
		return ExtPrefixTLV{}, ErrLength
	}
	tlv := ExtPrefixTLV{
		RouteType:     v[0],
		PrefixLength:  v[1],
		AF:            v[2],
		Flags:         v[3],
		AddressPrefix: readIPv4(v, 4),
	}
	sub, err := decodeExtSubTLVs(v[extPrefixTLVFixedLen:])
	if err != nil {
		return ExtPrefixTLV{}, err
	}
	tlv.SubTLVs = sub
	return tlv, nil
}
