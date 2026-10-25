package syncutil

import (
	"maps"
	"slices"
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
	c := NewCoalescer(
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
