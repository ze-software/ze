// Design: docs/architecture/plugin/rib-storage-design.md — BGP event parsing
// Related: route.go — Route struct used by event consumers
// Related: format.go — route command formatting
// Related: nlri.go — NLRI value parsing
package bgp

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var eventLogger = slogutil.LazyLogger("bgp.event")

// KnownFields are the standard Event fields that are not family operations.
// Note: "direction" is inside message wrapper, not at root level.
var KnownFields = map[string]bool{
	"type": true, "msg-id": true, "message": true,
	"peer": true, "state": true, "origin": true, "as-path": true,
	"med": true, "local-preference": true, "communities": true,
	"large-communities": true, "extended-communities": true, "aigp": true,
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
	// jsonKey stays a string for rawPayload map lookup; eventKind is the typed value.
	jsonKey := bgpPayload.Type
	if jsonKey == "" && bgpPayload.Message != nil && bgpPayload.Message.Type != rpc.EventKindUnspecified {
		jsonKey = bgpPayload.Message.Type.String()
		// For "sent" type, the nested data is under "update".
		if bgpPayload.Message.Type == rpc.EventKindSent {
			jsonKey = "update"
		}
	}

	// Start with the full bgp payload to get peer.
	var event Event
	if err := json.Unmarshal(payloadData, &event); err != nil {
		return nil, err
	}

	// For non-state events, merge in the nested event data.
	if jsonKey != "" && jsonKey != "state" {
		var rawPayload map[string]json.RawMessage
		if err := json.Unmarshal(payloadData, &rawPayload); err == nil {
			// Extract raw wire bytes BEFORE narrowing payloadData.
			// The "raw" key (format=full) is at the bgp level, sibling of "update".
			// After narrowing, payloadData points inside "update" where "raw" doesn't exist.
			if rawData, ok := rawPayload["raw"]; ok {
				parseRawFields(&event, rawData)
			}
			if nestedData, ok := rawPayload[jsonKey]; ok && len(nestedData) > 0 {
				// Only use nested data if it's an object (starts with '{'), not a string.
				if len(nestedData) > 0 && nestedData[0] == '{' {
					// Merge nested data into event (this adds attr, nlri, message, etc.).
					_ = json.Unmarshal(nestedData, &event)
					payloadData = nestedData
				}
			}
		}
	}

	// Preserve the event type parsed from the outer envelope.
	if jsonKey != "" {
		event.Type = jsonKey
		var ek rpc.EventKind
		_ = ek.UnmarshalText([]byte(jsonKey))
		event.TypeKind = ek
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
		AIGP                *uint64  `json:"aigp,omitempty"`
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
		if attrs.AIGP != nil {
			event.AIGP = attrs.AIGP
		}
	}
}

// parseRawFields extracts raw wire bytes from the "raw" JSON key (format=full).
// Per-family maps use string keys on the wire (registered "afi/safi" names) and
// are converted to family.Family keys here. Unknown families are dropped and
// aggregated into a single debug log per field -- same convention as
// gr.handleEOR and rs.parseTextUpdateFamilies, minus the per-key spam.
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
			if b, err := hex.DecodeString(rawFields.Attributes); err == nil {
				event.RawAttributeBytes = b
			}
		}
		if len(rawFields.NLRI) > 0 {
			event.RawNLRI = convertRawFamilyMap(rawFields.NLRI, "raw-nlri")
			event.RawNLRIBytes = decodeHexFamilyMap(event.RawNLRI)
		}
		if len(rawFields.Withdrawn) > 0 {
			event.RawWithdrawn = convertRawFamilyMap(rawFields.Withdrawn, "raw-withdrawn")
			event.RawWithdrawnBytes = decodeHexFamilyMap(event.RawWithdrawn)
		}
		if len(rawFields.AddPath) > 0 {
			event.AddPath = convertRawFamilyMap(rawFields.AddPath, "add-path")
		}
	}
}

// decodeHexFamilyMap decodes all hex strings in a family map to bytes.
// Invalid hex entries are silently skipped.
func decodeHexFamilyMap(in map[family.Family]string) map[family.Family][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make(map[family.Family][]byte, len(in))
	for fam, hexStr := range in {
		if b, err := hex.DecodeString(hexStr); err == nil {
			out[fam] = b
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// convertRawFamilyMap converts a JSON-decoded "afi/safi" -> value map to a
// family.Family-keyed map. Unregistered families are dropped and aggregated
// into a single debug log per call (one log regardless of how many keys
// were skipped). field names the source map for the log only.
func convertRawFamilyMap[V any](in map[string]V, field string) map[family.Family]V {
	if len(in) == 0 {
		return nil
	}
	out := make(map[family.Family]V, len(in))
	var dropped []string
	for k, v := range in {
		fam, ok := family.LookupFamily(k)
		if !ok {
			dropped = append(dropped, k)
			continue
		}
		out[fam] = v
	}
	if len(dropped) > 0 {
		eventLogger().Debug("dropped unregistered families", "field", field, "count", len(dropped), "keys", dropped)
	}
	return out
}

// ParseFamilyOps extracts family operations from JSON data into the event.
// Dynamic keys with the shape "afi/safi" are resolved via family.LookupFamily;
// unregistered families are dropped and aggregated into a single debug log.
func ParseFamilyOps(event *Event, data []byte) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}

	var dropped []string
	for key, val := range raw {
		if KnownFields[key] {
			continue
		}
		// Family keys contain "/" (e.g., "ipv4/unicast", "ipv6/unicast").
		if !strings.Contains(key, "/") {
			continue
		}

		fam, ok := family.LookupFamily(key)
		if !ok {
			dropped = append(dropped, key)
			continue
		}

		// Parse as array of FamilyOperation.
		var ops []FamilyOperation
		if err := json.Unmarshal(val, &ops); err != nil {
			continue // Skip if not valid operation array
		}

		if event.FamilyOps == nil {
			event.FamilyOps = make(map[family.Family][]FamilyOperation)
		}
		event.FamilyOps[fam] = ops
	}

	if len(dropped) > 0 {
		eventLogger().Debug("dropped unregistered families", "field", "nlri", "count", len(dropped), "keys", dropped)
	}
}

// Event represents a JSON event from ze.
// Handles both sent events (flat format) and received events (nested format).
// Event is single-owner: each instance is consumed by one goroutine.
// Lazy accessors (GetRawAttributesHex, GetRawNLRIHex) cache derived values
// by mutating string fields. Do not share an Event across goroutines.
type Event struct {
	// Sent events use top-level type (string: may carry non-BGP event types like "cache", "request").
	Type     string        `json:"type"`
	TypeKind rpc.EventKind `json:"-"`
	MsgID    uint64        `json:"msg-id"`

	// Received events use message wrapper (includes type, id, direction).
	Message *MessageInfo `json:"message,omitempty"`

	// Peer info - uses RawMessage to handle both flat and nested formats.
	Peer json.RawMessage `json:"peer"`

	// State event field.
	State string `json:"state,omitempty"`

	// UPDATE fields - new command-style format.
	// Family operations are parsed from raw JSON (dynamic keys like "ipv4/unicast").
	// Format: {"ipv4/unicast": [{"next-hop": "...", "action": "add", "nlri": [...]}]}.
	// Keys are typed family.Family -- the JSON "afi/safi" string is resolved via
	// family.LookupFamily at parse time and unregistered families are dropped.
	FamilyOps map[family.Family][]FamilyOperation `json:"-"` // Populated by ParseEvent

	// Path attributes at top level.
	Origin              string   `json:"origin,omitempty"`
	ASPath              []uint32 `json:"as-path,omitempty"`
	MED                 *uint32  `json:"med,omitempty"`
	LocalPreference     *uint32  `json:"local-preference,omitempty"`
	Communities         []string `json:"communities,omitempty"`
	LargeCommunities    []string `json:"large-communities,omitempty"`
	ExtendedCommunities []string `json:"extended-communities,omitempty"`
	AIGP                *uint64  `json:"aigp,omitempty"`

	// Request fields.
	Serial  string   `json:"serial,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// Route refresh fields (RFC 7313). JSON text form uses the registered
	// AFI/SAFI names (e.g. "ipv4"/"unicast") via family.AFI/SAFI TextMarshaler.
	AFI  family.AFI  `json:"afi,omitempty"`
	SAFI family.SAFI `json:"safi,omitempty"`

	// Pool storage raw fields (format=full only).
	// Hex-encoded wire bytes for pool-based storage. The per-family maps use
	// typed family.Family keys; ParseEvent resolves JSON "afi/safi" strings via
	// family.LookupFamily and drops entries whose family is not registered.
	RawAttributes string                   `json:"raw-attributes,omitempty"` // Path attributes (without MP_REACH/UNREACH)
	RawNLRI       map[family.Family]string `json:"-"`                        // family -> hex bytes; populated by ParseEvent
	RawWithdrawn  map[family.Family]string `json:"-"`                        // family -> hex bytes; populated by ParseEvent

	// Decoded byte caches: set directly by structured producers to skip the
	// hex encode/decode round-trip. Accessors return these when present,
	// falling back to hex decoding the string fields for JSON-sourced events.
	RawAttributeBytes []byte                   `json:"-"`
	RawNLRIBytes      map[family.Family][]byte `json:"-"`
	RawWithdrawnBytes map[family.Family][]byte `json:"-"`

	// RFC 7911: ADD-PATH per-family flags from negotiated capabilities (format=full only).
	// When true for a family, NLRI wire bytes include 4-byte path-ID prefix.
	AddPath map[family.Family]bool `json:"-"` // Populated by parseRawFields from raw.add-path

	// RouteMeta carries route-level metadata through events (e.g., "src-role" for OTC filtering).
	// Set from ReceivedUpdate.Meta on sent events; stored on Route.Meta in ribOut.
	RouteMeta map[string]any `json:"route-meta,omitempty"`
}

// FamilyOperation represents a single add or del operation for a family.
// RFC 7911: nlri items may have path-id when ADD-PATH is negotiated.
type FamilyOperation struct {
	NextHop string             `json:"next-hop,omitempty"` // Only for "add" operations
	Action  routeaction.Action `json:"action"`
	NLRIs   []any              `json:"nlri"` // Strings or {"prefix":"...", "path-id":N}
}

// MessageInfo contains message wrapper for received events.
type MessageInfo struct {
	Type      rpc.EventKind        `json:"type"`
	ID        uint64               `json:"id,omitempty"`
	Direction rpc.MessageDirection `json:"direction,omitempty"`
}

// GetEventType returns unified event type.
// For received events, uses message.type. For sent events, uses cached TypeKind
// (populated by ParseEvent), falling back to parsing Type on first call.
func (e *Event) GetEventType() rpc.EventKind {
	if e.Message != nil && e.Message.Type != rpc.EventKindUnspecified {
		return e.Message.Type
	}
	if e.TypeKind == rpc.EventKindUnspecified && e.Type != "" {
		_ = e.TypeKind.UnmarshalText([]byte(e.Type))
	}
	return e.TypeKind
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
		return e.Message.Direction.String()
	}
	return ""
}

// PeerRemoteInfo holds the remote peer identity (YANG: container remote).
type PeerRemoteInfo struct {
	Address string `json:"address,omitempty"`
	AS      uint32 `json:"as"`
}

// PeerLocalInfo holds local peer identity (YANG: container local).
type PeerLocalInfo struct {
	Address string `json:"address,omitempty"`
	AS      uint32 `json:"as,omitempty"`
}

// PeerInfoJSON is the YANG-aligned peer format for all events.
// Both local and remote are always present with address + as.
type PeerInfoJSON struct {
	Name   string         `json:"name,omitempty"`
	Group  string         `json:"group,omitempty"`
	Remote PeerRemoteInfo `json:"remote"`
	Local  *PeerLocalInfo `json:"local,omitempty"`
	State  string         `json:"state,omitempty"`
}

// GetPeerAddress extracts the peer address from remote.address.
func (e *Event) GetPeerAddress() string {
	if len(e.Peer) == 0 {
		return ""
	}

	var info PeerInfoJSON
	if err := json.Unmarshal(e.Peer, &info); err == nil && info.Remote.Address != "" {
		return info.Remote.Address
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

// GetRawAttributesHex returns the hex-encoded raw attributes string.
// If only the byte field is set (structured producer), encodes lazily.
func (e *Event) GetRawAttributesHex() string {
	if e.RawAttributes != "" {
		return e.RawAttributes
	}
	if len(e.RawAttributeBytes) > 0 {
		e.RawAttributes = hex.EncodeToString(e.RawAttributeBytes)
		return e.RawAttributes
	}
	return ""
}

// GetRawNLRIHex returns the hex-encoded NLRI string for a specific family.
// If only the byte field is set (structured producer), encodes lazily.
func (e *Event) GetRawNLRIHex(fam family.Family) string {
	if e.RawNLRI != nil {
		if s, ok := e.RawNLRI[fam]; ok {
			return s
		}
	}
	if e.RawNLRIBytes != nil {
		if b, ok := e.RawNLRIBytes[fam]; ok {
			s := hex.EncodeToString(b)
			if e.RawNLRI == nil {
				e.RawNLRI = make(map[family.Family]string, 1)
			}
			e.RawNLRI[fam] = s
			return s
		}
	}
	return ""
}

// GetRawAttributesBytes returns raw attribute bytes.
// Returns the cached []byte when set by a structured producer,
// otherwise decodes the hex string field.
func (e *Event) GetRawAttributesBytes() []byte {
	if e.RawAttributeBytes != nil {
		return e.RawAttributeBytes
	}
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
// Returns the cached []byte when set by a structured producer,
// otherwise decodes the hex string field.
func (e *Event) GetRawNLRIBytes(fam family.Family) []byte {
	if e.RawNLRIBytes != nil {
		if b, ok := e.RawNLRIBytes[fam]; ok {
			return b
		}
	}
	if e.RawNLRI == nil {
		return nil
	}
	hexStr, ok := e.RawNLRI[fam]
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
// Returns the cached []byte when set by a structured producer,
// otherwise decodes the hex string field.
func (e *Event) GetRawWithdrawnBytes(fam family.Family) []byte {
	if e.RawWithdrawnBytes != nil {
		if b, ok := e.RawWithdrawnBytes[fam]; ok {
			return b
		}
	}
	if e.RawWithdrawn == nil {
		return nil
	}
	hexStr, ok := e.RawWithdrawn[fam]
	if !ok {
		return nil
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}
	return b
}

// RawNLRIFamilies returns the set of families with raw NLRI data,
// merged from both byte and string maps.
func (e *Event) RawNLRIFamilies() []family.Family {
	if len(e.RawNLRIBytes) == 0 && len(e.RawNLRI) == 0 {
		return nil
	}
	var buf [4]family.Family
	fams := buf[:0]
	for fam := range e.RawNLRIBytes {
		fams = append(fams, fam)
	}
	for fam := range e.RawNLRI {
		if _, ok := e.RawNLRIBytes[fam]; !ok {
			fams = append(fams, fam)
		}
	}
	return fams
}

// RawWithdrawnFamilies returns the set of families with raw withdrawn data,
// merged from both byte and string maps.
func (e *Event) RawWithdrawnFamilies() []family.Family {
	if len(e.RawWithdrawnBytes) == 0 && len(e.RawWithdrawn) == 0 {
		return nil
	}
	var buf [4]family.Family
	fams := buf[:0]
	for fam := range e.RawWithdrawnBytes {
		fams = append(fams, fam)
	}
	for fam := range e.RawWithdrawn {
		if _, ok := e.RawWithdrawnBytes[fam]; !ok {
			fams = append(fams, fam)
		}
	}
	return fams
}
