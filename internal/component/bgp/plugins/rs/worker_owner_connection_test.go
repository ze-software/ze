// Design: docs/guide/route-reflection.md -- original-source worker isolation.
package rs

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Construction refuses absent provenance without deriving an owner or physical
// connection from a shared address. Exact wide identities remain independent.
func TestWorkerSourceKeyRequiresCapturedIdentity(t *testing.T) {
	for _, input := range []struct {
		address           string
		owner, connection uint64
	}{
		{"192.0.2.240", 0, 2}, {"192.0.2.240", 1, 0}, {"bad", 1, 2},
	} {
		_, err := newWorkerKey(input.address, input.owner, input.connection)
		require.Error(t, err)
	}
	key, err := newWorkerKey("192.0.2.240", math.MaxUint64, math.MaxUint64-1)
	require.NoError(t, err)
	require.Equal(t, uint64(math.MaxUint64), key.owner)
	require.Equal(t, uint64(math.MaxUint64-1), key.connectionID)
	require.Equal(t, "192.0.2.240", key.address.String())
}

// A held original connection cannot stall its useful successor or same-address
// sibling, and ending that connection cannot clear either survivor's lane.
// MUTATION: erase the physical component at the pool's actual dispatch boundary.
func TestWorkerPoolCapturedConnectionsStayIndependent(t *testing.T) {
	old, err := newWorkerKey("192.0.2.240", 31, 41)
	require.NoError(t, err)
	fresh, err := newWorkerKey("192.0.2.240", 31, 42)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.240", 32, 43)
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	processed := make(chan workerKey, 8)
	var releaseOnce sync.Once
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Hour
	pool := newWorkerPool(func(key workerKey, item workItem) {
		if key == old {
			close(entered)
			<-release
		}
		processed <- key
	}, cfg)
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); pool.Stop() })
	require.True(t, pool.Dispatch(old, workItem{msgID: 1}))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("original worker did not enter")
	}
	require.True(t, pool.Dispatch(fresh, workItem{msgID: 2}))
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 3}))
	got := make(map[workerKey]bool)
	for range 2 {
		select {
		case key := <-processed:
			got[key] = true
		case <-time.After(5 * time.Second):
			t.Fatal("independent useful lane was blocked")
		}
	}
	require.True(t, got[fresh])
	require.True(t, got[sibling])
	pool.inBackpressure.Store(fresh, true)
	pool.inBackpressure.Store(sibling, true)
	releaseOnce.Do(func() { close(release) })
	pool.PeerDown(old)
	require.Equal(t, 2, pool.WorkerCount())
	_, freshPressure := pool.inBackpressure.Load(fresh)
	require.True(t, freshPressure)
	_, siblingPressure := pool.inBackpressure.Load(sibling)
	require.True(t, siblingPressure)
}

// Policy settlement joins all already-admitted physical lanes of its exact
// original owner. Another original owner remains independently useful and does
// not hold the receipt, even when every source uses the same address.
// MUTATION: settle just the first matching physical worker.
func TestWorkerPoolSettleOwnerJoinsEveryCapturedConnection(t *testing.T) {
	old, err := newWorkerKey("192.0.2.241", 51, 61)
	require.NoError(t, err)
	fresh, err := newWorkerKey("192.0.2.241", 51, 62)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.241", 52, 63)
	require.NoError(t, err)
	releases := map[workerKey]*workerQuestionRelease{old: {channel: make(chan struct{})}, fresh: {channel: make(chan struct{})}, sibling: {channel: make(chan struct{})}}
	entered := make(chan workerKey, 3)
	handled := make(chan workerKey, 3)
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Hour
	pool := newWorkerPool(func(key workerKey, item workItem) {
		if item.barrier != nil {
			close(item.barrier)
			return
		}
		entered <- key
		<-releases[key].channel
		handled <- key
	}, cfg)
	t.Cleanup(func() {
		for _, release := range releases {
			release.release()
		}
		pool.Stop()
	})
	for _, key := range []workerKey{old, fresh, sibling} {
		require.True(t, pool.Dispatch(key, workItem{msgID: 1}))
	}
	for range 3 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("captured worker not admitted")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	settled := make(chan error, 1)
	go func() { settled <- pool.settleOwner(ctx, old.owner) }()
	// Inspect actual queued markers while all worker handlers are held. This is
	// the ordering barrier; elapsed time does not decide the assertion.
	require.Eventually(t, func() bool {
		pool.mu.Lock()
		defer pool.mu.Unlock()
		if len(pool.workers[old].ch) != 1 {
			return false
		}
		return len(pool.workers[fresh].ch) == 1
	}, time.Second, time.Millisecond, "both physical workers must hold a settle marker")
	releases[old].release()
	<-handled
	select {
	case err := <-settled:
		t.Fatalf("receipt escaped the held successor: %v", err)
	default:
	}
	releases[fresh].release()
	<-handled
	select {
	case err := <-settled:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("same-address sibling incorrectly held owner settlement")
	}
	releases[sibling].release()
	<-handled
}

// Actual owner-terminal removal joins all its physical lanes, while the same
// address under a different original owner remains admitted and useful.
func TestWorkerPoolOwnerRemovalKeepsSiblingLane(t *testing.T) {
	old, err := newWorkerKey("192.0.2.242", 71, 81)
	require.NoError(t, err)
	fresh, err := newWorkerKey("192.0.2.242", 71, 82)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.242", 72, 83)
	require.NoError(t, err)
	handled := make(chan workerKey, 4)
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Hour
	pool := newWorkerPool(func(key workerKey, item workItem) { handled <- key }, cfg)
	t.Cleanup(pool.Stop)
	for _, key := range []workerKey{old, fresh, sibling} {
		require.True(t, pool.Dispatch(key, workItem{msgID: 1}))
	}
	for range 3 {
		select {
		case <-handled:
		case <-time.After(5 * time.Second):
			t.Fatal("useful source work was not processed")
		}
	}
	require.NoError(t, pool.endOwner(old.owner))
	require.Equal(t, 1, pool.WorkerCount())
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 2}))
	select {
	case key := <-handled:
		require.Equal(t, sibling, key)
	case <-time.After(5 * time.Second):
		t.Fatal("owner removal lost useful sibling work")
	}
	require.Error(t, pool.endOwner(0))
	require.Error(t, pool.settleOwner(context.Background(), 0))
}

// A terminal owner removal can meet an already-running physical DOWN. Both
// callers join the one admitted worker without closing its queue twice.
func TestWorkerPoolTerminalRemovalJoinsConcurrentDown(t *testing.T) {
	key, err := newWorkerKey("192.0.2.243", 91, 101)
	require.NoError(t, err)
	entered := make(chan struct{})
	release := workerQuestionRelease{channel: make(chan struct{})}
	pool := newWorkerPool(func(workerKey, workItem) { close(entered); <-release.channel }, testPoolConfig())
	t.Cleanup(func() { release.release(); pool.Stop() })
	require.True(t, pool.Dispatch(key, workItem{msgID: 1}))
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("physical lane never entered")
	}
	physicalDone, ownerDone := make(chan struct{}), make(chan error, 1)
	go func() { pool.PeerDown(key); close(physicalDone) }()
	require.Eventually(t, func() bool {
		pool.mu.Lock()
		defer pool.mu.Unlock()
		select {
		case _, open := <-pool.workers[key].ch:
			return !open
		default:
			return false
		}
	}, time.Second, time.Millisecond, "physical DOWN must have closed its original queue and be joining its held handler")
	// First DOWN has already passed pressure cleanup before closing ch. A fresh
	// flag can now be consumed only by the second, terminal retirement caller.
	pool.backpressure.Store(key, true)
	go func() { ownerDone <- pool.endOwner(key.owner) }()
	require.Eventually(t, func() bool { _, present := pool.backpressure.Load(key); return !present }, time.Second, time.Millisecond, "terminal removal must enter the original retirement before its handler is released")
	release.release()
	select {
	case <-physicalDone:
	case <-time.After(5 * time.Second):
		t.Fatal("physical DOWN did not join")
	}
	select {
	case err := <-ownerDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal removal did not join")
	}
	require.Zero(t, pool.WorkerCount())
}

// Each question owns and releases every held handler even after an assertion
// aborts its body; a failed regression must not strand the shared test process.
type workerQuestionRelease struct {
	channel chan struct{}
	once    sync.Once
}

func (release *workerQuestionRelease) release() { release.once.Do(func() { close(release.channel) }) }
