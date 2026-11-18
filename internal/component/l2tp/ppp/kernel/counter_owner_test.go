// Design: docs/research/l2tpv2-ze-integration.md -- counter reads retain the kernel session owner

package kernel

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// REPLACE only TestKernelCounterReadExcludesClose in current counter_owner_test.go.
// The original owner rejects every late reply; a read cannot prevent the same
// owner from waking and closing both borrowed streams. The first actual read
// preserves the original test's exact live-snapshot positive control.
func TestKernelCounterReadExcludesClose(t *testing.T) {
	cfg := sessionTestConfig()
	want := ppp.SessionCounters{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19}
	entered, resume := make(chan struct{}), make(chan struct{})
	var release sync.Once
	releaseRead := func() { release.Do(func() { close(resume) }) }
	var calls atomic.Int32
	sessionTestUnit(cfg).readCounters = func() (ppp.SessionCounters, error) {
		if calls.Add(1) == 1 {
			return want, nil
		}
		close(entered)
		<-resume
		return want, nil
	}
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := sessionTestNewSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("actual owner type %T", handle)
	}
	if got, err := handle.Counters(); err != nil || got != want {
		t.Fatalf("original live snapshot %+v, %v", got, err)
	}
	type answer struct {
		counters ppp.SessionCounters
		err      error
	}
	readDone := make(chan answer, 1)
	closeDone := make(chan error, 1)
	readJoined, closeJoined, closeStarted := false, false, false
	t.Cleanup(func() {
		releaseRead()
		if !readJoined {
			select {
			case <-readDone:
			case <-time.After(3 * time.Second):
				t.Error("original reader did not join")
			}
		}
		if closeStarted && !closeJoined {
			select {
			case err := <-closeDone:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(3 * time.Second):
				t.Error("original closer did not join")
			}
		} else if !closeStarted {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	go func() { counters, err := handle.Counters(); readDone <- answer{counters, err} }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("second original read did not enter provider")
	}
	if !owner.mu.TryLock() {
		t.Fatal("read-only provider prevents native cleanup")
	}
	owner.mu.Unlock()
	closeStarted = true
	go func() { closeDone <- handle.Close() }()
	select {
	case err := <-closeDone:
		closeJoined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close waited for the blocked counter reply")
	}
	for fd, stream := range streams {
		select {
		case <-stream.closed:
		default:
			t.Fatalf("original lane %d was not woken before provider release", fd)
		}
		if stream.closes != 1 {
			t.Fatalf("original lane %d closes=%d, want1", fd, stream.closes)
		}
	}
	select {
	case <-readDone:
		readJoined = true
		t.Fatal("controlled provider returned before release")
	default:
	}
	releaseRead()
	select {
	case got := <-readDone:
		readJoined = true
		if !errors.Is(got.err, io.ErrClosedPipe) || got.counters != (ppp.SessionCounters{}) {
			t.Fatalf("late original snapshot %+v, %v", got.counters, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("released original reader did not join")
	}
	if _, err := handle.Counters(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed owner read: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("closed owner invoked provider again: calls=%d", calls.Load())
	}
}

// A closed original owner refuses locally, before a callback can resolve a
// replacement interface carrying the same reusable name.
func TestKernelCounterClosedOwnerNeverCallsProvider(t *testing.T) {
	calls := 0
	cfg := sessionTestConfig()
	sessionTestUnit(cfg).readCounters = func() (ppp.SessionCounters, error) {
		calls++
		return ppp.SessionCounters{TxBytes: 97}, nil
	}
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := sessionTestNewSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Counters(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed counter read = %v, want io.ErrClosedPipe", err)
	}
	if calls != 0 {
		t.Fatalf("closed owner invoked the provider %d times", calls)
	}
}

// A provider failure remains identifiable instead of becoming idle counters.
func TestKernelCounterProviderErrorPreserved(t *testing.T) {
	want := errors.New("original counter provider failed")
	cfg := sessionTestConfig()
	sessionTestUnit(cfg).readCounters = func() (ppp.SessionCounters, error) {
		return ppp.SessionCounters{}, want
	}
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := sessionTestNewSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := handle.Counters(); !errors.Is(err, want) {
		t.Fatalf("counter error = %v, want provider failure %v", err, want)
	}
}
