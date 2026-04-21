// Design: docs/architecture/api/json-format.md — message formatting

package format

import (
	"encoding/hex"
	"encoding/json"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// JSONEncoder produces ze-bgp JSON output.
// Format follows docs/architecture/api/ipc_protocol.md v2.0.
//
// ze-bgp JSON wraps all events:
//
//	{"type":"bgp","bgp":{"message":{"type":"update"},"peer":{...},"attr":{...},"nlri":{...}}}
//	{"type":"bgp","bgp":{"message":{"type":"state"},"peer":{...},"state":"up"}}
//
// The "type" field at top level indicates the payload key (always "bgp").
// Event type is in bgp.message.type, peer is at bgp level, event data at bgp level.
type JSONEncoder struct{}

// NewJSONEncoder creates a new JSON encoder.
// The version parameter is retained for API compatibility but no longer used.
func NewJSONEncoder(_ string) *JSONEncoder {
	return &JSONEncoder{}
}

// peerMap builds the "peer" JSON object from PeerInfo.
// Structure matches YANG peer-info grouping: address, name, remote.as, group.
// Always includes address, name, and remote.as. Includes group when non-empty.
func peerMap(peer *plugin.PeerInfo) map[string]any {
	m := map[string]any{
		"address": peer.Address.String(),
		"name":    peer.Name,
		"remote":  map[string]any{"as": peer.PeerAS},
	}
	if peer.GroupName != "" {
		m["group"] = peer.GroupName
	}
	if peer.LocalAS > 0 || peer.LocalAddress.IsValid() {
		local := map[string]any{}
		if peer.LocalAddress.IsValid() {
			local["address"] = peer.LocalAddress.String()
		}
		if peer.LocalAS > 0 {
			local["as"] = peer.LocalAS
		}
		m["local"] = local
	}
	return m
}

// message creates the BGP payload for an event.
// Returns the outer bgp payload and inner event payload.
// ze-bgp JSON Format: {"message":{"type":"<msgType>"},"peer":{...},"<msgType>":{...}}.
// Type is in message object, peer is at bgp level, event-specific data in inner map.
func (e *JSONEncoder) message(peer *plugin.PeerInfo, msgType string) (outer, inner map[string]any) {
	inner = make(map[string]any)
	outer = map[string]any{
		"message": map[string]any{
			"type": msgType,
		},
		"peer":  peerMap(peer),
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
// Accepts typed rpc.MessageDirection; calls .String() once for the map value.
func setMessageDirection(payload map[string]any, direction rpc.MessageDirection) {
	if direction != rpc.DirectionUnspecified {
		msg := getOrCreateMessage(payload)
		msg["direction"] = direction.String()
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
// ze-bgp JSON: {"message":{"type":"state"},"peer":{...},"state":"up"}.
func (e *JSONEncoder) StateUp(peer *plugin.PeerInfo) string {
	payload := map[string]any{
		"message": map[string]any{
			"type": "state",
		},
		"peer":  peerMap(peer),
		"state": "up",
	}
	return e.marshal(payload)
}

// StateDown returns JSON for a peer state "down" event.
// ze-bgp JSON: {"message":{"type":"state"},"peer":{...},"state":"down","reason":"..."}.
func (e *JSONEncoder) StateDown(peer *plugin.PeerInfo, reason string) string {
	payload := map[string]any{
		"message": map[string]any{
			"type": "state",
		},
		"peer":   peerMap(peer),
		"state":  "down",
		"reason": reason,
	}
	return e.marshal(payload)
}

// StateConnected returns JSON for a peer "connected" event.
// ze-bgp JSON: {"message":{"type":"state"},"peer":{...},"state":"connected"}.
func (e *JSONEncoder) StateConnected(peer *plugin.PeerInfo) string {
	payload := map[string]any{
		"message": map[string]any{
			"type": "state",
		},
		"peer":  peerMap(peer),
		"state": "connected",
	}
	return e.marshal(payload)
}

// EOR returns JSON for an End-of-RIB marker.
// EOR is an empty UPDATE message for a specific family.
func (e *JSONEncoder) EOR(peer *plugin.PeerInfo, family string) string {
	outer, inner := e.message(peer, "update")
	inner["eor"] = map[string]any{
		"afi":  family,
		"safi": bgptypes.SAFINameUnicast,
	}
	return e.marshal(outer)
}

// Notification returns JSON for a NOTIFICATION message.
// Fields in inner payload: code, subcode, data, code-name, subcode-name.
func (e *JSONEncoder) Notification(peer *plugin.PeerInfo, notify DecodedNotification, direction rpc.MessageDirection, msgID uint64) string {
	outer, inner := e.message(peer, "notification")
	setMessageDirection(outer, direction)
	setMessageID(outer, msgID)

	// Always include code, subcode, data fields
	inner["code"] = notify.ErrorCode
	inner["subcode"] = notify.ErrorSubcode

	inner["data"] = string(hex.AppendEncode(nil, notify.Data))

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
// Fields in inner payload: asn, router-id, timer { hold-time }, capabilities.
func (e *JSONEncoder) Open(peer *plugin.PeerInfo, open DecodedOpen, direction rpc.MessageDirection, msgID uint64) string {
	outer, inner := e.message(peer, "open")
	setMessageDirection(outer, direction)
	setMessageID(outer, msgID)

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
	inner["timer"] = map[string]any{"hold-time": open.HoldTime}
	inner["capabilities"] = caps

	return e.marshal(outer)
}

// Keepalive returns JSON for a KEEPALIVE message.
func (e *JSONEncoder) Keepalive(peer *plugin.PeerInfo, direction rpc.MessageDirection, msgID uint64) string {
	outer, _ := e.message(peer, "keepalive")
	setMessageDirection(outer, direction)
	setMessageID(outer, msgID)
	return e.marshal(outer)
}

// RouteRefresh returns JSON for a ROUTE-REFRESH message.
// RFC 7313: Type is "refresh" (subtype 0), "borr" (subtype 1), or "eorr" (subtype 2).
func (e *JSONEncoder) RouteRefresh(peer *plugin.PeerInfo, decoded DecodedRouteRefresh, direction rpc.MessageDirection, msgID uint64) string {
	// Use subtype name as event type for proper dispatch
	outer, inner := e.message(peer, decoded.SubtypeName)
	setMessageDirection(outer, direction)
	setMessageID(outer, msgID)

	// AFI / SAFI emitted as their registered names via the typed String()
	// methods (fallback to "afi-N" / "safi-N" for unregistered values).
	inner["afi"] = decoded.Family.AFI.String()
	inner["safi"] = decoded.Family.SAFI.String()
	return e.marshal(outer)
}

// Negotiated returns JSON for negotiated capabilities after OPEN exchange.
// Fields in inner payload: timer { hold-time }, asn4, families, add-path.
func (e *JSONEncoder) Negotiated(peer *plugin.PeerInfo, neg DecodedNegotiated) string {
	outer, inner := e.message(peer, "negotiated")

	// Fields in inner payload (hyphenated per json-format.md)
	inner["timer"] = map[string]any{"hold-time": neg.HoldTime}
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

// marshal wraps the payload in ze-bgp JSON format and converts to JSON string.
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
