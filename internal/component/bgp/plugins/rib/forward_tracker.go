// Design: docs/architecture/rib/forward-handle.md -- first production Change.Forward consumer
// Related: forward_observer.go -- the debug-only nil-check observer this complements
// Related: forward_handle.go -- ribForwardHandle is the producer side (AddRef/Release/Bytes)
// Related: rib.go -- SetLocRIB creates and wires the tracker

package rib

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

var (
	errFastpathRequiresArg = errors.New("request bgp rib fastpath: requires enable, disable or status")
	errFastpathNoLocRIB    = errors.New("request bgp rib fastpath: no Loc-RIB wired (fast path unavailable)")
)

// forwardTrackQueue bounds the tracker's off-lock work queue. A full queue
// drops the item (after releasing its handle) and counts the drop, so a burst
// of best-changes never blocks the RIB write lock and never leaks the buffer
// pool.
const forwardTrackQueue = 1024

// unreadUpdateLength is the per-prefix value for a BGP best the tracker
// recorded with no UPDATE bytes in hand: one installed before enable, or one
// the Loc-RIB elected with Change.Forward nil. A real UPDATE is at least 23
// octets, so the value never collides with a length that was read.
const unreadUpdateLength = -1

// forwardTrackItem is the copied-out-under-lock unit handed to the worker.
// prune removes the prefix: the Loc-RIB removed it, or its best is no longer a
// BGP path. Otherwise the prefix is recorded, and handle, when non-nil, carries
// a retained (AddRef'd) reference to the producer's wire buffer that the worker
// reads off-lock and Releases exactly once. epoch is the enable period the
// change belongs to; the worker discards an item from an earlier period,
// because the re-enable rebuilt the state from the Loc-RIB.
type forwardTrackItem struct {
	family family.Family
	prefix netip.Prefix
	prune  bool
	handle locrib.ForwardHandle
	epoch  uint64
}

// forwardStateTracker is the first production consumer of the zero-copy
// Change.Forward handle (spec rib-arch-6). Where observeForwardHandles only
// nil-checks and debug-logs, this AddRefs the handle under the RIB write lock
// (the bounded "copy out under lock"), then a worker reads the UPDATE wire
// bytes off-lock, records per-prefix forwarding state, and Releases the
// handle -- proving the producer wiring (docs/architecture/rib/forward-handle.md)
// end-to-end with a
// strict AddRef/Release lifecycle and no buffer-pool leak.
//
// The state is the set of prefixes whose Loc-RIB best is a BGP path. Enable
// rebuilds it from the Loc-RIB (replay), so a best installed before enable,
// or while the tracker was disabled, is recorded without waiting for it to
// change.
//
// It is inert until Enable() is called: onChange is a single atomic load
// otherwise, so a binary that never enables the fast path pays no per-change
// cost. The RS/RR fast path (and future sysrib mirroring) is the intended
// consumer of this state.
type forwardStateTracker struct {
	enabled atomic.Bool

	loc *locrib.RIB // the subscribed Loc-RIB, which Enable replays

	// controlMu serializes Enable and Disable, so one replay runs at a time.
	controlMu sync.Mutex
	// epoch counts enable periods. Written only by Enable, holding mu, and
	// compared by process holding mu, so no item of an earlier period lands
	// after the replay cleared the state.
	epoch atomic.Uint64

	ch     chan forwardTrackItem
	stopCh chan struct{}
	doneCh chan struct{}
	unsub  func()

	// deliveryMu fences callbacks retained in a Loc-RIB subscriber snapshot.
	// Unsubscribe alone cannot stop a snapshot already being dispatched.
	deliveryMu sync.Mutex
	stopped    bool

	mu    sync.Mutex
	state map[family.Family]map[netip.Prefix]int // prefix -> UPDATE length read for the current best, or unreadUpdateLength

	forwarded atomic.Uint64 // Add/Update changes whose bytes were read + recorded
	bytes     atomic.Uint64 // total UPDATE wire bytes read
	dropped   atomic.Uint64 // items dropped under queue backpressure (handle released)
	removes   atomic.Uint64 // prunes processed: a removal, or a best that is no longer BGP
	replayed  atomic.Uint64 // BGP best routes enable found already in the Loc-RIB
}

// newForwardStateTracker subscribes to loc and starts the worker. The returned
// tracker is inert until Enable(). The owner MUST call Stop to join the worker.
func newForwardStateTracker(loc *locrib.RIB) *forwardStateTracker {
	t := &forwardStateTracker{
		loc:    loc,
		ch:     make(chan forwardTrackItem, forwardTrackQueue),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		state:  make(map[family.Family]map[netip.Prefix]int),
	}
	t.unsub = loc.OnChange(t.onChange)
	go t.run()
	return t
}

// Enable turns on state tracking and rebuilds the state from the Loc-RIB, so
// every BGP best already installed is recorded before Enable returns. Enabling
// an enabled tracker changes nothing. Safe for concurrent use.
//
// Ordering: enabled is set before the replay walks the Loc-RIB, so a change
// the walk misses is delivered to onChange. The walk reads each shard under
// its read lock, which excludes that shard's writers, so every change a writer
// queued before the walk read a prefix ends in the state the walk read, and
// every later one is queued after it. Items of an earlier enable period are
// discarded by epoch, because the replay supersedes them.
func (t *forwardStateTracker) Enable() {
	t.controlMu.Lock()
	defer t.controlMu.Unlock()
	if t.enabled.Load() {
		return
	}
	t.mu.Lock()
	t.epoch.Add(1)
	clear(t.state)
	t.mu.Unlock()
	t.enabled.Store(true)
	t.replay()
}

// Disable stops state tracking. The state stays readable through status and is
// rebuilt by the next Enable. Safe for concurrent use.
func (t *forwardStateTracker) Disable() {
	t.controlMu.Lock()
	defer t.controlMu.Unlock()
	t.enabled.Store(false)
}

// Enabled reports whether the tracker is recording. Safe for concurrent use.
func (t *forwardStateTracker) Enabled() bool { return t.enabled.Load() }

// replay records every prefix whose Loc-RIB best is a BGP path. It runs inside
// Iterate's shard read lock and takes mu for one prefix at a time, so the
// worker and status are never held off for a whole shard.
//
// A prefix the worker already recorded in this period keeps its length: the
// shard read lock excludes writers, so that record came from a change queued
// before the walk read the prefix, and any later change for the prefix is
// queued behind it and lands after. MUST be called by Enable only.
func (t *forwardStateTracker) replay() {
	for _, fam := range t.loc.Families() {
		t.loc.Iterate(fam, func(prefix netip.Prefix, g locrib.PathGroup) bool {
			if !bestIsBGP(&g) {
				return true
			}
			t.mu.Lock()
			if _, known := t.state[fam][prefix]; !known {
				t.recordLocked(fam, prefix, unreadUpdateLength)
			}
			t.mu.Unlock()
			t.replayed.Add(1)
			return true
		})
	}
}

// bestIsBGP reports whether g's selected best path is the BGP mirror.
func bestIsBGP(g *locrib.PathGroup) bool {
	if g.Best < 0 {
		return false
	}
	if g.Best >= len(g.Paths) {
		return false
	}
	return g.Paths[g.Best].Source == bgpProtocolID
}

// onChange runs SYNCHRONOUSLY under the RIB write lock, so it must stay cheap:
// an enabled check, an epoch load, an AddRef (the bounded copy-out-under-lock)
// for a BGP best carrying a Forward handle, and a non-blocking enqueue. All
// byte reading and map mutation happen off-lock in run(); this never takes
// t.mu, so it cannot contend the worker under the RIB write lock.
func (t *forwardStateTracker) onChange(c locrib.Change) { //nolint:gocritic // hugeParam: locrib.ChangeHandler fixes the value signature; a pointer through that func value escapes and allocates per change
	if !t.enabled.Load() {
		return
	}
	t.deliveryMu.Lock()
	defer t.deliveryMu.Unlock()
	if t.stopped {
		return
	}
	item := forwardTrackItem{family: c.Family, prefix: c.Prefix, epoch: t.epoch.Load()}
	switch {
	case c.Kind == locrib.ChangeRemove:
		item.prune = true
	case c.Best.Source == bgpProtocolID:
		// A nil Forward is a BGP best with no UPDATE in hand, recorded
		// without bytes. Otherwise MUST AddRef inside the handler while the
		// producer still holds its reference; Release happens off-lock in
		// process().
		if c.Forward != nil {
			c.Forward.AddRef()
			item.handle = c.Forward
		}
	default:
		// The Loc-RIB now selects another protocol's path for the prefix.
		item.prune = true
	}
	select {
	case t.ch <- item:
	default:
		if item.handle != nil {
			item.handle.Release()
		}
		t.dropped.Add(1)
	}
}

func (t *forwardStateTracker) run() {
	defer close(t.doneCh)
	for {
		select {
		case item := <-t.ch:
			t.process(item)
		case <-t.stopCh:
			// Drain queued handles so the buffer pool stays balanced on teardown.
			for {
				select {
				case item := <-t.ch:
					if item.handle != nil {
						item.handle.Release()
					}
				default:
					return
				}
			}
		}
	}
}

func (t *forwardStateTracker) process(item forwardTrackItem) {
	if item.handle != nil {
		// Contract: read Bytes strictly between AddRef and Release.
		defer item.handle.Release()
	}
	n := unreadUpdateLength
	if fb, ok := item.handle.(locrib.ForwardBytes); ok {
		n = len(fb.Bytes())
	}
	t.mu.Lock()
	if item.epoch != t.epoch.Load() {
		// An earlier enable period: the replay already rebuilt this prefix.
		t.mu.Unlock()
		return
	}
	if item.prune {
		if fam := t.state[item.family]; fam != nil {
			delete(fam, item.prefix)
		}
		t.mu.Unlock()
		t.removes.Add(1)
		return
	}
	// A live change describes the current best, so it replaces whatever was
	// recorded: a BGP best that arrives with no handle is a different path
	// (AIGP re-election, fallback), and the previous UPDATE length is not its.
	t.recordLocked(item.family, item.prefix, n)
	t.mu.Unlock()
	if n == unreadUpdateLength {
		return
	}
	t.forwarded.Add(1)
	t.bytes.Add(uint64(n)) //nolint:gosec // G115: n is a bounded UPDATE length (>= 0)
}

// recordLocked records prefix with length. The caller MUST hold mu.
func (t *forwardStateTracker) recordLocked(fam family.Family, prefix netip.Prefix, length int) {
	prefixes := t.state[fam]
	if prefixes == nil {
		prefixes = make(map[netip.Prefix]int)
		t.state[fam] = prefixes
	}
	prefixes[prefix] = length
}

// Stop fences callbacks, unsubscribes and joins the worker, releasing queued
// handles. The owner MUST call Stop before releasing the tracker. A callback
// retained in an older subscriber snapshot cannot enqueue after this fence.
func (t *forwardStateTracker) Stop() {
	t.deliveryMu.Lock()
	if t.stopped {
		t.deliveryMu.Unlock()
		<-t.doneCh
		return
	}
	t.stopped = true
	t.deliveryMu.Unlock()
	if t.unsub != nil {
		t.unsub()
		t.unsub = nil
	}
	close(t.stopCh)
	<-t.doneCh
}

// forwardStats is a snapshot of the tracker's observable state.
type forwardStats struct {
	Enabled   bool   `json:"enabled"`
	Forwarded uint64 `json:"forwarded"`
	Bytes     uint64 `json:"bytes"`
	Dropped   uint64 `json:"dropped"`
	Removes   uint64 `json:"removes"`
	Replayed  uint64 `json:"replayed"`
	Prefixes  int    `json:"prefixes"`
}

// snapshot returns the tracker's counters and current per-prefix entry count.
func (t *forwardStateTracker) snapshot() forwardStats {
	t.mu.Lock()
	n := 0
	for _, fam := range t.state {
		n += len(fam)
	}
	t.mu.Unlock()
	return forwardStats{
		Enabled:   t.enabled.Load(),
		Forwarded: t.forwarded.Load(),
		Bytes:     t.bytes.Load(),
		Dropped:   t.dropped.Load(),
		Removes:   t.removes.Load(),
		Replayed:  t.replayed.Load(),
		Prefixes:  n,
	}
}

// fastpathCommand enables, disables, or reports the forward-handle fast-path
// consumer. Backs `request bgp rib fastpath <enable|disable|status>`.
func (r *RIBManager) fastpathCommand(args []string) (string, any, error) {
	if len(args) == 0 {
		return statusError, "", errFastpathRequiresArg
	}
	r.peerMu.RLock()
	t := r.forwardTracker
	r.peerMu.RUnlock()
	if t == nil {
		return statusError, "", errFastpathNoLocRIB
	}
	// One word for each action (ai/rules/writing.md, habit 1). The model
	// declares the same three, so the dispatcher refuses any other word
	// before this runs; the check stays for the plugin's own dispatch route.
	action := args[0]
	if action == "status" {
		return statusDone, t.snapshot(), nil
	}
	if action == "enable" {
		t.Enable()
		return statusDone, t.snapshot(), nil
	}
	if action == "disable" {
		t.Disable()
		return statusDone, t.snapshot(), nil
	}
	return statusError, "", fmt.Errorf("unknown fastpath action %q (use enable, disable or status)", action)
}
