// Design: docs/architecture/core-design.md — BGP CLI commands
// Related: decode_update.go — path attribute parsing calls parseBGPLSAttribute

package bgp

import (
	"encoding/binary"
	"fmt"
	"math"
	"net/netip"
)

// parseBGPLSAttribute parses BGP-LS attribute (type 29) TLVs.
// RFC 7752 Section 3.3 defines the attribute format and TLV types.
func parseBGPLSAttribute(data []byte) map[string]any {
	result := make(map[string]any)
	offset := 0

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if offset+4+tlvLen > len(data) {
			break
		}

		value := data[offset+4 : offset+4+tlvLen]

		switch tlvType {
		// Node Attribute TLVs (RFC 7752 Section 3.3.1)
		case 1024: // Node Flag Bits
			if len(value) >= 1 {
				flags := value[0]
				result["node-flags"] = map[string]any{
					"O":   (flags >> 7) & 1,
					"T":   (flags >> 6) & 1,
					"E":   (flags >> 5) & 1,
					"B":   (flags >> 4) & 1,
					"R":   (flags >> 3) & 1,
					"V":   (flags >> 2) & 1,
					"RSV": flags & 0x03,
				}
			}
		case 1026: // Node Name
			result["node-name"] = string(value)
		case 1027: // IS-IS Area Identifier
			// Output as hex with 0x prefix - ExaBGP accepts both decimal and 0x-prefixed hex
			result["area-id"] = fmt.Sprintf("0x%X", value)
		case 1028: // IPv4 Router-ID Local
			if len(value) == 4 {
				// Append to local-router-ids array
				addr := fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
				if existing, ok := result["local-router-ids"].([]string); ok {
					result["local-router-ids"] = append(existing, addr)
				} else {
					result["local-router-ids"] = []string{addr}
				}
			}
		case 1029: // IPv6 Router-ID Local
			if len(value) == 16 {
				addr := formatIPv6Compressed(value)
				if existing, ok := result["local-router-ids"].([]string); ok {
					result["local-router-ids"] = append(existing, addr)
				} else {
					result["local-router-ids"] = []string{addr}
				}
			}

		// Link Attribute TLVs (RFC 7752 Section 3.3.2)
		case 1030: // IPv4 Router-ID Remote
			if len(value) == 4 {
				addr := fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
				if existing, ok := result["remote-router-ids"].([]string); ok {
					result["remote-router-ids"] = append(existing, addr)
				} else {
					result["remote-router-ids"] = []string{addr}
				}
			}
		case 1031: // IPv6 Router-ID Remote
			if len(value) == 16 {
				addr := formatIPv6Compressed(value)
				if existing, ok := result["remote-router-ids"].([]string); ok {
					result["remote-router-ids"] = append(existing, addr)
				} else {
					result["remote-router-ids"] = []string{addr}
				}
			}
		case 1088: // Administrative Group (color)
			if len(value) >= 4 {
				result["admin-group-mask"] = binary.BigEndian.Uint32(value)
			}
		case 1089: // Maximum Link Bandwidth
			if len(value) >= 4 {
				result["maximum-link-bandwidth"] = float64(math.Float32frombits(binary.BigEndian.Uint32(value)))
			}
		case 1090: // Max. Reservable Link Bandwidth
			if len(value) >= 4 {
				result["maximum-reservable-link-bandwidth"] = float64(math.Float32frombits(binary.BigEndian.Uint32(value)))
			}
		case 1091: // Unreserved Bandwidth (8 values)
			if len(value) >= 32 {
				bws := make([]float64, 8)
				for i := range 8 {
					bws[i] = float64(math.Float32frombits(binary.BigEndian.Uint32(value[i*4:])))
				}
				result["unreserved-bandwidth"] = bws
			}
		case 1092: // TE Default Metric
			if len(value) >= 4 {
				result["te-metric"] = binary.BigEndian.Uint32(value)
			}
		case 1095: // IGP Metric
			switch len(value) {
			case 1:
				result["igp-metric"] = int(value[0] & 0x3F) // IS-IS small metric (6 bits)
			case 2:
				result["igp-metric"] = int(binary.BigEndian.Uint16(value)) // OSPF metric
			case 3:
				result["igp-metric"] = int(value[0])<<16 | int(value[1])<<8 | int(value[2]) // IS-IS wide
			default:
				if len(value) >= 4 {
					result["igp-metric"] = int(binary.BigEndian.Uint32(value))
				}
			}

		// Prefix Attribute TLVs (RFC 7752 Section 3.3.3)
		case 1155: // Prefix Metric
			if len(value) >= 4 {
				result["prefix-metric"] = binary.BigEndian.Uint32(value)
			}
		case 1170: // SR Prefix Attribute Flags
			if len(value) >= 1 {
				flags := value[0]
				result["sr-prefix-attribute-flags"] = map[string]any{
					"X":   (flags >> 7) & 1,
					"R":   (flags >> 6) & 1,
					"N":   (flags >> 5) & 1,
					"RSV": flags & 0x1F,
				}
			}

		// SRv6 Link Attribute TLVs (RFC 9514 Section 4)
		case 1099: // SR-MPLS Adjacency SID (RFC 9085)
			parseSRMPLSAdjSID(result, "sr-adj", value)

		case 1106: // SRv6 End.X SID
			sids := parseSRv6EndXSID(value, 0)
			appendSRv6SIDs(result, "srv6-endx", sids)

		case 1107: // IS-IS SRv6 LAN End.X SID
			sids := parseSRv6EndXSID(value, 6) // 6-byte IS-IS neighbor ID
			appendSRv6SIDs(result, "srv6-lan-endx-isis", sids)

		case 1108: // OSPFv3 SRv6 LAN End.X SID
			sids := parseSRv6EndXSID(value, 4) // 4-byte OSPFv3 neighbor ID
			appendSRv6SIDs(result, "srv6-lan-endx-ospf", sids)

		default:
			// Generic TLV - store as hex
			result[fmt.Sprintf("generic-lsid-%d", tlvType)] = []string{fmt.Sprintf("0x%X", value)}
		}

		offset += 4 + tlvLen
	}

	return result
}

// parseSRv6EndXSID parses SRv6 End.X SID or LAN End.X SID TLVs (RFC 9514 Section 4).
// neighborIDLen is 0 for End.X SID, 6 for IS-IS LAN End.X, 4 for OSPFv3 LAN End.X.
// Returns a slice of parsed SID entries.
func parseSRv6EndXSID(data []byte, neighborIDLen int) []map[string]any {
	var sids []map[string]any

	// Minimum: Behavior(2) + Flags(1) + Algo(1) + Weight(1) + Reserved(1) + NeighborID + SID(16)
	minLen := 6 + neighborIDLen + 16
	offset := 0

	for offset+minLen <= len(data) {
		behavior := binary.BigEndian.Uint16(data[offset : offset+2])
		flags := data[offset+2]
		algorithm := data[offset+3]
		weight := data[offset+4]
		// offset+5 is reserved

		sidOffset := offset + 6 + neighborIDLen
		if sidOffset+16 > len(data) {
			break
		}

		sid := data[sidOffset : sidOffset+16]

		entry := map[string]any{
			"behavior":  int(behavior),
			"algorithm": int(algorithm),
			"weight":    int(weight),
			"flags": map[string]any{
				"B":   int((flags >> 7) & 1),
				"S":   int((flags >> 6) & 1),
				"P":   int((flags >> 5) & 1),
				"RSV": int(flags & 0x1F),
			},
			"sid": formatIPv6Compressed(sid),
		}

		// Add neighbor ID if present (LAN End.X SID)
		if neighborIDLen > 0 {
			neighborID := data[offset+6 : offset+6+neighborIDLen]
			entry["neighbor-id"] = fmt.Sprintf("%X", neighborID)
		}

		// Parse sub-TLVs (SRv6 SID Structure)
		subTLVOffset := sidOffset + 16
		if subTLVOffset+4 <= len(data) {
			subTLVType := binary.BigEndian.Uint16(data[subTLVOffset : subTLVOffset+2])
			subTLVLen := int(binary.BigEndian.Uint16(data[subTLVOffset+2 : subTLVOffset+4]))

			if subTLVType == 1252 && subTLVLen == 4 && subTLVOffset+4+4 <= len(data) {
				// SRv6 SID Structure (RFC 9514 Section 8)
				structData := data[subTLVOffset+4 : subTLVOffset+8]
				entry["srv6-sid-structure"] = map[string]any{
					"loc_block_len": int(structData[0]),
					"loc_node_len":  int(structData[1]),
					"func_len":      int(structData[2]),
					"arg_len":       int(structData[3]),
				}
				offset = subTLVOffset + 4 + subTLVLen
			} else {
				offset = subTLVOffset
			}
		} else {
			offset = subTLVOffset
		}

		sids = append(sids, entry)
	}

	return sids
}

// appendSRv6SIDs appends SRv6 SID entries to the result map under the given key.
func appendSRv6SIDs(result map[string]any, key string, sids []map[string]any) {
	if len(sids) == 0 {
		return
	}
	if existing, ok := result[key].([]map[string]any); ok {
		result[key] = append(existing, sids...)
	} else {
		result[key] = sids
	}
}

// parseSRMPLSAdjSID parses SR-MPLS Adjacency SID TLV 1099 (RFC 9085 Section 2.2.1).
// Format: Flags(1) + Weight(1) + Reserved(2) + SID/Label(variable).
// When V=1 and L=1: 3-byte label. When V=0 and L=0: 4-byte index.
//
//nolint:unparam // key parameter for API consistency with other TLV parsers
func parseSRMPLSAdjSID(result map[string]any, key string, data []byte) {
	if len(data) < 4 {
		return
	}

	flags := data[0]
	weight := int(data[1])
	// data[2:4] is reserved

	// Parse flags: F(7), B(6), V(5), L(4), S(3), P(2), RSV(1), RSV(0)
	flagMap := map[string]any{
		"F":   int((flags >> 7) & 1),
		"B":   int((flags >> 6) & 1),
		"V":   int((flags >> 5) & 1),
		"L":   int((flags >> 4) & 1),
		"S":   int((flags >> 3) & 1),
		"P":   int((flags >> 2) & 1),
		"RSV": int(flags & 0x03),
	}

	vFlag := (flags >> 5) & 1
	lFlag := (flags >> 4) & 1

	sids := make([]int, 0)
	undecoded := make([]string, 0)
	sidData := data[4:]

	// Combine V and L flags: 0b00=index, 0b11=label, others=invalid
	flagCombo := (vFlag << 1) | lFlag
	for len(sidData) > 0 {
		switch flagCombo {
		case 0b11: // V=1, L=1: 3-byte label
			if len(sidData) < 3 {
				undecoded = append(undecoded, fmt.Sprintf("%X", sidData))
				sidData = nil
				continue
			}
			sid := (int(sidData[0]) << 16) | (int(sidData[1]) << 8) | int(sidData[2])
			sids = append(sids, sid)
			sidData = sidData[3:]
		case 0b00: // V=0, L=0: 4-byte index
			if len(sidData) < 4 {
				undecoded = append(undecoded, fmt.Sprintf("%X", sidData))
				sidData = nil
				continue
			}
			sid := int(binary.BigEndian.Uint32(sidData[:4]))
			sids = append(sids, sid)
			sidData = sidData[4:]
		default: // Invalid flag combination
			undecoded = append(undecoded, fmt.Sprintf("%X", sidData))
			sidData = nil
		}
	}

	entry := map[string]any{
		"flags":          flagMap,
		"sids":           sids,
		"weight":         weight,
		"undecoded-sids": undecoded,
	}

	// Accumulate multiple TLV instances into an array (proper JSON, no data loss)
	if existing, ok := result[key].([]map[string]any); ok {
		result[key] = append(existing, entry)
	} else {
		result[key] = []map[string]any{entry}
	}
}

// formatIPv6Compressed formats a 16-byte IPv6 address with zero compression.
func formatIPv6Compressed(addr []byte) string {
	if len(addr) != 16 {
		return fmt.Sprintf("%X", addr)
	}
	// Use netip for proper zero compression
	ip := netip.AddrFrom16([16]byte(addr))
	return ip.String()
}
