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
func SplitLabeled(data []byte, addPath bool, fn func(nlri []byte) bool) (int, error) {
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

		count++
		if fn != nil {
			if !fn(data[start : start+nlriLen]) {
				return count, nil
			}
		}
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

// LabeledWithdrawal rewrites section, a run of labeled NLRIs in announcement
// form (SplitLabeled framing, SAFI 4 or SAFI 128), in place into the
// withdrawal form and returns the withdrawal's length. Each NLRI keeps its
// ADD-PATH identifier and its prefix (the RD included for SAFI 128); its label
// stack, one label or several, becomes one three-octet Compatibility field set
// to 0x800000, and its length octet drops 24 bits for every label beyond the
// first. The result is never longer than the input.
//
// A section that does not frame is refused with an error and left exactly as
// it was: the count pass runs before the first byte is written.
//
// Announced NLRI (RFC 8277 Section 2.3, Figure 3) and withdrawn NLRI (Section
// 2.4, Figure 4), each after the four-octet Path Identifier under ADD-PATH:
//
//	+--------+-----------------+-----------------+--------------+
//	| Length | Label (3) |Rsrv|S| ... repeated until S = 1 | Prefix ~
//	+--------+-----------------+-----------------+--------------+
//	| Length | Compatibility 0x800000 (3)  | Prefix ~
//	+--------+-----------------------------+---------------------
//
// RFC 8277 Section 2.4: "This encoding is used whether or not the Multiple
// Labels Capability has been sent or received on the session." It names one
// Compatibility field and no label stack, and "there is never any reason for
// this field to contain a label value or a sequence of label values.".
func LabeledWithdrawal(section []byte, addPath bool) (int, error) {
	if _, err := SplitLabeled(section, addPath, nil); err != nil {
		return 0, err
	}
	head := 0
	if addPath {
		head = 4
	}
	// The count pass framed every NLRI, so each read below is in bounds. The
	// write offset never passes the read offset: an NLRI only shrinks, and its
	// four written header octets land on bytes already read.
	write := 0
	for read := 0; read < len(section); {
		copy(section[write:write+head], section[read:read+head])
		totalBits := int(section[read+head])
		labels := 0
		pos := read + head + 1
		for {
			bottom := section[pos+2]&0x01 == 1
			pos += 3
			labels++
			if bottom {
				break
			}
		}
		prefixBytes := (totalBits - labels*24 + 7) / 8
		// RFC 8277 Section 2.4: "the prefix length is not the same as the NLRI
		// length; to determine the prefix length of a prefix in an
		// MP_UNREACH_NLRI, the length of the Compatibility field must be
		// subtracted from the length of the NLRI."
		section[write+head] = byte(totalBits - (labels-1)*24)
		// RFC 8277 Section 2.4: "Upon transmission, the Compatibility field
		// SHOULD be set to 0x800000."
		section[write+head+1], section[write+head+2], section[write+head+3] = 0x80, 0x00, 0x00
		copy(section[write+head+4:], section[pos:pos+prefixBytes])
		write += head + 4 + prefixBytes
		read = pos + prefixBytes
	}
	return write, nil
}
