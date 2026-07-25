// Design: plan/learned/1008-cp-survival-4-on-demand-origination-design.md -- tag registry for on-demand route origination

package announce

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/selector"
)

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

	timer *time.Timer
}

// withdrawer is the function the registry calls to withdraw routes on the wire.
type withdrawer func(sel *selector.Selector, batch types.NLRIBatch) error

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
func (r *Registry) Announce(key, value string, sel *selector.Selector, batch types.NLRIBatch, source string, duration time.Duration) (uint64, error) {
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

// withdrawTag withdraws all entries matching the given tag key+value.
func (r *Registry) withdrawTag(key, value string) (int, error) {
	r.mu.Lock()
	var matched []*tagEntry
	for _, e := range r.entries {
		if e.TagKey == key && e.TagValue == value {
			matched = append(matched, e)
		}
	}
	for _, e := range matched {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	return r.withdrawEntries(matched)
}

// withdrawTagKey withdraws all entries under a tag key (all values).
func (r *Registry) withdrawTagKey(key string) (int, error) {
	r.mu.Lock()
	var matched []*tagEntry
	for _, e := range r.entries {
		if e.TagKey == key {
			matched = append(matched, e)
		}
	}
	for _, e := range matched {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	return r.withdrawEntries(matched)
}

// withdrawAllTags withdraws all tagged entries across all keys.
func (r *Registry) withdrawAllTags() (int, error) {
	r.mu.Lock()
	matched := make([]*tagEntry, 0, len(r.entries))
	for _, e := range r.entries {
		matched = append(matched, e)
	}
	for _, e := range matched {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	return r.withdrawEntries(matched)
}

// withdrawEntryByID withdraws a single entry by ID.
func (r *Registry) withdrawEntryByID(id uint64) bool {
	r.mu.Lock()
	e, ok := r.entries[id]
	if ok {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	if !ok {
		return false
	}

	_ = r.withdraw(e.Selector, e.Batch)
	return true
}

// withdrawAll withdraws all entries, optionally filtered by selector string.
func (r *Registry) withdrawAll(selFilter string) (int, error) {
	r.mu.Lock()
	var matched []*tagEntry
	for _, e := range r.entries {
		if selFilter != "" && e.Selector.String() != selFilter {
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

func (r *Registry) withdrawEntryByTimer(id uint64) {
	r.mu.Lock()
	e, ok := r.entries[id]
	if ok {
		r.removeEntryLocked(e)
	}
	r.mu.Unlock()

	if ok {
		_ = r.withdraw(e.Selector, e.Batch)
	}
}

func (r *Registry) withdrawEntries(entries []*tagEntry) (int, error) {
	var lastErr error
	for _, e := range entries {
		if err := r.withdraw(e.Selector, e.Batch); err != nil {
			lastErr = err
		}
	}
	return len(entries), lastErr
}
