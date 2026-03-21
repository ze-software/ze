// Design: docs/architecture/api/process-protocol.md -- event stream correlation
// Overview: sdk.go -- plugin SDK core
// Related: sdk_engine.go -- EmitEvent method used by event producers

package sdk

import (
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// defaultMaxPending is the maximum number of pending correlation entries.
const defaultMaxPending = 10000

// unionEntry holds one side of a correlated event pair awaiting its partner.
type unionEntry struct {
	primary   string
	secondary string
	hasPri    bool
	hasSec    bool
	peer      string
	msgID     uint64
	createdAt time.Time
}

// correlationKey uniquely identifies a correlated event pair.
func correlationKey(peer string, msgID uint64) string {
	return peer + "|" + strconv.FormatUint(msgID, 10)
}

// Union correlates two asynchronous event streams by message ID and peer address.
// The consumer creates a union, feeds events via OnEvent, and receives joined
// pairs via the handler callback. If the secondary event doesn't arrive within
// the timeout, the handler is called with an empty secondary.
//
// Stop() MUST be called when the Union is no longer needed to prevent goroutine leaks.
type Union struct {
	primaryType   string
	secondaryType string
	timeout       time.Duration
	handler       func(primary, secondary string)

	mu            sync.Mutex
	pending       map[string]*unionEntry
	order         []string       // insertion order for eviction
	orderIndex    map[string]int // key -> index in order for O(1) lookup
	maxPending    int
	sweepInterval time.Duration // configurable for tests
	stopCh        chan struct{}
	stopped       bool
}

// NewUnion creates a union that correlates events of primaryType and secondaryType.
// The handler is called with (primary event, secondary event) when both arrive,
// or (primary, "") on timeout/flush. If handler is nil, events are silently dropped.
//
// Stop() MUST be called when the Union is no longer needed.
func NewUnion(primaryType, secondaryType string, timeout time.Duration, handler func(primary, secondary string)) *Union {
	if handler == nil {
		slog.Error("union: nil handler, events will be dropped")
		handler = func(string, string) {}
	}

	u := &Union{
		primaryType:   primaryType,
		secondaryType: secondaryType,
		timeout:       timeout,
		handler:       handler,
		pending:       make(map[string]*unionEntry),
		orderIndex:    make(map[string]int),
		maxPending:    defaultMaxPending,
		sweepInterval: time.Second,
		stopCh:        make(chan struct{}),
	}

	// Long-lived sweep goroutine (per Ze goroutine-lifecycle rules).
	go u.sweepLoop()

	return u
}

// Stop stops the timeout sweep goroutine. Safe to call multiple times.
func (u *Union) Stop() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.stopped {
		u.stopped = true
		close(u.stopCh)
	}
}

// OnEvent feeds an event into the union for correlation.
// eventType identifies which stream (primary or secondary).
// peer and msgID form the correlation key.
func (u *Union) OnEvent(eventType, peer string, msgID uint64, event string) {
	u.mu.Lock()

	key := correlationKey(peer, msgID)
	entry, exists := u.pending[key]

	if !exists {
		entry = &unionEntry{
			peer:      peer,
			msgID:     msgID,
			createdAt: time.Now(),
		}
		u.pending[key] = entry
		u.appendOrderLocked(key)
	}

	// Evict oldest if over capacity. Collect evicted entries under lock,
	// deliver outside lock to avoid deadlock with handler callbacks.
	evicted := u.evictOldestLocked()

	switch eventType {
	case u.primaryType:
		entry.primary = event
		entry.hasPri = true
	case u.secondaryType:
		entry.secondary = event
		entry.hasSec = true
	}

	if entry.hasPri && entry.hasSec {
		pri, sec := entry.primary, entry.secondary
		u.removeLocked(key)
		u.mu.Unlock()
		// Deliver evicted entries first (FIFO order).
		for _, e := range evicted {
			u.handler(e, "")
		}
		u.handler(pri, sec)
		return
	}

	u.mu.Unlock()

	// Deliver evicted entries outside lock.
	for _, pri := range evicted {
		u.handler(pri, "")
	}
}

// FlushPeer delivers all pending entries for the given peer with empty secondary.
func (u *Union) FlushPeer(peer string) {
	u.mu.Lock()

	var toDeliver []string
	var keys []string

	for key, entry := range u.pending {
		if entry.peer == peer && entry.hasPri {
			toDeliver = append(toDeliver, entry.primary)
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		u.removeLocked(key)
	}

	u.mu.Unlock()

	for _, pri := range toDeliver {
		u.handler(pri, "")
	}
}

// sweepLoop runs periodically to deliver timed-out entries.
func (u *Union) sweepLoop() {
	for {
		u.mu.Lock()
		interval := u.sweepInterval
		u.mu.Unlock()

		timer := time.NewTimer(interval)
		select {
		case <-u.stopCh:
			timer.Stop()
			return
		case <-timer.C:
			u.sweepExpired()
		}
	}
}

// sweepExpired delivers entries that have exceeded the timeout.
func (u *Union) sweepExpired() {
	now := time.Now()
	u.mu.Lock()

	var toDeliver []string
	var keys []string

	for key, entry := range u.pending {
		if now.Sub(entry.createdAt) > u.timeout && entry.hasPri {
			toDeliver = append(toDeliver, entry.primary)
			keys = append(keys, key)
		} else if now.Sub(entry.createdAt) > u.timeout {
			// Orphan secondary (primary never arrived) -- discard.
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		u.removeLocked(key)
	}

	u.mu.Unlock()

	for _, pri := range toDeliver {
		u.handler(pri, "")
	}
}

// evictOldestLocked collects entries to evict when over maxPending.
// Returns primary events to deliver. Caller must hold u.mu.
func (u *Union) evictOldestLocked() []string {
	var toDeliver []string
	for len(u.pending) > u.maxPending && len(u.order) > 0 {
		oldKey := u.order[0]
		if entry, ok := u.pending[oldKey]; ok && entry.hasPri {
			toDeliver = append(toDeliver, entry.primary)
		}
		u.removeLocked(oldKey)
	}
	return toDeliver
}

// appendOrderLocked appends a key to the order slice and index. Caller must hold u.mu.
func (u *Union) appendOrderLocked(key string) {
	u.orderIndex[key] = len(u.order)
	u.order = append(u.order, key)
}

// removeLocked removes an entry from pending, order, and orderIndex. Caller must hold u.mu.
func (u *Union) removeLocked(key string) {
	delete(u.pending, key)

	idx, ok := u.orderIndex[key]
	if !ok {
		return
	}
	delete(u.orderIndex, key)

	// Swap with last element for O(1) removal.
	last := len(u.order) - 1
	if idx != last {
		u.order[idx] = u.order[last]
		u.orderIndex[u.order[idx]] = idx
	}
	u.order = u.order[:last]
}
