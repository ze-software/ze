// Design: docs/research/l2tpv2-ze-integration.md -- timer goroutine for retransmit + hello deadlines
// Related: reactor.go -- consumes tickReq, produces heapUpdate
// Related: subsystem.go -- owns the timer's lifecycle

package l2tp

import (
	"container/heap"
	"errors"
	"sync"
	"time"
)

// tickReq is sent from the timer goroutine to the reactor when a
// tunnel's deadline has expired. The reactor calls engine.Tick(now)
// and sends back a heapUpdate with the new deadline.
type tickReq struct {
	tunnelID uint16
}

// heapUpdate is sent from the reactor to the timer goroutine after it
// has processed a tick or any dispatch that changes a tunnel's next
// deadline. A zero deadline means the tunnel has no pending deadline
// (remove from heap). The reactor also sends a heapUpdate when a new
// tunnel is created or an existing tunnel's engine emits a new send.
type heapUpdate struct {
	tunnelID uint16
	deadline time.Time // zero = remove from heap
}

// timerEntry is one element in the min-heap, keyed by deadline.
type timerEntry struct {
	tunnelID uint16
	deadline time.Time
	index    int // maintained by heap.Interface
}

// timerHeap implements container/heap for timerEntry, ordered by
// earliest deadline first.
type timerHeap []*timerEntry

func (h timerHeap) Len() int           { return len(h) }
func (h timerHeap) Less(i, j int) bool { return h[i].deadline.Before(h[j].deadline) }
func (h timerHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i]; h[i].index = i; h[j].index = j }

//nolint:errcheck,forcetypeassert // heap.Interface contract guarantees *timerEntry; panic on violation is correct.
func (h *timerHeap) Push(x any) { e := x.(*timerEntry); e.index = len(*h); *h = append(*h, e) }
func (h *timerHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	old[n-1] = nil
	e.index = -1
	*h = old[:n-1]
	return e
}

// tunnelTimer is a single long-lived goroutine that maintains a
// min-heap of per-tunnel deadlines. It pops the earliest deadline,
// sleeps until it fires, and sends a tickReq to the reactor. The
// reactor processes the tick and sends back a heapUpdate with the new
// deadline (or zero to remove). The timer MUST NOT touch engine state;
// all engine calls happen in the reactor goroutine.
//
// Lifecycle: Start() launches the goroutine. Stop() signals it and
// waits. Caller MUST call Stop when done if Start returned nil.
type tunnelTimer struct {
	tick   chan<- tickReq    // timer -> reactor
	update <-chan heapUpdate // reactor -> timer

	mu      sync.Mutex
	stop    chan struct{}
	wg      sync.WaitGroup
	started bool

	// byTunnel maps tunnel ID to its heap entry for O(1) lookup on
	// heapUpdate. Only accessed from the timer goroutine (no lock needed
	// beyond the goroutine's own serial execution).
	byTunnel map[uint16]*timerEntry
	h        timerHeap
}

// newTunnelTimer constructs a timer that sends tick requests on tickCh
// and receives heap updates on updateCh.
func newTunnelTimer(tickCh chan<- tickReq, updateCh <-chan heapUpdate) *tunnelTimer {
	return &tunnelTimer{
		tick:     tickCh,
		update:   updateCh,
		byTunnel: make(map[uint16]*timerEntry),
	}
}

// Start launches the timer goroutine. Returns an error if already started.
func (t *tunnelTimer) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.started {
		return errAlreadyStarted
	}
	t.stop = make(chan struct{})
	t.started = true
	t.wg.Add(1)
	go t.run()
	return nil
}

// Stop signals the timer goroutine to exit and waits. Idempotent.
func (t *tunnelTimer) Stop() {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return
	}
	t.started = false
	stop := t.stop
	t.mu.Unlock()

	close(stop)
	t.wg.Wait()
}

// run is the timer's main loop. It sleeps until the earliest deadline,
// sends a tickReq, and processes heapUpdate responses. The loop exits
// when stop is closed.
func (t *tunnelTimer) run() {
	defer t.wg.Done()

	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	// timerC returns the channel to select on: nil if the heap is empty
	// (blocks forever in select), otherwise the timer's channel.
	timerC := func() <-chan time.Time {
		if timer == nil {
			return nil
		}
		return timer.C
	}

	// setTimer creates or resets the timer to fire at the heap's earliest
	// deadline. consumed indicates whether the caller already drained
	// timer.C (true after case <-timerC(); false after case update).
	setTimer := func(consumed bool) {
		if t.h.Len() == 0 {
			if timer != nil {
				if !consumed {
					timer.Stop()
				}
				timer = nil
			}
			return
		}
		earliest := t.h[0].deadline
		d := max(time.Until(earliest), 0)
		if timer == nil {
			timer = time.NewTimer(d)
			return
		}
		if consumed {
			// Timer already fired and we consumed it. Stop the old
			// timer (no-op since it already fired, but lets the runtime
			// reclaim it promptly) and create fresh.
			timer.Stop()
			timer = time.NewTimer(d)
			return
		}
		// Timer may or may not have fired. Stop it, drain if it fired,
		// then reset. Since we are the only reader of timer.C, a
		// non-stopped timer has its value sitting in the channel and
		// the receive will not block.
		if !timer.Stop() {
			<-timer.C
		}
		timer.Reset(d)
	}

	for {
		select {
		case <-t.stop:
			return

		case upd, ok := <-t.update:
			if !ok {
				return
			}
			t.applyUpdate(upd)
			setTimer(false)

		case <-timerC():
			// Earliest deadline fired. We consumed the channel value.
			if t.h.Len() == 0 {
				timer = nil
				continue
			}
			entry, _ := heap.Pop(&t.h).(*timerEntry) //nolint:errcheck // heap only contains *timerEntry
			delete(t.byTunnel, entry.tunnelID)

			// Send tickReq. If the reactor is stopped, the stop
			// channel will fire and we exit. Backpressure is
			// intentional per the handover design.
			select {
			case t.tick <- tickReq{tunnelID: entry.tunnelID}:
			case <-t.stop:
				return
			}
			// The reactor will send a heapUpdate in response; we loop
			// back and pick it up from the update channel.
			setTimer(true)
		}
	}
}

// applyUpdate processes one heapUpdate: inserts, updates, or removes
// a tunnel's deadline in the heap. Called only from the timer goroutine.
func (t *tunnelTimer) applyUpdate(upd heapUpdate) {
	existing, ok := t.byTunnel[upd.tunnelID]

	if upd.deadline.IsZero() {
		// Remove request.
		if ok {
			heap.Remove(&t.h, existing.index)
			delete(t.byTunnel, upd.tunnelID)
		}
		return
	}

	if ok {
		// Update existing entry.
		existing.deadline = upd.deadline
		heap.Fix(&t.h, existing.index)
	} else {
		// Insert new entry.
		entry := &timerEntry{tunnelID: upd.tunnelID, deadline: upd.deadline}
		heap.Push(&t.h, entry)
		t.byTunnel[upd.tunnelID] = entry
	}
}

var errAlreadyStarted = errors.New("l2tp: timer already started")
