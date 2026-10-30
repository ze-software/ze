// Design: docs/contributing/ze-go-style.md -- Goroutines: a fixed set of long-lived workers on one queue
// Related: wait.go -- the other concurrency helper of this package

package syncutil

import (
	"context"
	"sync"
)

// Coalescer is a fixed set of long-lived workers that hand each queued value
// to its handler, taking keys in the order each was first queued.
//
// A key queued again before a worker took its waiting value is merged into
// that value, so the queue holds at most one value per key. Its depth is
// therefore bounded by the number of distinct keys, and Enqueue never blocks:
// a caller on an event loop cannot be stalled by a slow handler.
//
// A key is handled by one worker at a time: a value queued while its key is
// being handled waits until that handler returns, so the values of one key
// are handled in order and never concurrently. Values of different keys run
// concurrently on up to the worker count, so one slow handler holds back only
// its own key and one worker, never the whole queue.
//
// Lifecycle: NewCoalescer creates the queue and starts the workers, Enqueue
// adds work, and Stop MUST be called once the owner stops queuing. Stop
// returns only after every value queued before it was handled. Safe for
// concurrent use.
type Coalescer[K comparable, V any] struct {
	merge  func(waiting, next V) V
	handle func(key K, value V)
	exited sync.WaitGroup

	mu sync.Mutex
	// ready is signaled under mu when a value may have become takeable (an
	// Enqueue, a handler returning) or Stop was called.
	ready   *sync.Cond
	order   []K
	waiting map[K]V
	// active holds the keys a worker is handling now. Bounded by the worker
	// count.
	active  map[K]struct{}
	stopped bool
}

// NewCoalescer starts workers long-lived workers, at least one. merge combines
// a value still waiting with one queued under the same key, and handle runs on
// a worker for each value taken. The caller MUST call Stop.
func NewCoalescer[K comparable, V any](workers int, merge func(waiting, next V) V, handle func(key K, value V)) *Coalescer[K, V] {
	c := &Coalescer[K, V]{
		merge:   merge,
		handle:  handle,
		waiting: make(map[K]V),
		active:  make(map[K]struct{}),
	}
	c.ready = sync.NewCond(&c.mu)
	workers = max(workers, 1)
	c.exited.Add(workers)
	for range workers {
		go c.run()
	}
	return c
}

// Enqueue queues value under key, merging it into a waiting value of the same
// key. It answers false, queuing nothing, once Stop was called.
func (c *Coalescer[K, V]) Enqueue(key K, value V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return false
	}
	if waiting, ok := c.waiting[key]; ok {
		c.waiting[key] = c.merge(waiting, value)
		return true
	}
	c.waiting[key] = value
	c.order = append(c.order, key)
	c.ready.Signal()
	return true
}

// Stop refuses further values and waits until the workers handled every
// value already queued, then the workers exit. A second call only waits.
func (c *Coalescer[K, V]) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.ready.Broadcast()
	c.mu.Unlock()
	c.exited.Wait()
}

// Await waits until no value of key is waiting or being handled, so every
// value queued under key before the call has been handled, or until ctx ends,
// which it returns. A value queued under key while it waits extends the wait.
// It never waits on another key. Safe for concurrent use.
func (c *Coalescer[K, V]) Await(ctx context.Context, key K) error {
	// The cond cannot watch ctx: its end wakes every waiter to look again.
	wake := context.AfterFunc(ctx, func() {
		c.mu.Lock()
		c.ready.Broadcast()
		c.mu.Unlock()
	})
	defer wake()
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.holds(key) {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.ready.Wait()
	}
	return nil
}

// holds reports whether a value of key waits or is being handled. MUST be
// called with mu held.
func (c *Coalescer[K, V]) holds(key K) bool {
	if _, waiting := c.waiting[key]; waiting {
		return true
	}
	_, busy := c.active[key]
	return busy
}

// run is one worker. It handles values until Stop was called and nothing is
// left waiting.
func (c *Coalescer[K, V]) run() {
	defer c.exited.Done()
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		key, value, taken := c.take()
		if taken {
			c.active[key] = struct{}{}
			c.mu.Unlock()
			c.handle(key, value)
			c.mu.Lock()
			delete(c.active, key)
			// A value of this key may wait behind the handler just finished,
			// and Stop may wait for this worker to see the queue drained.
			c.ready.Broadcast()
			continue
		}
		if c.stopped && len(c.order) == 0 {
			return
		}
		c.ready.Wait()
	}
}

// take removes the oldest waiting value whose key no worker is handling.
// taken is false when no such value waits. MUST be called with mu held.
func (c *Coalescer[K, V]) take() (key K, value V, taken bool) {
	for i, candidate := range c.order {
		if _, busy := c.active[candidate]; busy {
			continue
		}
		c.order = append(c.order[:i], c.order[i+1:]...)
		value = c.waiting[candidate]
		delete(c.waiting, candidate)
		return candidate, value, true
	}
	return key, value, false
}
