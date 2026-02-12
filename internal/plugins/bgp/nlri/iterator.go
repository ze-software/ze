package nlri

import "encoding/binary"

// NLRIIterator iterates over concatenated NLRI wire bytes.
// Zero-allocation: returns views into the underlying buffer.
//
// Wire format (RFC 4271 Section 4.3):
//
//	+---------------------------+
//	|   Length (1 octet)        |  <- prefix length in bits
//	+---------------------------+
//	|   Prefix (variable)       |  <- minimum octets for prefix
//	+---------------------------+
//
// With ADD-PATH (RFC 7911 Section 3):
//
//	+---------------------------+
//	|   Path ID (4 octets)      |
//	+---------------------------+
//	|   Length (1 octet)        |
//	+---------------------------+
//	|   Prefix (variable)       |
//	+---------------------------+
//
// Example usage:
//
//	iter := NewNLRIIterator(data, false)
//	for prefix, pathID, ok := iter.Next(); ok; prefix, pathID, ok = iter.Next() {
//	    // prefix is a view into data, do not modify
//	    fmt.Printf("prefix bytes: %v, path-id: %d\n", prefix, pathID)
//	}
type NLRIIterator struct {
	data    []byte
	offset  int
	addPath bool
}

// NewNLRIIterator creates an iterator over NLRI wire bytes.
// Set addPath=true when ADD-PATH is negotiated (RFC 7911).
//
// The iterator does not copy data - it returns views into the original buffer.
// Caller must not modify the buffer while iterating.
func NewNLRIIterator(data []byte, addPath bool) *NLRIIterator {
	return &NLRIIterator{
		data:    data,
		offset:  0,
		addPath: addPath,
	}
}

// Next returns the next NLRI.
//
// Returns:
//   - prefix: raw NLRI bytes (length byte + prefix bytes), view into buffer
//   - pathID: ADD-PATH path identifier (0 if addPath=false)
//   - ok: false when iteration is complete
//
// The returned prefix slice is a view into the original buffer.
// Do not modify it; copy if mutation is needed.
func (it *NLRIIterator) Next() (prefix []byte, pathID uint32, ok bool) {
	if it.offset >= len(it.data) {
		return nil, 0, false
	}

	start := it.offset

	// RFC 7911: Extract path identifier if ADD-PATH negotiated
	if it.addPath {
		if it.offset+4 > len(it.data) {
			return nil, 0, false // malformed
		}
		pathID = binary.BigEndian.Uint32(it.data[it.offset:])
		it.offset += 4
		start = it.offset // prefix starts after path-id
	}

	// RFC 4271: Extract prefix length (bits)
	if it.offset >= len(it.data) {
		return nil, 0, false // malformed
	}
	prefixLenBits := int(it.data[it.offset])
	prefixBytes := PrefixBytes(prefixLenBits)

	// Calculate total NLRI size: 1 (length byte) + prefix bytes
	nlriLen := 1 + prefixBytes
	if it.offset+nlriLen > len(it.data) {
		return nil, 0, false // malformed
	}

	prefix = it.data[start : start+nlriLen]
	it.offset += nlriLen

	return prefix, pathID, true
}

// Reset resets the iterator to the beginning.
func (it *NLRIIterator) Reset() {
	it.offset = 0
}

// Count returns the total number of NLRIs without consuming the iterator.
// Resets the iterator position after counting.
func (it *NLRIIterator) Count() int {
	savedOffset := it.offset
	it.offset = 0

	count := 0
	for _, _, ok := it.Next(); ok; _, _, ok = it.Next() {
		count++
	}

	it.offset = savedOffset
	return count
}

// Remaining returns the number of bytes not yet consumed.
func (it *NLRIIterator) Remaining() int {
	return len(it.data) - it.offset
}

// Offset returns the current position in the buffer.
func (it *NLRIIterator) Offset() int {
	return it.offset
}
