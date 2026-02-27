package sim

import (
	"container/heap"
	"sync"
	"time"
)

// VirtualClock implements Clock with a timer min-heap and Advance-driven firing.
//
// Unlike FakeClock (inert timers for unit tests), VirtualClock maintains pending
// timers sorted by deadline and fires them in order when Advance() is called.
// This enables deterministic simulation of FSM timers (hold, keepalive, connect-retry)
// without wall-clock waiting.
//
// Timer firing is synchronous: Advance() fires callbacks in the caller's goroutine,
// in deadline order (FIFO for same-deadline timers). This eliminates goroutine
// scheduling non-determinism.
//
// Design: DST analysis Section 11.2 — timer heap for ordered firing.
type VirtualClock struct {
	mu   sync.Mutex
	now  time.Time
	heap timerHeap
	seq  uint64 // monotonic counter for FIFO tie-breaking
}

// NewVirtualClock creates a VirtualClock starting at the given time.
func NewVirtualClock(start time.Time) *VirtualClock {
	return &VirtualClock{now: start}
}

// Now returns the current simulated time.
func (vc *VirtualClock) Now() time.Time {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	return vc.now
}

// Sleep blocks until the simulated time advances past now+d.
// Another goroutine must call Advance() to unblock.
func (vc *VirtualClock) Sleep(d time.Duration) {
	ch := vc.After(d)
	<-ch
}

// After returns a channel that receives the simulated time when now+d is reached.
func (vc *VirtualClock) After(d time.Duration) <-chan time.Time {
	timer := vc.NewTimer(d)
	return timer.C()
}

// AfterFunc schedules f to be called when now+d is reached via Advance().
// Returns a Timer whose Stop/Reset methods control the scheduled callback.
// The callback f is called synchronously during Advance(), not in a separate goroutine.
func (vc *VirtualClock) AfterFunc(d time.Duration, f func()) Timer {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	entry := &timerEntry{
		deadline: vc.now.Add(d),
		callback: f,
		seq:      vc.seq,
	}
	vc.seq++
	heap.Push(&vc.heap, entry)

	return &virtualTimer{clock: vc, entry: entry}
}

// NewTimer creates a Timer whose C() channel receives when now+d is reached via Advance().
func (vc *VirtualClock) NewTimer(d time.Duration) Timer {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	ch := make(chan time.Time, 1)
	entry := &timerEntry{
		deadline: vc.now.Add(d),
		ch:       ch,
		seq:      vc.seq,
	}
	vc.seq++
	heap.Push(&vc.heap, entry)

	return &virtualTimer{clock: vc, entry: entry}
}

// NewTicker returns a ticker backed by a repeating virtual timer.
// Each Advance() past a tick deadline fires the tick and re-schedules the next.
func (vc *VirtualClock) NewTicker(d time.Duration) Ticker {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	ch := make(chan time.Time, 1)
	vt := &virtualTicker{clock: vc, ch: ch, interval: d}
	entry := &timerEntry{
		deadline: vc.now.Add(d),
		callback: vt.fire,
		seq:      vc.seq,
	}
	vc.seq++
	vt.entry = entry
	heap.Push(&vc.heap, entry)

	return vt
}

// Advance moves simulated time forward by d and fires all timers with
// deadline <= new now, in deadline order (FIFO for same-deadline).
// Callbacks are called synchronously in the caller's goroutine.
func (vc *VirtualClock) Advance(d time.Duration) {
	vc.mu.Lock()
	target := vc.now.Add(d)
	vc.mu.Unlock()

	vc.advanceTo(target)
}

// AdvanceTo moves simulated time to the given absolute time and fires
// all timers with deadline <= target.
func (vc *VirtualClock) AdvanceTo(target time.Time) {
	vc.advanceTo(target)
}

// advanceTo is the shared implementation for Advance and AdvanceTo.
func (vc *VirtualClock) advanceTo(target time.Time) {
	for {
		vc.mu.Lock()

		// Pop the earliest timer if its deadline is <= target.
		if vc.heap.Len() == 0 {
			vc.now = target
			vc.mu.Unlock()
			return
		}

		earliest := vc.heap[0]
		if earliest.deadline.After(target) {
			vc.now = target
			vc.mu.Unlock()
			return
		}

		// Pop the timer and advance clock to its deadline.
		heap.Pop(&vc.heap)
		vc.now = earliest.deadline

		// Check if timer was stopped.
		if earliest.stopped {
			vc.mu.Unlock()
			continue
		}
		earliest.fired = true

		// Release lock before firing — callback may create new timers.
		vc.mu.Unlock()

		// Fire the timer.
		if earliest.callback != nil {
			earliest.callback()
		}
		if earliest.ch != nil {
			earliest.ch <- earliest.deadline
		}
	}
}

// timerEntry is an element in the timer min-heap.
type timerEntry struct {
	deadline time.Time
	callback func()         // non-nil for AfterFunc timers
	ch       chan time.Time // non-nil for NewTimer/After timers
	seq      uint64         // insertion order for FIFO tie-breaking
	index    int            // heap index (managed by container/heap)
	stopped  bool           // Stop() was called
	fired    bool           // timer has fired
}

// timerHeap implements heap.Interface for timerEntry pointers.
// Orders by deadline first, then by seq (insertion order) for FIFO.
type timerHeap []*timerEntry

func (h timerHeap) Len() int { return len(h) }

func (h timerHeap) Less(i, j int) bool {
	if h[i].deadline.Equal(h[j].deadline) {
		return h[i].seq < h[j].seq
	}
	return h[i].deadline.Before(h[j].deadline)
}

func (h timerHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *timerHeap) Push(x any) {
	entry, ok := x.(*timerEntry)
	if !ok {
		return
	}
	entry.index = len(*h)
	*h = append(*h, entry)
}

func (h *timerHeap) Pop() any {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil // avoid memory leak
	entry.index = -1
	*h = old[:n-1]
	return entry
}

// virtualTimer implements Timer backed by a timerEntry in the VirtualClock heap.
type virtualTimer struct {
	clock *VirtualClock
	entry *timerEntry
}

// Stop prevents the timer from firing.
// Returns true if the timer was active (not yet fired or stopped).
func (vt *virtualTimer) Stop() bool {
	vt.clock.mu.Lock()
	defer vt.clock.mu.Unlock()

	if vt.entry.stopped || vt.entry.fired {
		return false
	}
	vt.entry.stopped = true
	return true
}

// Reset changes the timer to expire after duration d from the current simulated time.
// Returns true if the timer was active, false if it had been stopped or fired.
func (vt *virtualTimer) Reset(d time.Duration) bool {
	vt.clock.mu.Lock()
	defer vt.clock.mu.Unlock()

	wasActive := !vt.entry.stopped && !vt.entry.fired

	// Mark old entry as stopped so it won't fire.
	vt.entry.stopped = true

	// Create a new entry with the new deadline.
	newEntry := &timerEntry{
		deadline: vt.clock.now.Add(d),
		callback: vt.entry.callback,
		ch:       vt.entry.ch,
		seq:      vt.clock.seq,
	}
	vt.clock.seq++
	heap.Push(&vt.clock.heap, newEntry)
	vt.entry = newEntry

	return wasActive
}

// C returns the timer's channel. For AfterFunc-created timers, returns nil.
func (vt *virtualTimer) C() <-chan time.Time {
	return vt.entry.ch
}

// virtualTicker implements Ticker backed by VirtualClock's timer heap.
// Each time the timer fires, fire() sends on the channel and re-schedules
// the next tick. Stop() prevents further ticks.
type virtualTicker struct {
	clock    *VirtualClock
	ch       chan time.Time
	interval time.Duration
	entry    *timerEntry
	stopped  bool
}

// Stop prevents future ticks from firing.
func (vt *virtualTicker) Stop() {
	vt.clock.mu.Lock()
	defer vt.clock.mu.Unlock()
	vt.stopped = true
	if vt.entry != nil {
		vt.entry.stopped = true
	}
}

// C returns the ticker's channel.
func (vt *virtualTicker) C() <-chan time.Time { return vt.ch }

// fire is the callback invoked by Advance() when a tick deadline is reached.
// It sends the current time on the channel and schedules the next tick.
// Called with the clock mutex released (Advance releases before firing).
func (vt *virtualTicker) fire() {
	vt.clock.mu.Lock()
	if vt.stopped {
		vt.clock.mu.Unlock()
		return
	}
	now := vt.clock.now
	vt.clock.mu.Unlock()

	// Non-blocking send — drop tick if consumer hasn't read the previous one.
	select {
	case vt.ch <- now:
	default: // buffer full — tick already pending, skip
	}

	// Schedule next tick.
	vt.clock.mu.Lock()
	defer vt.clock.mu.Unlock()
	if vt.stopped {
		return
	}
	entry := &timerEntry{
		deadline: vt.clock.now.Add(vt.interval),
		callback: vt.fire,
		seq:      vt.clock.seq,
	}
	vt.clock.seq++
	vt.entry = entry
	heap.Push(&vt.clock.heap, entry)
}
