// Design: docs/guide/graceful-restart.md -- captured original source identity.
package gr

import (
	"encoding/json"
	"github.com/ze-software/ze/internal/core/opererr"
	"net/netip"
	"strconv"
)

// grPeer is one captured actual engine Peer. Its zero value is invalid.
// A terminal peer-removed event ends this owner, even with no active Session.
type grPeer struct {
	owner   uint64
	address netip.Addr
}

// grSource is one physical connection of a validated original Peer. Its zero
// value is invalid. Every transport DOWN/OPEN/EOR MUST construct it; identifiers
// narrow existing rights and MUST NOT be treated as send authority.
type grSource struct {
	grPeer
	connectionID uint64
}

func newGRPeer(peer string, owner uint64) (grPeer, error) {
	if owner == 0 {
		return grPeer{}, opererr.New("GR event has no original peer owner", "A live source must carry its engine-captured Peer identity; an address cannot substitute for it", "ze request reload")
	}
	address, err := netip.ParseAddr(peer)
	if err != nil {
		return grPeer{}, opererr.Wrap(err, "GR event has an invalid original peer address", "The source owner and address must be captured from the same engine Peer", "ze request reload")
	}
	return grPeer{owner: owner, address: address}, nil
}

func newGRSource(peer string, owner, connectionID uint64) (grSource, error) {
	original, err := newGRPeer(peer, owner)
	if err != nil {
		return grSource{}, err
	}
	return original.source(connectionID)
}

func (peer grPeer) source(connectionID uint64) (grSource, error) {
	if connectionID == 0 {
		return grSource{}, opererr.New("GR event has no original connection ID", "A live source must carry its engine-captured physical connection; an address or replay receipt cannot substitute for it", "ze request reload")
	}
	return grSource{grPeer: peer, connectionID: connectionID}, nil
}

func (peer grPeer) peer() string   { return peer.address.String() }
func (peer grPeer) String() string { return peer.peer() }

func grExtraJSONValue() error {
	return opererr.New("GR event has more than one JSON value", "Each delivered event must contain exactly one complete event envelope", "ze request reload")
}

// JSON events are decoded with UseNumber. Only that exact integer representation
// can become a full-width owner; no float64 or address-derived fallback exists.
func grJSONPeer(payload map[string]any) (grPeer, error) {
	peer, _ := payload["peer"].(map[string]any)
	remote, _ := peer["remote"].(map[string]any)
	address, _ := remote["address"].(string)
	number, ok := payload["peer-owner"].(json.Number)
	if !ok {
		return newGRPeer(address, 0)
	}
	owner, err := strconv.ParseUint(string(number), 10, 64)
	if err != nil {
		return grPeer{}, opererr.Wrap(err, "GR event has an invalid original peer owner", "The owner must be the unsigned integer emitted by the engine", "ze request reload")
	}
	return newGRPeer(address, owner)
}

func grJSONSource(payload map[string]any) (grSource, error) {
	peer, err := grJSONPeer(payload)
	if err != nil {
		return grSource{}, err
	}
	connection, ok := payload["connection-id"].(string)
	if !ok {
		return peer.source(0)
	}
	connectionID, err := strconv.ParseUint(connection, 10, 64)
	if err != nil {
		return grSource{}, opererr.Wrap(err, "GR event has an invalid original connection ID", "The connection ID must be the decimal unsigned integer string emitted by the engine", "ze request reload")
	}
	return peer.source(connectionID)
}

// Capability codes are octets. Direct JSON-derived unit inputs may already be
// numeric; both accepted forms are exact in this bounded range.
func grCapabilityCode(value any) (uint8, bool) {
	switch number := value.(type) {
	case json.Number:
		code, err := strconv.ParseUint(string(number), 10, 8)
		return uint8(code), err == nil
	case float64:
		if number < 0 {
			return 0, false
		}
		if number > 255 {
			return 0, false
		}
		code := uint8(number)
		return code, float64(code) == number
	default:
		// The incoming JSON numeric representation is open; other values are not codes.
		return 0, false
	}
}
