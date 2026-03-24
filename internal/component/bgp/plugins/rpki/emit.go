// Design: docs/architecture/plugin/rib-storage-design.md -- RPKI validation event emission
// Overview: rpki.go -- plugin entry point calling emit after validation

package rpki

import (
	"encoding/json"
)

// Validation state JSON strings (used in rpki events and tests).
const (
	stateStringValid        = "valid"
	stateStringInvalid      = "invalid"
	stateStringNotFound     = "not-found"
	stateStringNotValidated = "not-validated"
)

// validationStateString converts a validation state to its JSON string.
func validationStateString(state uint8) string {
	switch state {
	case ValidationValid:
		return stateStringValid
	case ValidationInvalid:
		return stateStringInvalid
	case ValidationNotFound:
		return stateStringNotFound
	}
	return stateStringNotValidated
}

// rpkiEventJSON is the struct used to build RPKI validation events via json.Marshal.
// Using a struct ensures all string values are properly escaped (no injection).
type rpkiEventJSON struct {
	Type string       `json:"type"`
	BGP  rpkiEventBGP `json:"bgp"`
}

type rpkiEventBGP struct {
	Peer    rpkiEventPeer    `json:"peer"`
	Message rpkiEventMessage `json:"message"`
	RPKI    any              `json:"rpki"`
}

type rpkiEventPeer struct {
	Address string          `json:"address"`
	Name    string          `json:"name"`
	Remote  rpkiEventRemote `json:"remote"`
}

type rpkiEventRemote struct {
	AS uint32 `json:"as"`
}

type rpkiEventMessage struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
}

// buildRPKIEvent builds a JSON rpki event string for the given validation results.
// Per-prefix states are grouped under the family key. If results is nil or empty,
// the rpki section is an empty object (withdrawal).
func buildRPKIEvent(peerAddr, peerName string, peerASN uint32, msgID uint64, family string, results map[string]uint8) string {
	// Build per-prefix state strings.
	var rpkiSection any
	if len(results) == 0 {
		rpkiSection = map[string]any{}
	} else {
		prefixStates := make(map[string]string, len(results))
		for prefix, state := range results {
			prefixStates[prefix] = validationStateString(state)
		}
		rpkiSection = map[string]any{family: prefixStates}
	}

	evt := rpkiEventJSON{
		Type: "bgp",
		BGP: rpkiEventBGP{
			Peer:    rpkiEventPeer{Address: peerAddr, Name: peerName, Remote: rpkiEventRemote{AS: peerASN}},
			Message: rpkiEventMessage{ID: msgID, Type: "rpki"},
			RPKI:    rpkiSection,
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		// Should never happen with these types, but fail safe.
		return `{"type":"bgp","bgp":{"rpki":{}}}`
	}
	return string(data)
}

// buildRPKIEventUnavailable builds a JSON rpki event with "unavailable" status.
// Emitted when the ROA cache is empty or expired. The rpki field is an object
// with a "status" key (not a bare string) so consumers always get an object.
func buildRPKIEventUnavailable(peerAddr, peerName string, peerASN uint32, msgID uint64) string {
	evt := rpkiEventJSON{
		Type: "bgp",
		BGP: rpkiEventBGP{
			Peer:    rpkiEventPeer{Address: peerAddr, Name: peerName, Remote: rpkiEventRemote{AS: peerASN}},
			Message: rpkiEventMessage{ID: msgID, Type: "rpki"},
			RPKI:    map[string]string{"status": "unavailable"},
		},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		return `{"type":"bgp","bgp":{"rpki":{"status":"unavailable"}}}`
	}
	return string(data)
}
