package attribute

import (
	"encoding/binary"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp"
)

// AttrIterator iterates over concatenated path attribute wire bytes.
// Zero-allocation: returns Span views into the underlying buffer.
//
// Wire format (RFC 4271 Section 4.3):
//
//	+---------------------------+
//	|   Flags (1 octet)         |
//	+---------------------------+
//	|   Type Code (1 octet)     |
//	+---------------------------+
//	|   Length (1 or 2 octets)  |  <- 2 octets if Extended Length flag set
//	+---------------------------+
//	|   Value (variable)        |
//	+---------------------------+
//
// Example usage:
//
//	iter := NewAttrIterator(data)
//	for typeCode, flags, value, ok := iter.Next(); ok; typeCode, flags, value, ok = iter.Next() {
//	    valueBytes := value.Slice(data)
//	    // process attribute
//	}
type AttrIterator struct {
	data   []byte
	offset int
}

// NewAttrIterator creates an iterator over path attribute wire bytes.
//
// The iterator does not copy data - it returns Span views into the original buffer.
// Caller must not modify the buffer while iterating.
func NewAttrIterator(data []byte) *AttrIterator {
	return &AttrIterator{
		data:   data,
		offset: 0,
	}
}

// Next returns the next attribute.
//
// Returns:
//   - typeCode: attribute type code (ORIGIN=1, AS_PATH=2, etc.)
//   - flags: attribute flags byte
//   - value: Span pointing to attribute value in the buffer
//   - ok: false when iteration is complete or on malformed data
//
// The value Span references the original buffer. Use value.Slice(data) to get bytes.
func (it *AttrIterator) Next() (typeCode AttributeCode, flags AttributeFlags, value bgp.Span, ok bool) {
	if it.offset >= len(it.data) {
		return 0, 0, bgp.Span{}, false
	}

	// Need at least 3 bytes for header (flags + code + 1-byte length)
	if it.offset+3 > len(it.data) {
		return 0, 0, bgp.Span{}, false
	}

	flags = AttributeFlags(it.data[it.offset])
	typeCode = AttributeCode(it.data[it.offset+1])

	// RFC 4271: Extended Length flag (bit 4) means 2-byte length
	var length int
	var hdrLen int
	if flags&FlagExtLength != 0 {
		if it.offset+4 > len(it.data) {
			return 0, 0, bgp.Span{}, false // malformed
		}
		length = int(binary.BigEndian.Uint16(it.data[it.offset+2:]))
		hdrLen = 4
	} else {
		length = int(it.data[it.offset+2])
		hdrLen = 3
	}

	// Validate we have enough data
	valueStart := it.offset + hdrLen
	if valueStart+length > len(it.data) {
		return 0, 0, bgp.Span{}, false // malformed
	}

	value = bgp.Span{Start: valueStart, Len: length}
	it.offset = valueStart + length

	return typeCode, flags, value, true
}

// Reset resets the iterator to the beginning.
func (it *AttrIterator) Reset() {
	it.offset = 0
}

// Find searches for an attribute by type code.
// Returns the value span and true if found, or empty span and false if not found.
// Consumes the iterator up to (and including) the found attribute.
func (it *AttrIterator) Find(code AttributeCode) (bgp.Span, bool) {
	for typeCode, _, value, ok := it.Next(); ok; typeCode, _, value, ok = it.Next() {
		if typeCode == code {
			return value, true
		}
	}
	return bgp.Span{}, false
}

// Count returns the total number of attributes without consuming the iterator.
// Resets the iterator position after counting.
func (it *AttrIterator) Count() int {
	savedOffset := it.offset
	it.offset = 0

	count := 0
	for _, _, _, ok := it.Next(); ok; _, _, _, ok = it.Next() {
		count++
	}

	it.offset = savedOffset
	return count
}

// Remaining returns the number of bytes not yet consumed.
func (it *AttrIterator) Remaining() int {
	return len(it.data) - it.offset
}

// Offset returns the current position in the buffer.
func (it *AttrIterator) Offset() int {
	return it.offset
}
