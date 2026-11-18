// Design: docs/research/l2tpv2-ze-integration.md -- original unit read/close gate

package kernel

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func createdUnitTestOwner(t *testing.T) *CreatedUnit {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "unit-file-lease"))
	if err != nil {
		t.Fatal(err)
	}
	unit := &CreatedUnit{file: file, ready: true,
		identity: ppp.SessionIdentity{InterfaceName: "original", NativeIndex: 7},
		poll:     func(*os.File) error { return nil },
		readCounters: func() (ppp.SessionCounters, error) {
			return ppp.SessionCounters{RxBytes: 123}, nil
		},
	}
	t.Cleanup(func() {
		if err := unit.Close(); err != nil {
			t.Error(err)
		}
	})
	return unit
}

// MUTATION: keeping mu or the original unit-file lease across the reader keeps
// Close blocked; accepting a reply after Close exposes the sentinel snapshot.
func TestCreatedUnitCloseDoesNotWaitForCounterReply(t *testing.T) {
	unit := createdUnitTestOwner(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	unit.readCounters = func() (ppp.SessionCounters, error) {
		close(entered)
		<-release
		return ppp.SessionCounters{RxBytes: 123}, nil
	}
	finished := make(chan struct{})
	closeFinished := make(chan struct{})
	closeStarted := false
	var got ppp.SessionCounters
	var readErr error
	go func() {
		defer close(finished)
		got, readErr = unit.Counters()
	}()
	// Release and join the reader even if a deliberately broken Close blocks.
	defer func() {
		unblock()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("counter reader did not stop during cleanup")
		}
		if closeStarted {
			select {
			case <-closeFinished:
			case <-time.After(3 * time.Second):
				t.Error("Close worker did not stop during cleanup")
			}
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("counter query did not reach its controlled delay")
	}
	closed := make(chan error, 1)
	closeStarted = true
	go func() {
		defer close(closeFinished)
		closed <- unit.Close()
	}()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close waited for the native counter query")
	}
	// Join before inspecting the late result, without double-closing release.
	unblock()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("late query did not return")
	}
	if !errors.Is(readErr, os.ErrClosed) || got != (ppp.SessionCounters{}) {
		t.Fatalf("closed original returned late counters %+v, %v", got, readErr)
	}
}

func TestCreatedUnitDeadBeforeReadRefusesProvider(t *testing.T) {
	unit := createdUnitTestOwner(t)
	dead := errors.New("original unit file has POLLHUP")
	unit.poll = func(*os.File) error { return dead }
	called := false
	unit.readCounters = func() (ppp.SessionCounters, error) {
		called = true
		return ppp.SessionCounters{RxBytes: 123}, nil
	}
	got, err := unit.Counters()
	if !errors.Is(err, dead) || called || got != (ppp.SessionCounters{}) {
		t.Fatalf("dead original: counters %+v, error %v, provider called %t", got, err, called)
	}
}

func TestCreatedUnitDeadAfterReadDiscardsSnapshot(t *testing.T) {
	unit := createdUnitTestOwner(t)
	dead := errors.New("original unit died during query")
	polls := 0
	unit.poll = func(*os.File) error {
		polls++
		if polls == 2 {
			return dead
		}
		return nil
	}
	got, err := unit.Counters()
	if !errors.Is(err, dead) || polls != 2 || got != (ppp.SessionCounters{}) {
		t.Fatalf("late native death: counters %+v, error %v, polls %d", got, err, polls)
	}
}

func TestCreatedUnitReadFailurePreservesCause(t *testing.T) {
	unit := createdUnitTestOwner(t)
	refusal := errors.New("original socket history was lost")
	unit.readCounters = func() (ppp.SessionCounters, error) {
		return ppp.SessionCounters{RxBytes: 123}, refusal
	}
	got, err := unit.Counters()
	if !errors.Is(err, refusal) || got != (ppp.SessionCounters{}) {
		t.Fatalf("reader refusal became counters %+v, error %v", got, err)
	}
}

func TestCreatedUnitUnconfirmedReceiptCannotRead(t *testing.T) {
	unit := createdUnitTestOwner(t)
	unit.ready = false
	called := false
	unit.readCounters = func() (ppp.SessionCounters, error) {
		called = true
		return ppp.SessionCounters{RxBytes: 123}, nil
	}
	got, err := unit.Counters()
	if err == nil || called || got != (ppp.SessionCounters{}) {
		t.Fatalf("unconfirmed creation read %+v, error %v, provider called %t", got, err, called)
	}
}

func TestCreatedUnitUnsupportedBackendRefusedBeforeAllocation(t *testing.T) {
	creator, err := NewUnitCreator(&sessionTestBackend{})
	if creator != nil || err == nil {
		t.Fatalf("unsupported backend produced creator %v, error %v", creator, err)
	}
}
