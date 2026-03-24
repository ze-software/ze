// Design: docs/architecture/plugin/rib-storage-design.md — BGP event parsing
// Related: route.go — Route struct used by event consumers
// Related: format.go — route command formatting
// Related: nlri.go — NLRI value parsing
package bgp

import (
	"encoding/hex"
	"encoding/json"
	"strings"
)

// KnownFields are the standard Event fields that are not family operations.
// Note: "direction" is inside message wrapper, not at root level.
var KnownFields = map[string]bool{
	"type": true, "msg-id": true, "message": true,
	"peer": true, "state": true, "origin": true, "as-path": true,
	"med": true, "local-preference": true, "communities": true,
	"large-communities": true, "extended-communities": true,
	"serial": true, "command": true, "args": true, "afi": true, "safi": true,
	"raw": true, // format=full includes raw bytes
	// Pool storage raw fields (format=full).
	"raw-attributes": true, "raw-nlri": true, "raw-withdrawn": true,
	// ze-bgp JSON wrapper keys.
	"bgp": true, "rib": true,
	// ze-bgp JSON nested keys (event data nested under event type).
	"attributes": true, "nlri": true, "action": true, "attr": true,
	// ze-bgp JSON event type keys (events nested under their type name).
	"update": true, "notification": true, "open": true, "keepalive": true,
	"refresh": true, "borr": true, "eorr": true, "negotiated": true,
}

// ParseEvent parses a JSON event from ze.
// Handles ze-bgp JSON format where events are nested under their event type:
//
//	{"type":"bgp","bgp":{"type":"update","update":{...}}}
//	{"type":"bgp","bgp":{"type":"state","state":{...}}}
//
// Extracts family operations (ipv4/unicast, ipv6/unicast, etc.) from dynamic keys.
func ParseEvent(data []byte) (*Event, error) {
	// First check if this is ze-bgp JSON format (has "bgp" or "rib" wrapper).
	var wrapper struct {
		Type string          `json:"type"`
		BGP  json.RawMessage `json:"bgp"`
		RIB  json.RawMessage `json:"rib"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	// If ze-bgp JSON format, parse the nested payload.
	var payloadData []byte
	switch wrapper.Type {
	case "bgp":
		if len(wrapper.BGP) > 0 {
			payloadData = wrapper.BGP
		}
	case "rib":
		if len(wrapper.RIB) > 0 {
			payloadData = wrapper.RIB
		}
	}

	// Use nested payload or original data.
	if payloadData == nil {
		payloadData = data
	}

	// ze-bgp JSON: peer is at bgp level, event data nested under event type key.
	var bgpPayload struct {
		Type    string          `json:"type"`
		Peer    json.RawMessage `json:"peer"`
		Message *MessageInfo    `json:"message"`
	}
	_ = json.Unmarshal(payloadData, &bgpPayload)

	// Determine event type: use message.type for ze-bgp JSON format if top-level type is missing.
	eventType := bgpPayload.Type
	if eventType == "" && bgpPayload.Message != nil && bgpPayload.Message.Type != "" {
		eventType = bgpPayload.Message.Type
		// For "sent" type, we know the nested data is under "update".
		if eventType == "sent" {
			eventType = "update"
		}
	}

	// Start with the full bgp payload to get peer.
	var event Event
	if err := json.Unmarshal(payloadData, &event); err != nil {
		return nil, err
	}

	// For non-state events, merge in the nested event data.
	if eventType != "" && eventType != "state" {
		var rawPayload map[string]json.RawMessage
		if err := json.Unmarshal(payloadData, &rawPayload); err == nil {
			// Extract raw wire bytes BEFORE narrowing payloadData.
			// The "raw" key (format=full) is at the bgp level, sibling of "update".
			// After narrowing, payloadData points inside "update" where "raw" doesn't exist.
			if rawData, ok := rawPayload["raw"]; ok {
				parseRawFields(&event, rawData)
			}
			if nestedData, ok := rawPayload[eventType]; ok && len(nestedData) > 0 {
				// Only use nested data if it's an object (starts with '{'), not a string.
				if len(nestedData) > 0 && nestedData[0] == '{' {
					// Merge nested data into event (this adds attr, nlri, message, etc.).
					_ = json.Unmarshal(nestedData, &event)
					payloadData = nestedData
				}
			}
		}
	}

	// Preserve the event type.
	if eventType != "" {
		event.Type = eventType
	}
	// Preserve peer from bgp level.
	if len(bgpPayload.Peer) > 0 {
		event.Peer = bgpPayload.Peer
	}
	// Preserve message info.
	if bgpPayload.Message != nil {
		event.Message = bgpPayload.Message
	}

	// Parse raw JSON to extract family operations and ze-bgp JSON nested structures.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payloadData, &raw); err != nil {
		return &event, nil //nolint:nilerr // Return event without family ops if parsing fails
	}

	// ze-bgp JSON: attributes nested under "attributes" key.
	if attrsData, ok := raw["attributes"]; ok {
		parseAttributes(&event, attrsData)
	}

	// ze-bgp JSON: NLRIs nested under "nlri" key.
	if nlriData, ok := raw["nlri"]; ok {
		ParseFamilyOps(&event, nlriData)
	}

	// ze-bgp JSON: raw bytes nested under "raw" key (format=full).
	if rawData, ok := raw["raw"]; ok {
		parseRawFields(&event, rawData)
	}

	// Legacy format: Look for family keys at root level (format: "afi/safi").
	ParseFamilyOps(&event, payloadData)

	return &event, nil
}

// parseAttributes extracts path attributes from the "attributes" JSON key.
func parseAttributes(event *Event, attrsData json.RawMessage) {
	var attrs struct {
		Origin              string   `json:"origin,omitempty"`
		ASPath              []uint32 `json:"as-path,omitempty"`
		MED                 *uint32  `json:"med,omitempty"`
		LocalPreference     *uint32  `json:"local-preference,omitempty"`
		Communities         []string `json:"communities,omitempty"`
		LargeCommunities    []string `json:"large-communities,omitempty"`
		ExtendedCommunities []string `json:"extended-communities,omitempty"`
	}
	if err := json.Unmarshal(attrsData, &attrs); err == nil {
		if attrs.Origin != "" {
			event.Origin = attrs.Origin
		}
		if len(attrs.ASPath) > 0 {
			event.ASPath = attrs.ASPath
		}
		if attrs.MED != nil {
			event.MED = attrs.MED
		}
		if attrs.LocalPreference != nil {
			event.LocalPreference = attrs.LocalPreference
		}
		if len(attrs.Communities) > 0 {
			event.Communities = attrs.Communities
		}
		if len(attrs.LargeCommunities) > 0 {
			event.LargeCommunities = attrs.LargeCommunities
		}
		if len(attrs.ExtendedCommunities) > 0 {
			event.ExtendedCommunities = attrs.ExtendedCommunities
		}
	}
}

// parseRawFields extracts raw wire bytes from the "raw" JSON key (format=full).
func parseRawFields(event *Event, rawData json.RawMessage) {
	var rawFields struct {
		Attributes string            `json:"attributes,omitempty"`
		NLRI       map[string]string `json:"nlri,omitempty"`
		Withdrawn  map[string]string `json:"withdrawn,omitempty"`
		AddPath    map[string]bool   `json:"add-path,omitempty"` // RFC 7911: per-family ADD-PATH flags
	}
	if err := json.Unmarshal(rawData, &rawFields); err == nil {
		if rawFields.Attributes != "" {
			event.RawAttributes = rawFields.Attributes
		}
		if len(rawFields.NLRI) > 0 {
			event.RawNLRI = rawFields.NLRI
		}
		if len(rawFields.Withdrawn) > 0 {
			event.RawWithdrawn = rawFields.Withdrawn
		}
		if len(rawFields.AddPath) > 0 {
			event.AddPath = rawFields.AddPath
		}
	}
}

// ParseFamilyOps extracts family operations from JSON data into the event.
func ParseFamilyOps(event *Event, data []byte) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	for key, val := range raw {
		if KnownFields[key] {
			continue
		}
		// Family keys contain "/" (e.g., "ipv4/unicast", "ipv6/unicast").
		if !strings.Contains(key, "/") {
			continue
		}

		// Parse as array of FamilyOperation.
		var ops []FamilyOperation
		if err := json.Unmarshal(val, &ops); err != nil {
			continue // Skip if not valid operation array
		}

		if event.FamilyOps == nil {
			event.FamilyOps = make(map[string][]FamilyOperation)
		}
		event.FamilyOps[key] = ops
	}
}

// Event represents a JSON event from ze.
// Handles both sent events (flat format) and received events (nested format).
type Event struct {
	// Sent events use top-level type.
	Type  string `json:"type"`
	MsgID uint64 `json:"msg-id"`

	// Received events use message wrapper (includes type, id, direction).
	Message *MessageInfo `json:"message,omitempty"`

	// Peer info - uses RawMessage to handle both flat and nested formats.
	Peer json.RawMessage `json:"peer"`

	// State event field.
	State string `json:"state,omitempty"`

	// UPDATE fields - new command-style format.
	// Family operations are parsed from raw JSON (dynamic keys like "ipv4/unicast").
	// Format: {"ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]}.
	FamilyOps map[string][]FamilyOperation `json:"-"` // Populated by ParseEvent

	// Path attributes at top level.
	Origin              string   `json:"origin,omitempty"`
	ASPath              []uint32 `json:"as-path,omitempty"`
	MED                 *uint32  `json:"med,omitempty"`
	LocalPreference     *uint32  `json:"local-preference,omitempty"`
	Communities         []string `json:"communities,omitempty"`
	LargeCommunities    []string `json:"large-communities,omitempty"`
	ExtendedCommunities []string `json:"extended-communities,omitempty"`

	// Request fields.
	Serial  string   `json:"serial,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// Route refresh fields (RFC 7313).
	AFI  string `json:"afi,omitempty"`
	SAFI string `json:"safi,omitempty"`

	// Pool storage raw fields (format=full only).
	// Hex-encoded wire bytes for pool-based storage.
	RawAttributes string            `json:"raw-attributes,omitempty"` // Path attributes (without MP_REACH/UNREACH)
	RawNLRI       map[string]string `json:"raw-nlri,omitempty"`       // family -> hex bytes
	RawWithdrawn  map[string]string `json:"raw-withdrawn,omitempty"`  // family -> hex bytes

	// RFC 7911: ADD-PATH per-family flags from negotiated capabilities (format=full only).
	// When true for a family, NLRI wire bytes include 4-byte path-ID prefix.
	AddPath map[string]bool `json:"-"` // Populated by parseRawFields from raw.add-path
}

// FamilyOperation represents a single add or del operation for a family.
// RFC 7911: nlri items may have path-id when ADD-PATH is negotiated.
type FamilyOperation struct {
	NextHop string `json:"next-hop,omitempty"` // Only for "add" operations
	Action  string `json:"action"`             // "add" or "del"
	NLRIs   []any  `json:"nlri"`               // Strings or {"prefix":"...", "path-id":N}
}

// MessageInfo contains message wrapper for received events.
type MessageInfo struct {
	Type      string `json:"type"`
	ID        uint64 `json:"id,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// GetEventType returns unified event type.
// For received events, uses message.type. For sent events, uses type.
func (e *Event) GetEventType() string {
	if e.Message != nil && e.Message.Type != "" {
		return e.Message.Type
	}
	return e.Type
}

// GetMsgID returns message ID from either format.
func (e *Event) GetMsgID() uint64 {
	if e.Message != nil && e.Message.ID > 0 {
		return e.Message.ID
	}
	return e.MsgID
}

// GetDirection returns the direction from message wrapper.
func (e *Event) GetDirection() string {
	if e.Message != nil {
		return e.Message.Direction
	}
	return ""
}

// PeerRemoteInfo holds the remote peer identity (YANG: container remote).
type PeerRemoteInfo struct {
	AS uint32 `json:"as"`
}

// PeerLocalInfo holds local peer identity (YANG: container local).
type PeerLocalInfo struct {
	Address string `json:"address,omitempty"`
	AS      uint32 `json:"as,omitempty"`
}

// PeerInfoJSON is the YANG-aligned peer format for all events.
// Flat events (state, sent) omit Local. Full events include Local.
type PeerInfoJSON struct {
	Address string         `json:"address"`
	Name    string         `json:"name,omitempty"`
	Group   string         `json:"group,omitempty"`
	Remote  PeerRemoteInfo `json:"remote"`
	Local   *PeerLocalInfo `json:"local,omitempty"`
	State   string         `json:"state,omitempty"`
}

// GetPeerAddress extracts the peer address (YANG: address leaf).
func (e *Event) GetPeerAddress() string {
	if len(e.Peer) == 0 {
		return ""
	}

	var info PeerInfoJSON
	if err := json.Unmarshal(e.Peer, &info); err == nil && info.Address != "" {
		return info.Address
	}

	return ""
}

// GetPeerASN extracts the remote peer ASN (YANG: remote.as).
func (e *Event) GetPeerASN() uint32 {
	if len(e.Peer) == 0 {
		return 0
	}

	var info PeerInfoJSON
	if err := json.Unmarshal(e.Peer, &info); err == nil && info.Remote.AS > 0 {
		return info.Remote.AS
	}

	return 0
}

// GetPeerName extracts the peer name (YANG: name leaf).
func (e *Event) GetPeerName() string {
	if len(e.Peer) == 0 {
		return ""
	}

	var info PeerInfoJSON
	if err := json.Unmarshal(e.Peer, &info); err == nil {
		return info.Name
	}

	return ""
}

// GetPeerState extracts peer state from the peer object or top-level State field.
func (e *Event) GetPeerState() string {
	if e.State != "" {
		return e.State
	}

	if len(e.Peer) == 0 {
		return ""
	}

	var info PeerInfoJSON
	if err := json.Unmarshal(e.Peer, &info); err == nil && info.State != "" {
		return info.State
	}

	return ""
}

// GetPeerSelector extracts peer selector string for request events.
// For request events, ze sends peer as a JSON string (the selector).
// Returns empty string if not a request event or no selector specified.
func (e *Event) GetPeerSelector() string {
	if len(e.Peer) == 0 {
		return ""
	}

	// For request events, peer is a JSON string.
	var selector string
	if err := json.Unmarshal(e.Peer, &selector); err == nil {
		return selector
	}

	return ""
}

// GetRawAttributesBytes decodes RawAttributes from hex string.
// Returns nil if not present or invalid hex.
func (e *Event) GetRawAttributesBytes() []byte {
	if e.RawAttributes == "" {
		return nil
	}
	b, err := hex.DecodeString(e.RawAttributes)
	if err != nil {
		return nil
	}
	return b
}

// GetRawNLRIBytes returns decoded NLRI bytes for a specific family.
// Returns nil if not present or invalid hex.
func (e *Event) GetRawNLRIBytes(family string) []byte {
	if e.RawNLRI == nil {
		return nil
	}
	hexStr, ok := e.RawNLRI[family]
	if !ok {
		return nil
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}
	return b
}

// GetRawWithdrawnBytes returns decoded withdrawn bytes for a specific family.
// Returns nil if not present or invalid hex.
func (e *Event) GetRawWithdrawnBytes(family string) []byte {
	if e.RawWithdrawn == nil {
		return nil
	}
	hexStr, ok := e.RawWithdrawn[family]
	if !ok {
		return nil
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}
	return b
}
