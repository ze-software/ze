// Design: docs/architecture/wire/ospfv3.md -- RFC 8362 Extended-LSA body codec.
// An OSPFv3 Extended LSA (E-Router-LSA, E-Intra/Inter-Area-Prefix-LSA, E-AS-External,
// E-Type-7) body is a set of 4-octet-aligned top-level TLVs, each of which may nest
// sub-TLVs in the same format (RFC 8362 §3). This codec frames and parses that TLV
// stream buffer-first and bound-checked; the SR consumer (spec-ospf-ext-5) interprets
// the type-specific fixed fields and the RFC 8666 SR sub-TLVs. Unknown TLVs round-trip
// verbatim (a decoded body re-encodes byte-for-byte).
// RFC: rfc/short/rfc5340.md (§A.4.2.1 LSA header); RFC 8362 §3 (Extended-LSA TLV format)

package packet

// ExtendedTLV is one RFC 8362 Extended-LSA TLV: its type and its full value bytes
// (the type-specific fixed fields followed by any nested sub-TLVs). Value is a caller
// -owned copy on decode.
type ExtendedTLV struct {
	Type  uint16
	Value []byte
}

// ExtendedLSA is a decoded RFC 8362 Extended-LSA body: its top-level TLVs in wire
// order.
type ExtendedLSA struct {
	TLVs []ExtendedTLV
}

// EncodeExtendedLSABody serializes the Extended-LSA body (the bytes after the 20-byte
// LSA header) buffer-first: each TLV is 4-octet aligned with its unpadded Length.
func EncodeExtendedLSABody(lsa ExtendedLSA) []byte {
	tlvs := make([]tlv, len(lsa.TLVs))
	for i, t := range lsa.TLVs {
		tlvs[i] = tlv(t)
	}
	buf := make([]byte, tlvsEncodedLen(tlvs))
	writeTLVs(buf, tlvs)
	return buf
}

// DecodeExtendedLSABody parses an Extended-LSA body into its top-level TLVs. A
// malformed TLV (truncated header or an over-length value) makes the whole body
// malformed and returns an error; a missing pad on the FINAL TLV is tolerated (be
// liberal on receive, see tlvIterator); the parser never panics (RFC 8362 §4).
func DecodeExtendedLSABody(body []byte) (ExtendedLSA, error) {
	var out ExtendedLSA
	it := newTLVIterator(body)
	for it.Next() {
		val := make([]byte, len(it.Value()))
		copy(val, it.Value())
		out.TLVs = append(out.TLVs, ExtendedTLV{Type: it.Type(), Value: val})
	}
	if it.Err() != nil {
		return ExtendedLSA{}, it.Err()
	}
	return out, nil
}

// SubTLVsAt parses the nested sub-TLVs of a TLV value starting at fixedLen (the length
// of the TLV type's fixed-field prefix). A fixedLen at or beyond the value returns no
// sub-TLVs (a TLV may carry only fixed fields).
func SubTLVsAt(value []byte, fixedLen int) ([]ExtendedTLV, error) {
	if fixedLen >= len(value) {
		return nil, nil
	}
	var out []ExtendedTLV
	it := newTLVIterator(value[fixedLen:])
	for it.Next() {
		val := make([]byte, len(it.Value()))
		copy(val, it.Value())
		out = append(out, ExtendedTLV{Type: it.Type(), Value: val})
	}
	if it.Err() != nil {
		return nil, it.Err()
	}
	return out, nil
}

// encodeSubTLVs frames a set of sub-TLVs into a 4-octet-aligned byte slice for
// embedding after a TLV's fixed fields.
func encodeSubTLVs(subs []ExtendedTLV) []byte {
	tlvs := make([]tlv, len(subs))
	for i, s := range subs {
		tlvs[i] = tlv(s)
	}
	buf := make([]byte, tlvsEncodedLen(tlvs))
	writeTLVs(buf, tlvs)
	return buf
}

// AppendSubTLVs returns fixed followed by the framed sub-TLVs -- the value of an
// Extended-LSA TLV that carries both fixed fields and nested sub-TLVs.
func AppendSubTLVs(fixed []byte, subs []ExtendedTLV) []byte {
	sub := encodeSubTLVs(subs)
	out := make([]byte, len(fixed)+len(sub))
	copy(out, fixed)
	copy(out[len(fixed):], sub)
	return out
}
