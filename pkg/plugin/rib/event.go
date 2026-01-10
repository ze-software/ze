package rib

import "encoding/json"

// parseEvent parses a JSON event from ZeBGP.
func parseEvent(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Event represents a JSON event from ZeBGP.
// Handles both sent events (flat format) and received events (nested format).
type Event struct {
	// Sent events use top-level type
	Type  string `json:"type"`
	MsgID uint64 `json:"msg-id"`

	// Received events use message wrapper
	Message *MessageInfo `json:"message,omitempty"`

	// Direction for received events
	Direction string `json:"direction,omitempty"`

	// Peer info - uses RawMessage to handle both flat and nested formats
	Peer json.RawMessage `json:"peer"`

	// State event field
	State string `json:"state,omitempty"`

	// UPDATE fields - announce/withdraw are at top level for both formats
	// RFC 7911: NLRIs can be {"prefix":"...", "path-id":N} or legacy string format
	Announce map[string]map[string]any `json:"announce,omitempty"`
	Withdraw map[string][]any          `json:"withdraw,omitempty"`

	// Path attributes at top level (same level as announce/withdraw)
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
}

// MessageInfo contains message wrapper for received events.
type MessageInfo struct {
	Type string `json:"type"`
	ID   uint64 `json:"id,omitempty"`
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
