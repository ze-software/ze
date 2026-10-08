// Design: docs/architecture/core-design.md -- System RIB plugin
// Related: sysrib.go -- run, the worker that drains this feed
// Related: internal/core/rib/locrib/distance.go -- Reselect, the burst this absorbs

package sysrib

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// locRIBFeedSize is how many Loc-RIB changes wait for the worker before the
// feed stops queueing them and records their prefixes instead.
const locRIBFeedSize = 4096

// locRIBFeed carries Loc-RIB changes from the OnChange handler, which runs
// under a shard write lock and so MUST NOT block, to the worker in run().
// Safe for concurrent use: offer runs on any inserting goroutine, takeOverflow
// on the worker.
//
// A bounded channel alone loses changes. `(*locrib.RIB).Reselect`, run on a
// reload that changes a distance, dispatches one change per re-ranked prefix
// from a tight loop while it holds the shard lock, and the worker's next-hop
// resolution waits on that same lock, so a table larger than the channel
// overflows it. A dropped change is a winner the kernel never follows. So a
// change that does not fit records its prefix in overflow, and the worker
// re-reads those prefixes from the Loc-RIB once it has drained the channel
// (resyncOverflow): the burst coalesces to one read per prefix and nothing is
// lost. The overflow set is bounded by the number of prefixes the Loc-RIB holds.
type locRIBFeed struct {
	ch chan locrib.Change
	// wake tells the worker an overflow was recorded. Without it a prefix
	// recorded just after the worker drained the last queued change would wait
	// for an unrelated later change. One slot: a pending wake covers every
	// overflow recorded before the worker takes it.
	wake chan struct{}

	mu       sync.Mutex
	overflow map[overflowKey]struct{}
}

// overflowKey names one prefix whose change did not fit in the feed.
type overflowKey struct {
	fam    family.Family
	prefix netip.Prefix
}

func newLocRIBFeed(size int) *locRIBFeed {
	return &locRIBFeed{ch: make(chan locrib.Change, size), wake: make(chan struct{}, 1)}
}

// offer is the Loc-RIB OnChange handler. It never blocks.
func (f *locRIBFeed) offer(c *locrib.Change) {
	select {
	case f.ch <- *c:
		return
	default:
	}
	f.mu.Lock()
	if f.overflow == nil {
		f.overflow = make(map[overflowKey]struct{})
		logger().Warn("sysrib: change channel full, re-reading overflowed prefixes from the Loc-RIB")
	}
	f.overflow[overflowKey{fam: c.Family, prefix: c.Prefix}] = struct{}{}
	f.mu.Unlock()
	select {
	case f.wake <- struct{}{}:
	default: // a wake is already pending
	}
}

// takeOverflow returns the overflowed prefixes once the channel is empty, and
// nil before that. The lock is held across the emptiness check and the swap,
// so every queued change older than a recorded prefix has already been
// processed: the worker's read of the Loc-RIB is never followed by an older
// queued change for the same prefix.
func (f *locRIBFeed) takeOverflow() []overflowKey {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.overflow) == 0 {
		return nil
	}
	if len(f.ch) != 0 {
		return nil
	}
	keys := make([]overflowKey, 0, len(f.overflow))
	for k := range f.overflow {
		keys = append(keys, k)
	}
	f.overflow = nil
	return keys
}

// resyncOverflow processes each overflowed prefix as the Loc-RIB holds it now:
// its current best as an add (processEvent upserts), or a remove when no valid
// best is left. Called by the worker only: after each change it processes, and
// on a wake.
func (s *sysRIB) resyncOverflow(loc *locrib.RIB, feed *locRIBFeed) {
	for _, k := range feed.takeOverflow() {
		c := locrib.Change{Family: k.fam, Prefix: k.prefix, Kind: locrib.ChangeRemove}
		loc.Inspect(k.fam, k.prefix, func(g locrib.PathGroup) {
			if g.Best < 0 || g.Best >= len(g.Paths) {
				return
			}
			c.Kind = locrib.ChangeAdd
			c.Best = g.Paths[g.Best]
			c.ECMP = g.ECMPNextHops(c.Best)
		})
		s.processLocRIBChange(&c)
	}
}
