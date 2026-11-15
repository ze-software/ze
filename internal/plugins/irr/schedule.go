// Design: docs/architecture/irr-owner.md -- when each dataset is fetched
// Related: owner.go -- the worker that plans and runs the schedule
//
// The schedule is anchored on checked-at, the last time the server answered
// for a dataset, never on refreshed-at: refreshed-at dates the oldest prefixes
// a dataset carries, so a set that answers for one family only would read as
// old on every pass and be fetched every tick.

package irr

import (
	"slices"
	"time"
)

// retryInterval is how soon a fetch the server did not answer, or a dataset
// the store refused, is tried again. It is the readmitter's cadence the BGP
// filter had (one minute), and it stops at the first answer.
const retryInterval = time.Minute

// setState is what the schedule knows of one AS-SET.
type setState struct {
	// stored is true when a readable dataset of the configured server is
	// enforced for the set; storedCheckedAt is its checked-at.
	stored          bool
	storedCheckedAt time.Time
	// answeredAt is the last time the server answered with nothing the store
	// kept (an empty answer for a set with nothing stored, or a list over the
	// caps): it waits a full interval like any answer. Zero when none.
	answeredAt time.Time
	// retryAt is set while the last fetch or store write failed.
	retryAt time.Time
}

// dueOf returns when the set is fetched next. ok is false for a set the
// operator fetches by hand only: refresh-interval 0 and a stored copy, or an
// answer already received. A set with nothing stored and no answer is due at
// once, whatever the interval: a cold start fetches every referenced set once.
func dueOf(state setState, cfg ownerConfig, now time.Time) (time.Time, bool) {
	if !state.retryAt.IsZero() {
		return state.retryAt, true
	}
	answered := state.answeredAt
	if state.stored && state.storedCheckedAt.After(answered) {
		answered = state.storedCheckedAt
	}
	if answered.IsZero() {
		return now, true
	}
	return dueAfterAnswer(answered, cfg)
}

// dueAfterAnswer returns checkedAt plus the refresh interval. ok is false with
// refresh-interval 0: the dataset is fetched again only by `update irr`.
func dueAfterAnswer(checkedAt time.Time, cfg ownerConfig) (time.Time, bool) {
	if cfg.manual() {
		return time.Time{}, false
	}
	return checkedAt.Add(cfg.refreshInterval), true
}

// schedule holds the next fetch of every AS-SET that has one. A set absent
// from it is fetched only on demand. It is owned by the worker goroutine and
// is not safe for concurrent use.
type schedule struct {
	due map[string]time.Time
}

func newSchedule() schedule {
	return schedule{due: make(map[string]time.Time)}
}

// set arms asSet at at, replacing any earlier time.
func (s schedule) set(asSet string, at time.Time) {
	s.due[asSet] = at
}

// drop disarms asSet.
func (s schedule) drop(asSet string) {
	delete(s.due, asSet)
}

// at returns when asSet is due, and false when it is not armed.
func (s schedule) at(asSet string) (time.Time, bool) {
	at, ok := s.due[asSet]
	return at, ok
}

// next returns the earliest due time, and false when nothing is armed.
func (s schedule) next() (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, at := range s.due {
		if !found || at.Before(earliest) {
			earliest, found = at, true
		}
	}
	return earliest, found
}

// dueBy returns, sorted, every set due at or before now.
func (s schedule) dueBy(now time.Time) []string {
	var sets []string
	for asSet, at := range s.due {
		if !at.After(now) {
			sets = append(sets, asSet)
		}
	}
	slices.Sort(sets)
	return sets
}
