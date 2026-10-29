// Design: docs/architecture/zefs-format.md -- daemon-owned runtime state.
package statestore

import (
	"io/fs"
	"sync"
	"testing"
	"time"
)

// heldStore stands for the daemon's store while another writer holds its write
// lock across slow syncs, as the boot record does after the hub serves.
// AcquireLock reports that it was entered, then waits until the test lets it
// through. ReadKey answers at once, as a single-key read of the tree does. Every
// other Storage method is left to the nil embedded interface: the test reaches
// none of them, and a call would panic and name itself.
type heldStore struct {
	Storage
	entered chan struct{}
	release chan struct{}
}

func (s *heldStore) ReadKey(key string) ([]byte, error) { return []byte(key), nil }

func (s *heldStore) AcquireLock(string) (WriteGuard, error) {
	s.entered <- struct{}{}
	<-s.release
	return &heldGuard{}, nil
}

// heldGuard is the counter's guard once the lock is granted. Safe for use by
// the one Increment that acquired it.
type heldGuard struct {
	WriteGuard
	counter []byte
}

func (g *heldGuard) ReadFile(string) ([]byte, error) { return []byte{0, 0, 0, 1}, nil }

func (g *heldGuard) WriteFile(_ string, data []byte, _ fs.FileMode) error {
	g.counter = data
	return nil
}

func (g *heldGuard) Release() error { return nil }

// TestReadDoesNotWaitForAMutation proves a strict read of one key is answered
// while a mutation of another key waits for the store's write lock.
//
// Method: an Increment enters AcquireLock and is held there, the way OSPF's
// boot-count increment waits behind the boot record. A Read of another key,
// the way OSPF then reads its graceful restart fact, must answer before the
// hold ends. Before the fix every Read took the same mutex the waiting
// Increment held, so the Read waited for the hold and OSPF's five second
// deadline expired at startup.
func TestReadDoesNotWaitForAMutation(t *testing.T) {
	store := &heldStore{entered: make(chan struct{}), release: make(chan struct{})}
	SetStore(store)
	t.Cleanup(func() { SetStore(nil) })

	var wg sync.WaitGroup
	wg.Add(1)
	var incrementErr error
	go func() { // test-only worker: one increment, joined below.
		defer wg.Done()
		_, incrementErr = Increment("state/test/boot-count")
	}()
	<-store.entered

	answered := make(chan error, 1)
	go func() { // test-only worker: one read, joined through answered.
		data, found, err := Read("state/test/restart-fact")
		if err == nil && (!found || string(data) != "state/test/restart-fact") {
			t.Errorf("read answered found=%v data=%q", found, data)
		}
		answered <- err
	}()

	select {
	case err := <-answered:
		answered = nil
		if err != nil {
			t.Errorf("read while a mutation waits: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("read waited behind a mutation that is waiting for the store's write lock")
	}

	close(store.release)
	wg.Wait()
	if incrementErr != nil {
		t.Fatalf("increment after the hold: %v", incrementErr)
	}
	if answered != nil {
		<-answered // the read answers once the hold ends; join it.
	}
}
