// Design: docs/architecture/core-design.md — BGP CLI commands
// Overview: decode.go — top-level decode dispatch
// Related: decode_mp.go — MP_REACH/MP_UNREACH parsing
// Related: decode_extcomm.go — extended community parsing
// Related: decode_bgpls.go — BGP-LS attribute parsing

package bgp

import (
	"encoding/binary"
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
)

// decodeUpdateMessage decodes a BGP UPDATE message and returns Ze format.
func decodeUpdateMessage(data []byte, _ string, hasHeader bool) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, fmt.Errorf("data too short for header")
		}
		body = data[message.HeaderLen:]
	}

	update, err := message.UnpackUpdate(body)
	if err != nil {
		return nil, fmt.Errorf("unpack update: %w", err)
	}

	// Build Ze format update content
	updateContent := map[string]any{}

	// Parse path attributes - Ze format uses "attr" key
	attrs, mpReach, mpUnreach := parsePathAttributesZe(update.PathAttributes)

	// Extract and remove internal next-hop field (used for NLRI operations)
	nextHop := "0.0.0.0"
	if nh, ok := attrs["_next-hop"].(string); ok {
		nextHop = nh
		delete(attrs, "_next-hop")
	}

	if len(attrs) > 0 {
		updateContent["attr"] = attrs
	}

	// Ze format: family is direct key under update (no "nlri" wrapper)
	// Handle MP_REACH_NLRI (announcements)
	if mpReach != nil {
		family, ops := buildMPReachZe(mpReach)
		if family != "" && len(ops) > 0 {
			updateContent[family] = ops
		}
	}

	// Handle MP_UNREACH_NLRI (withdrawals)
	if mpUnreach != nil {
		family, ops := buildMPUnreachZe(mpUnreach)
		if family != "" && len(ops) > 0 {
			if existing, ok := updateContent[family].([]map[string]any); ok {
				updateContent[family] = append(existing, ops...)
			} else {
				updateContent[family] = ops
			}
		}
	}

	// Handle IPv4 withdrawn routes
	if len(update.WithdrawnRoutes) > 0 {
		prefixes := parseIPv4Prefixes(update.WithdrawnRoutes)
		if len(prefixes) > 0 {
			withdrawOp := map[string]any{"action": "del", "nlri": prefixes}
			if existing, ok := updateContent["ipv4/unicast"].([]map[string]any); ok {
				updateContent["ipv4/unicast"] = append(existing, withdrawOp)
			} else {
				updateContent["ipv4/unicast"] = []map[string]any{withdrawOp}
			}
		}
	}

	// Handle IPv4 NLRI (announcements)
	if len(update.NLRI) > 0 {
		prefixes := parseIPv4Prefixes(update.NLRI)
		if len(prefixes) > 0 {
			announceOp := map[string]any{"next-hop": nextHop, "action": "add", "nlri": prefixes}
			if existing, ok := updateContent["ipv4/unicast"].([]map[string]any); ok {
				updateContent["ipv4/unicast"] = append(existing, announceOp)
			} else {
				updateContent["ipv4/unicast"] = []map[string]any{announceOp}
			}
		}
	}

	return map[string]any{"update": updateContent}, nil
}

// parsePathAttributesZe parses path attributes for Ze format (uses simple AS_PATH array).
func parsePathAttributesZe(data []byte) (attrs map[string]any, mpReach, mpUnreach []byte) {
	attrs = make(map[string]any)
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		flags := data[offset]
		code := data[offset+1]

		hdrLen := 3
		var valueLen int
		if flags&0x10 != 0 {
			if offset+4 > len(data) {
				break
			}
			valueLen = int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			hdrLen = 4
		} else {
			if offset+3 > len(data) {
				break
			}
			valueLen = int(data[offset+2])
		}

		if offset+hdrLen+valueLen > len(data) {
			break
		}

		value := data[offset+hdrLen : offset+hdrLen+valueLen]

		switch code {
		case 1: // ORIGIN
			if len(value) >= 1 {
				origins := []string{"igp", "egp", "incomplete"}
				if int(value[0]) < len(origins) {
					attrs["origin"] = origins[value[0]]
				}
			}
		case 2: // AS_PATH - Ze format uses simple array
			asPath := parseASPathZe(value)
			if len(asPath) > 0 {
				attrs["as-path"] = asPath
			}
		case 3: // NEXT_HOP
			if len(value) == 4 {
				attrs["_next-hop"] = fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
			}
		case 4: // MED
			if len(value) == 4 {
				attrs["med"] = binary.BigEndian.Uint32(value)
			}
		case 5: // LOCAL_PREF
			if len(value) == 4 {
				attrs["local-preference"] = binary.BigEndian.Uint32(value)
			}
		case 6: // ATOMIC_AGGREGATE
			attrs["atomic-aggregate"] = true
		case 7: // AGGREGATOR
			if len(value) == 6 {
				asn := binary.BigEndian.Uint16(value[0:2])
				ip := fmt.Sprintf("%d.%d.%d.%d", value[2], value[3], value[4], value[5])
				attrs["aggregator"] = fmt.Sprintf("%d:%s", asn, ip)
			} else if len(value) == 8 {
				asn := binary.BigEndian.Uint32(value[0:4])
				ip := fmt.Sprintf("%d.%d.%d.%d", value[4], value[5], value[6], value[7])
				attrs["aggregator"] = fmt.Sprintf("%d:%s", asn, ip)
			}
		case 9: // ORIGINATOR_ID
			if len(value) == 4 {
				attrs["originator-id"] = fmt.Sprintf("%d.%d.%d.%d", value[0], value[1], value[2], value[3])
			}
		case 10: // CLUSTER_LIST
			var clusters []string
			for i := 0; i+4 <= len(value); i += 4 {
				clusters = append(clusters, fmt.Sprintf("%d.%d.%d.%d", value[i], value[i+1], value[i+2], value[i+3]))
			}
			if len(clusters) > 0 {
				attrs["cluster-list"] = clusters
			}
		case 16: // EXTENDED_COMMUNITIES
			extComms := parseExtendedCommunities(value)
			if len(extComms) > 0 {
				attrs["extended-community"] = extComms
			}
		case 14: // MP_REACH_NLRI
			mpReach = value
		case 15: // MP_UNREACH_NLRI
			mpUnreach = value
		case 29: // BGP-LS Attribute
			bgplsAttr := parseBGPLSAttribute(value)
			if len(bgplsAttr) > 0 {
				attrs["bgp-ls"] = bgplsAttr
			}
		}

		offset += hdrLen + valueLen
	}

	return attrs, mpReach, mpUnreach
}

// parseASPathZe parses AS_PATH attribute value into Ze format (simple array).
func parseASPathZe(data []byte) []uint32 {
	var result []uint32
	offset := 0

	for offset < len(data) {
		if offset+2 > len(data) {
			break
		}

		segLen := int(data[offset+1])
		offset += 2

		// Try 4-byte ASNs first, then 2-byte
		asnSize := 4
		if offset+segLen*4 > len(data) {
			asnSize = 2
		}
		if offset+segLen*asnSize > len(data) {
			break
		}

		for range segLen {
			var asn uint32
			if asnSize == 4 {
				asn = binary.BigEndian.Uint32(data[offset : offset+4])
			} else {
				asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			}
			result = append(result, asn)
			offset += asnSize
		}
	}

	return result
}
