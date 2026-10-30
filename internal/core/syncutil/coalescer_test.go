package syncutil

import (
	"context"
	"maps"
	"slices"
	"sync"
	"testing"
	"time"
)

// TestCoalescerOrderMergeAndStop holds the worker inside its first value,
// queues three more keys and one of them twice, then stops.
//
// VALIDATES: Enqueue returns while the handler is blocked; values are handled
// one at a time in the order their key was first queued; a key queued twice
// while waiting is handled once with both values merged; Stop handles every
// waiting value before it returns, and refuses values after it.
// PREVENTS: a queue that blocks its event-loop caller, reorders sources, or
// drops a departure queued before shutdown.
func TestCoalescerOrderMergeAndStop(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var handled []string
	values := make(map[string][]int)
	c := NewCoalescer(1,
		func(waiting, next []int) []int { return append(waiting, next...) },
		func(key string, value []int) {
			if key == "first" {
				close(entered)
				<-release
			}
			handled = append(handled, key)
			values[key] = value
		},
	)
	c.Enqueue("first", []int{0})
	<-entered

	queued := make(chan struct{})
	go func() {
		c.Enqueue("b", []int{1})
		c.Enqueue("a", []int{2})
		c.Enqueue("b", []int{3})
		c.Enqueue("c", []int{4})
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(5 * time.Second):
		t.Fatal("Enqueue blocked while the handler was busy")
	}
	close(release)
	c.Stop()

	if want := []string{"first", "b", "a", "c"}; !slices.Equal(handled, want) {
		t.Fatalf("handled %v, want %v", handled, want)
	}
	want := map[string][]int{"first": {0}, "b": {1, 3}, "a": {2}, "c": {4}}
	if !maps.EqualFunc(values, want, slices.Equal[[]int]) {
		t.Fatalf("values %v, want %v", values, want)
	}
	if c.Enqueue("late", []int{5}) {
		t.Fatal("Enqueue after Stop answered true")
	}
	c.Stop()
}

// TestCoalescerKeysRunConcurrently holds one key's handler and queues a
// second key, then queues the held key again.
//
// VALIDATES: with two workers, a second key is handled while the first key's
// handler is still blocked; a value queued for a key whose handler is running
// waits for that handler and never runs beside it; Stop drains both.
// PREVENTS: one stalled departure holding every other source's departure
// behind it (head-of-line), and two departures of one source racing.
func TestCoalescerKeysRunConcurrently(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	otherDone := make(chan struct{})
	var mu sync.Mutex
	running := make(map[string]int)
	overlap := false
	calls := make(map[string]int)
	c := NewCoalescer(2,
		func(waiting, next []int) []int { return append(waiting, next...) },
		func(key string, _ []int) {
			mu.Lock()
			running[key]++
			if running[key] > 1 {
				overlap = true
			}
			calls[key]++
			first := key == "stalled" && calls[key] == 1
			mu.Unlock()
			if first {
				close(entered)
				<-release
			}
			if key == "other" {
				close(otherDone)
			}
			mu.Lock()
			running[key]--
			mu.Unlock()
		},
	)
	c.Enqueue("stalled", []int{0})
	<-entered
	c.Enqueue("stalled", []int{1})
	c.Enqueue("other", []int{2})
	select {
	case <-otherDone:
	case <-time.After(5 * time.Second):
		t.Fatal("a second key waited behind a stalled handler")
	}
	close(release)
	c.Stop()

	mu.Lock()
	defer mu.Unlock()
	if overlap {
		t.Fatal("two values of one key were handled concurrently")
	}
	if calls["stalled"] != 2 || calls["other"] != 1 {
		t.Fatalf("calls %v, want stalled:2 other:1", calls)
	}
}

// TestCoalescerAwait holds the worker inside one key's value, then waits on
// that key, on a key nothing queued, and on the held key under a context that
// ends.
//
// VALIDATES: Await on a key with nothing queued returns at once; Await on a key
// being handled returns only after its handler returned; a context that ends
// first is returned as the error while the handler is still held.
// PREVENTS: a caller that settles one source's departure before draining its
// in-flight work reading "settled" while the departure is still being sent.
func TestCoalescerAwait(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var done sync.WaitGroup
	handled := make(chan string, 2)
	c := NewCoalescer(2,
		func(waiting, _ int) int { return waiting },
		func(key string, _ int) {
			if key == "held" {
				close(entered)
				<-release
			}
			handled <- key
		},
	)
	defer c.Stop()
	// Registered after Stop so it runs first: a failure below must not leave
	// the handler held while Stop waits for it.
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseHandler()
	c.Enqueue("held", 1)
	<-entered

	if err := c.Await(t.Context(), "idle"); err != nil {
		t.Fatalf("Await on a key nothing queued: %v", err)
	}
	short, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := c.Await(short, "held"); err == nil {
		t.Fatal("Await returned nil while the key's handler was still held")
	}

	awaited := make(chan error, 1)
	done.Go(func() {
		awaited <- c.Await(t.Context(), "held")
	})
	select {
	case err := <-awaited:
		t.Fatalf("Await returned %v before the handler returned", err)
	case <-time.After(20 * time.Millisecond):
	}
	releaseHandler()
	done.Wait()
	if err := <-awaited; err != nil {
		t.Fatalf("Await after the handler returned: %v", err)
	}
	if got := <-handled; got != "held" {
		t.Fatalf("handled %q, want held", got)
	}
}
