// Design: docs/architecture/wire/attributes.md — path attribute encoding
// RFC: rfc/short/rfc4271.md — path attribute TLV iteration (Section 4.3)

package attribute

import (
	"encoding/binary"
)

// AttrIterator iterates over concatenated path attribute wire bytes.
// Zero-allocation: returns []byte views into the underlying buffer.
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
//	    // value is []byte - use directly
//	    // process attribute
//	}
type AttrIterator struct {
	data   []byte
	offset int
}

// NewAttrIterator creates an iterator over path attribute wire bytes.
//
// The iterator does not copy data - it returns []byte slices into the original buffer.
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
//   - value: []byte slice of attribute value in the buffer
//   - ok: false when iteration is complete or on malformed data
//
// The value is a view into the original buffer - do not modify.
func (it *AttrIterator) Next() (typeCode AttributeCode, flags AttributeFlags, value []byte, ok bool) {
	if it.offset >= len(it.data) {
		return 0, 0, nil, false
	}

	// Need at least 3 bytes for header (flags + code + 1-byte length)
	if it.offset+3 > len(it.data) {
		return 0, 0, nil, false
	}

	flags = AttributeFlags(it.data[it.offset])
	typeCode = AttributeCode(it.data[it.offset+1])

	// RFC 4271: Extended Length flag (bit 4) means 2-byte length
	var length int
	var hdrLen int
	if flags&FlagExtLength != 0 {
		if it.offset+4 > len(it.data) {
			return 0, 0, nil, false // malformed
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
		return 0, 0, nil, false // malformed
	}

	value = it.data[valueStart : valueStart+length]
	it.offset = valueStart + length

	return typeCode, flags, value, true
}

// Reset resets the iterator to the beginning.
func (it *AttrIterator) Reset() {
	it.offset = 0
}

// Find searches for an attribute by type code.
// Returns the value bytes and true if found, or nil and false if not found.
// Consumes the iterator up to (and including) the found attribute.
func (it *AttrIterator) Find(code AttributeCode) ([]byte, bool) {
	for typeCode, _, value, ok := it.Next(); ok; typeCode, _, value, ok = it.Next() {
		if typeCode == code {
			return value, true
		}
	}
	return nil, false
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
