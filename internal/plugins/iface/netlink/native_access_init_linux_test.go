// Design: docs/features/interfaces.md -- original native transport initialization
//go:build linux

package ifacenetlink

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/iface"
)

func waitNativeAccessInitialization(t *testing.T, done <-chan struct{}, stage string) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatalf("native initialization did not reach %s", stage)
	}
}

// Guarded creator rollback and first worker admission MUST share one original
// transport. Hold the actual rollback factory and observe either its worker's
// failed initialization TryLock or an actual second opener before release.
// MUTATION: run's independent opener reaches the duplicate-open assertion;
// a missing contention observer alone is a setup failure, never causal proof.
func TestNativeAccessFirstWorkerSharesGuardedRollbackTransport(t *testing.T) {
	b := &netlinkBackend{}
	resources := &b.nativeVLANs
	target := &netlink.Dummy{Name: "rollback-first", Index: 71}
	cause := errors.New("original creator setup failed")
	entered, contended, secondEntered, resume := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	rollbackDone, workerDone := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(resume) })
	announceOpen := sync.OnceFunc(func() { close(entered) })
	announceSecondOpen := sync.OnceFunc(func() { close(secondEntered) })
	announceContention := sync.OnceFunc(func() { close(contended) })
	var opens, removes, closes atomic.Int32
	var rollbackErr, workerErr error
	workerStarted := false
	original := &nativeVLANOps{
		list: func() ([]netlink.Link, error) { return []netlink.Link{target}, nil },
		remove: func(link netlink.Link) error {
			if link != target {
				return errors.New("rollback substituted its original target")
			}
			removes.Add(1)
			return nil
		},
		timeout: func(time.Duration) error { return nil },
		close:   func() error { closes.Add(1); return nil },
	}
	resources.openOpsForTest = func() (*nativeVLANOps, error) {
		attempt := opens.Add(1)
		announceOpen()
		if attempt > 1 {
			announceSecondOpen() // Actual entry, before either factory may wait.
		}
		<-resume
		if attempt == 1 {
			return original, nil
		}
		other := *original
		return &other, nil
	}
	resources.observeOpsContentionForTest = announceContention
	t.Cleanup(func() {
		release()
		waitNativeAccessInitialization(t, rollbackDone, "rollback cleanup join")
		if workerStarted {
			waitNativeAccessInitialization(t, workerDone, "worker cleanup join")
		}
		if err := b.Close(); err != nil {
			t.Errorf("close original rollback transport: %v", err)
		}
	})
	go func() {
		qosMapMu.Lock()
		rollbackErr = b.rollbackNativeCreate(target, target.Name, "dummy", "set up", cause)
		qosMapMu.Unlock()
		close(rollbackDone)
	}()
	waitNativeAccessInitialization(t, entered, "synchronous guarded rollback factory")
	workerStarted = true
	go func() {
		_, workerErr = b.CaptureNativeVLAN(t.Context(), iface.Binding{OsName: target.Name, Ifindex: target.Index})
		close(workerDone)
	}()
	gate := time.NewTimer(5 * time.Second)
	defer gate.Stop()
	select {
	case <-contended:
	case <-secondEntered:
		got := opens.Load()
		if got <= 1 {
			t.Fatalf("second factory entered without a counted opener: %d", got)
		}
		t.Fatalf("first worker opened another transport before rollback release: %d openers", got)
	case <-gate.C:
		t.Fatal("setup did not reach actual initialization contention or a second factory entry")
	}
	if resources.workerMu.TryLock() {
		resources.workerMu.Unlock()
		t.Fatal("contention did not come from the first worker admission")
	}
	if qosMapMu.TryLock() {
		qosMapMu.Unlock()
		t.Fatal("rollback released its creator fence before contention")
	}
	if got := opens.Load(); got != 1 {
		t.Fatalf("openers before first release: %d", got)
	}
	if got := removes.Load(); got != 0 {
		t.Fatalf("rollback advanced before first release: %d", got)
	}
	release()
	waitNativeAccessInitialization(t, rollbackDone, "original rollback return")
	waitNativeAccessInitialization(t, workerDone, "first worker return")
	if !errors.Is(rollbackErr, cause) {
		t.Fatalf("original creation failure lost: %v", rollbackErr)
	}
	// Depending on the next scheduler turn, the first worker either observes the
	// still-held creator fence or reaches the genuine absent creator-record guard.
	if !errors.Is(workerErr, iface.ErrNativeAccessPending) {
		if !errors.Is(workerErr, iface.ErrNativeAccessUnsupported) {
			t.Fatalf("unowned rollback target acquired a native lease: %v", workerErr)
		}
	}
	if got := opens.Load(); got != 1 {
		t.Fatalf("original transport reopened: %d", got)
	}
	if resources.ops != original {
		t.Fatal("first worker replaced the rollback's original transport")
	}
	if got := removes.Load(); got != 1 {
		t.Fatalf("original guarded rollback count: %d", got)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if got := closes.Load(); got != 1 {
		t.Fatalf("original transport close count: %d", got)
	}
}

// A failed original socket open MUST allow a later legitimate caller to retry.
// Fail the real rollback initialization once, retry under the same creator
// fence, then close through first worker admission without reopening success.
// MUTATION: replacing initialization with sync.Once poisons the successful retry.
func TestNativeAccessFailedInitializationRetriesOriginalTransport(t *testing.T) {
	b := &netlinkBackend{}
	resources := &b.nativeVLANs
	target := &netlink.Dummy{Name: "rollback-retry", Index: 72}
	openErr := errors.New("original socket open failed")
	var opens, removes, closes int
	original := &nativeVLANOps{
		list:    func() ([]netlink.Link, error) { return []netlink.Link{target}, nil },
		remove:  func(netlink.Link) error { removes++; return nil },
		timeout: func(time.Duration) error { return nil },
		close:   func() error { closes++; return nil },
	}
	resources.openOpsForTest = func() (*nativeVLANOps, error) {
		opens++
		if opens == 1 {
			return nil, openErr
		}
		return original, nil
	}
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Errorf("close retried original transport: %v", err)
		}
	})
	rollback := func() error {
		qosMapMu.Lock()
		defer qosMapMu.Unlock()
		return resources.nativeVLANRollbackLocked(target)
	}
	first := rollback()
	if !errors.Is(first, openErr) {
		t.Fatalf("socket failure lost: %v", first)
	}
	if resources.ops != nil {
		t.Fatal("failed socket factory published a transport")
	}
	if removes != 0 {
		t.Fatal("failed socket factory reached native rollback")
	}
	if err := rollback(); err != nil {
		t.Fatalf("successful original rollback: %v", err)
	}
	if opens != 2 {
		t.Fatalf("failed-open retry count: %d", opens)
	}
	if removes != 1 {
		t.Fatalf("successful guarded rollback count: %d", removes)
	}
	if resources.ops != original {
		t.Fatal("retry did not retain its successful original transport")
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if opens != 2 {
		t.Fatalf("first closing worker reopened success: %d", opens)
	}
	if closes != 1 {
		t.Fatalf("retried original transport close count: %d", closes)
	}
}

// Close MUST join the original worker, not merely observe admission stopped.
// Hold actual native close past caller cancellation, then hold its abandoned
// result discard after the real loop sets stopped but before it closes done.
// MUTATION: restoring stopped=>nil makes the public retry succeed at the
// explicit premature-Close assertion, while the original done is still open.
func TestNativeAccessCloseWaitsForStoppedOriginalWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := &netlinkBackend{}
		resources := &b.nativeVLANs
		nativeEntered, nativeResume := make(chan struct{}), make(chan struct{})
		discardEntered, discardResume := make(chan struct{}), make(chan struct{})
		releaseNative := sync.OnceFunc(func() { close(nativeResume) })
		releaseDiscard := sync.OnceFunc(func() { close(discardResume) })
		var physicalCloses, discarded, reopened atomic.Int32
		originalOps := &nativeVLANOps{
			close: func() error {
				physicalCloses.Add(1)
				close(nativeEntered)
				<-nativeResume
				return nil
			},
		}
		resources.ops = originalOps // Existing transport fixture; no resource or worker state is fabricated.
		resources.openOpsForTest = func() (*nativeVLANOps, error) {
			reopened.Add(1)
			return nil, errors.New("stopped original transport must not reopen")
		}
		t.Cleanup(func() {
			releaseNative()
			releaseDiscard()
			synctest.Wait()
			if err := b.Close(); err != nil {
				t.Errorf("final original backend close: %v", err)
			}
		})
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		answer := make(chan error, 1)
		originalResult := new(int)
		go func() {
			_, err := resources.run(ctx, true, func(turn context.Context) (any, error) {
				return originalResult, resources.closeDirect(turn)
			}, func(value any) error {
				if value != originalResult {
					t.Error("late close result substituted another reference")
				}
				discarded.Add(1)
				close(discardEntered)
				<-discardResume
				return nil
			})
			answer <- err
		}()
		<-nativeEntered
		cancel()
		firstErr := <-answer
		if !errors.Is(firstErr, context.Canceled) {
			t.Fatalf("original held-close caller lost cancellation: %v", firstErr)
		}
		if !errors.Is(firstErr, iface.ErrNativeAccessPending) {
			t.Fatalf("original held-close caller lost pending custody: %v", firstErr)
		}
		resources.workerMu.Lock()
		originalWorker := resources.worker
		resources.workerMu.Unlock()
		if originalWorker == nil {
			t.Fatal("actual close did not retain its admitted worker")
		}
		releaseNative()
		<-discardEntered
		resources.workerMu.Lock()
		stopped, current := originalWorker.stopped, originalWorker.current
		resources.workerMu.Unlock()
		if !stopped {
			t.Fatal("actual successful close did not stop admission before discard")
		}
		if current != nil {
			t.Fatal("actual successful close did not retire its admission before discard")
		}
		select {
		case <-originalWorker.done:
			t.Fatal("held actual result discard already joined the worker")
		default:
		}
		// Both calls reach the same production body. Virtual time advances only
		// because the held original discard cannot finish; no wall-clock sleep.
		retryErr := b.Close()
		if !errors.Is(retryErr, iface.ErrNativeAccessPending) {
			t.Fatalf("public Close succeeded before original worker join: %v", retryErr)
		}
		if !errors.Is(retryErr, context.DeadlineExceeded) {
			t.Fatalf("public Close lost its finite join deadline: %v", retryErr)
		}
		joinErr := resources.join(t.Context())
		if !errors.Is(joinErr, iface.ErrNativeAccessPending) {
			t.Fatalf("held original worker falsely joined: %v", joinErr)
		}
		if !errors.Is(joinErr, context.DeadlineExceeded) {
			t.Fatalf("held original worker lost its finite join deadline: %v", joinErr)
		}
		if resources.ops != originalOps {
			t.Fatal("pending Close changed the original transport")
		}
		if physicalCloses.Load() != 1 {
			t.Fatal("pending Close closed the original transport twice")
		}
		if reopened.Load() != 0 {
			t.Fatal("pending Close reopened a stopped transport")
		}
		resources.workerMu.Lock()
		same := resources.worker == originalWorker
		resources.workerMu.Unlock()
		if !same {
			t.Fatal("pending Close replaced its stopped original worker")
		}
		releaseDiscard()
		<-originalWorker.done
		if err := b.Close(); err != nil {
			t.Fatalf("physically joined original backend did not close: %v", err)
		}
		if err := b.Close(); err != nil {
			t.Fatalf("physically joined Close was not idempotent: %v", err)
		}
		if discarded.Load() != 1 {
			t.Fatalf("original late result discard count: %d", discarded.Load())
		}
		if physicalCloses.Load() != 1 {
			t.Fatalf("original physical close count: %d", physicalCloses.Load())
		}
		if reopened.Load() != 0 {
			t.Fatalf("original transport reopened: %d", reopened.Load())
		}
		if resources.ops != originalOps {
			t.Fatal("joined Close replaced the original transport")
		}
	})
}
