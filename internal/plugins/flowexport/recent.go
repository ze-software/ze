// Design: docs/architecture/ddos/cp-survival-5-detect-5-characterization.md -- Phase 2 recent-flow tap.
// A bounded, drop-oldest ring of recently exported conntrack flows. It is fed
// from ExportFlows (the conntrack dump / destroy fan-out) and read by the
// `show flow recent` RPC so the DDoS detector's Stage-2 characterizer can
// inspect the live flow mix (proto/ports/TCP-state/top-sources) without a
// packet capture. Value-type snapshots only; safe to copy out under the lock.

package flowexport

import (
	"net/netip"
	"sync"
)

// recentRing is a fixed-size ring buffer of ConntrackFlow records. When full it
// overwrites the oldest entry and counts the drop (for the recent-ring-drops
// metric). A zero-size ring is inert: append is a no-op and snapshot returns
// nil, so a nil or disabled ring costs nothing.
type recentRing struct {
	mu    sync.Mutex
	buf   []ConntrackFlow
	next  int    // index of the next write (oldest entry once full)
	size  int    // capacity; 0 means disabled
	full  bool   // true once the ring has wrapped at least once
	drops uint64 // entries overwritten before being read (cumulative)
}

// newRecentRing allocates a ring of the given capacity. size <= 0 yields an
// inert ring (all operations no-op).
func newRecentRing(size int) *recentRing {
	if size <= 0 {
		return &recentRing{}
	}
	return &recentRing{buf: make([]ConntrackFlow, size), size: size}
}

// append copies flows into the ring, overwriting the oldest when full. Safe on
// a nil or zero-size ring. Called from ExportFlows under the exporter mutex; the
// ring's own mutex serializes against concurrent snapshot readers.
func (r *recentRing) append(flows []ConntrackFlow) {
	if r == nil || r.size == 0 || len(flows) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range flows {
		if r.full {
			r.drops++
		}
		r.buf[r.next] = flows[i]
		r.next++
		if r.next == r.size {
			r.next = 0
			r.full = true
		}
	}
}

// snapshot returns a copy of the ring contents in oldest-to-newest order,
// optionally filtered to flows whose destination address is inside dst. Pass the
// zero-value prefix to disable filtering. Safe on a nil or zero-size ring.
func (r *recentRing) snapshot(dst netip.Prefix) []ConntrackFlow {
	if r == nil || r.size == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	n := r.size
	start := r.next // oldest lives at next once full
	if !r.full {
		n = r.next
		start = 0
	}
	out := make([]ConntrackFlow, 0, n)
	for i := range n {
		f := r.buf[(start+i)%r.size]
		if dst.IsValid() && !dst.Contains(f.DstAddr) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// dropCount returns the cumulative number of entries overwritten before being
// read. Exposed for the recent-ring-drops metric.
func (r *recentRing) dropCount() uint64 {
	if r == nil || r.size == 0 {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drops
}
