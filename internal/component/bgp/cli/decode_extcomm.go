// Design: docs/architecture/core-design.md — BGP CLI commands
// Related: decode_update.go — path attribute parsing calls parseExtendedCommunities

package cli

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ze-software/ze/internal/core/textbuf"
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
			// RFC 8955 Section 7.1: Traffic-rate-bytes uses a 4-octet IEEE float.
			comm["string"] = formatFlowSpecTrafficRate("rate-limit", "", data[4:8])
		case typeHigh == 0x80 && typeLow == 0x0c:
			// RFC 8955 Section 7.2: Traffic-rate-packets uses the same encoding as traffic-rate-bytes.
			comm["string"] = formatFlowSpecTrafficRate("rate-limit", "packets", data[4:8])
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

func formatFlowSpecTrafficRate(name, unit string, data []byte) string {
	rate := math.Float32frombits(binary.BigEndian.Uint32(data))
	// RFC 8955 Section 7.1/7.2: negative decoded rates MUST be treated as zero.
	if rate < 0 || math.IsNaN(float64(rate)) {
		rate = 0
	}
	var tb textbuf.Buffer
	tb.Str(name).Byte(':').Uint(uint64(rate))
	if unit != "" {
		tb.Byte(':').Str(unit)
	}
	return tb.String()
}
