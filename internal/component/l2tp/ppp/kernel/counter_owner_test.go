// Design: docs/research/l2tpv2-ze-integration.md -- counter reads retain the kernel session owner

package kernel

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// A blocked native read holds the owner's operation lock until its result is
// available. Close must then finish before that owner can admit another read.
func TestKernelCounterReadExcludesClose(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	cfg := sessionTestConfig()
	want := ppp.SessionCounters{RxBytes: 11, TxBytes: 13, RxPackets: 17, TxPackets: 19}
	cfg.ReadCounters = func(string) (ppp.SessionCounters, error) {
		close(entered)
		<-release
		return want, nil
	}
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := newSession(cfg, sessionTestOps(streams, &released))
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("owner has type %T, want *session", handle)
	}
	type result struct {
		counters ppp.SessionCounters
		err      error
	}
	readDone := make(chan struct{})
	closeDone := make(chan struct{})
	var readResult result
	var closeErr error
	releasedRead := false
	closeStarted := false
	t.Cleanup(func() {
		if !releasedRead {
			close(release)
		}
		select {
		case <-readDone:
		case <-time.After(3 * time.Second):
			t.Error("counter worker did not finish during cleanup")
		}
		if closeStarted {
			select {
			case <-closeDone:
				if closeErr != nil {
					t.Error(closeErr)
				}
			case <-time.After(3 * time.Second):
				t.Error("close worker did not finish during cleanup")
			}
			return
		}
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	// These workers each perform one bounded test lifecycle operation.
	go func() {
		counters, err := handle.Counters()
		readResult = result{counters: counters, err: err}
		close(readDone)
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("counter provider was not entered")
	}
	if owner.mu.TryLock() {
		owner.mu.Unlock()
		t.Fatal("counter provider ran without the owner's operation lock")
	}
	closeStarted = true
	go func() {
		closeErr = handle.Close()
		close(closeDone)
	}()
	select {
	case <-streams[31].closed:
		t.Fatal("Close released the control stream during a counter read")
	case <-streams[32].closed:
		t.Fatal("Close released the unit stream during a counter read")
	default:
	}
	close(release)
	releasedRead = true
	select {
	case <-readDone:
		if readResult.err != nil {
			t.Fatal(readResult.err)
		}
		if readResult.counters != want {
			t.Fatalf("counter result = %+v, want %+v", readResult.counters, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("counter read did not finish after its provider was released")
	}
	select {
	case <-closeDone:
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not finish after the counter read")
	}
}

// A closed original owner refuses locally, before a callback can resolve a
// replacement interface carrying the same reusable name.
func TestKernelCounterClosedOwnerNeverCallsProvider(t *testing.T) {
	calls := 0
	cfg := sessionTestConfig()
	cfg.ReadCounters = func(string) (ppp.SessionCounters, error) {
		calls++
		return ppp.SessionCounters{TxBytes: 97}, nil
	}
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := newSession(cfg, sessionTestOps(streams, &released))
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
	cfg.ReadCounters = func(string) (ppp.SessionCounters, error) {
		return ppp.SessionCounters{}, want
	}
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	_, handle, err := newSession(cfg, sessionTestOps(streams, &released))
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
