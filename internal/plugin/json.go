package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONEncoder produces ze-bgp JSON output.
// Format follows docs/architecture/api/ipc_protocol.md v2.0.
//
// IPC Protocol 2.0 wraps all events:
//
//	{"type":"bgp","bgp":{"type":"update","peer":{...},"update":{...}}}
//	{"type":"bgp","bgp":{"type":"state","peer":{...},"state":"up"}}
//
// The "type" field at top level indicates the payload key.
// Event type is in bgp.type, peer is at bgp level, event data in bgp.<type>.
// Exception: state events use simple "state":"<value>" at bgp level.
type JSONEncoder struct{}

// NewJSONEncoder creates a new JSON encoder.
// The version parameter is retained for API compatibility but no longer used.
func NewJSONEncoder(_ string) *JSONEncoder {
	return &JSONEncoder{}
}

// message creates the BGP payload for an event.
// Returns the outer bgp payload and inner event payload.
// Format: {"type":"<msgType>","peer":{...},"<msgType>":{...}}.
// Peer is at bgp level, event-specific data goes in the inner map.
func (e *JSONEncoder) message(peer PeerInfo, msgType string) (outer map[string]any, inner map[string]any) {
	inner = make(map[string]any)
	outer = map[string]any{
		"type": msgType,
		"peer": map[string]any{
			"address": peer.Address.String(),
			"asn":     peer.PeerAS,
		},
		msgType: inner,
	}
	return outer, inner
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
// State events use simple string value: {"type":"state","peer":{...},"state":"up"}.
func (e *JSONEncoder) StateUp(peer PeerInfo) string {
	payload := map[string]any{
		"type": "state",
		"peer": map[string]any{
			"address": peer.Address.String(),
			"asn":     peer.PeerAS,
		},
		"state": "up",
	}
	return e.marshal(payload)
}

// StateDown returns JSON for a peer state "down" event.
// State events use simple string value: {"type":"state","peer":{...},"state":"down","reason":"..."}.
func (e *JSONEncoder) StateDown(peer PeerInfo, reason string) string {
	payload := map[string]any{
		"type": "state",
		"peer": map[string]any{
			"address": peer.Address.String(),
			"asn":     peer.PeerAS,
		},
		"state":  "down",
		"reason": reason,
	}
	return e.marshal(payload)
}

// StateConnected returns JSON for a peer "connected" event.
// State events use simple string value: {"type":"state","peer":{...},"state":"connected"}.
func (e *JSONEncoder) StateConnected(peer PeerInfo) string {
	payload := map[string]any{
		"type": "state",
		"peer": map[string]any{
			"address": peer.Address.String(),
			"asn":     peer.PeerAS,
		},
		"state": "connected",
	}
	return e.marshal(payload)
}

// EOR returns JSON for an End-of-RIB marker.
// EOR is an empty UPDATE message for a specific family.
func (e *JSONEncoder) EOR(peer PeerInfo, family string) string {
	outer, inner := e.message(peer, "update")
	inner["eor"] = map[string]any{
		"afi":  family,
		"safi": SAFINameUnicast,
	}
	return e.marshal(outer)
}

// Notification returns JSON for a NOTIFICATION message.
// Fields in inner payload: code, subcode, data, code-name, subcode-name.
func (e *JSONEncoder) Notification(peer PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	outer, inner := e.message(peer, "notification")
	setMessageDirection(inner, direction)
	setMessageID(inner, msgID)

	// Always include code, subcode, data fields
	inner["code"] = notify.ErrorCode
	inner["subcode"] = notify.ErrorSubcode

	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}
	inner["data"] = dataHex

	// Human-readable names (hyphenated per json-format.md)
	if notify.ErrorCodeName != "" {
		inner["code-name"] = notify.ErrorCodeName
	}
	if notify.ErrorSubcodeName != "" {
		inner["subcode-name"] = notify.ErrorSubcodeName
	}

	return e.marshal(outer)
}

// Open returns JSON for an OPEN message.
// Fields in inner payload: asn, router-id, hold-time, capabilities.
func (e *JSONEncoder) Open(peer PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	outer, inner := e.message(peer, "open")
	setMessageDirection(inner, direction)
	setMessageID(inner, msgID)

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

	// Fields in inner payload (hyphenated per json-format.md)
	inner["asn"] = open.ASN
	inner["router-id"] = open.RouterID
	inner["hold-time"] = open.HoldTime
	inner["capabilities"] = caps

	return e.marshal(outer)
}

// Keepalive returns JSON for a KEEPALIVE message.
func (e *JSONEncoder) Keepalive(peer PeerInfo, direction string, msgID uint64) string {
	outer, inner := e.message(peer, "keepalive")
	setMessageDirection(inner, direction)
	setMessageID(inner, msgID)
	return e.marshal(outer)
}

// RouteRefresh returns JSON for a ROUTE-REFRESH message.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
func (e *JSONEncoder) RouteRefresh(peer PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	// Use subtype name as event type for proper dispatch
	outer, inner := e.message(peer, decoded.SubtypeName)
	setMessageDirection(inner, direction)
	setMessageID(inner, msgID)

	// Parse family "afi/safi" into separate fields
	if idx := strings.Index(decoded.Family, "/"); idx >= 0 {
		inner["afi"] = decoded.Family[:idx]
		inner["safi"] = decoded.Family[idx+1:]
	} else {
		// Fallback if format is unexpected
		inner["afi"] = decoded.Family
		inner["safi"] = ""
	}
	return e.marshal(outer)
}

// Negotiated returns JSON for negotiated capabilities after OPEN exchange.
// Fields in inner payload: hold-time, asn4, families, add-path.
func (e *JSONEncoder) Negotiated(peer PeerInfo, neg DecodedNegotiated) string {
	outer, inner := e.message(peer, "negotiated")

	// Fields in inner payload (hyphenated per json-format.md)
	inner["hold-time"] = neg.HoldTime
	inner["asn4"] = neg.ASN4
	inner["families"] = neg.Families

	// ADD-PATH: separate send/receive lists
	if len(neg.AddPathSend) > 0 || len(neg.AddPathReceive) > 0 {
		addPath := map[string]any{}
		if len(neg.AddPathSend) > 0 {
			addPath["send"] = neg.AddPathSend
		}
		if len(neg.AddPathReceive) > 0 {
			addPath["receive"] = neg.AddPathReceive
		}
		inner["add-path"] = addPath
	}

	return e.marshal(outer)
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
