package rib

import (
	"encoding/hex"
	"encoding/json"
	"strings"
)

// knownFields are the standard Event fields that are not family operations.
// Note: "direction" is inside message wrapper, not at root level.
var knownFields = map[string]bool{
	"type": true, "msg-id": true, "message": true,
	"peer": true, "state": true, "origin": true, "as-path": true,
	"med": true, "local-preference": true, "communities": true,
	"large-communities": true, "extended-communities": true,
	"serial": true, "command": true, "args": true, "afi": true, "safi": true,
	"raw": true, // format=full includes raw bytes
	// Pool storage raw fields (format=full)
	"raw-attributes": true, "raw-nlri": true, "raw-withdrawn": true,
	// IPC 2.0 wrapper keys
	"bgp": true, "rib": true,
	// IPC 2.0 nested keys
	"attributes": true, "nlri": true, "action": true,
}

// parseEvent parses a JSON event from ZeBGP.
// Handles both IPC 2.0 format (type/bgp or type/rib wrapper) and legacy flat format.
// Extracts family operations (ipv4/unicast, ipv6/unicast, etc.) from dynamic keys.
func parseEvent(data []byte) (*Event, error) {
	// First check if this is IPC 2.0 format (has "bgp" or "rib" wrapper)
	var wrapper struct {
		Type string          `json:"type"`
		BGP  json.RawMessage `json:"bgp"`
		RIB  json.RawMessage `json:"rib"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	// If IPC 2.0 format, parse the nested payload
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

	// Use nested payload or original data
	if payloadData == nil {
		payloadData = data
	}

	// Parse the event from payload
	var event Event
	if err := json.Unmarshal(payloadData, &event); err != nil {
		return nil, err
	}

	// Parse raw JSON to extract family operations and IPC 2.0 nested structures
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payloadData, &raw); err != nil {
		return &event, nil //nolint:nilerr // Return event without family ops if parsing fails
	}

	// IPC 2.0: attributes nested under "attributes" key
	if attrsData, ok := raw["attributes"]; ok {
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

	// IPC 2.0: NLRIs nested under "nlri" key
	if nlriData, ok := raw["nlri"]; ok {
		parseFamilyOps(&event, nlriData)
	}

	// IPC 2.0: raw bytes nested under "raw" key (format=full)
	if rawData, ok := raw["raw"]; ok {
		var rawFields struct {
			Attributes string            `json:"attributes,omitempty"`
			NLRI       map[string]string `json:"nlri,omitempty"`
			Withdrawn  map[string]string `json:"withdrawn,omitempty"`
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
		}
	}

	// Legacy format: Look for family keys at root level (format: "afi/safi")
	parseFamilyOps(&event, payloadData)

	return &event, nil
}

// parseFamilyOps extracts family operations from JSON data into the event.
func parseFamilyOps(event *Event, data []byte) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	for key, val := range raw {
		if knownFields[key] {
			continue
		}
		// Family keys contain "/" (e.g., "ipv4/unicast", "ipv6/unicast")
		if !strings.Contains(key, "/") {
			continue
		}

		// Parse as array of FamilyOperation
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

// Event represents a JSON event from ZeBGP.
// Handles both sent events (flat format) and received events (nested format).
type Event struct {
	// Sent events use top-level type
	Type  string `json:"type"`
	MsgID uint64 `json:"msg-id"`

	// Received events use message wrapper (includes type, id, direction)
	Message *MessageInfo `json:"message,omitempty"`

	// Peer info - uses RawMessage to handle both flat and nested formats
	Peer json.RawMessage `json:"peer"`

	// State event field
	State string `json:"state,omitempty"`

	// UPDATE fields - new command-style format
	// Family operations are parsed from raw JSON (dynamic keys like "ipv4/unicast")
	// Format: {"ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]}
	FamilyOps map[string][]FamilyOperation `json:"-"` // Populated by parseEvent

	// Path attributes at top level
	Origin              string   `json:"origin,omitempty"`
	ASPath              []uint32 `json:"as-path,omitempty"`
	MED                 *uint32  `json:"med,omitempty"`
	LocalPreference     *uint32  `json:"local-preference,omitempty"`
	Communities         []string `json:"communities,omitempty"`
	LargeCommunities    []string `json:"large-communities,omitempty"`
	ExtendedCommunities []string `json:"extended-communities,omitempty"`

	// Request fields
	Serial  string   `json:"serial,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// Route refresh fields (RFC 7313)
	AFI  string `json:"afi,omitempty"`
	SAFI string `json:"safi,omitempty"`

	// Pool storage raw fields (format=full only)
	// Hex-encoded wire bytes for pool-based storage
	RawAttributes string            `json:"raw-attributes,omitempty"` // Path attributes (without MP_REACH/UNREACH)
	RawNLRI       map[string]string `json:"raw-nlri,omitempty"`       // family → hex bytes
	RawWithdrawn  map[string]string `json:"raw-withdrawn,omitempty"`  // family → hex bytes
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

// PeerInfoFlat is the flat peer format (sent events, state events).
type PeerInfoFlat struct {
	Address string `json:"address"`
	ASN     uint32 `json:"asn"`
}

// PeerInfoNested is the nested peer format (received events).
type PeerInfoNested struct {
	Address struct {
		Local string `json:"local"`
		Peer  string `json:"peer"`
	} `json:"address"`
	ASN struct {
		Local uint32 `json:"local"`
		Peer  uint32 `json:"peer"`
	} `json:"asn"`
	State string `json:"state,omitempty"`
}

// GetPeerAddress extracts the peer address from either format.
func (e *Event) GetPeerAddress() string {
	if len(e.Peer) == 0 {
		return ""
	}

	// Try flat format first (sent events, state events)
	var flat PeerInfoFlat
	if err := json.Unmarshal(e.Peer, &flat); err == nil && flat.Address != "" {
		return flat.Address
	}

	// Try nested format (received events)
	var nested PeerInfoNested
	if err := json.Unmarshal(e.Peer, &nested); err == nil && nested.Address.Peer != "" {
		return nested.Address.Peer
	}

	return ""
}

// GetPeerASN extracts the peer ASN from either format.
func (e *Event) GetPeerASN() uint32 {
	if len(e.Peer) == 0 {
		return 0
	}

	// Try flat format first
	var flat PeerInfoFlat
	if err := json.Unmarshal(e.Peer, &flat); err == nil && flat.ASN > 0 {
		return flat.ASN
	}

	// Try nested format
	var nested PeerInfoNested
	if err := json.Unmarshal(e.Peer, &nested); err == nil && nested.ASN.Peer > 0 {
		return nested.ASN.Peer
	}

	return 0
}

// GetPeerState extracts peer state from nested format (state events in new format).
func (e *Event) GetPeerState() string {
	// First check top-level State field
	if e.State != "" {
		return e.State
	}

	// Check nested format
	if len(e.Peer) == 0 {
		return ""
	}

	var nested PeerInfoNested
	if err := json.Unmarshal(e.Peer, &nested); err == nil && nested.State != "" {
		return nested.State
	}

	return ""
}

// GetPeerSelector extracts peer selector string for request events.
// For request events, ZeBGP sends peer as a JSON string (the selector).
// Returns empty string if not a request event or no selector specified.
func (e *Event) GetPeerSelector() string {
	if len(e.Peer) == 0 {
		return ""
	}

	// For request events, peer is a JSON string
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
