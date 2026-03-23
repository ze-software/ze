// Design: (none -- new tool, predates documentation)
// Related: benchmark.go -- benchmark orchestration using prefix extraction

package perf

import (
	"encoding/binary"
	"net/netip"
)

// ExtractPrefixes extracts all announced prefixes from a BGP UPDATE message
// body (everything after the 19-byte header). Returns both inline IPv4/unicast
// NLRI and MP_REACH_NLRI prefixes (IPv4 and IPv6). Does NOT extract withdrawn
// prefixes -- only announcements are needed for benchmarking receive rates.
//
// The UPDATE body format:
//  1. Withdrawn Routes Length (2 bytes)
//  2. Withdrawn Routes (variable, skipped)
//  3. Total Path Attribute Length (2 bytes)
//  4. Path Attributes (variable, scanned for type 14 = MP_REACH_NLRI)
//  5. NLRI (remaining bytes, IPv4/unicast)
func ExtractPrefixes(body []byte) []netip.Prefix {
	if len(body) < 4 {
		return nil
	}

	// Skip withdrawn routes section.
	withdrawnLen := int(binary.BigEndian.Uint16(body[0:2]))
	off := 2 + withdrawnLen
	if off+2 > len(body) {
		return nil
	}

	// Path attribute length.
	attrLen := int(binary.BigEndian.Uint16(body[off : off+2]))
	off += 2

	var result []netip.Prefix

	// Walk path attributes looking for MP_REACH_NLRI (type 14).
	attrEnd := off + attrLen
	if attrEnd > len(body) {
		return nil
	}
	for off < attrEnd {
		if off+3 > attrEnd {
			break
		}
		flags := body[off]
		code := body[off+1]
		off += 2

		// Attribute length: 1 byte normally, 2 bytes if extended-length flag set.
		var aLen int
		if flags&0x10 != 0 {
			if off+2 > attrEnd {
				break
			}
			aLen = int(binary.BigEndian.Uint16(body[off : off+2]))
			off += 2
		} else {
			aLen = int(body[off])
			off++
		}
		if off+aLen > attrEnd {
			break
		}

		if code == 14 {
			result = append(result, parseMPReachPrefixes(body[off:off+aLen])...)
		}
		off += aLen
	}

	// Parse remaining bytes as inline IPv4/unicast NLRI.
	off = attrEnd
	for off < len(body) {
		prefix, n := parseIPv4Prefix(body[off:])
		if n <= 0 {
			break
		}
		result = append(result, prefix)
		off += n
	}

	if result == nil {
		return []netip.Prefix{}
	}
	return result
}

// parseMPReachPrefixes extracts prefixes from an MP_REACH_NLRI attribute value.
// Format: AFI(2) + SAFI(1) + NH_len(1) + NH(variable) + reserved(1) + NLRI...
// Only AFI=1 (IPv4) and AFI=2 (IPv6) unicast are supported. Other AFIs are
// ignored -- the perf tool only benchmarks unicast prefix throughput.
func parseMPReachPrefixes(data []byte) []netip.Prefix {
	if len(data) < 5 {
		return nil
	}

	afi := binary.BigEndian.Uint16(data[0:2])

	// Only IPv4 (AFI=1) and IPv6 (AFI=2) are supported for prefix extraction.
	if afi != 1 && afi != 2 {
		return nil
	}

	nhLen := int(data[3])
	off := 4 + nhLen + 1 // Skip next-hop + reserved byte.
	if off > len(data) {
		return nil
	}

	parsePrefix := parseIPv4Prefix
	if afi == 2 {
		parsePrefix = parseIPv6Prefix
	}

	var result []netip.Prefix
	for off < len(data) {
		prefix, n := parsePrefix(data[off:])
		if n <= 0 {
			break
		}
		result = append(result, prefix)
		off += n
	}
	return result
}

// parseIPv4Prefix parses a single IPv4 prefix from BGP wire format.
// Wire format: prefix_bits(1 byte) + addr_bytes(ceil(prefix_bits/8) bytes).
// Returns the parsed prefix and the number of bytes consumed.
// Returns an invalid prefix and 0 on error.
func parseIPv4Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 32 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [4]byte
	copy(addr[:], data[1:1+byteLen])

	return netip.PrefixFrom(netip.AddrFrom4(addr), prefixLen), 1 + byteLen
}

// parseIPv6Prefix parses a single IPv6 prefix from BGP wire format.
// Wire format: prefix_bits(1 byte) + addr_bytes(ceil(prefix_bits/8) bytes).
// Returns the parsed prefix and the number of bytes consumed.
// Returns an invalid prefix and 0 on error.
func parseIPv6Prefix(data []byte) (netip.Prefix, int) {
	if len(data) < 1 {
		return netip.Prefix{}, 0
	}

	prefixLen := int(data[0])
	if prefixLen > 128 {
		return netip.Prefix{}, 0
	}

	byteLen := (prefixLen + 7) / 8
	if len(data) < 1+byteLen {
		return netip.Prefix{}, 0
	}

	var addr [16]byte
	copy(addr[:], data[1:1+byteLen])

	return netip.PrefixFrom(netip.AddrFrom16(addr), prefixLen), 1 + byteLen
}
