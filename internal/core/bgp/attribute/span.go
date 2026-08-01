// Design: docs/architecture/wire/attributes.md — path attribute span index
// RFC: rfc/short/rfc4271.md — path attribute wire format (Section 4.3)
// RFC: rfc/short/rfc8654.md — extended message body ceiling
// Overview: wire.go — AttributesWire, the immutable base that carries this index
// Related: iterator.go — AttrFind, the index-free single-code scan

package attribute

import (
	"errors"
	"fmt"
)

// SpanInline is the number of attribute spans a SpanIndex holds without a heap
// allocation. It carries forward the pre-size judgement the lazy builder made
// (make([]attrIndex, 0, 8)); only the constant would change if a traffic
// histogram said otherwise.
const SpanInline = 8

// spanMaxOffset is the largest value a Span offset or length can hold. RFC 8654
// caps an UPDATE body at 65516 octets, so the attribute section is always well
// inside this and the 16-bit fields cannot truncate.
const spanMaxOffset = 65535

// ErrAttrSectionTooLarge reports a path-attribute section that cannot be indexed
// with 16-bit offsets. It is unreachable from the wire (RFC 8654 bounds the body
// at 65516 octets) and exists so an in-process caller that hands over a larger
// buffer is refused rather than silently given truncated offsets.
var ErrAttrSectionTooLarge = errors.New("path attribute section exceeds 65535 octets")

// Span locates one path attribute inside a path-attribute section.
//
// Offset and Length describe the attribute VALUE. Offsets are relative to the
// start of the attribute section, not to the UPDATE payload, so a span stays
// valid against any byte array holding the same section contents — which is what
// lets WireUpdate.Snapshot carry an index across its copy without arithmetic.
//
// The flags byte of the attribute is at section[Offset-HdrLen]; it is derived
// rather than stored, because the header size class already implies where it is.
type Span struct {
	Offset uint16        // value start, relative to the attribute section
	Length uint16        // value length
	Code   AttributeCode // attribute type code (RFC 4271 Section 4.3)
	HdrLen uint8         // 3, or 4 when the Extended Length flag is set
}

// SpanIndex is the attribute index of one path-attribute section: one span per
// attribute in wire order, plus a presence bit per type code.
//
// It is built exactly once, by BuildSpanIndex, and is never written afterwards.
// Nothing in it points at the indexed bytes, so it inherits its base's buffer
// lifetime contract exactly and must never be published separately from the base
// (docs/architecture/memory/lifetime-contracts.md).
type SpanIndex struct {
	inline   [SpanInline]Span
	spill    []Span    // nil unless the section holds more than SpanInline attributes
	presence [4]uint64 // one bit per attribute type code, 0..255
	n        uint16
	spilled  bool
}

// BuildSpanIndex walks a path-attribute section once and returns its index.
//
// The verdicts are the ones RFC 4271 Section 4.3 requires and are unchanged from
// the lazy builder this replaced: a header that does not parse, a duplicate type
// code (Malformed Attribute List), and an attribute whose value runs past the end
// of the section are each an error, and an error yields no index at all.
func BuildSpanIndex(packed []byte) (SpanIndex, error) {
	var idx SpanIndex
	if err := idx.build(packed); err != nil {
		return SpanIndex{}, err
	}
	return idx, nil
}

// Rebuild fills the receiver in place, so a caller that indexes one section per
// destination can reuse one index rather than producing a fresh value each time.
//
// It is the same walk and the same verdicts as BuildSpanIndex; only the storage
// differs. The receiver is zeroed first, so a reused index never carries a span
// or a presence bit from the section it held before.
//
// Use it where the index is per-destination work on a fan-out path: returning by
// value there forces the index to the heap once per destination, which is an
// allocation on the path the exactly-sized rebuild exists to keep free.
func (x *SpanIndex) Rebuild(packed []byte) error { return x.build(packed) }

// build fills the receiver in place. It is the in-place form BuildSpanIndex wraps,
// used where the index already lives inside its owner so the inline array is never
// copied. The receiver is cleared first, so a reused one carries nothing forward,
// and on error it is left zero-valued: a partially built index must never be
// readable as "this UPDATE has these attributes".
func (x *SpanIndex) build(packed []byte) error {
	*x = SpanIndex{}

	if len(packed) > spanMaxOffset {
		return fmt.Errorf("%w: %d", ErrAttrSectionTooLarge, len(packed))
	}

	offset := 0
	for offset < len(packed) {
		_, code, length, hdrLen, err := ParseHeader(packed[offset:])
		if err != nil {
			*x = SpanIndex{}
			return fmt.Errorf("parsing header at offset %d: %w", offset, err)
		}

		// RFC 4271 Section 4.3: duplicate attributes are malformed. The presence
		// bitset is the same set the old builder kept in a separate [256]bool.
		if x.has(code) {
			*x = SpanIndex{}
			return fmt.Errorf("duplicate attribute %s at offset %d", code, offset)
		}

		if offset+hdrLen+int(length) > len(packed) {
			*x = SpanIndex{}
			return fmt.Errorf("attribute %s truncated at offset %d", code, offset)
		}

		x.mark(code)
		x.add(Span{
			Offset: uint16(offset + hdrLen), //nolint:gosec // G115: bounded by the spanMaxOffset check above
			Length: length,
			Code:   code,
			HdrLen: uint8(hdrLen), //nolint:gosec // G115: hdrLen is 3 or 4
		})

		offset += hdrLen + int(length)
	}
	return nil
}

// add appends one span, spilling to the heap past the inline capacity.
func (x *SpanIndex) add(s Span) {
	if int(x.n) < SpanInline {
		x.inline[x.n] = s
	} else {
		x.spill = append(x.spill, s)
		x.spilled = true
	}
	x.n++
}

func (x *SpanIndex) has(code AttributeCode) bool {
	return x.presence[code>>6]&(uint64(1)<<(code&63)) != 0
}

func (x *SpanIndex) mark(code AttributeCode) {
	x.presence[code>>6] |= uint64(1) << (code & 63)
}

// Len returns the number of attributes indexed.
func (x *SpanIndex) Len() int { return int(x.n) }

// Spilled reports whether the section held more attributes than the inline
// capacity, so an operator can see attacker-driven or unusual attribute counts.
func (x *SpanIndex) Spilled() bool { return x.spilled }

// Has reports whether an attribute with this type code is present, in constant
// time and without walking either the spans or the wire bytes.
func (x *SpanIndex) Has(code AttributeCode) bool { return x.has(code) }

// At returns the i-th span in wire order. i must be less than Len.
func (x *SpanIndex) At(i int) Span {
	if i < 0 || i >= int(x.n) {
		panic("BUG: SpanIndex.At index out of range")
	}
	if i < SpanInline {
		return x.inline[i]
	}
	return x.spill[i-SpanInline]
}

// Find returns the span for a type code and whether it is present.
// The presence bitset answers absence without touching a span.
func (x *SpanIndex) Find(code AttributeCode) (Span, bool) {
	i := x.findIndex(code)
	if i < 0 {
		return Span{}, false
	}
	return x.At(i), true
}

// findIndex returns the wire-order position of a type code, or -1 when absent.
// The presence bitset settles absence before any span is read.
func (x *SpanIndex) findIndex(code AttributeCode) int {
	if !x.has(code) {
		return -1
	}
	for i := range min(int(x.n), SpanInline) {
		if x.inline[i].Code == code {
			return i
		}
	}
	for i := range x.spill {
		if x.spill[i].Code == code {
			return SpanInline + i
		}
	}
	// Unreachable: build sets a presence bit only together with its span.
	return -1
}
