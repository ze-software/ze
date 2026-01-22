package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONEncoder produces ZeBGP JSON output.
// Format follows docs/architecture/api/ipc_protocol.md v2.0.
//
// IPC Protocol 2.0 wraps all events:
//
//	{"type":"bgp","bgp":{"type":"update",...}}
//
// The "type" field at top level indicates the payload key.
// Event type is in bgp.type (update, state, notification, etc).
type JSONEncoder struct{}

// NewJSONEncoder creates a new JSON encoder.
// The version parameter is retained for API compatibility but no longer used.
func NewJSONEncoder(_ string) *JSONEncoder {
	return &JSONEncoder{}
}

// message creates the BGP payload for an event.
// Returns the inner payload that will be wrapped by marshal().
// Format: {"type":"...","peer":{"address":"...","asn":...}}.
func (e *JSONEncoder) message(peer PeerInfo, msgType string) map[string]any {
	return map[string]any{
		"type": msgType,
		"peer": map[string]any{
			"address": peer.Address.String(),
			"asn":     peer.PeerAS,
		},
	}
}

// setMessageID sets the id field in the message metadata if msgID > 0.
// Creates the "message" object if it doesn't exist.
func setMessageID(payload map[string]any, msgID uint64) {
	if msgID > 0 {
		msg := getOrCreateMessage(payload)
		msg["id"] = msgID
	}
}

// setMessageDirection sets the direction field in the message metadata.
// Creates the "message" object if it doesn't exist.
func setMessageDirection(payload map[string]any, direction string) {
	if direction != "" {
		msg := getOrCreateMessage(payload)
		msg["direction"] = direction
	}
}

// getOrCreateMessage returns the "message" object from the payload,
// creating it if it doesn't exist.
func getOrCreateMessage(payload map[string]any) map[string]any {
	if existing, ok := payload["message"].(map[string]any); ok {
		return existing
	}
	msg := make(map[string]any)
	payload["message"] = msg
	return msg
}

// StateUp returns JSON for a peer state "up" event.
func (e *JSONEncoder) StateUp(peer PeerInfo) string {
	payload := e.message(peer, "state")
	payload["state"] = "up"
	return e.marshal(payload)
}

// StateDown returns JSON for a peer state "down" event.
func (e *JSONEncoder) StateDown(peer PeerInfo, reason string) string {
	payload := e.message(peer, "state")
	payload["state"] = "down"
	payload["reason"] = reason
	return e.marshal(payload)
}

// StateConnected returns JSON for a peer "connected" event.
func (e *JSONEncoder) StateConnected(peer PeerInfo) string {
	payload := e.message(peer, "state")
	payload["state"] = "connected"
	return e.marshal(payload)
}

// EOR returns JSON for an End-of-RIB marker.
// EOR is an empty UPDATE message for a specific family.
func (e *JSONEncoder) EOR(peer PeerInfo, family string) string {
	payload := e.message(peer, "update")
	payload["eor"] = map[string]any{
		"afi":  family,
		"safi": SAFINameUnicast,
	}
	return e.marshal(payload)
}

// Notification returns JSON for a NOTIFICATION message.
// Fields in payload: code, subcode, data, code-name, subcode-name.
func (e *JSONEncoder) Notification(peer PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	payload := e.message(peer, "notification")
	setMessageDirection(payload, direction)
	setMessageID(payload, msgID)

	// Always include code, subcode, data fields
	payload["code"] = notify.ErrorCode
	payload["subcode"] = notify.ErrorSubcode

	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}
	payload["data"] = dataHex

	// Human-readable names (hyphenated per json-format.md)
	if notify.ErrorCodeName != "" {
		payload["code-name"] = notify.ErrorCodeName
	}
	if notify.ErrorSubcodeName != "" {
		payload["subcode-name"] = notify.ErrorSubcodeName
	}

	return e.marshal(payload)
}

// Open returns JSON for an OPEN message.
// Fields in payload: asn, router-id, hold-time, capabilities.
func (e *JSONEncoder) Open(peer PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	payload := e.message(peer, "open")
	setMessageDirection(payload, direction)
	setMessageID(payload, msgID)

	// Convert capabilities to structured JSON format
	caps := make([]map[string]any, 0, len(open.Capabilities))
	for _, cap := range open.Capabilities {
		capObj := map[string]any{
			"code": cap.Code,
			"name": cap.Name,
		}
		if cap.Value != "" {
			capObj["value"] = cap.Value
		}
		caps = append(caps, capObj)
	}

	// Fields in payload (hyphenated per json-format.md)
	payload["asn"] = open.ASN
	payload["router-id"] = open.RouterID
	payload["hold-time"] = open.HoldTime
	payload["capabilities"] = caps

	return e.marshal(payload)
}

// Keepalive returns JSON for a KEEPALIVE message.
func (e *JSONEncoder) Keepalive(peer PeerInfo, direction string, msgID uint64) string {
	payload := e.message(peer, "keepalive")
	setMessageDirection(payload, direction)
	setMessageID(payload, msgID)
	return e.marshal(payload)
}

// RouteRefresh returns JSON for a ROUTE-REFRESH message.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
func (e *JSONEncoder) RouteRefresh(peer PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	// Use subtype name as event type for proper dispatch
	payload := e.message(peer, decoded.SubtypeName)
	setMessageDirection(payload, direction)
	setMessageID(payload, msgID)

	// Parse family "afi/safi" into separate fields
	if idx := strings.Index(decoded.Family, "/"); idx >= 0 {
		payload["afi"] = decoded.Family[:idx]
		payload["safi"] = decoded.Family[idx+1:]
	} else {
		// Fallback if format is unexpected
		payload["afi"] = decoded.Family
		payload["safi"] = ""
	}
	return e.marshal(payload)
}

// Negotiated returns JSON for negotiated capabilities after OPEN exchange.
// Fields in payload: hold-time, asn4, families, add-path.
func (e *JSONEncoder) Negotiated(peer PeerInfo, neg DecodedNegotiated) string {
	payload := e.message(peer, "negotiated")

	// Fields in payload (hyphenated per json-format.md)
	payload["hold-time"] = neg.HoldTime
	payload["asn4"] = neg.ASN4
	payload["families"] = neg.Families

	// ADD-PATH: separate send/receive lists
	if len(neg.AddPathSend) > 0 || len(neg.AddPathReceive) > 0 {
		addPath := map[string]any{}
		if len(neg.AddPathSend) > 0 {
			addPath["send"] = neg.AddPathSend
		}
		if len(neg.AddPathReceive) > 0 {
			addPath["receive"] = neg.AddPathReceive
		}
		payload["add-path"] = addPath
	}

	return e.marshal(payload)
}

// marshal wraps the payload in IPC 2.0 format and converts to JSON string.
// Output: {"type":"bgp","bgp":{...payload...}}.
func (e *JSONEncoder) marshal(payload map[string]any) string {
	outer := map[string]any{
		"type": "bgp",
		"bgp":  payload,
	}
	data, err := json.Marshal(outer)
	if err != nil {
		// Should never happen with our controlled input
		return `{"type":"bgp","bgp":{"error":"json marshal failed"}}`
	}
	return string(data)
}
