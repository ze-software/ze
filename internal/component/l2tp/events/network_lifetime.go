// Design: docs/architecture/l2tp/subscriber-session-model.md -- one accounting lifetime per LCP network phase.
package events

import (
	"errors"
	"math"
	"strconv"
)

// NetworkLifetime scopes a measurement to one network phase of an existing
// Birth. It is a copied event value, never mutation authority. The zero value
// is absent. The PPP worker MUST retain each result through that network's end
// and advance it only before publishing the next network's first assignment.
type NetworkLifetime struct{ sequence uint64 }

// Next advances this admitted worker's sequence without recycling old values.
// The caller MUST pair the result with its unchanged complete logical Birth.
func (lifetime NetworkLifetime) Next() (NetworkLifetime, error) {
	if lifetime.sequence == math.MaxUint64 {
		return NetworkLifetime{}, errors.New("l2tp: network lifetime sequence exhausted")
	}
	return NetworkLifetime{sequence: lifetime.sequence + 1}, nil
}

// IsValid distinguishes an absent network phase from its first measurement.
func (lifetime NetworkLifetime) IsValid() bool { return lifetime.sequence != 0 }

// MarshalText carries the typed sequence through the JSON EventBus boundary.
func (lifetime NetworkLifetime) MarshalText() ([]byte, error) {
	if !lifetime.IsValid() {
		return nil, errors.New("l2tp: cannot encode absent network lifetime")
	}
	return strconv.AppendUint(nil, lifetime.sequence, 10), nil
}

// UnmarshalText accepts only canonical positive decimal. A rejected value leaves
// the receiver unchanged. Twenty decimal digits bound the uint64 conversion.
func (lifetime *NetworkLifetime) UnmarshalText(text []byte) error {
	if lifetime == nil {
		return errors.New("l2tp: nil network lifetime receiver")
	}
	if len(text) == 0 {
		return errors.New("l2tp: absent network lifetime")
	}
	if len(text) > 20 {
		return errors.New("l2tp: network lifetime exceeds uint64")
	}
	if text[0] < '1' {
		return errors.New("l2tp: network lifetime must use canonical positive decimal")
	}
	if text[0] > '9' {
		return errors.New("l2tp: network lifetime must use canonical positive decimal")
	}
	sequence, err := strconv.ParseUint(string(text), 10, 64)
	if err != nil {
		return err
	}
	*lifetime = NetworkLifetime{sequence: sequence}
	return nil
}
