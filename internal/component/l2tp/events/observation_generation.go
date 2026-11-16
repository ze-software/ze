// Design: docs/architecture/l2tp/l2tp-9-observer.md -- native observation scope
// Related: events.go -- lifecycle payload definitions

package events

import (
	"errors"
	"math"
	"strconv"
	"sync"
)

// ObservationGeneration scopes CQM to one native generation in this process.
// It is a comparable self-contained value; zero is invalid. It is not a
// subscriber identity or authority to mutate any transport or native resource.
type ObservationGeneration struct {
	sequence uint64
}

// observationSequence is a cold, process-local allocator. Its zero value is
// ready for use. The mutex protects exhaustion refusal and increment together;
// it is never acquired by an event consumer or on a packet-processing path.
type observationSequence struct {
	mu   sync.Mutex
	last uint64
}

var observationGenerations observationSequence

// NewObservationGeneration allocates once when the native root is created,
// before any reactor or observer publishes that root. Exhaustion refuses a
// root rather than recycling a value that an old callback may still carry.
func NewObservationGeneration() (ObservationGeneration, error) {
	return observationGenerations.next()
}

func (source *observationSequence) next() (ObservationGeneration, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.last == math.MaxUint64 {
		return ObservationGeneration{}, errors.New("l2tp: observation generation sequence exhausted")
	}
	source.last++
	return ObservationGeneration{sequence: source.last}, nil
}

// IsValid distinguishes an absent scope at the event boundary.
func (generation ObservationGeneration) IsValid() bool {
	return generation.sequence != 0
}

// MarshalText is the event JSON boundary. Internal observers compare the
// packed value directly; they never format or parse it to select an owner.
func (generation ObservationGeneration) MarshalText() ([]byte, error) {
	if !generation.IsValid() {
		return nil, errors.New("l2tp: cannot encode absent observation generation")
	}
	return strconv.AppendUint(nil, generation.sequence, 10), nil
}

// UnmarshalText accepts only canonical positive decimal. Failed decoding,
// including overflow, leaves the previous scope unchanged. A nil receiver is
// invalid. Twenty decimal digits bound a uint64 before any string allocation.
func (generation *ObservationGeneration) UnmarshalText(text []byte) error {
	if generation == nil {
		return errors.New("l2tp: nil observation generation receiver")
	}
	if len(text) == 0 {
		return errors.New("l2tp: absent observation generation")
	}
	if len(text) > 20 {
		return errors.New("l2tp: observation generation exceeds uint64")
	}
	if text[0] < '1' {
		return errors.New("l2tp: observation generation must use canonical positive decimal")
	}
	if text[0] > '9' {
		return errors.New("l2tp: observation generation must use canonical positive decimal")
	}
	sequence, err := strconv.ParseUint(string(text), 10, 64)
	if err != nil {
		return err
	}
	*generation = ObservationGeneration{sequence: sequence}
	return nil
}
