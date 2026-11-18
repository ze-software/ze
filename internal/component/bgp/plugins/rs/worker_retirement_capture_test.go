// Design: docs/guide/route-reflection.md -- original worker retirement completion.
package rs

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Terminal removal captures and seals both original physical lanes before it
// waits. Stop then enters while both handlers are held: it must retain those
// exact registrations until their real work has completed. The independently
// useful sibling is processed before global Stop deliberately ends every lane.
// MUTATION: endOwner snapshots keys and relooks them up later, or Stop removes
// captured registrations before its handlers and close callbacks finish.
func TestWorkerPoolOwnerRetirementKeepsCaptureAcrossStop(t *testing.T) {
	original, err := newWorkerKey("192.0.2.244", 111, 121)
	require.NoError(t, err)
	successor, err := newWorkerKey("192.0.2.244", 111, 122)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.244", 112, 123)
	require.NoError(t, err)
	releases := map[workerKey]*workerQuestionRelease{
		original:  {channel: make(chan struct{})},
		successor: {channel: make(chan struct{})},
		sibling:   {channel: make(chan struct{})},
	}
	entered := make(chan workerKey, 3)
	processed := make(chan workerKey, 3)
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Hour
	pool := newWorkerPool(func(key workerKey, _ workItem) {
		entered <- key
		<-releases[key].channel
		processed <- key
	}, cfg)
	ownerFinished := make(chan struct{})
	stopDone := make(chan struct{})
	ownerStarted, stopStarted := false, false
	t.Cleanup(func() {
		for _, release := range releases {
			release.release()
		}
		pool.Stop()
		if ownerStarted {
			joinWorkerRetirementQuestion(t, ownerFinished, "terminal removal")
		}
		if stopStarted {
			joinWorkerRetirementQuestion(t, stopDone, "concurrent Stop")
		}
	})
	for _, key := range []workerKey{original, successor, sibling} {
		require.True(t, pool.Dispatch(key, workItem{msgID: 1}))
	}
	for range 3 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("original worker did not admit its useful input")
		}
	}
	releases[sibling].release()
	select {
	case key := <-processed:
		require.Equal(t, sibling, key, "another original owner remains useful")
	case <-time.After(5 * time.Second):
		t.Fatal("same-address sibling could not finish useful work")
	}
	pool.mu.Lock()
	first := pool.workers[original]
	second := pool.workers[successor]
	pool.mu.Unlock()
	require.NotNil(t, first)
	require.NotNil(t, second)
	ownerDone := make(chan error, 1)
	ownerStarted = true
	go func() {
		defer close(ownerFinished)
		ownerDone <- pool.endOwner(original.owner)
	}()
	// Both seals are the actual capture barrier. No elapsed-time absence
	// decides whether Stop interleaves before a later key lookup.
	for _, captured := range []*worker{first, second} {
		select {
		case <-captured.closeCh:
		case <-time.After(5 * time.Second):
			t.Fatal("terminal removal waited before sealing every captured lane")
		}
	}
	stopStarted = true
	go func() {
		pool.Stop()
		close(stopDone)
	}()
	require.Eventually(t, func() bool {
		pool.mu.Lock()
		defer pool.mu.Unlock()
		return pool.stopped
	}, time.Second, time.Millisecond, "Stop must have entered its actual admission fence")
	pool.mu.Lock()
	firstStillRegistered := pool.workers[original] == first
	secondStillRegistered := pool.workers[successor] == second
	pool.mu.Unlock()
	require.True(t, firstStillRegistered, "Stop cannot hide an original handler still being joined")
	require.True(t, secondStillRegistered, "terminal removal must retain its second captured worker")
	select {
	case err := <-ownerDone:
		t.Fatalf("terminal removal acknowledged held original work: %v", err)
	default:
	}
	releases[original].release()
	releases[successor].release()
	for range 2 {
		select {
		case <-processed:
		case <-time.After(5 * time.Second):
			t.Fatal("captured work was lost rather than joined")
		}
	}
	select {
	case err := <-ownerDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal removal did not join its captured workers")
	}
	select {
	case <-stopDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not join the same captured workers")
	}
	require.Zero(t, pool.WorkerCount())
}

// Hold the real idle callback, seal its physical worker, then release idle into
// a held close callback. No identical-key replacement is admitted until close
// finishes. A later repeated join of the captured old worker must not delete
// the useful replacement's registration or pressure state.
// MUTATION: skip the close seal in idle exit, or clean a different registration
// after joining an already-completed old worker.
func TestWorkerPoolIdleRetirementKeepsOriginalLaneUntilClose(t *testing.T) {
	original, err := newWorkerKey("192.0.2.245", 131, 141)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.245", 132, 142)
	require.NoError(t, err)
	idleEntered := make(chan struct{})
	closeEntered := make(chan struct{})
	replacementEntered := make(chan struct{})
	idleRelease := workerQuestionRelease{channel: make(chan struct{})}
	closeRelease := workerQuestionRelease{channel: make(chan struct{})}
	replacementRelease := workerQuestionRelease{channel: make(chan struct{})}
	var idleOnce atomic.Bool
	var closeOnce sync.Once
	processed := make(chan workerKey, 4)
	cfg := testPoolConfig()
	cfg.idleTimeout = time.Millisecond
	cfg.onIdle = func(key workerKey) {
		if key != original {
			return
		}
		if !idleOnce.CompareAndSwap(false, true) {
			return
		}
		close(idleEntered)
		<-idleRelease.channel
	}
	cfg.onClose = func(key workerKey) {
		if key != original {
			return
		}
		closeOnce.Do(func() {
			close(closeEntered)
			<-closeRelease.channel
		})
	}
	physicalDone := make(chan struct{})
	physicalStarted := false
	pool := newWorkerPool(func(key workerKey, item workItem) {
		if item.msgID == 2 {
			close(replacementEntered)
			<-replacementRelease.channel
		}
		processed <- key
	}, cfg)
	t.Cleanup(func() {
		idleRelease.release()
		closeRelease.release()
		replacementRelease.release()
		pool.Stop()
		if physicalStarted {
			joinWorkerRetirementQuestion(t, physicalDone, "physical DOWN")
		}
	})
	require.True(t, pool.Dispatch(original, workItem{msgID: 1}))
	select {
	case key := <-processed:
		require.Equal(t, original, key)
	case <-time.After(5 * time.Second):
		t.Fatal("original useful work did not finish")
	}
	select {
	case <-idleEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("actual original idle callback did not enter")
	}
	pool.mu.Lock()
	captured := pool.workers[original]
	pool.mu.Unlock()
	require.NotNil(t, captured)
	// onIdle is still held, so the exact mapped incarnation cannot expire.
	pool.mu.Lock()
	retirement := pool.captureRetirementLocked(original, captured)
	pool.mu.Unlock()
	physicalStarted = true
	go func() {
		pool.PeerDown(original)
		close(physicalDone)
	}()
	require.False(t, pool.Dispatch(original, workItem{msgID: 9}), "a sealed physical lane cannot admit more work")
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 3}))
	select {
	case key := <-processed:
		require.Equal(t, sibling, key, "unrelated same-address work remains useful")
	case <-time.After(5 * time.Second):
		t.Fatal("held idle retirement blocked the useful sibling")
	}
	idleRelease.release()
	select {
	case <-closeEntered:
	case <-captured.done:
		t.Fatal("idle exit skipped the captured worker's close callback")
	case <-time.After(5 * time.Second):
		t.Fatal("retired idle worker never reached its real close callback")
	}
	require.False(t, pool.Dispatch(original, workItem{msgID: 9}), "close callback must finish before identical-key replacement")
	pool.mu.Lock()
	stillOriginal := pool.workers[original] == captured
	pool.mu.Unlock()
	require.True(t, stillOriginal)
	closeRelease.release()
	select {
	case <-physicalDone:
	case <-time.After(5 * time.Second):
		t.Fatal("physical DOWN did not join its actual close callback")
	}
	require.True(t, pool.Dispatch(original, workItem{msgID: 2}))
	select {
	case <-replacementEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("useful replacement was never admitted after completed retirement")
	}
	pool.inBackpressure.Store(original, true)
	pool.backpressure.Store(original, true)
	pool.bpLastLog.Store(original, time.Now())
	pool.finishRetirement(retirement)
	pool.mu.Lock()
	replacementKept := pool.workers[original] != nil
	replacementDifferent := pool.workers[original] != captured
	pool.mu.Unlock()
	require.True(t, replacementKept, "an old captured join cannot remove the replacement")
	require.True(t, replacementDifferent)
	_, activePressure := pool.inBackpressure.Load(original)
	_, reportedPressure := pool.backpressure.Load(original)
	_, reminderKept := pool.bpLastLog.Load(original)
	require.True(t, activePressure)
	require.True(t, reportedPressure)
	require.True(t, reminderKept)
	replacementRelease.release()
	select {
	case key := <-processed:
		require.Equal(t, original, key, "replacement still finishes useful work")
	case <-time.After(5 * time.Second):
		t.Fatal("replacement was lost after a repeated original retirement join")
	}
}

// A physical retirement joins the actual in-flight overflow producer before
// its worker closes. Hold the first handler and then the admitted overflow
// handler while a sibling still completes useful work; all admitted work must
// drain FIFO, and the close callback follows the final original handler.
// MUTATION: retirement closes the input before its overflow producer joins,
// drops admitted overflow, or acknowledges close before its last handler ends.
func TestWorkerPoolRetirementClosesAfterOverflowProducer(t *testing.T) {
	original, err := newWorkerKey("192.0.2.246", 151, 161)
	require.NoError(t, err)
	sibling, err := newWorkerKey("192.0.2.246", 152, 162)
	require.NoError(t, err)
	handlerEntered := make(chan struct{})
	overflowEntered := make(chan struct{})
	siblingUseful := make(chan struct{})
	handlerRelease := workerQuestionRelease{channel: make(chan struct{})}
	overflowRelease := workerQuestionRelease{channel: make(chan struct{})}
	var overflowFinished atomic.Bool
	handled := make(chan uint64, 3)
	closedAfterOverflow := make(chan bool, 1)
	physicalDone := make(chan struct{})
	physicalStarted := false
	cfg := testPoolConfig()
	cfg.chanSize = 1
	cfg.idleTimeout = time.Hour
	cfg.onClose = func(key workerKey) {
		if key == original {
			closedAfterOverflow <- overflowFinished.Load()
		}
	}
	pool := newWorkerPool(func(key workerKey, item workItem) {
		if key == sibling {
			close(siblingUseful)
			return
		}
		if item.msgID == 1 {
			close(handlerEntered)
			<-handlerRelease.channel
		}
		if item.msgID == 3 {
			close(overflowEntered)
			<-overflowRelease.channel
			overflowFinished.Store(true)
		}
		handled <- item.msgID
	}, cfg)
	t.Cleanup(func() {
		handlerRelease.release()
		overflowRelease.release()
		pool.Stop()
		if physicalStarted {
			joinWorkerRetirementQuestion(t, physicalDone, "overflow physical DOWN")
		}
	})
	require.True(t, pool.Dispatch(original, workItem{msgID: 1}))
	select {
	case <-handlerEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("original first handler did not enter")
	}
	require.True(t, pool.Dispatch(original, workItem{msgID: 2}))
	require.True(t, pool.Dispatch(original, workItem{msgID: 3}))
	pool.mu.Lock()
	lane := pool.workers[original]
	pool.mu.Unlock()
	require.NotNil(t, lane)
	// Item2 fills the channel behind held item1; the real overflow producer
	// has removed item3 from overflow but cannot yet send it to that channel.
	require.Eventually(t, func() bool {
		lane.overflowMu.Lock()
		defer lane.overflowMu.Unlock()
		return lane.draining && len(lane.overflow) == 0 && len(lane.ch) == 1
	}, 5*time.Second, time.Millisecond, "the actual admitted overflow producer enters its blocked send")
	physicalStarted = true
	go func() {
		pool.PeerDown(original)
		close(physicalDone)
	}()
	select {
	case <-lane.closeCh:
	case <-time.After(5 * time.Second):
		t.Fatal("physical retirement did not seal the original worker")
	}
	select {
	case <-physicalDone:
		t.Fatal("retirement acknowledged while its original overflow producer remained blocked")
	default:
	}
	select {
	case <-closedAfterOverflow:
		t.Fatal("close callback preceded the original overflow producer")
	default:
	}
	require.True(t, pool.Dispatch(sibling, workItem{msgID: 4}))
	select {
	case <-siblingUseful:
	case <-time.After(5 * time.Second):
		t.Fatal("held overflow producer blocked another original owner")
	}
	handlerRelease.release()
	for _, expected := range []uint64{1, 2} {
		select {
		case actual := <-handled:
			require.Equal(t, expected, actual)
		case <-time.After(5 * time.Second):
			t.Fatal("retirement lost originally queued work")
		}
	}
	select {
	case <-overflowEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("retirement lost its actually admitted overflow item")
	}
	select {
	case <-physicalDone:
		t.Fatal("retirement acknowledged before its final admitted handler completed")
	default:
	}
	select {
	case <-closedAfterOverflow:
		t.Fatal("close callback preceded its held final admitted handler")
	default:
	}
	overflowRelease.release()
	select {
	case actual := <-handled:
		require.Equal(t, uint64(3), actual, "the actual overflow item drains after both queued items")
	case <-time.After(5 * time.Second):
		t.Fatal("overflow producer did not account for its original admitted item")
	}
	select {
	case <-physicalDone:
	case <-time.After(5 * time.Second):
		t.Fatal("retirement did not join its overflow producer and worker")
	}
	select {
	case afterOverflow := <-closedAfterOverflow:
		require.True(t, afterOverflow, "close callback cannot precede the original overflow completion")
	case <-time.After(5 * time.Second):
		t.Fatal("retirement skipped its original close callback")
	}
	require.Empty(t, handled, "every admitted original item was handled exactly once")
}

// Each question MUST release its held callbacks and join the lifecycle callers
// in cleanup, including when a failing assertion aborts the test body.
func joinWorkerRetirementQuestion(t *testing.T, done <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Errorf("%s did not join during question cleanup", operation)
	}
}
