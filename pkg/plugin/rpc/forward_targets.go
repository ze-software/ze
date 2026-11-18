// Design: docs/architecture/api/process-protocol.md -- original captured forwarding destinations.
package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/netip"
	"time"
)

// ForwardTarget is a raw protocol value. Its zero value is invalid. The producer
// MUST capture all three fields together. The receiver MUST validate them and
// match the numeric owner and original UP token before changing output.
type ForwardTarget struct {
	Destination        string `json:"destination"`
	DestinationOwner   uint64 `json:"destination-owner,string"`
	DestinationSession uint64 `json:"destination-session,string"`
}

func (target ForwardTarget) Address() (netip.Addr, error) {
	if target.DestinationOwner == 0 {
		return netip.Addr{}, errors.New("forward target has no original destination owner; preserve the captured peer UP")
	}
	if target.DestinationSession == 0 {
		return netip.Addr{}, errors.New("forward target has no original destination session; preserve the captured peer UP")
	}
	if endpoint, err := netip.ParseAddrPort(target.Destination); err == nil {
		return endpoint.Addr(), nil
	}
	address, err := netip.ParseAddr(target.Destination)
	if err != nil {
		return netip.Addr{}, errors.New("forward target destination is invalid; supply its captured address or address:port")
	}
	return address, nil
}

// Bound distinguishes raw variants, including an empty or malformed attempted
// bound request. No malformed constraint can fall back to unbound addressing.
func (input ForwardCachedInput) Bound() bool {
	return input.targetsPresent || input.Targets != nil || input.DeadlineMS != 0
}

func (input ForwardCachedInput) Validate() error {
	if !input.Bound() {
		return nil
	}
	if len(input.Destinations) != 0 {
		return errors.New("forward-cached mixes targets and destinations; supply one complete captured target set")
	}
	if len(input.Targets) == 0 {
		return errors.New("bound forward-cached has no targets; preserve the complete captured target set")
	}
	if input.DeadlineMS <= 0 {
		return errors.New("bound forward-cached deadline is missing; supply its positive absolute deadline")
	}
	if input.DeadlineMS <= time.Now().UnixMilli() {
		return errors.New("bound forward-cached deadline expired; send a live bounded request")
	}
	if input.DeadlineMS > time.Now().Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("bound forward-cached deadline exceeds thirty seconds; send a bounded request")
	}
	return ValidateForwardTargets(input.Targets)
}

// UnmarshalJSON preserves attempted bound scope even when targets is null or
// empty, and refuses unknown scalar constraints instead of ignoring authority.
func (input *ForwardCachedInput) UnmarshalJSON(raw []byte) error {
	var fields struct {
		IDs          []uint64        `json:"ids"`
		Destinations []string        `json:"destinations"`
		Targets      json.RawMessage `json:"targets"`
		DeadlineMS   json.RawMessage `json:"deadline-ms"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fields); err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("forward-cached input is null; supply its complete request object")
	}
	next := ForwardCachedInput{IDs: fields.IDs, Destinations: fields.Destinations, targetsPresent: fields.Targets != nil || fields.DeadlineMS != nil}
	if fields.DeadlineMS != nil {
		if bytes.Equal(bytes.TrimSpace(fields.DeadlineMS), []byte("null")) {
			return errors.New("forward-cached deadline is null; supply its positive absolute deadline")
		}
		if err := json.Unmarshal(fields.DeadlineMS, &next.DeadlineMS); err != nil {
			return err
		}
	}
	if fields.Targets != nil {
		if err := json.Unmarshal(fields.Targets, &next.Targets); err != nil {
			return err
		}
	}
	*input = next
	return nil
}

func ValidateForwardTargets(targets []ForwardTarget) error {
	if len(targets) == 0 {
		return errors.New("captured forwarding target set is empty; preserve the original set")
	}
	if len(targets) > 4096 {
		return errors.New("forward-cached target set exceeds 4096; send a bounded complete target set")
	}
	seen := make(map[uint64]struct{}, len(targets))
	for _, target := range targets {
		if _, err := target.Address(); err != nil {
			return err
		}
		if _, duplicate := seen[target.DestinationOwner]; duplicate {
			return errors.New("forward-cached repeats a destination owner; supply each captured peer once")
		}
		seen[target.DestinationOwner] = struct{}{}
	}
	return nil
}
