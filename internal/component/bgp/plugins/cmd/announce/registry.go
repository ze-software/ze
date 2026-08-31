// Design: docs/architecture/bgp/on-demand-origination.md -- tag registry for on-demand route origination

package announce

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// announceLogger reports what a caller cannot be told: a timer-driven withdraw
// the wire refused.
var announceLogger = slogutil.LazyLogger("bgp.announce")

// tagEntry is a tracked on-demand announcement in the tag registry.
type tagEntry struct {
	ID        uint64
	TagKey    string
	TagValue  string
	Family    string
	Selector  *selector.Selector
	Batch     types.NLRIBatch
	CreatedAt time.Time
	ExpiresAt *time.Time
	Source    string
	// Sender is who made this announcement: an attached process, or the
	// operator. The withdraw that retracts it carries the same sender, so a
	// timed announce expires under the permission that placed it rather than
	// under the daemon's own authority (reactor/send_permission.go).
	Sender plugin.Sender

	timer *time.Timer
}

// withdrawer is the function the registry calls to withdraw routes on the wire.
// sender is the identity the announcement was made under; see tagEntry.Sender.
type withdrawer func(sel *selector.Selector, batch types.NLRIBatch, sender plugin.Sender) error

// Registry tracks on-demand announcements by tag key+value.
type Registry struct {
	mu       sync.Mutex
	entries  map[uint64]*tagEntry
	nextID   atomic.Uint64
	withdraw withdrawer
	nowFunc  func() time.Time
}

// NewRegistry creates a tag registry with the given withdraw function.
func NewRegistry(withdraw withdrawer) *Registry {
	r := &Registry{
		entries:  make(map[uint64]*tagEntry),
		withdraw: withdraw,
		nowFunc:  time.Now,
	}
	r.nextID.Store(1)
	return r
}

// Announce registers a tagged announcement and returns its entry ID.
func (r *Registry) Announce(key, value string, sel *selector.Selector, batch types.NLRIBatch, source string, sender plugin.Sender, duration time.Duration) (uint64, error) {
	id := r.nextID.Add(1) - 1

	entry := &tagEntry{
		ID:        id,
		TagKey:    key,
		TagValue:  value,
		Family:    batch.Family.String(),
		Selector:  sel,
		Batch:     batch,
		CreatedAt: r.nowFunc(),
		Source:    source,
		Sender:    sender,
	}

	if duration > 0 {
		expires := entry.CreatedAt.Add(duration)
		entry.ExpiresAt = &expires
		entry.timer = time.AfterFunc(duration, func() {
			r.withdrawEntryByTimer(id)
		})
	}

	r.mu.Lock()
	r.entries[id] = entry
	r.mu.Unlock()

	return id, nil
}

// withdrawMatching withdraws every entry the peer filter and the predicate both
// accept, and answers how many it withdrew.
//
// peer is the selector the operator typed BEFORE the verb, or "" when they typed
// none. It is compared against the selector each announcement was MADE with,
// rather than resolved against the peer table.
//
// An entry records the fan-out it went to. So `peer 192.0.2.9 withdraw all` asks
// for the announcements sent to that fan-out and nothing else, and an operator
// who names a peer that received nothing withdraws nothing.
//
// The removal happens under the lock and the wire withdraw happens after it, so
// a slow or refusing peer never holds the registry.
func (r *Registry) withdrawMatching(peer string, match func(*tagEntry) bool) (int, error) {
	r.mu.Lock()
	var matched []*tagEntry
	for _, e := range r.entries {
		if peer != "" && e.Selector.String() != peer {
			continue
		}
		if !match(e) {
			continue
		}
		matched = append(matched, e)
	}
	for _, e := range matched {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	return r.withdrawEntries(matched)
}

// withdrawTag withdraws the entries carrying one tag key and one tag value.
func (r *Registry) withdrawTag(peer, key, value string) (int, error) {
	return r.withdrawMatching(peer, func(e *tagEntry) bool {
		return e.TagKey == key && e.TagValue == value
	})
}

// withdrawTagKey withdraws every entry under a tag key, whatever its value.
func (r *Registry) withdrawTagKey(peer, key string) (int, error) {
	return r.withdrawMatching(peer, func(e *tagEntry) bool { return e.TagKey == key })
}

// withdrawAll withdraws every entry the registry holds.
//
// It answers both `withdraw all` and `withdraw tag *`. Those two name one set
// rather than two. An announcement enters the registry only when the operator
// gave it a tag (announceAndTrack), so every tracked announcement is a tagged
// one. A second function for the tagged subset would differ from this one in
// name only.
func (r *Registry) withdrawAll(peer string) (int, error) {
	return r.withdrawMatching(peer, func(*tagEntry) bool { return true })
}

// withdrawEntryByID withdraws a single entry by ID.
//
// peer narrows the same way it does for every other form. An id whose
// announcement went to a different fan-out is reported as not found. A scoped
// withdraw therefore never reaches an announcement the operator did not name.
//
// The second result reports a withdraw the wire refused. The entry has already
// been dropped from the registry by then, so a discarded error left the route on
// the peer while `show announcements` said it was gone. That became reachable
// when the send permission made a withdraw refusable: a reload that removes a
// peer's attach block turns every later withdraw for that peer into a refusal
// (reactor/send_permission.go).
func (r *Registry) withdrawEntryByID(peer string, id uint64) (found bool, err error) {
	r.mu.Lock()
	e, ok := r.entries[id]
	if ok && peer != "" && e.Selector.String() != peer {
		ok = false
	}
	if ok {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	if !ok {
		return false, nil
	}

	return true, r.withdraw(e.Selector, e.Batch, e.Sender)
}

// listFilter controls which entries List returns.
type listFilter struct {
	TagKey   string
	TagValue string
	Selector string
	Family   string
}

// List returns entries matching the filter. Empty filter fields match everything.
func (r *Registry) List(f listFilter) []*tagEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []*tagEntry
	for _, e := range r.entries {
		if f.TagKey != "" && e.TagKey != f.TagKey {
			continue
		}
		if f.TagValue != "" && e.TagValue != f.TagValue {
			continue
		}
		if f.Selector != "" && e.Selector.String() != f.Selector {
			continue
		}
		if f.Family != "" && e.Family != f.Family {
			continue
		}
		result = append(result, e)
	}
	return result
}

// Len returns the number of tracked entries.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

func (r *Registry) removeEntryLocked(e *tagEntry) {
	if e.timer != nil {
		e.timer.Stop()
	}
	delete(r.entries, e.ID)
}

// withdrawEntryByTimer retracts an announcement whose duration has run out.
//
// It runs on a timer goroutine, so there is nobody to return an error to and the
// refusal is REPORTED instead. Losing it is what makes this the worst of the two
// paths: the entry is already gone from the registry, nothing will retry, and
// the route stays on the peer for the life of the session with every operator
// view agreeing it was withdrawn. A withdraw became refusable when the send
// permission landed, and a reload that removes the peer's attach block between
// the announce and the expiry is enough to reach it.
func (r *Registry) withdrawEntryByTimer(id uint64) {
	r.mu.Lock()
	e, ok := r.entries[id]
	if ok {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	if !ok {
		return
	}
	if err := r.withdraw(e.Selector, e.Batch, e.Sender); err != nil {
		announceLogger().Error("timed announcement expired but its withdraw was refused",
			"id", id, "tag", e.TagKey, "value", e.TagValue,
			"selector", e.Selector.String(), "family", e.Family,
			"process", e.Sender.String(), "err", err,
			"effect", "the route may still be on the peer, and this announcement is no longer tracked",
			"action", "check the peer's `attach process` block still grants send [ update ], then withdraw the prefix by hand")
	}
}

func (r *Registry) withdrawEntries(entries []*tagEntry) (int, error) {
	var lastErr error
	for _, e := range entries {
		if err := r.withdraw(e.Selector, e.Batch, e.Sender); err != nil {
			lastErr = err
		}
	}
	return len(entries), lastErr
}
