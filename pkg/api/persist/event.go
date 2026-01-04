package persist

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
type Event struct {
	Type  string   `json:"type"`
	MsgID uint64   `json:"msg-id"`
	Peer  PeerInfo `json:"peer"`
	State string   `json:"state,omitempty"` // For state events: "up", "down"
	// UPDATE fields - announce/withdraw are at top level, not in message wrapper
	Announce map[string]map[string]any `json:"announce,omitempty"`
	Withdraw map[string][]string       `json:"withdraw,omitempty"`
	// Request fields
	Serial  string `json:"serial,omitempty"`
	Command string `json:"command,omitempty"`
}

// PeerInfo contains peer identification.
type PeerInfo struct {
	Address string `json:"address"` // Peer IP address
	ASN     uint32 `json:"asn"`     // Peer AS number
}
