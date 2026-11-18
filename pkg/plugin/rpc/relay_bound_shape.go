// Design: docs/architecture/api/process-protocol.md -- scalar bound relay validation.
package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/netip"
	"time"
)

// Bound distinguishes intentional unbound calls from every attempted raw
// constraint. A malformed original destination must never become unbound.
func (input RelayStoredRouteInput) Bound() bool {
	return input.constraintsPresent || input.DestinationOwner != 0 || input.DestinationSession != 0 || input.DeadlineMS != 0
}

func (input RelayStoredRouteInput) Validate() error {
	if !input.Bound() {
		return nil
	}
	if input.DestinationOwner == 0 {
		return errors.New("stored relay destination-owner is missing; preserve its original destination")
	}
	if err := ValidateCachedWrite(input.DestinationOwner, input.DestinationSession, input.DeadlineMS); err != nil {
		return err
	}
	if _, err := netip.ParseAddr(input.Destination); err != nil {
		return errors.New("stored relay destination is invalid; supply its original address")
	}
	now := time.Now()
	if input.DeadlineMS <= now.UnixMilli() {
		return errors.New("stored relay deadline expired; send a live bounded request")
	}
	if input.DeadlineMS > now.Add(CancellableRequestTimeout).UnixMilli() {
		return errors.New("stored relay deadline exceeds thirty seconds; send a bounded request")
	}
	return nil
}

// UnmarshalJSON rejects unknown keys and explicit null constraints. The alias
// retains the DTO's single owning field declarations and lossless string tags.
func (input *RelayStoredRouteInput) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("stored relay input is null; supply its complete request object")
	}
	type scalar RelayStoredRouteInput
	var next scalar
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&next); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, key := range []string{"destination-owner", "destination-session", "deadline-ms"} {
		value, present := fields[key]
		if !present {
			continue
		}
		next.constraintsPresent = true
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("stored relay destination constraint is null; preserve its complete original scope")
		}
	}
	*input = RelayStoredRouteInput(next)
	return nil
}
