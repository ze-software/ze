// Design: (none -- predates documentation)
// Overview: store.go -- BlobStore uses netstrings for disk framing

package zefs

import (
	"fmt"
	"strconv"
)

// headerLen is the fixed-width netstring header size: "NNNNNNN:NNNNNNN:" = 16 bytes.
const headerLen = 16

// maxHeaderVal is the largest value representable in a 7-digit field.
const maxHeaderVal = 9_999_999

// encodeNetstring writes data into a netstring with fixed-width header and zero padding.
// Format: "UUUUUUU:CCCCCCC:<data><zero-padding>" where U=used, C=capacity.
// Returns an error if used or capacity exceeds the 7-digit header limit.
func encodeNetstring(data []byte, capacity int) ([]byte, error) {
	if len(data) > maxHeaderVal || capacity > maxHeaderVal {
		return nil, fmt.Errorf("zefs: value exceeds header limit (%d): used=%d, capacity=%d", maxHeaderVal, len(data), capacity)
	}
	buf := make([]byte, headerLen+capacity)
	writeHeader(buf, len(data), capacity)
	copy(buf[headerLen:], data)
	// remaining bytes are zero (Go initializes slices to zero)
	return buf, nil
}

// decodeNetstring reads a netstring at the given offset, returning a copy.
// This is the safe-copy variant of decodeNetstringRef (which returns sub-slices
// of the input buffer). Used by tests to verify round-trip correctness.
func decodeNetstring(buf []byte, off int) (data []byte, capacity, next int, err error) {
	if off+headerLen > len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: truncated header at offset %d", off)
	}

	used, cap_, err := parseHeader(buf[off : off+headerLen])
	if err != nil {
		return nil, 0, 0, err
	}

	dataStart := off + headerLen
	if dataStart+cap_ > len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: truncated data at offset %d: need %d, have %d", off, cap_, len(buf)-dataStart)
	}

	// Return a copy of the used portion so caller doesn't hold the backing array
	result := make([]byte, used)
	copy(result, buf[dataStart:dataStart+used])

	return result, cap_, dataStart + cap_, nil
}

// writeHeader writes "UUUUUUU:CCCCCCC:" into buf[:headerLen].
func writeHeader(buf []byte, used, capacity int) {
	h := fmt.Sprintf("%07d:%07d:", used, capacity)
	copy(buf, h)
}

// parseHeader reads "UUUUUUU:CCCCCCC:" and returns used and capacity.
func parseHeader(hdr []byte) (used, capacity int, err error) {
	if len(hdr) < headerLen {
		return 0, 0, fmt.Errorf("zefs: header too short: %d", len(hdr))
	}
	if hdr[7] != ':' || hdr[15] != ':' {
		return 0, 0, fmt.Errorf("zefs: malformed header: %q", hdr[:headerLen])
	}

	used, err = strconv.Atoi(string(hdr[:7]))
	if err != nil {
		return 0, 0, fmt.Errorf("zefs: invalid used: %w", err)
	}

	capacity, err = strconv.Atoi(string(hdr[8:15]))
	if err != nil {
		return 0, 0, fmt.Errorf("zefs: invalid capacity: %w", err)
	}

	if used < 0 || capacity < 0 {
		return 0, 0, fmt.Errorf("zefs: negative header values: used=%d, capacity=%d", used, capacity)
	}
	if used > capacity {
		return 0, 0, fmt.Errorf("zefs: used %d exceeds capacity %d", used, capacity)
	}

	return used, capacity, nil
}

// decodeNetstringRef reads a netstring at the given offset without copying.
// The returned data is a sub-slice of buf and shares its backing array.
// Callers must not modify the returned data.
func decodeNetstringRef(buf []byte, off int) (data []byte, capacity, next int, err error) {
	if off+headerLen > len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: truncated header at offset %d", off)
	}

	used, cap_, err := parseHeader(buf[off : off+headerLen])
	if err != nil {
		return nil, 0, 0, err
	}

	dataStart := off + headerLen
	if dataStart+cap_ > len(buf) {
		return nil, 0, 0, fmt.Errorf("zefs: truncated data at offset %d: need %d, have %d", off, cap_, len(buf)-dataStart)
	}

	// Zero-copy: return sub-slice with capped length to prevent access to padding
	return buf[dataStart : dataStart+used : dataStart+used], cap_, dataStart + cap_, nil
}

// growCapacity returns a new capacity that fits dataLen with at least 10% spare.
// The result is capped at maxHeaderVal to fit the 7-digit header field.
func growCapacity(dataLen, currentCap int) int {
	needed := max(dataLen+dataLen/10, dataLen+1)
	cap_ := max(currentCap, 64)
	for cap_ < needed {
		cap_ *= 2
	}
	return min(cap_, maxHeaderVal)
}
