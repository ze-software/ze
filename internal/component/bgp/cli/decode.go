// Design: docs/architecture/core-design.md — BGP CLI commands
// Detail: decode_open.go — OPEN message decoding
// Detail: decode_update.go — UPDATE message decoding
// Detail: decode_mp.go — MP_REACH/MP_UNREACH NLRI decoding
// Detail: decode_plugin.go — plugin invocation for decode
// Detail: decode_human.go — human-readable output formatting
// Detail: decode_answer.go — the show bgp decode handler and its answer

package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

// Message type constants.
const (
	msgTypeOpen         = "open"
	msgTypeUpdate       = "update"
	msgTypeNLRI         = "nlri"
	msgTypeNotification = "notification"
	msgTypeKeepalive    = "keepalive"
)

// Keys of the decode output map. jsonKeyNLRI is the key that carries the route
// list inside an operation; msgTypeNLRI above is the name of the decode mode,
// so the two spellings stay separate names.
const (
	jsonKeyAction = "action"
	jsonKeyNLRI   = "nlri"
	jsonKeyParsed = "parsed"
	jsonKeyRaw    = "raw"
)

// capNameUnknown is the capability name used when the code has no decoder.
const capNameUnknown = "unknown"

// decodeHexPacket decodes a hex BGP packet and returns formatted output.
// If outputJSON is true, returns JSON; otherwise returns human-readable format.
func decodeHexPacket(hexStr, msgType, family string, outputJSON bool) (string, error) {
	// Normalize hex input - remove colons, spaces, uppercase
	hexStr = strings.ReplaceAll(hexStr, ":", "")
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ToUpper(hexStr)

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", fmt.Errorf("invalid hex: %w", err)
	}

	// Detect format: if FF*16 marker present, it's a full message
	// Otherwise assume UPDATE body
	hasHeader := hasValidMarker(data)

	if msgType == "" {
		if hasHeader {
			msgType = detectMessageType(data)
		} else {
			msgType = msgTypeUpdate // Default to UPDATE body
		}
	}

	// For NLRI-only mode, don't wrap in envelope
	if msgType == msgTypeNLRI {
		return decodeNLRIOnly(data, family, outputJSON)
	}

	// Build output based on message type
	var result map[string]any
	switch msgType {
	case msgTypeOpen:
		result, err = decodeOpenMessage(data, hasHeader)
	case msgTypeUpdate:
		result, err = decodeUpdateMessage(data, family, hasHeader)
	case msgTypeNotification:
		result, err = decodeNotificationMessage(data, hasHeader)
	case msgTypeKeepalive:
		result, err = decodeKeepaliveMessage(data, hasHeader)
	default:
		return "", fmt.Errorf("unsupported message type: %s", msgType)
	}

	if err != nil {
		return "", err
	}

	// Human-readable output
	if !outputJSON {
		switch msgType {
		case msgTypeOpen:
			return formatOpenHuman(result), nil
		case msgTypeUpdate:
			return formatUpdateHuman(result), nil
		case msgTypeNotification:
			return formatNotificationHuman(result), nil
		case msgTypeKeepalive:
			return "KEEPALIVE", nil
		}
	}

	// Ze format: {"type": "bgp", "bgp": {"type": "<event>", "peer": {...}, "<event>": {...}}}.
	envelope := makeZeEnvelope(msgType)
	bgp, _ := envelope["bgp"].(map[string]any)

	// Merge event-specific content into bgp.<event> section
	if eventContent, ok := result[msgType].(map[string]any); ok {
		bgp[msgType] = eventContent
	} else {
		// Fallback: use result directly as event content
		bgp[msgType] = result
	}

	jsonData, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}

	return string(jsonData), nil
}

// detectMessageType reads the BGP message type from the header.
func detectMessageType(data []byte) string {
	if len(data) < message.HeaderLen {
		return msgTypeUpdate
	}
	switch data[18] {
	case 1:
		return msgTypeOpen
	case 2:
		return msgTypeUpdate
	case 3:
		return msgTypeNotification
	case 4:
		return msgTypeKeepalive
	default:
		return msgTypeUpdate
	}
}

// makeZeEnvelope creates the Ze ze-bgp JSON envelope structure.
// Ze format: {"type": "bgp", "bgp": {"peer": {...}, "message": {..., "type": "<event>"}, "<event>": {...}}}.
// The message type can be determined either from message.type or by checking which key exists (open/update).
func makeZeEnvelope(msgType string) map[string]any {
	return map[string]any{
		"type": "bgp",
		"bgp": map[string]any{
			"peer": map[string]any{
				"address": "127.0.0.1",
				"remote":  map[string]any{"as": 65533},
			},
			"message": map[string]any{
				"id":        0,
				"direction": "received",
				"type":      msgType,
			},
		},
	}
}

// hasValidMarker checks if data has the BGP marker (16 0xFF bytes).
func hasValidMarker(data []byte) bool {
	if len(data) < 16 {
		return false
	}
	for i := range 16 {
		if data[i] != 0xFF {
			return false
		}
	}
	return true
}
