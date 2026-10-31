// Design: docs/architecture/bgp/interface-event-reactions.md -- the kernel table watches BGP runs with or without an interface block
// Overview: reactor.go — Reactor struct and lifecycle
// Related: reactor_iface.go — neighborWatchWorker, one of the two watches supervised here
// Related: reactor_address_watch.go — addressWatchWorker, the other

package reactor

import (
	"context"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

// tableWatchRetryFirst and tableWatchRetryMax bound how soon a kernel table
// watch that could not start, or that ended under a live reactor, is started
// again. The delay doubles from the first to the max, so a source that keeps
// failing costs one start, one stopped state and one warning per delay. It
// resets to the first only after a watch ran tableWatchRetryMax: a watch that
// starts and then ends at once is the failing source the doubling exists for.
const (
	tableWatchRetryFirst = time.Second
	tableWatchRetryMax   = time.Minute
)

// tableWatch is one kernel table watch superviseTableWatch keeps open: the
// neighbor table (neighborWatchWorker) or the interface address table
// (addressWatchWorker). Each watch answers the channel contract of
// network.NeighborWatcher and network.AddressWatcher: one coalesced signal
// means "read the table again", and a close while ctx is live means the table
// is no longer watched.
type tableWatch struct {
	// start opens a new watch.
	start func(context.Context) (<-chan struct{}, error)
	// wait waits delay on clk before the next start, and reports false when
	// ctx ended first.
	wait func(ctx context.Context, clk clock.Clock, delay time.Duration) bool
	// ended is the error a watch that closed while ctx was live is reported
	// as.
	ended error
	// running runs once each watch has started. The table may have moved
	// while no watch ran, so it reads the table again.
	running func()
	// changed runs on each signal.
	changed func()
	// stopped runs when a watch could not start or ended under a live ctx,
	// with the reason.
	stopped func(error)
}

// superviseTableWatch keeps w open for as long as ctx lives. changes and err
// are the answer of the first start, which the caller makes itself so it can
// order that start against its own startup (addressWatchWorker opens its
// watch before the listeners start).
//
// It owns each watch it starts: it ranges over the watch's channel until the
// watch closes it, so no watch goroutine outlives it. A watch that cannot
// start, or that closes while ctx is live, is reported stopped and started
// again after a delay that doubles up to tableWatchRetryMax. The loop has no
// count bound because the reactor needs the watch for as long as it runs; it
// ends only with ctx, the reactor's run context.
func (r *Reactor) superviseTableWatch(ctx context.Context, w tableWatch, changes <-chan struct{}, err error) {
	retry := tableWatchRetryFirst
	for {
		if err == nil {
			started := r.clock.Now()
			w.running()
			for range changes {
				if ctx.Err() != nil {
					continue // draining until the watch closes the channel
				}
				w.changed()
			}
			if ctx.Err() != nil {
				return
			}
			// A watch that ran a long time failed once, and starts again
			// from the shortest delay.
			if r.clock.Now().Sub(started) >= tableWatchRetryMax {
				retry = tableWatchRetryFirst
			}
			err = w.ended
		}
		w.stopped(err)
		if !w.wait(ctx, r.clock, retry) {
			return
		}
		retry = min(2*retry, tableWatchRetryMax)
		changes, err = w.start(ctx)
	}
}

// waitTableWatchRetry waits delay on clk, and reports false when ctx ended
// first.
func waitTableWatchRetry(ctx context.Context, clk clock.Clock, delay time.Duration) bool {
	timer := clk.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C():
		return true
	}
}
