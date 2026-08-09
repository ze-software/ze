// Design: docs/architecture/ospf/ospf-ext-9-graceful-restart.md -- OSPFv3 Grace-LSA tlv carriage.
// RFC: rfc/short/rfc5187.md (§2.2 grace-LSA TLVs use the RFC 3630 §2.3.2 format),
// rfc/short/rfc3630.md (§2.3.2 the 4-octet-aligned Type/Length/Value convention).
//
// OSPFv3 has no opaque-LSA carrier, so the Grace-LSA (RFC 5187 §2.2) carries its TLVs
// natively. The wire shape (2-byte Type, 2-byte Length, value, pad to a 4-byte boundary,
// Length excluding the pad) is identical to the OSPFv2 opaque tlv convention, but the
// codec is re-implemented here rather than importing the v2 packet helpers: the v3 codec
// never depends on the OSPFv2 wire package (docs/architecture/ospf/ospf-af-unify.md -- the
// AF-specific wire lives entirely under v3/). This mirrors v3/packet/checksum.go, which
// re-derives the Fletcher
// checksum for the same reason.

package packet

// tlvHeaderLen is the fixed tlv header: a 2-byte Type plus a 2-byte Length.
const tlvHeaderLen = 4

// alignTLV rounds a value length up to the next 4-byte boundary. tlv values are padded to
// a 4-byte boundary so the following tlv starts aligned; the pad bytes are not counted in
// the tlv Length field (RFC 3630 §2.3.2).
func alignTLV(n int) int { return (n + 3) &^ 3 }

// tlv is one type-length-value triple inside an OSPFv3 Grace-LSA body. Value is the raw
// value bytes without the 4-byte alignment padding.
type tlv struct {
	Type  uint16
	Value []byte
}

// EncodedLen returns the on-wire length of the tlv: the 4-byte header plus the value padded
// to a 4-byte boundary.
func (t tlv) EncodedLen() int { return tlvHeaderLen + alignTLV(len(t.Value)) }

// WriteTo writes the tlv into buf at off (buffer-first, no allocation): the 2-byte Type, the
// 2-byte Length (the unpadded value length), the value, then explicit zero pad bytes to the
// next 4-byte boundary. It returns the offset past the pad. The caller owns buf and must
// size it for EncodedLen (use tlvsEncodedLen for a set).
func (t tlv) WriteTo(buf []byte, off int) int {
	off += writeUint16(buf, off, t.Type)
	off += writeUint16(buf, off, uint16(len(t.Value)))
	copy(buf[off:off+len(t.Value)], t.Value)
	off += len(t.Value)
	for pad := alignTLV(len(t.Value)) - len(t.Value); pad > 0; pad-- {
		buf[off] = 0
		off++
	}
	return off
}

// tlvsEncodedLen returns the total on-wire length of a tlv set (each 4-byte aligned).
func tlvsEncodedLen(tlvs []tlv) int {
	n := 0
	for _, t := range tlvs {
		n += t.EncodedLen()
	}
	return n
}

// writeTLVs writes a tlv set into buf from offset 0 in order and returns the total length.
// The caller sizes buf with tlvsEncodedLen.
func writeTLVs(buf []byte, tlvs []tlv) int {
	off := 0
	for _, t := range tlvs {
		off = t.WriteTo(buf, off)
	}
	return off
}

// tlvIterator walks a tlv region without allocating. Value() returns a view into the
// caller's bytes (zero-copy). It is bound-checked and NEVER panics on malformed input; Err
// reports a truncated header or a length that runs past the region. A FINAL tlv whose value
// fits but whose 4-octet pad is missing is TOLERATED on receive (RFC "be liberal in what you
// accept"), matching the OSPFv2 SR sub-TLV iterator in internal/plugins/ospf/sr/codec.go so
// both address families accept the same wire.
type tlvIterator struct {
	data []byte
	off  int
	typ  uint16
	val  []byte
	err  error
}

// newTLVIterator returns an iterator over a Grace-LSA body (the bytes after the 20-byte LSA
// header).
func newTLVIterator(body []byte) tlvIterator { return tlvIterator{data: body} }

// Next advances to the next tlv. It returns false at the end of the region or on the first
// malformed tlv (Err is then set).
func (it *tlvIterator) Next() bool {
	if it.err != nil || it.off >= len(it.data) {
		return false
	}
	if len(it.data)-it.off < tlvHeaderLen {
		it.err = ErrTruncated
		return false
	}
	typ := readUint16(it.data, it.off)
	length := int(readUint16(it.data, it.off+2))
	valStart := it.off + tlvHeaderLen
	if valStart+length > len(it.data) {
		it.err = ErrLength
		return false
	}
	it.typ = typ
	it.val = it.data[valStart : valStart+length]
	// The value fits; advance past its 4-byte pad. A missing pad on the FINAL tlv (the pad
	// would run past the region) is tolerated -- be liberal on receive, consistent with the
	// OSPFv2 SR sub-TLV iterator -- by clamping to end-of-region so the next Next() stops
	// cleanly.
	it.off = min(valStart+alignTLV(length), len(it.data))
	return true
}

// Type returns the current tlv type.
func (it *tlvIterator) Type() uint16 { return it.typ }

// Value returns the current tlv value as a zero-copy view into the caller's bytes.
func (it *tlvIterator) Value() []byte { return it.val }

// Err returns the first malformed-tlv error, if any.
func (it *tlvIterator) Err() error { return it.err }
