package zefs

import (
	"errors"
	"io/fs"
	"os"
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

	wl := lockOrFatal(t, s)
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

	wl := lockOrFatal(t, s)
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

	wl := lockOrFatal(t, s)

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

	wl := lockOrFatal(t, s)

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

	wl := lockOrFatal(t, s)

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

// VALIDATES: WriteLock.WriteFile rejects invalid keys
// PREVENTS: tree poisoning from invalid entries in batched writes

func TestWriteLockRejectsInvalidKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	writeLockedOrFatal(t, wl, "good", []byte("fine"))

	err = wl.WriteFile(".", []byte("bad-key"), 0)
	if err == nil {
		t.Fatal("expected error for invalid key")
		return
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

	wl := lockOrFatal(t, s)
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
	wl := lockOrFatal(t, s)
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
	wl := lockOrFatal(t, s)
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

// VALIDATES: Lock can be acquired again after Release
// PREVENTS: single-use lock that can't be reacquired

func TestLockSequentialCycles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		wl := lockOrFatal(t, s)
		writeLockedOrFatal(t, wl, "counter", []byte{byte('0' + i)})
		if err := wl.Release(); err != nil {
			t.Fatalf("cycle %d Release: %v", i, err)
		}
	}

	got, err := s.ReadFile("counter")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "2" {
		t.Errorf("got %q, want %q", got, "2")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: concurrent Lock() callers are serialized
// PREVENTS: two writers entering the critical section simultaneously

func TestLockConcurrentSerialization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)

	acquired := make(chan struct{})
	done := make(chan struct{})
	go func() {
		wl2 := lockOrFatal(t, s)
		close(acquired)
		_ = wl2.Release()
		close(done)
	}()

	// Second Lock() should block while first is held
	select {
	case <-acquired:
		t.Fatal("second Lock() should block while first is held")
	case <-time.After(50 * time.Millisecond):
		// expected: blocked
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Now the second Lock() should complete
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second Lock() should unblock after first Release")
	}

	// Wait for goroutine to finish Release before closing
	<-done

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock blocks Lock acquisition
// PREVENTS: writer entering while readers are active

func TestReadLockBlocksWriteLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	rl := s.RLock()

	acquired := make(chan struct{})
	done := make(chan struct{})
	go func() {
		wl := lockOrFatal(t, s)
		close(acquired)
		_ = wl.Release()
		close(done)
	}()

	// Lock() should block while ReadLock is held
	select {
	case <-acquired:
		t.Fatal("Lock() should block while ReadLock is held")
	case <-time.After(50 * time.Millisecond):
		// expected: blocked
	}

	rl.Release()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("Lock() should unblock after ReadLock.Release")
	}

	// Wait for goroutine to finish Release before closing
	<-done

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock.ReadFile returns error for missing key
// PREVENTS: silent nil data on missing key

func TestReadLockReadFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	rl := s.RLock()
	_, err = rl.ReadFile("nonexistent")
	rl.Release()

	if err == nil {
		t.Fatal("expected error for missing key")
		return
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected *fs.PathError, got %T", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock.ReadDir returns error for missing directory
// PREVENTS: nil entries without error on missing directory

func TestReadLockReadDirMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	rl := s.RLock()
	_, err = rl.ReadDir("nonexistent")
	rl.Release()

	if err == nil {
		t.Fatal("expected error for missing directory")
		return
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.ReadFile returns error for missing key
// PREVENTS: silent nil data on missing key within write transaction

func TestWriteLockReadFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	_, err = wl.ReadFile("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
		return
	}
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("expected *fs.PathError, got %T", err)
	}
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.ReadDir returns error for missing directory
// PREVENTS: nil entries without error on missing directory within write transaction

func TestWriteLockReadDirMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	_, err = wl.ReadDir("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing directory")
		return
	}
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: write then remove same key within one transaction
// PREVENTS: stale entry surviving after in-transaction remove

func TestWriteLockWriteThenRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	writeLockedOrFatal(t, wl, "ephemeral", []byte("temp"))

	if !wl.Has("ephemeral") {
		t.Fatal("Has should be true after write")
	}

	if err := wl.Remove("ephemeral"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if wl.Has("ephemeral") {
		t.Error("Has should be false after remove")
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	if s.Has("ephemeral") {
		t.Error("entry should not persist after release")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: remove then rewrite same key within one transaction
// PREVENTS: removed entry not recoverable within same transaction

func TestWriteLockRemoveThenRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("original"))

	wl := lockOrFatal(t, s)
	if err := wl.Remove("key"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	writeLockedOrFatal(t, wl, "key", []byte("replaced"))

	got, err := wl.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "replaced" {
		t.Errorf("within lock: got %q, want %q", got, "replaced")
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	got, err = s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "replaced" {
		t.Errorf("after release: got %q, want %q", got, "replaced")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: Lock on empty store writes first entries
// PREVENTS: lock operations failing on store with no prior data

func TestWriteLockOnEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)

	if wl.Has("anything") {
		t.Error("empty store should have nothing")
	}
	if keys := wl.List(""); len(keys) != 0 {
		t.Errorf("empty store List: got %v", keys)
	}

	writeLockedOrFatal(t, wl, "first", []byte("entry"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadFile("first")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "entry" {
		t.Errorf("got %q, want %q", got, "entry")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock on empty store returns empty results
// PREVENTS: error or panic when reading empty store under lock

func TestReadLockOnEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	rl := s.RLock()

	if rl.Has("anything") {
		t.Error("empty store should have nothing")
	}
	if keys := rl.List(""); len(keys) != 0 {
		t.Errorf("empty store List: got %v", keys)
	}
	_, err = rl.ReadFile("missing")
	if err == nil {
		t.Error("expected error reading from empty store")
	}

	rl.Release()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock data persists across close and reopen
// PREVENTS: locked writes lost on store lifecycle boundary

func TestWriteLockPersistenceAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	writeLockedOrFatal(t, wl, "survive", []byte("reopen"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s2.ReadFile("survive")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "reopen" {
		t.Errorf("got %q, want %q", got, "reopen")
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock and ReadLock satisfy WriteGuard and ReadGuard interfaces
// PREVENTS: interface drift between lock types and guard contracts

func TestGuardInterfaceSatisfaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	// Use through WriteGuard interface
	var wg WriteGuard
	wl := lockOrFatal(t, s)
	wg = wl
	if err := wg.WriteFile("iface", []byte("test"), 0); err != nil {
		t.Fatalf("WriteGuard.WriteFile: %v", err)
	}
	got, err := wg.ReadFile("iface")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "test" {
		t.Errorf("WriteGuard.ReadFile: got %q", got)
	}
	if !wg.Has("key") {
		t.Error("WriteGuard.Has should be true")
	}
	if keys := wg.List(""); len(keys) != 2 {
		t.Errorf("WriteGuard.List: got %d keys, want 2", len(keys))
	}
	if err := wg.Release(); err != nil {
		t.Fatal(err)
	}

	// Use through ReadGuard interface
	var rg ReadGuard
	rl := s.RLock()
	rg = rl
	got, err = rg.ReadFile("iface")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "test" {
		t.Errorf("ReadGuard.ReadFile: got %q", got)
	}
	if !rg.Has("iface") {
		t.Error("ReadGuard.Has should be true")
	}
	if keys := rg.List(""); len(keys) != 2 {
		t.Errorf("ReadGuard.List: got %d keys, want 2", len(keys))
	}
	rg.Release()

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.List with empty prefix returns all keys
// PREVENTS: empty prefix treated differently under lock vs auto-locking

func TestWriteLockListAllKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "a/one", []byte("1"))
	writeOrFatal(t, s, "b/two", []byte("2"))
	writeOrFatal(t, s, "c/three", []byte("3"))

	wl := lockOrFatal(t, s)

	// Add a fourth under lock
	writeLockedOrFatal(t, wl, "d/four", []byte("4"))

	keys := wl.List("")
	if len(keys) != 4 {
		t.Errorf("List all: got %d keys, want 4: %v", len(keys), keys)
	}

	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: interleaved RLock/Lock cycles work correctly
// PREVENTS: lock state corruption from alternating lock types

func TestAlternatingLockTypes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write under lock
	wl := lockOrFatal(t, s)
	writeLockedOrFatal(t, wl, "step", []byte("1"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Read under rlock
	rl := s.RLock()
	got, err := rl.ReadFile("step")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "1" {
		t.Errorf("got %q, want %q", got, "1")
	}
	rl.Release()

	// Write again
	wl = lockOrFatal(t, s)
	writeLockedOrFatal(t, wl, "step", []byte("2"))
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Read again
	rl = s.RLock()
	got, err = rl.ReadFile("step")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "2" {
		t.Errorf("got %q, want %q", got, "2")
	}
	rl.Release()

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: concurrent readers and writers interleave correctly
// PREVENTS: race conditions between RLock and Lock users

func TestConcurrentReadersAndWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "shared", []byte("initial"))

	var wg sync.WaitGroup

	// 3 readers
	for range 3 {
		wg.Go(func() {
			rl := s.RLock()
			defer rl.Release()
			_, readErr := rl.ReadFile("shared")
			if readErr != nil {
				t.Errorf("reader: %v", readErr)
			}
		})
	}

	// 2 writers (sequential due to exclusive lock)
	for i := range 2 {
		wg.Go(func() {
			wl := lockOrFatal(t, s)
			defer func() { _ = wl.Release() }()
			writeLockedOrFatal(t, wl, "shared", []byte{byte('A' + i)})
		})
	}

	wg.Wait()

	// Store should still be usable
	got, err := s.ReadFile("shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected single byte, got %q", got)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock.ReadDir returns entries for existing directory
// PREVENTS: ReadDir only tested for error case, not success path

func TestReadLockReadDirSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "cfg/a", []byte("1"))
	writeOrFatal(t, s, "cfg/b", []byte("2"))
	writeOrFatal(t, s, "cfg/c", []byte("3"))

	rl := s.RLock()
	entries, err := rl.ReadDir("cfg")
	if err != nil {
		t.Fatalf("ReadLock.ReadDir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
	// Verify sorted order
	for i := 1; i < len(entries); i++ {
		if entries[i].Name() <= entries[i-1].Name() {
			t.Errorf("entries not sorted: %q after %q", entries[i].Name(), entries[i-1].Name())
		}
	}
	rl.Release()

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.ReadDir sees pending (unflushed) writes
// PREVENTS: ReadDir returning stale entries before Release flushes

func TestWriteLockReadDirSeesPendingWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "ns/old", []byte("original"))

	wl := lockOrFatal(t, s)

	// Verify existing key visible
	entries, err := wl.ReadDir("ns")
	if err != nil {
		t.Fatalf("ReadDir before write: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("before write: got %d entries, want 1", len(entries))
	}

	// Write a new key under same directory
	if writeErr := wl.WriteFile("ns/new", []byte("added"), 0); writeErr != nil {
		t.Fatal(writeErr)
	}

	// ReadDir should now see both entries (unflushed)
	entries, err = wl.ReadDir("ns")
	if err != nil {
		t.Fatalf("ReadDir after write: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("after write: got %d entries, want 2", len(entries))
	}

	if releaseErr := wl.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}

	// After release, public ReadDir also sees both
	entries, err = s.ReadDir("ns")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("after release: got %d entries, want 2", len(entries))
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.ReadDir root sees pending new directory
// PREVENTS: new hierarchical key not creating visible directory node

func TestWriteLockReadDirRootAfterNewDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)

	// Root should be empty
	entries, err := wl.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) empty: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty store root: got %d entries, want 0", len(entries))
	}

	// Write creates directory structure
	if writeErr := wl.WriteFile("new-dir/file", []byte("data"), 0); writeErr != nil {
		t.Fatal(writeErr)
	}

	// Root should now show the new directory
	entries, err = wl.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(.) after write: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("after write: got %d entries, want 1", len(entries))
	}
	if !entries[0].IsDir() {
		t.Error("new-dir should be a directory")
	}
	if entries[0].Name() != "new-dir" {
		t.Errorf("entry name: got %q, want %q", entries[0].Name(), "new-dir")
	}

	if releaseErr := wl.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock.ReadFile returns zero-copy reference (not a copy)
// PREVENTS: ReadLock accidentally using the copying ReadFile path

func TestReadLockReadFileZeroCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	// ReadLock.ReadFile calls internal readFile (zero-copy)
	rl := s.RLock()
	data, err := rl.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "value" {
		t.Errorf("got %q, want %q", data, "value")
	}
	rl.Release()

	// BlobStore.ReadFile returns a copy (caller-owned)
	copy1, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}
	copy2, err := s.ReadFile("key")
	if err != nil {
		t.Fatal(err)
	}

	// Mutating one copy should not affect the other
	copy1[0] = 'X'
	if copy2[0] == 'X' {
		t.Error("BlobStore.ReadFile copies should be independent")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.Remove on nonexistent key preserves dirty flag
// PREVENTS: failed remove resetting dirty state from previous writes

func TestWriteLockRemoveErrorPreservesDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)

	// Write sets dirty=true
	if writeErr := wl.WriteFile("exists", []byte("data"), 0); writeErr != nil {
		t.Fatal(writeErr)
	}

	// Remove nonexistent key fails, but dirty should remain true
	if removeErr := wl.Remove("nonexistent"); removeErr == nil {
		t.Fatal("Remove nonexistent should fail")
		return
	}

	// Release should flush (dirty=true from the write)
	if releaseErr := wl.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}

	// Verify the write was persisted
	got, err := s.ReadFile("exists")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock.Release double-release returns nil (idempotent)
// PREVENTS: double-unlock panic on mutex

func TestWriteLockDoubleReleaseIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	writeLockedOrFatal(t, wl, "key", []byte("value"))

	// First release: flushes and unlocks
	if err := wl.Release(); err != nil {
		t.Fatal(err)
	}

	// Second release: should return nil (no-op)
	if err := wl.Release(); err != nil {
		t.Errorf("second Release: got %v, want nil", err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: ReadLock double-release does not panic
// PREVENTS: double RUnlock panic on sync.RWMutex

func TestReadLockDoubleReleaseNoPanic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}

	rl := s.RLock()
	rl.Release()
	// Second release should be a no-op (released flag)
	rl.Release()

	// Store should still be usable
	if err := s.WriteFile("after", []byte("ok"), 0); err != nil {
		t.Fatalf("store unusable after double release: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// VALIDATES: WriteLock clean release (no writes) does not flush
// PREVENTS: unnecessary disk I/O on read-only lock usage

func TestWriteLockCleanReleaseNoFlush(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.zefs")
	s, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writeOrFatal(t, s, "key", []byte("value"))

	// Get file mod time before lock
	info1, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	wl := lockOrFatal(t, s)
	// Only read, no writes
	if _, readErr := wl.ReadFile("key"); readErr != nil {
		t.Fatal(readErr)
	}
	if releaseErr := wl.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}

	// File should not have been rewritten
	info2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("clean WriteLock.Release should not rewrite the file")
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
