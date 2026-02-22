// Design: docs/architecture/core-design.md — BGP CLI commands
// Related: decode_update.go — path attribute parsing calls parseExtendedCommunities

package bgp

import (
	"encoding/binary"
	"fmt"
)

// parseExtendedCommunities parses extended communities (type 16).
// Each extended community is 8 bytes.
func parseExtendedCommunities(data []byte) []map[string]any {
	var comms []map[string]any

	for len(data) >= 8 {
		// Read 8-byte extended community
		value := binary.BigEndian.Uint64(data[:8])
		typeHigh := data[0]
		typeLow := data[1]

		comm := map[string]any{
			"value": value,
		}

		// Parse based on type
		switch {
		case typeHigh == 0x80 && typeLow == 0x06:
			// Traffic-rate (FlowSpec)
			rate := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("rate-limit:%d", rate)
		case typeHigh == 0x80 && typeLow == 0x07:
			// Traffic-action (FlowSpec)
			comm["string"] = "traffic-action"
		case typeHigh == 0x80 && typeLow == 0x08:
			// Redirect (FlowSpec)
			asn := binary.BigEndian.Uint16(data[2:4])
			localAdmin := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("redirect:%d:%d", asn, localAdmin)
		case typeHigh == 0x80 && typeLow == 0x09:
			// Traffic-marking (FlowSpec)
			dscp := data[7]
			comm["string"] = fmt.Sprintf("mark:%d", dscp)
		case typeHigh == 0x00 && typeLow == 0x02:
			// Route Target
			asn := binary.BigEndian.Uint16(data[2:4])
			localAdmin := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("target:%d:%d", asn, localAdmin)
		case typeHigh == 0x00 && typeLow == 0x03:
			// Route Origin
			asn := binary.BigEndian.Uint16(data[2:4])
			localAdmin := binary.BigEndian.Uint32(data[4:8])
			comm["string"] = fmt.Sprintf("origin:%d:%d", asn, localAdmin)
		default:
			// Generic format
			comm["string"] = fmt.Sprintf("0x%02x%02x:%x", typeHigh, typeLow, data[2:8])
		}

		comms = append(comms, comm)
		data = data[8:]
	}

	return comms
}
