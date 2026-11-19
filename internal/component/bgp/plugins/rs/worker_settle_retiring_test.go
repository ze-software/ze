// Design: docs/guide/route-reflection.md -- joined original physical workers.
// Related: worker_retirement_capture_test.go -- captured lane retirement owners.
package rs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A policy/cleanup waiter captures both live and retiring lanes of one actual
// owner. It waits through the original close callback, while another owner at
// the same address remains useful and does not enter its settlement frontier.
// MUTATION: return a closing error instead of joining the captured retiring lane.
func TestWorkerPoolSettleOwnerJoinsRetiringClose(t *testing.T) {
	original, err := newWorkerKey("192.0.2.248", 191, 201)
	require.NoError(t, err)
	fresh, err := newWorkerKey("192.0.2.248", 191, 202)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.248", 192, 203)
	require.NoError(t, err)
	originalRelease := workerQuestionRelease{channel: make(chan struct{})}
	closeRelease := workerQuestionRelease{channel: make(chan struct{})}
	entered := make(chan workerKey, 4)
	closeEntered := make(chan struct{})
	markers := make(chan workerKey, 4)
	var closeOnce sync.Once
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Hour
	cfg.onClose = func(key workerKey) {
		if key == original {
			closeOnce.Do(func() { close(closeEntered) })
			<-closeRelease.channel
		}
	}
	pool := newWorkerPool(func(key workerKey, item workItem) {
		if item.barrier != nil {
			markers <- key
			close(item.barrier)
			return
		}
		entered <- key
		if key == original {
			<-originalRelease.channel
		}
	}, cfg)
	var wg sync.WaitGroup
	t.Cleanup(func() { originalRelease.release(); closeRelease.release(); pool.Stop(); wg.Wait() })
	for _, key := range []workerKey{original, fresh, sibling} {
		require.True(t, pool.Dispatch(key, workItem{msgID: 1}))
	}
	for range 3 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("original useful worker never admitted")
		}
	}
	pool.mu.Lock()
	captured := pool.workers[original]
	pool.mu.Unlock()
	down := make(chan struct{})
	wg.Add(1)
	go func() { defer wg.Done(); pool.PeerDown(original); close(down) }()
	select {
	case <-captured.closeCh:
	case <-time.After(5 * time.Second):
		t.Fatal("physical DOWN never sealed the original")
	}
	settled := make(chan error, 1)
	wg.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { defer wg.Done(); settled <- pool.settleOwner(ctx, original.owner) }()
	// The fresh lane's actual marker proves the settlement snapshot reached the
	// live lane after sealing original. It cannot be a timer-only absence check.
	select {
	case key := <-markers:
		require.Equal(t, fresh, key)
	case err := <-settled:
		t.Fatalf("owner settlement refused before admitting every captured physical lane: %v", err)
	case <-ctx.Done():
		t.Fatal("settlement never admitted the useful fresh physical lane")
	}
	originalRelease.release()
	select {
	case <-closeEntered:
	case <-ctx.Done():
		t.Fatal("retiring original never reached its close callback")
	}
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 2}))
	select {
	case key := <-entered:
		require.Equal(t, sibling, key)
	case <-ctx.Done():
		t.Fatal("another owner lost useful work")
	}
	select {
	case err := <-settled:
		t.Fatalf("settlement escaped held original close: %v", err)
	default:
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	require.ErrorIs(t, pool.settleOwner(canceled, original.owner), context.Canceled)
	closeRelease.release()
	select {
	case err := <-settled:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("original completion never settled")
	}
	select {
	case <-down:
	case <-ctx.Done():
		t.Fatal("physical DOWN never completed")
	}
	require.NoError(t, pool.settleOwner(context.Background(), 999), "an owner with no current work has nothing to join")
	pool.Stop()
	require.ErrorIs(t, pool.settleOwner(context.Background(), original.owner), errWorkerPoolClosing, "a stopped role cannot prepare new policy work")
}
