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
	errFastpathRequiresArg = errors.New("request bgp rib fastpath: requires <enable|disable>")
	errFastpathNoLocRIB    = errors.New("request bgp rib fastpath: no Loc-RIB wired (fast path unavailable)")
)

// forwardTrackQueue bounds the tracker's off-lock work queue. A full queue
// drops the item (after releasing its handle) and counts the drop, so a burst
// of best-changes never blocks the RIB write lock and never leaks the buffer
// pool.
const forwardTrackQueue = 1024

// forwardTrackItem is the copied-out-under-lock unit handed to the worker.
// For Add/Update, handle carries a retained (AddRef'd) reference to the
// producer's wire buffer; the worker reads Bytes off-lock and Releases exactly
// once. For Remove, handle is nil and the worker prunes the per-prefix state.
type forwardTrackItem struct {
	family family.Family
	prefix netip.Prefix
	kind   locrib.ChangeKind
	handle locrib.ForwardHandle
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
// It is inert until Enable() is called: onChange is a single atomic load
// otherwise, so a binary that never enables the fast path pays no per-change
// cost. The RS/RR fast path (and future sysrib mirroring) is the intended
// consumer of this state.
type forwardStateTracker struct {
	enabled atomic.Bool

	ch     chan forwardTrackItem
	stopCh chan struct{}
	doneCh chan struct{}
	unsub  func()

	mu    sync.Mutex
	state map[family.Family]map[netip.Prefix]int // prefix -> last forwarded UPDATE byte length

	forwarded atomic.Uint64 // Add/Update changes whose bytes were read + recorded
	bytes     atomic.Uint64 // total UPDATE wire bytes read
	dropped   atomic.Uint64 // items dropped under queue backpressure (handle released)
	removes   atomic.Uint64 // Remove changes processed (state pruned)
}

// newForwardStateTracker subscribes to loc and starts the worker. The returned
// tracker is inert until Enable(). Stop() unsubscribes and joins the worker.
func newForwardStateTracker(loc *locrib.RIB) *forwardStateTracker {
	t := &forwardStateTracker{
		ch:     make(chan forwardTrackItem, forwardTrackQueue),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		state:  make(map[family.Family]map[netip.Prefix]int),
	}
	t.unsub = loc.OnChange(t.onChange)
	go t.run()
	return t
}

// Enable turns on byte reading + state tracking; Disable turns it off. The
// per-prefix state is retained across a disable so a re-enable resumes.
func (t *forwardStateTracker) Enable()       { t.enabled.Store(true) }
func (t *forwardStateTracker) Disable()      { t.enabled.Store(false) }
func (t *forwardStateTracker) Enabled() bool { return t.enabled.Load() }

// onChange runs SYNCHRONOUSLY under the RIB write lock, so it must stay cheap:
// an enabled check, an AddRef (the bounded copy-out-under-lock) for Add/Update,
// and a non-blocking enqueue. All byte reading and map mutation happen off-lock
// in run(); this never takes t.mu, so it cannot contend the worker under the
// RIB write lock.
func (t *forwardStateTracker) onChange(c locrib.Change) {
	if !t.enabled.Load() {
		return
	}
	item := forwardTrackItem{family: c.Family, prefix: c.Prefix, kind: c.Kind}
	if c.Kind != locrib.ChangeRemove {
		if c.Forward == nil {
			// Add/Update from a non-BGP producer (no wire buffer): nothing to read.
			return
		}
		// MUST AddRef inside the handler while the producer still holds its
		// reference; Release happens off-lock in process().
		c.Forward.AddRef()
		item.handle = c.Forward
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
	if item.kind == locrib.ChangeRemove {
		t.mu.Lock()
		if fam := t.state[item.family]; fam != nil {
			delete(fam, item.prefix)
		}
		t.mu.Unlock()
		t.removes.Add(1)
		return
	}
	// Contract: read Bytes strictly between AddRef and Release.
	defer item.handle.Release()
	var n int
	if fb, ok := item.handle.(locrib.ForwardBytes); ok {
		n = len(fb.Bytes())
	}
	t.mu.Lock()
	fam := t.state[item.family]
	if fam == nil {
		fam = make(map[netip.Prefix]int)
		t.state[item.family] = fam
	}
	fam[item.prefix] = n
	t.mu.Unlock()
	t.forwarded.Add(1)
	t.bytes.Add(uint64(n)) //nolint:gosec // G115: n is a bounded UPDATE length (>= 0)
}

// Stop unsubscribes from loc and joins the worker, releasing any queued handles.
// A final non-blocking drain catches a handle enqueued by an onChange that
// raced the stop signal, keeping the pool balanced.
func (t *forwardStateTracker) Stop() {
	if t.unsub != nil {
		t.unsub()
		t.unsub = nil
	}
	close(t.stopCh)
	<-t.doneCh
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

// forwardStats is a snapshot of the tracker's observable state.
type forwardStats struct {
	Enabled   bool   `json:"enabled"`
	Forwarded uint64 `json:"forwarded"`
	Bytes     uint64 `json:"bytes"`
	Dropped   uint64 `json:"dropped"`
	Removes   uint64 `json:"removes"`
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
	action := args[0]
	if action == "status" || action == "show" {
		return statusDone, t.snapshot(), nil
	}
	if action == "enable" || action == "on" {
		t.Enable()
		return statusDone, t.snapshot(), nil
	}
	if action == "disable" || action == "off" {
		t.Disable()
		return statusDone, t.snapshot(), nil
	}
	return statusError, "", fmt.Errorf("unknown fastpath action %q (use enable|disable|status)", action)
}
