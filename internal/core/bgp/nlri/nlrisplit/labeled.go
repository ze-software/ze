// Design: docs/architecture/wire/nlri.md -- labeled unicast NLRI splitting
// RFC: rfc/short/rfc8277.md -- labeled unicast NLRI wire format
// Related: register.go -- binds SplitLabeled to ipv4/ipv6 mpls-label families
// Related: cidr.go -- SplitCIDR reference for the Splitter pattern

package nlrisplit

import (
	"errors"
	"fmt"
)

var (
	errNlrisplitTruncatedLabeledNlri         = errors.New("nlrisplit: truncated labeled NLRI")
	errNlrisplitTruncatedLabelStack          = errors.New("nlrisplit: truncated label stack")
	errNlrisplitTruncatedPrefixInLabeledNlri = errors.New("nlrisplit: truncated prefix in labeled NLRI")
)

// SplitLabeled is the Splitter for labeled unicast families (SAFI 4,
// RFC 8277). Each NLRI is framed as:
//
//	[totalBits:1][label(3)]...[prefix-bytes]
//
// where totalBits counts both label bits (24 per label entry) and prefix
// bits. Label entries are read until the S bit (bottom-of-stack, bit 0 of
// byte 2) is set.
//
// Under ADD-PATH (RFC 7911) each NLRI is prefixed with a 4-byte path-id
// that is included in the visited slice.
//
// Visits the full wire bytes per NLRI (labels + prefix). The caller
// extracts labels and CIDR bytes separately via ExtractLabels.
//
// Slices alias `data`. A malformed entry returns the count of the entries
// visited before it plus a non-nil error.
//
// The walk is bounded by len(data): every entry advances the offset by at
// least the length octet and one label entry.
func SplitLabeled(data []byte, addPath bool, fn func(nlri []byte)) (int, error) {
	count := 0
	offset := 0
	for offset < len(data) {
		start := offset
		head := 0
		if addPath {
			head = 4
		}

		if start+head+1 > len(data) {
			return count, fmt.Errorf("nlrisplit: truncated labeled NLRI header at offset %d", start)
		}

		totalBits := int(data[start+head])

		// Count label bytes by reading 3-byte entries until S bit.
		labelStart := start + head + 1
		labelBytes := 0
		for {
			if labelStart+labelBytes+3 > len(data) {
				return count, fmt.Errorf("nlrisplit: truncated label stack at offset %d", start)
			}
			sbit := data[labelStart+labelBytes+2] & 0x01
			labelBytes += 3
			if sbit == 1 {
				break
			}
		}

		prefixBits := totalBits - (labelBytes/3)*24
		if prefixBits < 0 {
			return count, fmt.Errorf("nlrisplit: invalid totalBits %d with %d label bytes at offset %d", totalBits, labelBytes, start)
		}
		prefixBytes := (prefixBits + 7) / 8
		nlriLen := head + 1 + labelBytes + prefixBytes

		if start+nlriLen > len(data) {
			return count, fmt.Errorf("nlrisplit: labeled NLRI at offset %d extends past data", start)
		}

		if fn != nil {
			fn(data[start : start+nlriLen])
		}
		count++
		offset = start + nlriLen
	}
	return count, nil
}

// ExtractLabels parses a single labeled NLRI entry (as returned by
// SplitLabeled) and returns the MPLS label stack and the CIDR wire bytes
// with labels stripped. Under ADD-PATH the 4-byte path-id is preserved
// in the returned CIDR bytes.
//
// CIDR output format: [path-id(4)?][prefixBits(1)][prefix-bytes].
func ExtractLabels(entry []byte, addPath bool) ([]uint32, []byte, error) {
	head := 0
	if addPath {
		head = 4
	}
	if len(entry) < head+4 { // minimum: [path-id?] + totalBits + 3 label bytes
		return nil, nil, errNlrisplitTruncatedLabeledNlri
	}

	totalBits := int(entry[head])

	pos := head + 1
	var labels []uint32
	for {
		if pos+3 > len(entry) {
			return nil, nil, errNlrisplitTruncatedLabelStack
		}
		label := uint32(entry[pos])<<12 | uint32(entry[pos+1])<<4 | uint32(entry[pos+2])>>4
		labels = append(labels, label)
		sbit := entry[pos+2] & 0x01
		pos += 3
		if sbit == 1 {
			break
		}
	}

	prefixBits := totalBits - len(labels)*24
	if prefixBits < 0 {
		return nil, nil, fmt.Errorf("nlrisplit: invalid totalBits %d with %d labels", totalBits, len(labels))
	}
	prefixBytes := (prefixBits + 7) / 8
	if pos+prefixBytes > len(entry) {
		return nil, nil, errNlrisplitTruncatedPrefixInLabeledNlri
	}

	// Build CIDR wire bytes: [path-id?][prefixBits][prefix-bytes]
	cidrLen := head + 1 + prefixBytes
	cidr := make([]byte, cidrLen)
	if addPath {
		copy(cidr[:4], entry[:4])
	}
	cidr[head] = byte(prefixBits)
	copy(cidr[head+1:], entry[pos:pos+prefixBytes])

	return labels, cidr, nil
}
