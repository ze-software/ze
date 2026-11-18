// Design: docs/architecture/plugin/rib-storage-design.md -- joined worker forward-batch completion.
// Related: worker_retirement_capture_test.go -- original captured retirement lifecycle.
package rs

import (
	"context"
	"fmt"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type workerFlushReceipt struct {
	ids     []uint64
	targets []rpc.ForwardTarget
}

// A physical retirement MUST flush its real accumulated forward batch before
// its close callback and completion. Hold an admitted overflow handler with
// another original item still owed, then hold the actual final SDK write. A
// different owner at the same address must complete useful work before and
// after the original leaves. The real flushWorkerBatch/flushBatch producers
// carry the complete captured target through the SDK completion receipt.
// MUTATION: omit flush from the shared worker close finalization.
func TestWorkerPoolRetirementFlushesActualBatch(t *testing.T) {
	original := rsFixtureKey("192.0.2.247", 171, 181)
	sibling := rsFixtureKey("192.0.2.247", 172, 182)
	target := rsFixtureTarget("192.0.2.254", 254, 454)
	flushed := make(chan workerFlushReceipt, 8)
	firstEntered, secondHandled, overflowEntered := make(chan struct{}), make(chan struct{}), make(chan struct{})
	finalEntered := make(chan struct{})
	firstRelease := workerQuestionRelease{channel: make(chan struct{})}
	overflowRelease := workerQuestionRelease{channel: make(chan struct{})}
	finalRelease := workerQuestionRelease{channel: make(chan struct{})}
	server := newWorkerFlushQuestion(t, func(ctx context.Context, input rpc.ForwardCachedInput) error {
		if slices.Equal(input.IDs, []uint64{1, 2, 3, 4}) {
			close(finalEntered)
			select {
			case <-finalRelease.channel:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		flushed <- workerFlushReceipt{ids: slices.Clone(input.IDs), targets: slices.Clone(input.Targets)}
		return nil
	})
	closed := make(chan []uint64, 1)
	retired := make(chan struct{})
	retirementStarted := false
	var originalDrains atomic.Uint32
	cfg := testPoolConfig()
	cfg.chanSize = 1
	cfg.idleTimeout = time.Hour
	cfg.onDrained = func(key workerKey) {
		if key == original {
			originalDrains.Add(1)
		}
		server.flushWorkerBatch(key)
	}
	cfg.onClose = func(key workerKey) {
		server.dropParkedValidations(key)
		if key == original {
			closed <- slices.Clone(server.workerBatch(key).ids)
		}
	}
	pool := newWorkerPool(func(key workerKey, item workItem) {
		batch := server.workerBatch(key)
		batch.ids = append(batch.ids, item.msgID)
		batch.targets = []rpc.ForwardTarget{target}
		if key == sibling {
			return
		}
		switch item.msgID {
		case 1:
			close(firstEntered)
			<-firstRelease.channel
		case 2:
			close(secondHandled)
		case 3:
			close(overflowEntered)
			<-overflowRelease.channel
		}
	}, cfg)
	t.Cleanup(func() {
		firstRelease.release()
		overflowRelease.release()
		finalRelease.release()
		pool.Stop()
		if retirementStarted {
			joinWorkerRetirementQuestion(t, retired, "final forward-batch retirement")
		}
	})
	require.True(t, pool.Dispatch(original, workItem{msgID: 1}))
	replayQuestionWait(t, firstEntered, "original forward handler")
	for _, id := range []uint64{2, 3, 4} {
		require.True(t, pool.Dispatch(original, workItem{msgID: id}))
	}
	pool.mu.Lock()
	captured := pool.workers[original]
	pool.mu.Unlock()
	require.NotNil(t, captured)
	retirementStarted = true
	go func() { pool.PeerDown(original); close(retired) }()
	replayQuestionWait(t, captured.closeCh, "captured physical retirement seal")
	firstRelease.release()
	replayQuestionWait(t, secondHandled, "original channel final item")
	replayQuestionWait(t, overflowEntered, "admitted original overflow handler")
	// Original3 is still actually executing and admitted4 remains owed. A
	// useful different owner at the same address must complete independently.
	require.Zero(t, originalDrains.Load())
	select {
	case <-retired:
		t.Fatal("retirement acknowledged its held admitted overflow handler")
	default:
	}
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 41}))
	select {
	case receipt := <-flushed:
		require.Equal(t, []uint64{41}, receipt.ids)
		require.Equal(t, []rpc.ForwardTarget{target}, receipt.targets)
	case <-time.After(5 * time.Second):
		t.Fatal("held original retirement blocked useful same-address sibling")
	}
	overflowRelease.release()
	replayQuestionWait(t, finalEntered, "actual accumulated final SDK write")
	select {
	case <-retired:
		t.Fatal("retirement acknowledged a held actual final SDK write")
	default:
	}
	select {
	case pending := <-closed:
		t.Fatalf("close callback preceded final SDK completion: %v", pending)
	default:
	}
	finalRelease.release()
	replayQuestionWait(t, retired, "joined original final batch")
	select {
	case receipt := <-flushed:
		require.Equal(t, []uint64{1, 2, 3, 4}, receipt.ids)
		require.Equal(t, []rpc.ForwardTarget{target}, receipt.targets)
	case <-time.After(5 * time.Second):
		t.Fatal("retirement acknowledged without its actual admitted forward batch")
	}
	select {
	case pending := <-closed:
		require.Empty(t, pending, "onClose cannot stand in for flushWorkerBatch")
	case <-time.After(5 * time.Second):
		t.Fatal("original close callback did not finish")
	}
	require.Equal(t, uint32(1), originalDrains.Load(), "completed batch earns no duplicate callback")
	require.Equal(t, 1, pool.WorkerCount())
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 42}))
	select {
	case receipt := <-flushed:
		require.Equal(t, []uint64{42}, receipt.ids)
		require.Equal(t, []rpc.ForwardTarget{target}, receipt.targets)
	case <-time.After(5 * time.Second):
		t.Fatal("original retirement removed its independently useful sibling")
	}
}

// Once the actual partial batch has completed its normal drain, retirement
// must not run that callback again. This is the opposite boundary of the held
// overflow question: no outstanding batch remains to settle at close.
// MUTATION: unconditionally call onDrained in the shared close finalizer.
func TestWorkerPoolRetirementDoesNotRepeatCompletedDrain(t *testing.T) {
	key, err := newWorkerKey("192.0.2.248", 191, 201)
	require.NoError(t, err)
	flushed := make(chan []uint64, 2)
	server := newWorkerFlushQuestion(t, func(_ context.Context, input rpc.ForwardCachedInput) error {
		flushed <- slices.Clone(input.IDs)
		return nil
	})
	var drains atomic.Uint32
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Hour
	cfg.onDrained = func(key workerKey) {
		drains.Add(1)
		server.flushWorkerBatch(key)
	}
	cfg.onClose = server.dropParkedValidations
	pool := newWorkerPool(func(key workerKey, item workItem) {
		batch := server.workerBatch(key)
		batch.ids = append(batch.ids, item.msgID)
		batch.targets = []rpc.ForwardTarget{rsFixtureTarget("192.0.2.254", 254, 454)}
	}, cfg)
	t.Cleanup(pool.Stop)
	require.True(t, pool.Dispatch(key, workItem{msgID: 51}))
	select {
	case ids := <-flushed:
		require.Equal(t, []uint64{51}, ids)
	case <-time.After(5 * time.Second):
		t.Fatal("the ordinary drain did not emit its useful actual batch")
	}
	pool.PeerDown(key)
	require.Equal(t, uint32(1), drains.Load())
	require.Empty(t, server.workerBatch(key).ids)
	require.Zero(t, pool.WorkerCount())
	select {
	case ids := <-flushed:
		t.Fatalf("completed batch was emitted again: %v", ids)
	default:
	}
}

// newWorkerFlushQuestion answers the actual SDK cached-write request only
// after its callback returns, preserving the complete original target tuple.
func newWorkerFlushQuestion(t *testing.T, write func(context.Context, rpc.ForwardCachedInput) error) *routeServer {
	t.Helper()
	bridge := rpc.NewDirectBridge()
	consumer, engine := net.Pipe()
	p := sdk.NewWithConn("worker-final-write-question", rpc.NewBridgedConn(consumer, bridge))
	bridge.SetForwardCached(func(ctx context.Context, input rpc.ForwardCachedInput) (*rpc.CachedWriteOutput, error) {
		if !slices.Equal(input.Targets, []rpc.ForwardTarget{rsFixtureTarget("192.0.2.254", 254, 454)}) {
			return nil, fmt.Errorf("final write changed its captured target: %+v", input.Targets)
		}
		if err := write(ctx, input); err != nil {
			return nil, err
		}
		return &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}, nil
	})
	bridge.SetReady()
	server := &routeServer{plugin: p}
	t.Cleanup(func() { require.NoError(t, p.Close()); require.NoError(t, engine.Close()) })
	return server
}
