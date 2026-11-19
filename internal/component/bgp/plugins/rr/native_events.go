// Design: docs/architecture/plugin/rib-storage-design.md -- physical state and OPEN.
package rr

import (
	"errors"
	"strconv"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// eventNumbers validates the complete reserved header before exposing values.
// Only declared numeric metadata are parsed; no route/body token supplies it.
func eventNumbers(fields []string, start int) (map[string]uint64, int, error) {
	values := make(map[string]uint64)
	for index := start; index < len(fields); index += 2 {
		key := fields[index]
		switch key {
		case "peer-owner", "connection-id", "initial-replay", "session-generation", "destination-session":
		default:
			return values, index, nil
		}
		if _, duplicate := values[key]; duplicate {
			return nil, 0, errors.New("bgp-rr: duplicate reserved event field " + key)
		}
		if index+1 == len(fields) {
			return nil, 0, errors.New("bgp-rr: reserved event field has no value: " + key)
		}
		value, err := strconv.ParseUint(fields[index+1], 10, 64)
		if err != nil {
			return nil, 0, err
		}
		values[key] = value
	}
	return values, len(fields), nil
}

func (rr *routeReflector) textState(fields []string) error {
	if len(fields) < 7 {
		return errors.New("bgp-rr: state event has no state")
	}
	values := make(map[string]uint64)
	reason := ""
	seenReason := false
	seenFlags := make(map[string]bool)
	for index := 7; index < len(fields); {
		key := fields[index]
		if key == "selected-destination" || key == "ordinary-replay-owned" || key == "update-send-denied" {
			if seenFlags[key] {
				return errors.New("bgp-rr: duplicate state flag " + key)
			}
			if index+1 == len(fields) {
				return errors.New("bgp-rr: state flag has no value")
			}
			if _, err := strconv.ParseBool(fields[index+1]); err != nil {
				return err
			}
			seenFlags[key] = true
			index += 2
			continue
		}
		if fields[index] == "reason" {
			if seenReason {
				return errors.New("bgp-rr: duplicate state reason")
			}
			if index+1 == len(fields) {
				return errors.New("bgp-rr: state reason has no value")
			}
			reason = fields[index+1]
			seenReason = true
			index += 2
			continue
		}
		parsed, next, err := eventNumbers(fields, index)
		if err != nil {
			return err
		}
		if next == index {
			return errors.New("bgp-rr: unknown state metadata " + fields[index])
		}
		for key, value := range parsed {
			if _, duplicate := values[key]; duplicate {
				return errors.New("bgp-rr: duplicate reserved state field " + key)
			}
			values[key] = value
		}
		index = next
	}
	kind := rpc.SessionStateUnspecified
	switch fields[6] {
	case "up":
		kind = rpc.SessionStateUp
	case "down":
		kind = rpc.SessionStateDown
	default:
		return errors.New("bgp-rr: unknown state value")
	}
	return rr.handleStructuredState(&rpc.StructuredEvent{PeerAddress: fields[1], PeerOwner: values["peer-owner"], ConnectionID: values["connection-id"],
		DestinationSession: values["destination-session"], SessionGeneration: values["session-generation"], InitialReplay: values["initial-replay"], State: kind, Reason: reason})
}

func (rr *routeReflector) textOpen(fields []string) error {
	values, index, err := eventNumbers(fields, 8)
	if err != nil {
		return err
	}
	identity, err := newPeerIdentity(fields[1], values["peer-owner"], values["connection-id"])
	if err != nil {
		return err
	}
	families := make(map[family.Family]bool)
	capabilities := make(map[string]bool)
	var peerASN uint32
	hasMP := false
	for index < len(fields) {
		key := fields[index]
		index++
		switch key {
		case "asn", "router-id", "hold-time":
			if index == len(fields) {
				return errors.New("bgp-rr: OPEN field has no value")
			}
			if key == "asn" {
				// asn.Parse reads every RFC 5396 spelling, as every AS number in Ze is read.
				peerASN, err = asn.Parse(fields[index])
				if err != nil {
					return err
				}
			}
			index++
		case "cap":
			if index+1 >= len(fields) {
				return errors.New("bgp-rr: OPEN capability is incomplete")
			}
			code, err := strconv.ParseUint(fields[index], 10, 8)
			if err != nil {
				return err
			}
			name := fields[index+1]
			index += 2
			capabilities[name] = true
			if code == 1 {
				hasMP = true
				if index == len(fields) {
					return errors.New("bgp-rr: multiprotocol capability has no family")
				}
				fam, known := family.LookupFamily(fields[index])
				if known {
					families[fam] = true
				}
				index++
			} else if code == 65 {
				if index == len(fields) {
					return errors.New("bgp-rr: four-octet ASN capability has no ASN")
				}
				peerASN, err = asn.Parse(fields[index])
				if err != nil {
					return err
				}
				index++
			} else {
				// Other capability values may have several tokens. Their boundary is
				// the actual formatter's next OPEN keyword, never an address lookup.
				for index < len(fields) {
					if fields[index] == "cap" {
						break
					}
					if fields[index] == "asn" {
						break
					}
					if fields[index] == "router-id" {
						break
					}
					if fields[index] == "hold-time" {
						break
					}
					if fields[index] == "peer-owner" {
						return errors.New("bgp-rr: OPEN repeats reserved owner in its body")
					}
					if fields[index] == "connection-id" {
						return errors.New("bgp-rr: OPEN repeats reserved connection in its body")
					}
					index++
				}
			}
		default:
			return errors.New("bgp-rr: unexpected OPEN field " + key)
		}
	}
	if !hasMP {
		families[family.IPv4Unicast] = true
	}
	return rr.recordOpen(identity, peerASN, families, capabilities)
}
