// Design: docs/architecture/zefs-format.md -- netcapstring encoding
// Overview: store.go -- BlobStore uses netcapstrings for disk framing

package zefs

import (
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strconv"
)

// CRC32cTable is the CRC32c (Castagnoli) table used for netcapstring checksums.
var CRC32cTable = crc32.MakeTable(crc32.Castagnoli)

// maxNumberWidth limits the number field to prevent pathological inputs.
// 19 digits covers the full range of int64.
const maxNumberWidth = 19

// writeNetcapstringHeader writes the header <number>:<cap>:<used>:<crc>\n into buf at off.
// Capacity first, then dataLen (derived from data), then CRC32c as 8-char lowercase hex.
// Returns bytes written. Caller must ensure buf has sufficient space.
func writeNetcapstringHeader(buf []byte, off, capacity, dataLen int, crc uint32) int {
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
	buf[off] = ':'
	off++
	off += writeCRC(buf[off:], crc)
	buf[off] = '\n'
	off++

	return off - start
}

// writeNetcapstring writes a complete netcapstring into buf at off.
// Format: <number>:<cap>:<used>:<crc>\n<data><space-padding>\n
// CRC32c is computed over the data bytes. Padding is space-filled.
// Trailing '\n' is the section terminator.
// Caller must ensure buf has sufficient space.
func writeNetcapstring(buf []byte, off int, data []byte, capacity int) int {
	start := off
	crc := crc32.Checksum(data, CRC32cTable)
	off += writeNetcapstringHeader(buf, off, capacity, len(data), crc)
	off += copy(buf[off:], data)

	padding := capacity - len(data)
	for i := range padding {
		buf[off+i] = ' '
	}
	off += padding

	buf[off] = '\n'
	off++

	return off - start
}

// netcapstringTotalLen returns the total on-disk size of a netcapstring (header + capacity + terminator).
func netcapstringTotalLen(capacity int) int {
	return netcapstringHeaderLen(capacity) + capacity + 1
}

// EncodeNetcapstring allocates a buffer and writes a netcapstring into it.
// Convenience wrapper around writeNetcapstring for callers that need standalone bytes.
func EncodeNetcapstring(data []byte, capacity int) ([]byte, error) {
	if capacity < 0 {
		return nil, fmt.Errorf("zefs: negative capacity: %d", capacity)
	}
	if len(data) > capacity {
		return nil, fmt.Errorf("zefs: data length %d exceeds capacity %d", len(data), capacity)
	}
	if capacity > int(^uint(0)>>1)-netcapstringHeaderLen(capacity)-1 {
		return nil, fmt.Errorf("zefs: encoded capacity %d overflows int", capacity)
	}
	buf := make([]byte, netcapstringTotalLen(capacity))
	writeNetcapstring(buf, 0, data, capacity)
	return buf, nil
}

// decodeNetcapstring reads a netcapstring at the given offset, returning a copy.
// This is the safe-copy variant of DecodeNetcapstringRef (which returns sub-slices
// of the input buffer). Used by tests to verify round-trip correctness.
func decodeNetcapstring(buf []byte, off int) (data []byte, capacity, next int, err error) {
	ref, cap_, next, err := DecodeNetcapstringRef(buf, off)
	if err != nil {
		return nil, 0, 0, err
	}
	result := make([]byte, len(ref))
	copy(result, ref)
	return result, cap_, next, nil
}

// DecodeNetcapstringRef reads and verifies one frame without copying its data.
// The result shares buf's backing array; callers MUST keep buf valid and MUST
// NOT modify it while using the result. A standalone frame caller MUST require
// next == len(buf); blob callers use next to decode the following frame.
func DecodeNetcapstringRef(buf []byte, off int) (data []byte, capacity, next int, err error) {
	start := off
	dataOff, capacity, used, headerCRC, err := netcapstringHeader(buf, off, true)
	if err != nil {
		return nil, 0, 0, err
	}
	// Reserve the terminator by comparison, never by adding to an untrusted cap.
	if capacity >= len(buf)-dataOff {
		return nil, 0, 0, fmt.Errorf("zefs: truncated data at offset %d: capacity %d, available %d", start, capacity, len(buf)-dataOff)
	}
	end := dataOff + capacity
	if buf[end] != '\n' {
		return nil, 0, 0, fmt.Errorf("zefs: expected trailing '\\n' at offset %d, got 0x%02X", end, buf[end])
	}
	data = buf[dataOff : dataOff+used : dataOff+used]
	actualCRC := crc32.Checksum(data, CRC32cTable)
	if actualCRC != headerCRC {
		return nil, 0, 0, fmt.Errorf("zefs: CRC mismatch at offset %d: header=%08x computed=%08x", start, headerCRC, actualCRC)
	}
	return data, capacity, end + 1, nil
}

// netcapstringHeader validates the header independently of the payload so repair
// can inspect a truncated container. Diagnostic callers may ignore invalid CRC
// text; DecodeNetcapstringRef always requires a valid checksum field.
func netcapstringHeader(buf []byte, off int, requireCRC bool) (dataOff, capacity, used int, checksum uint32, err error) {
	start := off
	if off < 0 {
		return 0, 0, 0, 0, fmt.Errorf("zefs: negative offset %d", off)
	}
	if off >= len(buf) {
		return 0, 0, 0, 0, fmt.Errorf("zefs: unexpected end of buffer at offset %d", off)
	}
	// Width has at most two digits (1..19). Bound the scan before parsing.
	for off < len(buf) {
		if buf[off] == ':' {
			break
		}
		if off-start == 2 {
			return 0, 0, 0, 0, fmt.Errorf("zefs: invalid number field at offset %d", start)
		}
		off++
	}
	if off == len(buf) {
		return 0, 0, 0, 0, fmt.Errorf("zefs: unterminated number field at offset %d", start)
	}
	width, parseErr := netcapstringDecimal(buf[start:off])
	if parseErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("zefs: invalid number field at offset %d: %w", start, parseErr)
	}
	if width < 1 {
		return 0, 0, 0, 0, fmt.Errorf("zefs: invalid number field at offset %d", start)
	}
	if width > maxNumberWidth {
		return 0, 0, 0, 0, fmt.Errorf("zefs: invalid number field at offset %d", start)
	}
	off++
	// width is now bounded, and subtraction protects off near MaxInt.
	if 2*width+11 > len(buf)-off {
		return 0, 0, 0, 0, fmt.Errorf("zefs: truncated header at offset %d", start)
	}
	capacity, parseErr = netcapstringDecimal(buf[off : off+width])
	if parseErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("zefs: invalid capacity at offset %d: %w", start, parseErr)
	}
	off += width
	if buf[off] != ':' {
		return 0, 0, 0, 0, fmt.Errorf("zefs: expected ':' after capacity at offset %d", start)
	}
	off++
	used, parseErr = netcapstringDecimal(buf[off : off+width])
	if parseErr != nil {
		return 0, 0, 0, 0, fmt.Errorf("zefs: invalid used at offset %d: %w", start, parseErr)
	}
	if used > capacity {
		return 0, 0, 0, 0, fmt.Errorf("zefs: used %d exceeds capacity %d at offset %d", used, capacity, start)
	}
	off += width
	if buf[off] != ':' {
		return 0, 0, 0, 0, fmt.Errorf("zefs: expected ':' after used at offset %d", start)
	}
	off++
	var decoded [4]byte
	if _, parseErr := hex.Decode(decoded[:], buf[off:off+8]); parseErr != nil {
		if requireCRC {
			return 0, 0, 0, 0, fmt.Errorf("zefs: invalid CRC hex at offset %d: %w", start, parseErr)
		}
	}
	checksum = uint32(decoded[0])<<24 | uint32(decoded[1])<<16 | uint32(decoded[2])<<8 | uint32(decoded[3])
	off += 8
	if buf[off] != '\n' {
		return 0, 0, 0, 0, fmt.Errorf("zefs: expected '\\n' after CRC at offset %d", start)
	}
	return off + 1, capacity, used, checksum, nil
}

// netcapstringDecimal accepts decimal digits only and checks before multiplying.
func netcapstringDecimal(buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, fmt.Errorf("empty decimal")
	}
	value := 0
	for _, c := range buf {
		if c < '0' {
			return 0, fmt.Errorf("invalid decimal digit")
		}
		if c > '9' {
			return 0, fmt.Errorf("invalid decimal digit")
		}
		digit := int(c - '0')
		if value > (int(^uint(0)>>1)-digit)/10 {
			return 0, fmt.Errorf("decimal overflows int")
		}
		value = value*10 + digit
	}
	return value, nil
}

// writeZeroPadded writes n as a zero-padded decimal of the given width into buf.
// Returns width (number of bytes written).
func writeZeroPadded(buf []byte, n, width int) int {
	s := fmt.Sprintf("%0*d", width, n)
	copy(buf, s)
	return width
}

// writeCRC writes a CRC32c value as 8-char zero-padded lowercase hex into buf.
// Returns 8 (number of bytes written).
func writeCRC(buf []byte, crc uint32) int {
	var b [4]byte
	b[0] = byte(crc >> 24)
	b[1] = byte(crc >> 16)
	b[2] = byte(crc >> 8)
	b[3] = byte(crc)
	hex.Encode(buf[:8], b[:])
	return 8
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
// Header format: number-colon-cap-colon-used-colon-crc-newline.
func netcapstringHeaderLen(capacity int) int {
	number := digitCount(capacity)
	numberWidth := digitCount(number)
	return 3 + numberWidth + 2*number + 1 + 8
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
	}
	dataCRC := crc32.Checksum(buf[start:start+s.used], CRC32cTable)
	writeNetcapstringHeader(buf, s.offset, s.capacity, s.used, dataCRC)
	return nil
}

// growCapacity returns a new capacity for data that outgrew currentCap.
// Adds 10% to dataLen so the entry has room to grow before the next reallocation.
func growCapacity(dataLen int) int {
	return dataLen + dataLen/10
}
