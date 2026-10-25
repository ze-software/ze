// Design: docs/contributing/ze-go-style.md -- Goroutines: one long-lived worker on a channel
// Related: wait.go -- the other concurrency helper of this package

package syncutil

import "sync"

// Coalescer is one long-lived worker that hands each queued value to its
// handler, one at a time, in the order the value's key was first queued.
//
// A key queued again before the worker took its waiting value is merged into
// that value, so the queue holds at most one value per key. Its depth is
// therefore bounded by the number of distinct keys, and Enqueue never blocks:
// a caller on an event loop cannot be stalled by a slow handler.
//
// Lifecycle: NewCoalescer creates the queue and starts the worker, Enqueue
// adds work, and Stop MUST be called once the owner stops queuing. Stop
// returns only after every value queued before it was handled. Safe for
// concurrent use.
type Coalescer[K comparable, V any] struct {
	merge  func(waiting, next V) V
	handle func(key K, value V)
	// wake holds one pending signal: an Enqueue or Stop that finds it full
	// knows the worker will look at the queue again.
	wake chan struct{}
	done chan struct{}

	mu      sync.Mutex
	order   []K
	waiting map[K]V
	stopped bool
}

// NewCoalescer starts the worker. merge combines a value still waiting with
// one queued under the same key, and handle runs on the worker for each value
// taken. The caller MUST call Stop.
func NewCoalescer[K comparable, V any](merge func(waiting, next V) V, handle func(key K, value V)) *Coalescer[K, V] {
	c := &Coalescer[K, V]{
		merge:   merge,
		handle:  handle,
		wake:    make(chan struct{}, 1),
		done:    make(chan struct{}),
		waiting: make(map[K]V),
	}
	go c.run()
	return c
}

// Enqueue queues value under key, merging it into a waiting value of the same
// key. It answers false, queuing nothing, once Stop was called.
func (c *Coalescer[K, V]) Enqueue(key K, value V) bool {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return false
	}
	if waiting, ok := c.waiting[key]; ok {
		c.waiting[key] = c.merge(waiting, value)
	} else {
		c.waiting[key] = value
		c.order = append(c.order, key)
	}
	c.mu.Unlock()
	c.signal()
	return true
}

// Stop refuses further values and waits until the worker handled every value
// already queued, then the worker exits. A second call only waits.
func (c *Coalescer[K, V]) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()
	c.signal()
	<-c.done
}

// signal wakes the worker without blocking: a full wake channel already
// holds a signal the worker has not consumed.
func (c *Coalescer[K, V]) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// run is the worker. It handles values until Stop was called and the queue
// is empty.
func (c *Coalescer[K, V]) run() {
	defer close(c.done)
	for {
		key, value, taken, stopped := c.take()
		if taken {
			c.handle(key, value)
			continue
		}
		if stopped {
			return
		}
		<-c.wake
	}
}

// take removes the oldest waiting value. taken is false when nothing waits,
// and stopped then tells the worker whether more can arrive.
func (c *Coalescer[K, V]) take() (key K, value V, taken, stopped bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.order) == 0 {
		return key, value, false, c.stopped
	}
	key = c.order[0]
	c.order = c.order[1:]
	value = c.waiting[key]
	delete(c.waiting, key)
	return key, value, true, c.stopped
}
