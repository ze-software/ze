// Design: docs/architecture/wire/attributes.md — path attribute encoding

package attribute

import "encoding/binary"

// ASPathIterator iterates over AS_PATH segments.
// Zero-allocation: returns views into the underlying buffer.
//
// Wire format (RFC 4271 Section 5.1.2):
//
//	+---------------------------+
//	|  Segment Type (1 octet)   |  <- 1=AS_SET, 2=AS_SEQUENCE
//	+---------------------------+
//	|  Segment Length (1 octet) |  <- number of ASNs in segment
//	+---------------------------+
//	|  ASN 1 (2 or 4 octets)    |
//	+---------------------------+
//	|  ASN 2 (2 or 4 octets)    |
//	+---------------------------+
//	|  ...                      |
//	+---------------------------+
//
// Example usage:
//
//	iter := NewASPathIterator(data, true)  // asn4=true for 4-byte ASNs
//	for segType, asns, ok := iter.Next(); ok; segType, asns, ok = iter.Next() {
//	    asnIter := NewASNIterator(asns, true)
//	    for asn, ok := asnIter.Next(); ok; asn, ok = asnIter.Next() {
//	        fmt.Printf("ASN: %d\n", asn)
//	    }
//	}
type ASPathIterator struct {
	data   []byte
	offset int
	asn4   bool // true for 4-byte ASNs, false for 2-byte
}

// NewASPathIterator creates an iterator over AS_PATH attribute value.
// Set asn4=true when 4-byte AS capability is negotiated (RFC 6793).
//
// The iterator does not copy data - it returns views into the original buffer.
// Caller must not modify the buffer while iterating.
func NewASPathIterator(data []byte, asn4 bool) *ASPathIterator {
	return &ASPathIterator{
		data:   data,
		offset: 0,
		asn4:   asn4,
	}
}

// Next returns the next AS_PATH segment.
//
// Returns:
//   - segType: segment type (AS_SET=1, AS_SEQUENCE=2, etc.)
//   - asns: raw ASN bytes, view into buffer
//   - ok: false when iteration is complete
//
// Use NewASNIterator on asns to iterate individual ASNs.
func (it *ASPathIterator) Next() (segType ASPathSegmentType, asns []byte, ok bool) {
	if it.offset >= len(it.data) {
		return 0, nil, false
	}

	// Need at least 2 bytes for segment header
	if it.offset+2 > len(it.data) {
		return 0, nil, false // malformed
	}

	segType = ASPathSegmentType(it.data[it.offset])
	segLen := int(it.data[it.offset+1]) // number of ASNs
	it.offset += 2

	// Calculate bytes for ASNs
	asnSize := 4
	if !it.asn4 {
		asnSize = 2
	}
	asnBytes := segLen * asnSize

	if it.offset+asnBytes > len(it.data) {
		return 0, nil, false // malformed
	}

	asns = it.data[it.offset : it.offset+asnBytes]
	it.offset += asnBytes

	return segType, asns, true
}

// Reset resets the iterator to the beginning.
func (it *ASPathIterator) Reset() {
	it.offset = 0
}

// Count returns the total number of segments without consuming the iterator.
// Resets the iterator position after counting.
func (it *ASPathIterator) Count() int {
	savedOffset := it.offset
	it.offset = 0

	count := 0
	for _, _, ok := it.Next(); ok; _, _, ok = it.Next() {
		count++
	}

	it.offset = savedOffset
	return count
}

// ASNIterator iterates over ASNs within a segment.
// Zero-allocation: parses ASNs directly from buffer.
type ASNIterator struct {
	data   []byte
	offset int
	asn4   bool
}

// NewASNIterator creates an iterator over ASN bytes.
// Use with the asns bytes returned from ASPathIterator.Next().
func NewASNIterator(data []byte, asn4 bool) *ASNIterator {
	return &ASNIterator{
		data:   data,
		offset: 0,
		asn4:   asn4,
	}
}

// Next returns the next ASN.
//
// Returns:
//   - asn: the AS number
//   - ok: false when iteration is complete
func (it *ASNIterator) Next() (asn uint32, ok bool) {
	asnSize := 4
	if !it.asn4 {
		asnSize = 2
	}

	if it.offset+asnSize > len(it.data) {
		return 0, false
	}

	if it.asn4 {
		asn = binary.BigEndian.Uint32(it.data[it.offset:])
	} else {
		asn = uint32(binary.BigEndian.Uint16(it.data[it.offset:]))
	}
	it.offset += asnSize

	return asn, true
}

// Reset resets the iterator to the beginning.
func (it *ASNIterator) Reset() {
	it.offset = 0
}

// Count returns the total number of ASNs.
func (it *ASNIterator) Count() int {
	asnSize := 4
	if !it.asn4 {
		asnSize = 2
	}
	return len(it.data) / asnSize
}
