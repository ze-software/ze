// Design: docs/architecture/ospf/ospf-ext-1-opaque-framework.md -- generic opaque-LSA TLV carriage.
// RFC: rfc/short/rfc5250.md -- opaque LSA carrier and the 4-byte-aligned TLV convention.
// RFC 5250 is the opaque carrier; the TLV shape (2-byte type, 2-byte length, value,
// pad to a 4-byte boundary) is the convention every opaque consumer body reuses (RFC
// 3630 sec 2.3.1 TE TLVs, RFC 7770 RI TLVs, RFC 7684 extended TLVs). The framework
// carries TLVs but interprets NO type: this file is a library for the consumers
// (ext-2/3/4/9), never called by the carrier itself.

package packet

// OpaqueTLVHeaderLen is the fixed TLV header: a 2-byte Type plus a 2-byte Length.
const OpaqueTLVHeaderLen = 4

// alignOpaqueTLV rounds a value length up to the next 4-byte boundary. TLV values are
// padded to a 4-byte boundary so the following TLV starts aligned; the pad bytes are
// not counted in the TLV Length field.
func alignOpaqueTLV(n int) int { return (n + 3) &^ 3 }

// opaqueTLV is one type-length-value triple inside an opaque LSA body. Value is the raw
// value bytes without the 4-byte alignment padding.
type opaqueTLV struct {
	Type  uint16
	Value []byte
}

// EncodedLen returns the on-wire length of the TLV: the 4-byte header plus the value
// padded to a 4-byte boundary.
func (t opaqueTLV) EncodedLen() int { return OpaqueTLVHeaderLen + alignOpaqueTLV(len(t.Value)) }

// WriteTo writes the TLV into buf at off (buffer-first, no allocation): the 2-byte
// Type, the 2-byte Length (the unpadded value length), the value, then explicit
// zero pad bytes to the next 4-byte boundary. It returns the offset past the pad.
// The caller owns buf and must size it for EncodedLen (use opaqueTLVsLen for a set).
func (t opaqueTLV) WriteTo(buf []byte, off int) int {
	off += writeUint16(buf, off, t.Type)
	off += writeUint16(buf, off, uint16(len(t.Value)))
	copy(buf[off:off+len(t.Value)], t.Value)
	off += len(t.Value)
	for pad := alignOpaqueTLV(len(t.Value)) - len(t.Value); pad > 0; pad-- {
		buf[off] = 0
		off++
	}
	return off
}

// opaqueTLVsLen returns the total on-wire length of a TLV set (each 4-byte aligned).
func opaqueTLVsLen(tlvs []opaqueTLV) int {
	n := 0
	for _, t := range tlvs {
		n += t.EncodedLen()
	}
	return n
}

// writeOpaqueTLVs writes a TLV set into buf from offset 0 in order and returns the new
// offset. The caller sizes buf with opaqueTLVsLen; sub-TLV regions are built into their own
// buffer and then carried as a value, so this always starts at 0 (WriteTo carries the
// running offset within a set).
func writeOpaqueTLVs(buf []byte, tlvs []opaqueTLV) int {
	off := 0
	for _, t := range tlvs {
		off = t.WriteTo(buf, off)
	}
	return off
}

// opaqueTLVIterator walks a TLV region without allocating. Value() returns a view into
// the caller's bytes (zero-copy). It is bound-checked and NEVER panics on malformed
// input; Err reports a truncated header, a length that runs past the region, or a
// truncated pad (R-8).
type opaqueTLVIterator struct {
	data []byte
	off  int
	typ  uint16
	val  []byte
	err  error
}

// newOpaqueTLVIterator returns an iterator over an opaque LSA body (the bytes after
// the 20-byte LSA header).
func newOpaqueTLVIterator(body []byte) opaqueTLVIterator { return opaqueTLVIterator{data: body} }

// Next advances to the next TLV. It returns false at the end of the region or on the
// first malformed TLV (Err is then set).
func (it *opaqueTLVIterator) Next() bool {
	if it.err != nil || it.off >= len(it.data) {
		return false
	}
	if len(it.data)-it.off < OpaqueTLVHeaderLen {
		it.err = ErrTruncated
		return false
	}
	typ := readUint16(it.data, it.off)
	length := int(readUint16(it.data, it.off+2))
	valStart := it.off + OpaqueTLVHeaderLen
	if valStart+length > len(it.data) {
		it.err = ErrLength
		return false
	}
	padded := alignOpaqueTLV(length)
	if valStart+padded > len(it.data) {
		// The value fits but its 4-byte pad runs past the region: the last TLV in a
		// correctly built body is always padded, so a missing pad is malformed input.
		it.err = ErrTruncated
		return false
	}
	it.typ = typ
	it.val = it.data[valStart : valStart+length]
	it.off = valStart + padded
	return true
}

// OpaqueTLVView is one decoded TLV (type, declared value length, zero-copy value view)
// from a generic opaque LSA body. It is the ext-14 debug decode fallback (RFC 5250 Section
// 3): when no typed consumer decoder is registered for an Opaque Type, the body renders as
// this generic (type, length, value-hex) stream. Value is a view into the caller's bytes.
type OpaqueTLVView struct {
	Type   uint16
	Length int
	Value  []byte
}

// DecodeOpaqueTLVs walks an opaque LSA body as a generic (type, length, value) TLV stream
// (RFC 5250: the 4-byte-aligned opaque TLV convention). It is bound-checked and NEVER
// panics on a malformed body; it returns the TLVs decoded before the fault plus the
// iterator error, so a caller can render the good prefix and note the truncation (AC-3).
func DecodeOpaqueTLVs(body []byte) ([]OpaqueTLVView, error) {
	it := newOpaqueTLVIterator(body)
	var out []OpaqueTLVView
	for it.Next() {
		v := it.Value()
		out = append(out, OpaqueTLVView{Type: it.Type(), Length: len(v), Value: v})
	}
	return out, it.Err()
}

// Type returns the current TLV type.
func (it *opaqueTLVIterator) Type() uint16 { return it.typ }

// Value returns the current TLV value as a zero-copy view into the caller's bytes.
func (it *opaqueTLVIterator) Value() []byte { return it.val }

// Err returns the first malformed-TLV error, if any.
func (it *opaqueTLVIterator) Err() error { return it.err }
