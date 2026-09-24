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
	Local  rpkiEventLocal  `json:"local"`
	Name   string          `json:"name"`
	Remote rpkiEventRemote `json:"remote"`
}

type rpkiEventLocal struct {
	Address string `json:"address"`
	AS      uint32 `json:"as"`
}

type rpkiEventRemote struct {
	Address string `json:"address"`
	AS      uint32 `json:"as"`
}

type rpkiEventMessage struct {
	ID   uint64 `json:"id"`
	Type string `json:"type"`
}

// buildRPKIEvent builds a JSON rpki event string for the given validation results.
// Per-prefix states are grouped under the family key. If results is nil or empty,
// the rpki section is an empty object (withdrawal).
// When aspaState != aspaStateNone, an "aspa-state" field is included.
func buildRPKIEvent(peerAddr, peerName string, peerASN uint32, msgID uint64, results map[string]map[string]uint8, aspaState uint8) string {
	rpkiSection := make(map[string]any, len(results)+1)
	for family, prefixes := range results {
		if len(prefixes) == 0 {
			continue
		}
		prefixStates := make(map[string]string, len(prefixes))
		for prefix, state := range prefixes {
			prefixStates[prefix] = validationStateString(state)
		}
		rpkiSection[family] = prefixStates
	}
	if aspaState != aspaStateNone {
		rpkiSection["aspa-state"] = aspaStateString(aspaState)
	}

	evt := rpkiEventJSON{
		Type: configRootBGP,
		BGP: rpkiEventBGP{
			Peer:    rpkiEventPeer{Name: peerName, Remote: rpkiEventRemote{Address: peerAddr, AS: peerASN}},
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
		Type: configRootBGP,
		BGP: rpkiEventBGP{
			Peer:    rpkiEventPeer{Name: peerName, Remote: rpkiEventRemote{Address: peerAddr, AS: peerASN}},
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

// The decorator correlates one RPKI event with one UPDATE by peer and MsgID.
// Neither an address family nor a prefix is a separate correlation unit.
type rpkiUpdateKey struct {
	peerAddr string
	msgID    uint64
}

type rpkiUpdateResults struct {
	peerName    string
	peerASN     uint32
	results     map[string]map[string]uint8
	unavailable bool
	aspaState   uint8
}

func collectRPKIUpdate(updates map[rpkiUpdateKey]*rpkiUpdateResults, key routeKey, route originRoute) {
	updateKey := rpkiUpdateKey{peerAddr: key.peerAddr, msgID: route.msgID}
	update := updates[updateKey]
	if update == nil {
		update = &rpkiUpdateResults{
			peerName: route.peerName, peerASN: route.peerASN,
			results:     make(map[string]map[string]uint8),
			unavailable: route.unavailable, aspaState: aspaStateNone,
		}
		updates[updateKey] = update
	}
	prefixes := update.results[key.family]
	if prefixes == nil {
		prefixes = make(map[string]uint8)
		update.results[key.family] = prefixes
	}
	prefixes[key.prefix] = route.state
	// ASPA is a path verdict shared by the UPDATE's applicable families.
	if route.aspaState != aspaStateNone {
		update.aspaState = route.aspaState
	}
}

func (rp *rPKIPlugin) emitRPKIUpdates(updates map[rpkiUpdateKey]*rpkiUpdateResults) {
	for key, update := range updates {
		rp.emitRPKIEvent(key.peerAddr, update.peerName, update.peerASN, key.msgID,
			update.results, update.unavailable, update.aspaState)
	}
}
