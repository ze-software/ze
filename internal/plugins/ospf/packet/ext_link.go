// Design: docs/architecture/ospf/ospf-ext-4-extended-link-prefix.md -- RFC 7684 Extended Link Opaque LSA
// (Opaque Type 8) body codec.
// RFC: rfc/short/rfc7684.md -- sec 3 (Extended Link Opaque LSA), sec 3.1 (Extended Link TLV,
// one per LSA SHALL), sec 5 (malformed-LSA rules). RFC 2328 App A.4.2 (Link Type/ID/Data).
//
// The Extended Link Opaque LSA is an Opaque LSA (Opaque type 8, LS type 10 area only) whose
// body carries exactly one Extended Link TLV (top-level type 1): the fixed Link Type /
// Reserved / Link ID / Link Data fields (mirrored from the Router-LSA link, RFC 2328 A.4.2)
// then a nested sub-TLV region. This file codes that body ON TOP of the generic
// 4-byte-aligned TLV iterator/builder from opaque_tlv.go (spec-ospf-ext-1). It defines only
// the container; the sub-TLV VALUES (Adj-SID, etc.) live in RFC 8665 (spec-ospf-ext-5).

package packet

const (
	// ExtLinkOpaqueType is the RFC 7684 sec 3 Extended Link Opaque LSA Opaque type.
	ExtLinkOpaqueType uint8 = 8

	// ExtLinkTLVType is the RFC 7684 sec 3.1 top-level Extended Link TLV type.
	ExtLinkTLVType uint16 = 1

	// extLinkTLVFixedLen is the Extended Link TLV value bytes before the sub-TLV region:
	// Link Type(1) + Reserved(3) + Link ID(4) + Link Data(4). Already 4-octet aligned.
	extLinkTLVFixedLen = 12
)

// ExtLinkTLV is one decoded RFC 7684 sec 3.1 Extended Link TLV: the fixed Link Type / Link ID
// / Link Data fields (RFC 2328 App A.4.2) plus the nested sub-TLV region. The 3-octet
// Reserved field is not carried (transmitted as zero, ignored on receive).
type ExtLinkTLV struct {
	LinkType uint8
	LinkID   [4]byte
	LinkData [4]byte
	SubTLVs  []ExtSubTLV
}

// ExtLinkLSA is one decoded Extended Link Opaque LSA body. RFC 7684 sec 3.1 SHALL: exactly one
// Extended Link TLV per LSA; ExtraLinkTLVs counts any beyond the first (which decode ignores,
// using only the first) so the consumer can log the violation (AC-8).
type ExtLinkLSA struct {
	Link          ExtLinkTLV
	HasLink       bool
	ExtraLinkTLVs int
}

// encodeValue renders the Extended Link TLV value (fixed fields + aligned sub-TLVs). Reserved
// (v[1..3]) is zero.
func (t ExtLinkTLV) encodeValue() []byte {
	sub := extSubTLVsToOpaque(t.SubTLVs)
	v := make([]byte, extLinkTLVFixedLen+opaqueTLVsLen(sub))
	v[0] = t.LinkType
	writeIPv4(v, 4, t.LinkID)
	writeIPv4(v, 8, t.LinkData)
	off := extLinkTLVFixedLen
	for _, s := range sub {
		off = s.WriteTo(v, off)
	}
	return v
}

// EncodeExtLinkLSA renders an Extended Link Opaque LSA body (the bytes after the 20-byte LSA
// header) carrying exactly one Extended Link TLV, using the ext-1 4-byte-aligned TLV builder.
//
// Cold-path exception to ai/rules/buffer-first: see EncodeExtPrefixLSA (origination/refresh
// only, never packet forwarding).
func EncodeExtLinkLSA(link ExtLinkTLV) []byte {
	tlv := opaqueTLV{Type: ExtLinkTLVType, Value: link.encodeValue()}
	b := make([]byte, tlv.EncodedLen())
	tlv.WriteTo(b, 0)
	return b
}

// DecodeExtLinkLSA parses an Extended Link Opaque LSA body. It walks the top-level TLVs with
// the bound-checked ext-1 iterator and NEVER panics on malformed input (RFC 7684 sec 5 /
// AC-7). RFC 7684 sec 3.1 SHALL: only the FIRST Extended Link TLV is used; any extra is
// counted (ExtraLinkTLVs) for the consumer to log (AC-8). An unrecognized top-level TLV is
// skipped via its Length. Sub-TLV Values are zero-copy views into body.
func DecodeExtLinkLSA(body []byte) (ExtLinkLSA, error) {
	var out ExtLinkLSA
	it := newOpaqueTLVIterator(body)
	for it.Next() {
		if it.Type() != ExtLinkTLVType {
			continue // unknown top-level TLV: skip via Length (iterator advanced)
		}
		if out.HasLink {
			out.ExtraLinkTLVs++ // §3.1: only one SHALL be advertised; use the first
			continue
		}
		tlv, err := decodeExtLinkTLV(it.Value())
		if err != nil {
			return ExtLinkLSA{}, err
		}
		out.Link = tlv
		out.HasLink = true
	}
	if it.Err() != nil {
		return ExtLinkLSA{}, it.Err()
	}
	return out, nil
}

// decodeExtLinkTLV parses an Extended Link TLV value: the 12-octet fixed header (Link Type /
// Reserved / Link ID / Link Data) then the sub-TLV region. A value shorter than the fixed
// header is malformed (RFC 7684 sec 5).
func decodeExtLinkTLV(v []byte) (ExtLinkTLV, error) {
	if len(v) < extLinkTLVFixedLen {
		return ExtLinkTLV{}, ErrLength
	}
	tlv := ExtLinkTLV{
		LinkType: v[0],
		LinkID:   readIPv4(v, 4),
		LinkData: readIPv4(v, 8),
	}
	sub, err := decodeExtSubTLVs(v[extLinkTLVFixedLen:])
	if err != nil {
		return ExtLinkTLV{}, err
	}
	tlv.SubTLVs = sub
	return tlv, nil
}
