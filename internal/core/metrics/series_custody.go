// Design: docs/architecture/decisions/001-pull-model-metrics.md -- exact registered series lifetime
package metrics

import (
	"slices"
	"strconv"
	"sync"
)

// SeriesRetainer is an additive vector capability for shared series cleanup.
// A caller MUST retain before publishing under the labels and MUST release its
// lease after its final update. Delete MUST refuse while any lease remains.
// Ordinary Registry and vector interfaces do not require this capability.
type SeriesRetainer interface {
	RetainSeries(labelValues ...string) (SeriesLease, error)
}

// SeriesCustody belongs to one physical registered vector or publisher family.
// Its zero value is ready. Owners MUST NOT copy it after use. It stores only
// label sets with live leases, never a history of released labels.
type SeriesCustody struct {
	mu        sync.Mutex
	interests map[string]*seriesInterest
}

type seriesInterest struct {
	key     string
	labels  []string
	holders uint64
	remove  func(...string) bool
}

// SeriesLease is one holder of a vector's original series. Copies share the
// same release state, so Release remains idempotent through value copies.
// The zero lease has no metric effect, as used by disabled metrics.
type SeriesLease struct{ state *seriesLeaseState }

type seriesLeaseState struct {
	custody  *SeriesCustody
	interest *seriesInterest
	released bool // protected by custody.mu
}

// Retain records one holder after the vector validated its labels. The optional
// admission operation runs under the acquisition lock before retaining any key.
// A refusal leaves no custody entry and is returned to the caller. The vector
// MUST pass its original raw delete operation, never its public checked Delete.
// remove runs only under the custody lock and MUST NOT call back into custody.
// It operates on the retained label copy, never the caller's borrowed slice.
func (c *SeriesCustody) Retain(labels []string, admit func(...string) error, remove func(...string) bool) (SeriesLease, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if admit != nil {
		if err := admit(labels...); err != nil {
			return SeriesLease{}, err
		}
	}
	key := seriesLabelsKey(labels)
	if c.interests == nil {
		c.interests = make(map[string]*seriesInterest)
	}
	entry := c.interests[key]
	if entry == nil {
		entry = &seriesInterest{key: key, labels: slices.Clone(labels), remove: remove}
		c.interests[key] = entry
	}
	entry.holders++
	return SeriesLease{state: &seriesLeaseState{custody: c, interest: entry}}, nil
}

// Delete serializes unleased deletion with acquisition and final release.
// remove has the same no-callback obligation as Retain's raw delete operation.
func (c *SeriesCustody) Delete(labels []string, remove func(...string) bool) bool {
	key := seriesLabelsKey(labels)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.interests[key] != nil {
		return false
	}
	return remove(labels...)
}

// Release gives back this holder exactly once. The final holder removes the
// original physical series before another acquisition can recreate it.
func (lease SeriesLease) Release() {
	state := lease.state
	if state == nil {
		return
	}
	c := state.custody
	c.mu.Lock()
	defer c.mu.Unlock()
	if state.released {
		return
	}
	state.released = true
	entry := state.interest
	entry.holders--
	if entry.holders != 0 {
		return
	}
	delete(c.interests, entry.key)
	entry.remove(entry.labels...)
}

// Labels are arbitrary strings. Length framing retains exact equality even
// when a value contains separators; this cold acquisition path does not hash
// a tuple down to a narrower identity or retain caller-owned memory.
func seriesLabelsKey(labels []string) string {
	var encoded []byte
	for _, value := range labels {
		encoded = strconv.AppendUint(encoded, uint64(len(value)), 10)
		encoded = append(encoded, ':')
		encoded = append(encoded, value...)
	}
	return string(encoded)
}
