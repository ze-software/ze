// Design: docs/architecture/core-design.md -- BGP CLI commands
// Overview: decode.go -- top-level decode dispatch calls notification/keepalive decoders

package bgp

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// notificationErrorNames maps NOTIFICATION error codes to names (RFC 4271 Section 4.5).
var notificationErrorNames = map[byte]string{
	1: "Message Header Error",
	2: "OPEN Message Error",
	3: "UPDATE Message Error",
	4: "Hold Timer Expired",
	5: "FSM Error",
	6: "Cease",
	7: "ROUTE-REFRESH Message Error",
}

// decodeNotificationMessage decodes a BGP NOTIFICATION message.
// Returns a map with "notification" key containing error-code, error-subcode, and optional data.
func decodeNotificationMessage(data []byte, hasHeader bool) (map[string]any, error) {
	body := data
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, fmt.Errorf("message too short for header: %d bytes", len(data))
		}
		body = data[message.HeaderLen:]
	}

	if len(body) < 2 {
		return nil, fmt.Errorf("NOTIFICATION body too short: %d bytes (need at least 2)", len(body))
	}

	errCode := body[0]
	errSubcode := body[1]

	codeName := "Unknown"
	if name, ok := notificationErrorNames[errCode]; ok {
		codeName = name
	}

	notif := map[string]any{
		"error-code":    int(errCode),
		"error-subcode": int(errSubcode),
		"error-name":    codeName,
	}

	if len(body) > 2 {
		notif["data"] = strings.ToUpper(hex.EncodeToString(body[2:]))
	}

	return map[string]any{msgTypeNotification: notif}, nil
}

// decodeKeepaliveMessage decodes a BGP KEEPALIVE message.
// KEEPALIVE has no body -- the 19-byte header is the entire message.
func decodeKeepaliveMessage(data []byte, hasHeader bool) (map[string]any, error) {
	if hasHeader {
		if len(data) < message.HeaderLen {
			return nil, fmt.Errorf("message too short for header: %d bytes", len(data))
		}
		// KEEPALIVE length must be exactly 19 (header only, no body).
		msgLen := int(binary.BigEndian.Uint16(data[16:18]))
		if msgLen != message.HeaderLen {
			return nil, fmt.Errorf("KEEPALIVE length %d, expected %d", msgLen, message.HeaderLen)
		}
	}

	return map[string]any{msgTypeKeepalive: map[string]any{}}, nil
}

// formatNotificationHuman formats a NOTIFICATION decode result for human display.
func formatNotificationHuman(result map[string]any) string {
	notif, ok := result[msgTypeNotification].(map[string]any)
	if !ok {
		return "NOTIFICATION (no data)"
	}

	var sb strings.Builder
	sb.WriteString("NOTIFICATION\n")
	if code, ok := notif["error-code"].(int); ok {
		name, _ := notif["error-name"].(string)
		fmt.Fprintf(&sb, "  Error Code:    %d (%s)\n", code, name)
	}
	if subcode, ok := notif["error-subcode"].(int); ok {
		fmt.Fprintf(&sb, "  Error Subcode: %d\n", subcode)
	}
	if data, ok := notif["data"].(string); ok {
		fmt.Fprintf(&sb, "  Data:          %s\n", data)
	}
	return sb.String()
}
