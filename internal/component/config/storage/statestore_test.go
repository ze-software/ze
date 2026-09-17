// Design: docs/architecture/zefs-format.md -- daemon-owned runtime state.
// These tests drive internal/core/statestore over the real store. They live
// beside the component because the leaf tier cannot import it.
package storage_test

// statestore's API changed from path-based (PutAt/GetAt/Path, one
// transient BlobStore per call) to the shared-instance API (SetStore/Put/Get)
// after the adversarial review found the transient design let the config store's
// flush drop state keys. The old path-injection assertions are replaced by the
// shared-store round-trip plus TestConfigWriteDoesNotDropStateKey, which pins the
// exact regression (a config write through the shared handle must not drop state).

import (
	"encoding/binary"
	"errors"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/statestore"
)

// withStore registers a fresh owned tree and unregisters it before closing.
func withStore(t *testing.T) storage.Storage {
	t.Helper()
	bs, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatalf("storage.Create: %v", err)
	}
	statestore.SetStore(bs)
	t.Cleanup(func() {
		statestore.SetStore(nil)
		if cerr := bs.Close(); cerr != nil {
			t.Errorf("close store: %v", cerr)
		}
	})
	return bs
}

func TestPutGetRoundTrip(t *testing.T) {
	withStore(t)
	wrote, err := statestore.Put("meta/test/round-trip", []byte("hello"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !wrote {
		t.Fatal("Put reported no write with a registered store")
	}
	got, ok := statestore.Get("meta/test/round-trip")
	if !ok || string(got) != "hello" {
		t.Errorf("Get = %q ok=%v, want hello", got, ok)
	}
	// Overwrite replaces the value.
	if _, err := statestore.Put("meta/test/round-trip", []byte("world")); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	if got, ok := statestore.Get("meta/test/round-trip"); !ok || string(got) != "world" {
		t.Errorf("Get after overwrite = %q ok=%v, want world", got, ok)
	}
}

// TestConfigWriteDoesNotDropStateKey is the regression test for the reviewed
// blocker: because statestore writes through the config system's OWN handle (one
// in-memory tree), a config write cannot drop a previously-written state key.
// With the old two-instance design the config store's flush re-encoded from its
// stale tree and wiped this key.
func TestConfigWriteDoesNotDropStateKey(t *testing.T) {
	bs := withStore(t)
	if _, err := statestore.Put("meta/ddos/detect-baseline", []byte("STATE")); err != nil {
		t.Fatalf("Put state: %v", err)
	}
	// Simulate the config system writing a NEW key through the same shared store
	// (this is the flushFull path that dropped state keys under the old design).
	if err := bs.WriteFile("meta/config/active-hash", []byte("CONFIG"), 0); err != nil {
		t.Fatalf("config write: %v", err)
	}
	if got, ok := statestore.Get("meta/ddos/detect-baseline"); !ok || string(got) != "STATE" {
		t.Errorf("state key dropped by a config write: got %q ok=%v, want STATE", got, ok)
	}
	// And the state write did not drop the config key either.
	if got, err := bs.ReadFile("meta/config/active-hash"); err != nil || string(got) != "CONFIG" {
		t.Errorf("config key = %q err=%v, want CONFIG", got, err)
	}
}

func TestNoStoreIsNoOp(t *testing.T) {
	statestore.SetStore(nil)
	wrote, err := statestore.Put("meta/test/x", []byte("data"))
	if err != nil {
		t.Errorf("Put with no store should not error, got %v", err)
	}
	if wrote {
		t.Error("Put with no store reported a write")
	}
	if _, ok := statestore.Get("meta/test/x"); ok {
		t.Error("Get with no store returned ok=true")
	}
	if err := statestore.Remove("meta/test/x"); err != nil {
		t.Errorf("Remove with no store should be a no-op, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	withStore(t)
	if _, err := statestore.Put("meta/test/rm", []byte("bye")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := statestore.Remove("meta/test/rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := statestore.Get("meta/test/rm"); ok {
		t.Error("key still present after Remove")
	}
	// Removing an absent key is a no-op, not an error.
	if err := statestore.Remove("meta/test/absent"); err != nil {
		t.Errorf("Remove on absent key errored: %v", err)
	}
}

// Concurrent increments must allocate distinct durable sequence spaces, and a
// corrupt counter must never be reset to one.
func TestIncrementDurableConcurrent(t *testing.T) {
	store := withStore(t)
	const key = "meta/test/counter"
	const workers = 32
	values := make(chan uint32, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			value, err := statestore.Increment(key)
			if err != nil {
				t.Errorf("increment: %v", err)
				return
			}
			values <- value
		})
	}
	wg.Wait()
	close(values)
	seen := make(map[uint32]bool)
	for value := range values {
		if seen[value] {
			t.Fatalf("duplicate counter %d", value)
		}
		seen[value] = true
	}
	for value := uint32(1); value <= workers; value++ {
		if !seen[value] {
			t.Fatalf("missing counter %d", value)
		}
	}
	data, err := store.ReadKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(data) != workers {
		t.Fatalf("durable counter = %x", data)
	}
	if err := store.WriteKey(key, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if _, err := statestore.Increment(key); !errors.Is(err, statestore.ErrCorrupt) {
		t.Fatalf("corrupt counter: %v", err)
	}
	if err := store.WriteKey(key, []byte{255, 255, 255, 255}); err != nil {
		t.Fatal(err)
	}
	if _, err := statestore.Increment(key); err == nil {
		t.Fatal("exhausted counter wrapped")
	}
}

// Strict RPC consumers must distinguish storeless mode from an absent key.
func TestStrictStateUnavailableAndAbsent(t *testing.T) {
	statestore.SetStore(nil)
	if _, _, err := statestore.Read("meta/test/x"); !errors.Is(err, statestore.ErrUnavailable) {
		t.Fatalf("read unavailable: %v", err)
	}
	if err := statestore.Write("meta/test/x", nil); !errors.Is(err, statestore.ErrUnavailable) {
		t.Fatalf("write unavailable: %v", err)
	}
	if err := statestore.Delete("meta/test/x"); !errors.Is(err, statestore.ErrUnavailable) {
		t.Fatalf("delete unavailable: %v", err)
	}
	withStore(t)
	if _, found, err := statestore.Read("meta/test/x"); err != nil || found {
		t.Fatalf("absent read: found=%v error=%v", found, err)
	}
}
