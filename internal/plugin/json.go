package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONEncoder produces ZeBGP JSON output.
// Format follows docs/architecture/api/json-format.md.
type JSONEncoder struct{}

// NewJSONEncoder creates a new JSON encoder.
// The version parameter is retained for API compatibility but no longer used.
func NewJSONEncoder(_ string) *JSONEncoder {
	return &JSONEncoder{}
}

// message creates the base message structure.
// Format: {"message":{"type":"..."},"peer":{"address":"...","asn":...}}.
func (e *JSONEncoder) message(peer PeerInfo, msgType string) map[string]any {
	return map[string]any{
		"message": map[string]any{
			"type": msgType,
		},
		"peer": map[string]any{
			"address": peer.Address.String(),
			"asn":     peer.PeerAS,
		},
	}
}

// setMessageID sets the id field in the message wrapper if msgID > 0.
func setMessageID(msg map[string]any, msgID uint64) {
	if msgID > 0 {
		if wrapper, ok := msg["message"].(map[string]any); ok {
			wrapper["id"] = msgID
		}
	}
}

// setMessageDirection sets the direction field in the message wrapper.
func setMessageDirection(msg map[string]any, direction string) {
	if direction != "" {
		if wrapper, ok := msg["message"].(map[string]any); ok {
			wrapper["direction"] = direction
		}
	}
}

// StateUp returns JSON for a peer state "up" event.
func (e *JSONEncoder) StateUp(peer PeerInfo) string {
	msg := e.message(peer, "state")
	msg["state"] = "up"
	return e.marshal(msg)
}

// StateDown returns JSON for a peer state "down" event.
func (e *JSONEncoder) StateDown(peer PeerInfo, reason string) string {
	msg := e.message(peer, "state")
	msg["state"] = "down"
	msg["reason"] = reason
	return e.marshal(msg)
}

// StateConnected returns JSON for a peer "connected" event.
func (e *JSONEncoder) StateConnected(peer PeerInfo) string {
	msg := e.message(peer, "state")
	msg["state"] = "connected"
	return e.marshal(msg)
}

// EOR returns JSON for an End-of-RIB marker.
// EOR is an empty UPDATE message for a specific family.
func (e *JSONEncoder) EOR(peer PeerInfo, family string) string {
	msg := e.message(peer, "update")
	msg["eor"] = map[string]any{
		"afi":  family,
		"safi": SAFINameUnicast,
	}
	return e.marshal(msg)
}

// Notification returns JSON for a NOTIFICATION message.
// Fields at top level: code, subcode, data, code-name, subcode-name.
func (e *JSONEncoder) Notification(peer PeerInfo, notify DecodedNotification, direction string, msgID uint64) string {
	msg := e.message(peer, "notification")
	setMessageDirection(msg, direction)
	setMessageID(msg, msgID)

	// Always include code, subcode, data fields
	msg["code"] = notify.ErrorCode
	msg["subcode"] = notify.ErrorSubcode

	dataHex := ""
	if len(notify.Data) > 0 {
		dataHex = fmt.Sprintf("%x", notify.Data)
	}
	msg["data"] = dataHex

	// Human-readable names (hyphenated per json-format.md)
	if notify.ErrorCodeName != "" {
		msg["code-name"] = notify.ErrorCodeName
	}
	if notify.ErrorSubcodeName != "" {
		msg["subcode-name"] = notify.ErrorSubcodeName
	}

	return e.marshal(msg)
}

// Open returns JSON for an OPEN message.
// Fields at top level: asn, router-id, hold-time, capabilities.
func (e *JSONEncoder) Open(peer PeerInfo, open DecodedOpen, direction string, msgID uint64) string {
	msg := e.message(peer, "open")
	setMessageDirection(msg, direction)
	setMessageID(msg, msgID)

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

	// Fields at top level (hyphenated per json-format.md)
	msg["asn"] = open.ASN
	msg["router-id"] = open.RouterID
	msg["hold-time"] = open.HoldTime
	msg["capabilities"] = caps

	return e.marshal(msg)
}

// Keepalive returns JSON for a KEEPALIVE message.
func (e *JSONEncoder) Keepalive(peer PeerInfo, direction string, msgID uint64) string {
	msg := e.message(peer, "keepalive")
	setMessageDirection(msg, direction)
	setMessageID(msg, msgID)
	return e.marshal(msg)
}

// RouteRefresh returns JSON for a ROUTE-REFRESH message.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
func (e *JSONEncoder) RouteRefresh(peer PeerInfo, decoded DecodedRouteRefresh, direction string, msgID uint64) string {
	// Use subtype name as event type for proper dispatch
	msg := e.message(peer, decoded.SubtypeName)
	setMessageDirection(msg, direction)
	setMessageID(msg, msgID)

	// Parse family "afi/safi" into separate fields at top level
	if idx := strings.Index(decoded.Family, "/"); idx >= 0 {
		msg["afi"] = decoded.Family[:idx]
		msg["safi"] = decoded.Family[idx+1:]
	} else {
		// Fallback if format is unexpected
		msg["afi"] = decoded.Family
		msg["safi"] = ""
	}
	return e.marshal(msg)
}

// Negotiated returns JSON for negotiated capabilities after OPEN exchange.
// Fields at top level: hold-time, asn4, families, add-path.
func (e *JSONEncoder) Negotiated(peer PeerInfo, neg DecodedNegotiated) string {
	msg := e.message(peer, "negotiated")

	// Fields at top level (hyphenated per json-format.md)
	msg["hold-time"] = neg.HoldTime
	msg["asn4"] = neg.ASN4
	msg["families"] = neg.Families

	// ADD-PATH: separate send/receive lists
	if len(neg.AddPathSend) > 0 || len(neg.AddPathReceive) > 0 {
		addPath := map[string]any{}
		if len(neg.AddPathSend) > 0 {
			addPath["send"] = neg.AddPathSend
		}
		if len(neg.AddPathReceive) > 0 {
			addPath["receive"] = neg.AddPathReceive
		}
		msg["add-path"] = addPath
	}

	return e.marshal(msg)
}

// marshal converts a message to JSON string.
func (e *JSONEncoder) marshal(msg map[string]any) string {
	data, err := json.Marshal(msg)
	if err != nil {
		// Should never happen with our controlled input
		return `{"error":"json marshal failed"}`
	}
	return string(data)
}
