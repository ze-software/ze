// Design: docs/features/interfaces.md -- Bus Topics: the neighbor table watch BGP runs with or without an interface block
// Overview: ifacenetlink.go -- package hub
// Related: neighbor_linux.go -- ListNeighbors, the read this watch says to repeat
// RFC: rfc/short/rfc2545.md

//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
)

// neighborWatchQueueDepth is the depth of the channel the netlink subscription
// delivers neighbor updates on. The watch drains it as fast as it compares two
// small values, and a full queue only delays the subscription's reader: the
// kernel side is bounded by monitorReceiveBufferBytes.
const neighborWatchQueueDepth = 64

// WatchNeighbors implements iface.Backend. It opens a netlink subscription of
// its own to RTM_NEWNEIGH and RTM_DELNEIGH, independent of the interface
// monitor, because the monitor runs only when an interface block configures a
// backend while BGP needs the changes in every config (RFC 2545 Section 3's
// third-party Link-Local). The subscription first dumps the table, so an entry
// that existed before the watch started is known and its later deletion
// signals. Each watch is one long-lived goroutine reading the subscription
// channel; it ends with ctx, with Close, or when the subscription fails, and
// closes the returned channel last.
func (b *netlinkBackend) WatchNeighbors(ctx context.Context) (<-chan struct{}, error) {
	w := &neighborWatch{
		updates: make(chan netlink.NeighUpdate, neighborWatchQueueDepth),
		stop:    make(chan struct{}),
		changes: make(chan struct{}, 1),
		seen:    make(map[neighKey]neighborSeen),
	}
	if err := netlink.NeighSubscribeWithOptions(w.updates, w.stop, netlink.NeighSubscribeOptions{
		ReceiveBufferSize: monitorReceiveBufferBytes,
		// The dump fills seen with the entries that existed before the
		// subscription, so note can tell their RTM_DELNEIGH from a delete of
		// an entry the watch never knew.
		ListExisting:  true,
		ErrorCallback: w.recordFailure,
	}); err != nil {
		close(w.stop)
		return nil, fmt.Errorf("subscribe to kernel neighbor changes: %w", err)
	}
	go w.run(ctx, b.closed.channel())
	return w.changes, nil
}

// neighborWatch is one running WatchNeighbors. run is its only goroutine and
// the only reader and writer of seen.
type neighborWatch struct {
	updates chan netlink.NeighUpdate
	// stop ends the netlink subscription; closing it closes its socket, after
	// which the subscription closes updates.
	stop    chan struct{}
	changes chan struct{}
	// seen holds the last link-layer address and binding class of every IPv6
	// neighbor, so a signal goes out only for a change that can move a pairing.
	// An entry leaves with its RTM_DELNEIGH, so the map is bounded by the
	// kernel neighbor table (net.ipv6.neigh.default.gc_thresh3).
	seen map[neighKey]neighborSeen
	// dumpIncomplete is set when the kernel flagged the initial dump as
	// interrupted (NLM_F_DUMP_INTR): the table changed during the dump, which
	// can then skip an entry, so seen may lack an entry the kernel holds. From
	// then on a delete of an unknown entry signals too. The subscription sets
	// it from its own goroutine; note reads it from run.
	dumpIncomplete atomic.Bool

	failureMu sync.Mutex
	failure   error
}

// neighborSeen is what note compares between two updates of one entry.
type neighborSeen struct {
	linkAddress string
	resolved    bool
}

// recordFailure keeps the last error the subscription reported, so the line
// that says the watch ended names why. An interrupted dump is not a failure:
// the subscription goes on, and the watch notes that seen may be incomplete.
// The subscription calls it from its own goroutine.
func (w *neighborWatch) recordFailure(err error) {
	if errors.Is(err, netlink.ErrDumpInterrupted) {
		w.dumpIncomplete.Store(true)
		return
	}
	w.failureMu.Lock()
	w.failure = err
	w.failureMu.Unlock()
}

// run reads the subscription until ctx ends, the backend closes, or the
// subscription fails. It releases the subscription before it closes changes,
// so a caller that sees changes closed knows no goroutine of this watch runs.
func (w *neighborWatch) run(ctx context.Context, closed <-chan struct{}) {
	defer close(w.changes)
	defer w.release()
	for {
		select {
		case <-ctx.Done():
			return
		case <-closed:
			return
		case update, ok := <-w.updates:
			if !ok {
				w.failureMu.Lock()
				failure := w.failure
				w.failureMu.Unlock()
				logger().Warn("kernel neighbor table watch ended: the netlink subscription closed",
					"error", failure,
					"effect", "the consumer starts a new watch and reads the table again",
					"fix", "none if the watch starts again; if this repeats, raise net.core.rmem_max for an ENOBUFS error, and look for kernel messages with: dmesg | grep -i -e netlink -e neigh")
				return
			}
			if w.note(update) {
				w.signal()
			}
		}
	}
}

// release closes the subscription and drains it until it closes updates, so
// its reader never blocks on a send nobody receives.
func (w *neighborWatch) release() {
	close(w.stop)
	for {
		if _, open := <-w.updates; !open {
			return
		}
	}
}

// signal leaves ONE pending signal and never blocks: the consumer re-reads the
// whole table, so a second change while one is pending is already covered.
func (w *neighborWatch) signal() {
	select {
	case w.changes <- struct{}{}:
	default:
	}
}

// note records update and reports whether it can move a pairing: an IPv6
// entry that appears or goes away, changes its link-layer address, or crosses
// between a resolved and an unresolved state (iface.NeighborStateResolved). A
// flip inside one class, REACHABLE to STALE for instance, reports nothing, so
// the consumer does not re-read the table for every reachability transition.
// Global and Link-Local entries both count: the pairing joins a global with
// the Link-Local that shares its link-layer address. The initial dump arrives
// as RTM_NEWNEIGH, so it fills seen like any new entry. A delete of an entry
// seen never held reports nothing, unless the dump was interrupted and seen
// may lack it (dumpIncomplete).
func (w *neighborWatch) note(update netlink.NeighUpdate) bool {
	if update.IP == nil {
		return false
	}
	if update.IP.To4() != nil {
		return false
	}
	key := neighKey{linkIndex: update.LinkIndex, ip: update.IP.String()}
	if update.Type == unix.RTM_DELNEIGH {
		_, known := w.seen[key]
		delete(w.seen, key)
		if known {
			return true
		}
		return w.dumpIncomplete.Load()
	}
	seen := neighborSeen{resolved: iface.NeighborStateResolved(neighStateString(update.State))}
	if len(update.HardwareAddr) > 0 {
		seen.linkAddress = update.HardwareAddr.String()
	}
	previous, known := w.seen[key]
	w.seen[key] = seen
	if known {
		return previous != seen
	}
	return true
}

// closeSignal is a channel closed once, by a backend's Close. Its zero value
// is ready, so a backend built as a literal (the tests do) has one too. Safe
// for concurrent use.
type closeSignal struct {
	mu     sync.Mutex
	ch     chan struct{}
	closed bool
}

// channel answers the channel close closes.
func (c *closeSignal) channel() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ch == nil {
		c.ch = make(chan struct{})
	}
	return c.ch
}

// close closes the channel; a second call does nothing.
func (c *closeSignal) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ch == nil {
		c.ch = make(chan struct{})
	}
	if c.closed {
		return
	}
	c.closed = true
	close(c.ch)
}
