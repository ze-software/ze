// Design: docs/architecture/zefs-format.md -- netcapstring encoding
// Overview: store.go -- BlobStore uses netcapstrings for disk framing

package zefs

import (
	"fmt"
	"strconv"
)

// maxNumberWidth limits the number field to prevent pathological inputs.
// 19 digits covers the full range of int64.
const maxNumberWidth = 19

// writeNetcapstringHeader writes the header <number>:<cap>:<used>\n into buf at off.
// Capacity first, then dataLen (derived from data). Returns bytes written.
// Caller must ensure buf has sufficient space.
func writeNetcapstringHeader(buf []byte, off, capacity, dataLen int) int {
	start := off
	number := digitCount(capacity)
	numberStr := strconv.Itoa(number)

	off += copy(buf[off:], numberStr)
	buf[off] = ':'
	off++
	off += writeZeroPadded(buf[off:], capacity, number)
	buf[off] = ':'
	off++
	off += writeZeroPadded(buf[off:], dataLen, number)
	buf[off] = '\n'
	off++

	return off - start
}

// writeNetcapstring writes a complete netcapstring into buf at off.
// Format: <number>:<cap>:<used>\n<data><space-padding>\n
// Padding is space-filled. Trailing '\n' is the section terminator.
// The container's terminator is overwritten to ',' by the caller.
// Caller must ensure buf has sufficient space.
func writeNetcapstring(buf []byte, off int, data []byte, capacity int) int {
	start := off
	off += writeNetcapstringHeader(buf, off, capacity, len(data))
	off += copy(buf[off:], data)

	// Space-fill remaining padding.
	padding := capacity - len(data)
	for i := range padding {
		buf[off+i] = ' '
	}
	off += padding

	buf[off] = '\n' // section terminator
	off++

	return off - start
}

// netcapstringTotalLen returns the total on-disk size of a netcapstring (header + capacity + terminator).
func netcapstringTotalLen(capacity int) int {
	return netcapstringHeaderLen(capacity) + capacity + 1
}

// encodeNetcapstring allocates a buffer and writes a netcapstring into it.
// Convenience wrapper around writeNetcapstring for callers that need standalone bytes.
func encodeNetcapstring(data []byte, capacity int) ([]byte, error) {
	if capacity < 0 {
		return nil, fmt.Errorf("zefs: negative capacity: %d", capacity)
	}
	if len(data) > capacity {
		return nil, fmt.Errorf("zefs: data length %d exceeds capacity %d", len(data), capacity)
	}
	buf := make([]byte, netcapstringTotalLen(capacity))
	writeNetcapstring(buf, 0, data, capacity)
	return buf, nil
}

// decodeNetcapstring reads a netcapstring at the given offset, returning a copy.
// This is the safe-copy variant of decodeNetcapstringRef (which returns sub-slices
// of the input buffer). Used by tests to verify round-trip correctness.
func decodeNetcapstring(buf []byte, off int) (data []byte, capacity, next int, err error) {
	ref, cap_, next, err := decodeNetcapstringRef(buf, off)
	if err != nil {
		return nil, 0, 0, err
	}
	result := make([]byte, len(ref))
	copy(result, ref)
	return result, cap_, next, nil
}

// decodeNetcapstringRef reads a netcapstring at the given offset without copying.
// The returned data is a sub-slice of buf and shares its backing array.
// Callers must not modify the returned data.
func decodeNetcapstringRef(buf []byte, off int) (data []byte, capacity, next int, err error) {
	start := off

	if off >= len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: unexpected end of buffer at offset %d", start)
	}

	// Scan for number field (digits until next ':')
	numStart := off
	for off < len(buf) && buf[off] != ':' {
		off++
	}
	if off >= len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: unterminated number field at offset %d", start)
	}
	number, err := strconv.Atoi(string(buf[numStart:off]))
	if err != nil || number <= 0 || number > maxNumberWidth {
		return nil, 0, 0, fmt.Errorf("zefs: invalid number field at offset %d: %q", start, buf[numStart:off])
	}
	off++ // skip ':'

	// Read cap (number digits)
	if off+number > len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: truncated capacity at offset %d", start)
	}
	cap_, err := strconv.Atoi(string(buf[off : off+number]))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("zefs: invalid capacity at offset %d: %w", start, err)
	}
	off += number

	// Expect ':'
	if off >= len(buf) || buf[off] != ':' {
		return nil, 0, 0, fmt.Errorf("zefs: expected ':' after capacity at offset %d", start)
	}
	off++

	// Read used (number digits)
	if off+number > len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: truncated used at offset %d", start)
	}
	used, err := strconv.Atoi(string(buf[off : off+number]))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("zefs: invalid used at offset %d: %w", start, err)
	}
	off += number

	// Expect '\n'
	if off >= len(buf) || buf[off] != '\n' {
		return nil, 0, 0, fmt.Errorf("zefs: expected '\\n' after used at offset %d", start)
	}
	off++

	// Validate
	if cap_ < 0 {
		return nil, 0, 0, fmt.Errorf("zefs: negative capacity at offset %d: %d", start, cap_)
	}
	if used < 0 || used > cap_ {
		return nil, 0, 0, fmt.Errorf("zefs: used %d exceeds capacity %d at offset %d", used, cap_, start)
	}

	// Check data region + trailing colon fits in buffer (subtraction avoids int overflow on crafted cap_ values)
	if cap_+1 > len(buf)-off {
		return nil, 0, 0, fmt.Errorf("zefs: truncated data at offset %d: need %d, have %d", start, cap_+1, len(buf)-off)
	}

	// Expect trailing '\n'
	endOff := off + cap_
	if buf[endOff] != '\n' {
		return nil, 0, 0, fmt.Errorf("zefs: expected trailing '\\n' at offset %d, got 0x%02X", endOff, buf[endOff])
	}

	// Zero-copy: return sub-slice with capped length to prevent access to padding
	return buf[off : off+used : off+used], cap_, endOff + 1, nil
}

// writeZeroPadded writes n as a zero-padded decimal of the given width into buf.
// Returns width (number of bytes written).
func writeZeroPadded(buf []byte, n, width int) int {
	s := fmt.Sprintf("%0*d", width, n)
	copy(buf, s)
	return width
}

// digitCount returns the number of decimal digits needed to represent n.
func digitCount(n int) int {
	if n == 0 {
		return 1
	}
	count := 0
	v := n
	for v > 0 {
		count++
		v /= 10
	}
	return count
}

// netcapstringHeaderLen returns the header length for a netcapstring with the given capacity.
// Header format: number-colon-cap-colon-used-newline.
func netcapstringHeaderLen(capacity int) int {
	number := digitCount(capacity)
	numberWidth := digitCount(number)
	return 3 + numberWidth + 2*number
}

// netcapSlot describes a single netcapstring's on-disk layout.
// Tracks position, capacity, and current used length within the backing buffer.
type netcapSlot struct {
	offset   int // byte offset of the netcapstring header in the backing buffer
	capacity int // allocated data capacity (from header)
	used     int // current data length (from header, updated on writes)
}

// headerLen returns the header length for this slot.
func (s netcapSlot) headerLen() int {
	return netcapstringHeaderLen(s.capacity)
}

// totalLen returns the total on-disk size (header + capacity).
func (s netcapSlot) totalLen() int {
	return netcapstringTotalLen(s.capacity)
}

// dataOffset returns the byte offset where data starts in the buffer.
func (s netcapSlot) dataOffset() int {
	return s.offset + s.headerLen()
}

// data returns a zero-copy sub-slice of the used data from buf.
func (s netcapSlot) data(buf []byte) []byte {
	start := s.dataOffset()
	return buf[start : start+s.used : start+s.used]
}

// writeData writes data into this slot's position in buf.
// Updates used from len(data). Writes header, data, and space padding.
// Returns an error if len(data) exceeds capacity.
func (s *netcapSlot) writeData(buf, data []byte) error {
	if len(data) > s.capacity {
		return fmt.Errorf("zefs: writeData: data length %d exceeds slot capacity %d", len(data), s.capacity)
	}
	s.used = len(data)
	writeNetcapstring(buf, s.offset, data, s.capacity)
	return nil
}

// writeAt writes data at a local offset within the slot's data region.
// Updates used if the write extends past current used. Updates the header.
// Returns an error if localOff is negative, data is empty, or write extends past capacity.
func (s *netcapSlot) writeAt(buf []byte, localOff int, data []byte) error {
	if localOff < 0 {
		return fmt.Errorf("zefs: writeAt: negative offset %d", localOff)
	}
	if len(data) == 0 {
		return fmt.Errorf("zefs: writeAt: empty data")
	}
	end := localOff + len(data)
	if end > s.capacity {
		return fmt.Errorf("zefs: writeAt: write ends at %d, exceeds slot capacity %d", end, s.capacity)
	}
	start := s.dataOffset()
	copy(buf[start+localOff:], data)
	if end > s.used {
		s.used = end
		writeNetcapstringHeader(buf, s.offset, s.capacity, s.used)
	}
	return nil
}

// growCapacity returns a new capacity for data that outgrew currentCap.
// Adds 10% to dataLen so the entry has room to grow before the next reallocation.
func growCapacity(dataLen int) int {
	return dataLen + dataLen/10
}
