package iface

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// watchingBackend is a fakeBackend whose neighbor watch the test drives: it
// signals the current watch, ends it the way a failed kernel subscription
// does, and ends it on Close, as Backend.WatchNeighbors requires.
type watchingBackend struct {
	*fakeBackend
	mu      sync.Mutex
	changes chan struct{}
	ended   bool
	starts  int
}

func newWatchingBackend() *watchingBackend {
	return &watchingBackend{fakeBackend: &fakeBackend{ifaces: map[string]fakeIface{}}}
}

func (b *watchingBackend) WatchNeighbors(_ context.Context) (<-chan struct{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.starts++
	b.changes = make(chan struct{}, 1)
	b.ended = false
	return b.changes, nil
}

// end closes the current watch's channel once.
func (b *watchingBackend) end() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.changes == nil {
		return
	}
	if b.ended {
		return
	}
	b.ended = true
	close(b.changes)
}

func (b *watchingBackend) signal() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.changes <- struct{}{}
}

func (b *watchingBackend) startCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.starts
}

func (b *watchingBackend) Close() error {
	b.end()
	return nil
}

// loadWatchingBackend registers b under a name unique to the test and makes
// it the active backend.
func loadWatchingBackend(t *testing.T, b *watchingBackend, suffix string) {
	t.Helper()
	name := "test-" + t.Name() + "-" + suffix
	require.NoError(t, RegisterBackend(name, func() (Backend, error) { return b, nil }))
	require.NoError(t, LoadBackend(name))
}

// awaitSignal fails unless out carries a signal within five seconds while
// staying open.
func awaitSignal(t *testing.T, out <-chan struct{}, why string) {
	t.Helper()
	select {
	case _, open := <-out:
		require.True(t, open, "the watch closed: %s", why)
	case <-time.After(5 * time.Second):
		t.Fatal(why)
	}
}

// awaitClosed fails unless out closes within five seconds.
func awaitClosed(t *testing.T, out <-chan struct{}, why string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-out:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal(why)
		}
	}
}

// TestWatchIPv6NeighborsFollowsAReplacedBackend proves the watch BGP holds
// survives a replaced backend: when LoadBackend closes the old backend (a
// reload, or the boot sequence's second load), the watch moves to the backend
// that replaced it and signals once so BGP reads the new table, and its
// channel stays open, so BGP neither logs that the table is not watched nor
// waits a retry delay. Signals of the new backend's watch then reach BGP.
//
// MUTATION: return from followNeighborWatch when the backend's channel
// closes, without the GetBackend test: the channel closes at the replacement
// and the first await fails.
func TestWatchIPv6NeighborsFollowsAReplacedBackend(t *testing.T) {
	first, second := newWatchingBackend(), newWatchingBackend()
	loadWatchingBackend(t, first, "first")
	t.Cleanup(func() { _ = CloseBackend() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	out, err := watchIPv6Neighbors(ctx)
	require.NoError(t, err)
	first.signal()
	awaitSignal(t, out, "a signal of the first backend's watch never arrived")

	loadWatchingBackend(t, second, "second")
	awaitSignal(t, out, "the replacement raised no signal, so BGP never reads the new backend's table")
	require.Eventually(t, func() bool { return second.startCount() == 1 },
		5*time.Second, time.Millisecond, "the watch moved to the replacing backend")

	second.signal()
	awaitSignal(t, out, "a signal of the replacing backend's watch never arrived")
}

// TestWatchIPv6NeighborsEndsWhenTheWatchFails proves the watch closes its
// channel when the active backend's own watch ends with that backend still
// in place (a failed kernel subscription), and when the backend is closed
// with none in its place, so BGP learns the table is no longer watched.
func TestWatchIPv6NeighborsEndsWhenTheWatchFails(t *testing.T) {
	t.Run("backend watch failed", func(t *testing.T) {
		b := newWatchingBackend()
		loadWatchingBackend(t, b, "failed")
		t.Cleanup(func() { _ = CloseBackend() })
		out, err := watchIPv6Neighbors(context.Background())
		require.NoError(t, err)
		b.end()
		awaitClosed(t, out, "the watch stayed open after the backend's own watch ended")
		require.Equal(t, 1, b.startCount(), "a failed watch is not restarted here: the caller retries it")
	})
	t.Run("backend closed with none in its place", func(t *testing.T) {
		b := newWatchingBackend()
		loadWatchingBackend(t, b, "closed")
		out, err := watchIPv6Neighbors(context.Background())
		require.NoError(t, err)
		require.NoError(t, CloseBackend())
		awaitClosed(t, out, "the watch stayed open after the backend was closed")
	})
}
