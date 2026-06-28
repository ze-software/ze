// Design: plan/learned/928-isis-2-wire.md -- unknown-TLV opaque retention + verbatim re-serialization
// ISO/IEC 10589 clause 7.3.14: an IS MUST accept and re-flood unknown TLVs
// verbatim. The codec keeps every TLV (known or not) as an opaque span so the
// LSDB (isis-6) can re-encode a received LSP byte-for-byte.
//
// RFC: rfc/short/rfc5304.md -- TLV 10 (Authentication) MUST be the first TLV (sec 1)

package packet

// TLV is one decoded TLV retained as a type plus its raw value bytes. It is the
// opaque carrier used both for unknown TLVs (re-flooded verbatim per ISO/IEC
// 10589 clause 7.3.14) and as a uniform encode unit for TLVs the caller has
// already serialized. Value aliases the source buffer on decode; callers that
// retain it past the buffer's lifetime must copy (see CopyValue).
type TLV struct {
	Type  uint8
	Value []byte
}

// CopyValue returns a TLV whose Value is an independent copy, safe to retain
// after the source buffer is recycled (isis-6 LSDB retention).
func (t TLV) CopyValue() TLV {
	cp := make([]byte, len(t.Value))
	copy(cp, t.Value)
	return TLV{Type: t.Type, Value: cp}
}

// EncodedLen returns the number of octets this TLV occupies on the wire.
func (t TLV) EncodedLen() int { return tlvOverhead(len(t.Value)) }

// WriteTo serializes the TLV (type + length + value) into buf at off and
// returns the new offset. Buffer-first; the caller guarantees room. The value
// length is assumed valid (<= MaxTLVValueLen); DecodeTLVs cannot produce a
// longer value because the wire length field is a single octet.
func (t TLV) WriteTo(buf []byte, off int) int {
	return writeTLV(buf, off, t.Type, t.Value)
}

// DecodeTLVs walks the entire TLV region and returns every TLV in order,
// retaining each value as an opaque span (zero-copy: the values alias buf).
// Unknown types are kept identically to known ones, so re-encoding the slice
// reproduces the region byte-for-byte (AC-5, R-6). A truncated final TLV stops
// the walk and is reported via the error; the TLVs decoded before the
// truncation are still returned.
//
// Allocation is bounded by the input, not an explicit cap: buf is a single
// MTU-bounded TLV region (the caller has already framed one received PDU), and
// each TLV consumes at least TLVHeaderLen octets, so the slice can hold at most
// len(buf)/TLVHeaderLen entries. The make() below pre-sizes the slice to that
// bound, so a crafted region cannot force unbounded allocation (security
// review: resource exhaustion).
func DecodeTLVs(buf []byte) ([]TLV, error) {
	if len(buf) == 0 {
		return nil, nil
	}
	out := make([]TLV, 0, len(buf)/TLVHeaderLen)
	it := NewTLVIterator(buf)
	for {
		typ, value, ok := it.Next()
		if !ok {
			break
		}
		out = append(out, TLV{Type: typ, Value: value})
	}
	return out, it.Err()
}

// writeTLVs serializes a slice of TLVs in order into buf at off and returns the
// new offset. Used by the PDU encoders to emit a TLV list (the caller is
// responsible for TLV ordering, e.g. authentication first per RFC 5304).
func writeTLVs(buf []byte, off int, tlvs []TLV) int {
	for _, t := range tlvs {
		off = t.WriteTo(buf, off)
	}
	return off
}

// tlvsEncodedLen returns the total encoded size of a TLV slice.
func tlvsEncodedLen(tlvs []TLV) int {
	n := 0
	for _, t := range tlvs {
		n += t.EncodedLen()
	}
	return n
}

// AuthTLVIndex returns the index of the first Authentication TLV (type 10) in
// the slice, or -1 if absent. RFC 5304 sec 1 requires TLV 10 to be the first
// TLV when present; isis-10 uses this to enforce ordering on receive. (The
// codec itself does not reject misordered auth TLVs; it only surfaces the
// position, per the spec's separation of codec from enforcement.)
func AuthTLVIndex(tlvs []TLV) int {
	for i, t := range tlvs {
		if t.Type == TLVAuthentication {
			return i
		}
	}
	return -1
}
