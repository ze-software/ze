package zefs

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// VALIDATES: ReadLock allows concurrent readers
// PREVENTS: shared lock behaving as exclusive

func TestReadLockConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	var wg sync.WaitGroup
	started := make(chan struct{}, 3)

	for range 3 {
		wg.Go(func() {
			rl := s.RLock()
			defer rl.Release()
			started <- struct{}{}

			data, readErr := rl.ReadFile("key")
			if readErr != nil {
				t.Errorf("ReadFile: %v", readErr)
				return
			}
			if string(data) != "value" {
				t.Errorf("got %q, want %q", data, "value")
			}
			// Hold the lock briefly to ensure concurrency
			time.Sleep(10 * time.Millisecond)
		})
	}

	// All 3 readers should start without blocking each other
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("readers blocked -- shared lock acting as exclusive")
		}
	}

	wg.Wait()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock is exclusive and batches flushes to Release
// PREVENTS: interleaved writes from concurrent goroutines

func TestWriteLockExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := s.Lock()
	writeLockedOrFatal(t, wl, "a", []byte("aaa"))
	writeLockedOrFatal(t, wl, "b", []byte("bbb"))

	// Before Release, auto-locking ReadFile should block.
	done := make(chan struct{})
	go func() {
		if _, readErr := s.ReadFile("a"); readErr != nil {
			t.Errorf("blocked ReadFile: %v", readErr)
		}
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("ReadFile should block while WriteLock is held")
	case <-time.After(50 * time.Millisecond):
		// expected: blocked
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Now the blocked reader should complete
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ReadFile should unblock after Release")
	}

	// Verify both writes persisted
	got, err := s.ReadFile("a")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "aaa" {
		t.Errorf("got %q, want %q", got, "aaa")
	}
	got, err = s.ReadFile("b")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "bbb" {
		t.Errorf("got %q, want %q", got, "bbb")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock batch -- single flush on Release, not per WriteFile
// PREVENTS: redundant disk I/O during multi-write transactions

func TestWriteLockBatchPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := s.Lock()
	writeLockedOrFatal(t, wl, "x", []byte("111"))
	writeLockedOrFatal(t, wl, "y", []byte("222"))
	writeLockedOrFatal(t, wl, "z", []byte("333"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen to verify persistence
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ key, want string }{
		{"x", "111"}, {"y", "222"}, {"z", "333"},
	} {
		got, readErr := s2.ReadFile(tc.key)
		if readErr != nil {
			t.Fatalf("ReadFile(%s): %v", tc.key, readErr)
		}
		if string(got) != tc.want {
			t.Errorf("ReadFile(%s): got %q, want %q", tc.key, got, tc.want)
		}
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock exposes read-only operations
// PREVENTS: missing read methods on lock guard

func TestReadLockOperations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "bgp/peers/n1.conf", []byte("peer1"))
	writeOrFatal(t, s, "bgp/config.conf", []byte("config"))

	rl := s.RLock()
	defer rl.Release()

	got, err := rl.ReadFile("bgp/peers/n1.conf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "peer1" {
		t.Errorf("ReadFile: got %q", got)
	}

	if !rl.Has("bgp/config.conf") {
		t.Error("Has should be true")
	}
	if rl.Has("nonexistent") {
		t.Error("Has should be false")
	}

	keys := rl.List("bgp/peers")
	if len(keys) != 1 || keys[0] != "bgp/peers/n1.conf" {
		t.Errorf("List: got %v", keys)
	}

	entries, err := rl.ReadDir("bgp")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("ReadDir: got %d entries, want 2", len(entries))
	}
}

// VALIDATES: WriteLock read-modify-write pattern
// PREVENTS: stale reads within a write transaction

func TestWriteLockReadModifyWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "counter", []byte("0"))

	wl := s.Lock()

	data, err := wl.ReadFile("counter")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "0" {
		t.Fatalf("got %q, want %q", data, "0")
	}

	writeLockedOrFatal(t, wl, "counter", []byte("1"))

	// Read within same lock should see new value
	data, err = wl.ReadFile("counter")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1" {
		t.Errorf("within lock: got %q, want %q", data, "1")
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.Remove deletes entries with batched flush
// PREVENTS: missing remove support in write transactions

func TestWriteLockRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "keep", []byte("yes"))
	writeOrFatal(t, s, "drop", []byte("no"))

	wl := s.Lock()

	if err := wl.Remove("drop"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if wl.Has("drop") {
		t.Error("Has(drop) should be false after Remove")
	}
	if !wl.Has("keep") {
		t.Error("Has(keep) should be true")
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Verify persistence after Release
	got, err := s.ReadFile("keep")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "yes" {
		t.Errorf("keep: got %q, want %q", got, "yes")
	}
	if s.Has("drop") {
		t.Error("drop should not exist after Release")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock exposes read operations (Has, List, ReadDir)
// PREVENTS: asymmetric read access between lock types

func TestWriteLockReadOperations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "bgp/peers/n1.conf", []byte("peer1"))
	writeOrFatal(t, s, "bgp/config.conf", []byte("config"))

	wl := s.Lock()

	if !wl.Has("bgp/config.conf") {
		t.Error("Has should be true")
	}
	if wl.Has("nonexistent") {
		t.Error("Has should be false")
	}

	keys := wl.List("bgp/peers")
	if len(keys) != 1 || keys[0] != "bgp/peers/n1.conf" {
		t.Errorf("List: got %v", keys)
	}

	entries, err := wl.ReadDir("bgp")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("ReadDir: got %d entries, want 2", len(entries))
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.WriteFile rejects oversized data
// PREVENTS: tree poisoning from oversized entries in batched writes

func TestWriteLockRejectsOversized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := s.Lock()
	writeLockedOrFatal(t, wl, "good", []byte("fine"))

	big := make([]byte, maxHeaderVal+1)
	err = wl.WriteFile("toobig", big, 0)
	if err == nil {
		t.Fatal("expected error for oversized data")
	}

	// The good write should still succeed on Release
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadFile("good")
	if err != nil {
		t.Fatalf("ReadFile after rejected locked write: %v", err)
	}
	if string(got) != "fine" {
		t.Errorf("got %q, want %q", got, "fine")
	}
	if s.Has("toobig") {
		t.Error("rejected entry should not exist")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.Remove returns error for nonexistent key
// PREVENTS: silent no-op on locked remove of missing key

func TestWriteLockRemoveNonexistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := s.Lock()
	err = wl.Remove("does-not-exist")
	if err == nil {
		t.Error("expected error removing nonexistent key under lock")
	}
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.Release without writes skips flush
// PREVENTS: unnecessary disk I/O on clean release

func TestWriteLockReleaseClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	// Acquire write lock, read but don't write, release
	wl := s.Lock()
	got, err := wl.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Store should still work normally
	got, err = s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "value" {
		t.Errorf("after clean release: got %q, want %q", got, "value")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Release is idempotent on both lock types
// PREVENTS: panic on double-release

func TestLockDoubleRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	// ReadLock double-release should not panic
	rl := s.RLock()
	rl.Release()
	rl.Release() // no-op

	// WriteLock double-release should not panic
	wl := s.Lock()
	writeLockedOrFatal(t, wl, "key", []byte("updated"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := wl.Release(); err != nil {
		t.Errorf("double Release should return nil, got: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
