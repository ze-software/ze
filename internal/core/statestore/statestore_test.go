package statestore

// test-relax: statestore's API changed from path-based (PutAt/GetAt/Path, one
// transient BlobStore per call) to the shared-instance API (SetStore/Put/Get)
// after the adversarial review found the transient design let the config store's
// flush drop state keys. The old path-injection assertions are replaced by the
// shared-store round-trip plus TestConfigWriteDoesNotDropStateKey, which pins the
// exact regression (a config write through the shared handle must not drop state).

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

// withStore registers a fresh temp database.zefs as the shared store for the test
// and resets to nil on cleanup, so tests do not leak the global store.
func withStore(t *testing.T) *zefs.BlobStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.zefs")
	bs, err := zefs.Create(path)
	if err != nil {
		t.Fatalf("zefs.Create: %v", err)
	}
	SetStore(bs)
	t.Cleanup(func() {
		SetStore(nil)
		if cerr := bs.Close(); cerr != nil {
			t.Errorf("close store: %v", cerr)
		}
	})
	return bs
}

func TestPutGetRoundTrip(t *testing.T) {
	withStore(t)
	wrote, err := Put("meta/test/round-trip", []byte("hello"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !wrote {
		t.Fatal("Put reported no write with a registered store")
	}
	got, ok := Get("meta/test/round-trip")
	if !ok || string(got) != "hello" {
		t.Errorf("Get = %q ok=%v, want hello", got, ok)
	}
	// Overwrite replaces the value.
	if _, err := Put("meta/test/round-trip", []byte("world")); err != nil {
		t.Fatalf("Put overwrite: %v", err)
	}
	if got, ok := Get("meta/test/round-trip"); !ok || string(got) != "world" {
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
	if _, err := Put("meta/ddos/detect-baseline", []byte("STATE")); err != nil {
		t.Fatalf("Put state: %v", err)
	}
	// Simulate the config system writing a NEW key through the same shared store
	// (this is the flushFull path that dropped state keys under the old design).
	if err := bs.WriteFile("meta/config/active-hash", []byte("CONFIG"), 0); err != nil {
		t.Fatalf("config write: %v", err)
	}
	if got, ok := Get("meta/ddos/detect-baseline"); !ok || string(got) != "STATE" {
		t.Errorf("state key dropped by a config write: got %q ok=%v, want STATE", got, ok)
	}
	// And the state write did not drop the config key either.
	if got, err := bs.ReadFile("meta/config/active-hash"); err != nil || string(got) != "CONFIG" {
		t.Errorf("config key = %q err=%v, want CONFIG", got, err)
	}
}

func TestNoStoreIsNoOp(t *testing.T) {
	SetStore(nil)
	wrote, err := Put("meta/test/x", []byte("data"))
	if err != nil {
		t.Errorf("Put with no store should not error, got %v", err)
	}
	if wrote {
		t.Error("Put with no store reported a write")
	}
	if _, ok := Get("meta/test/x"); ok {
		t.Error("Get with no store returned ok=true")
	}
	if err := Remove("meta/test/x"); err != nil {
		t.Errorf("Remove with no store should be a no-op, got %v", err)
	}
}

func TestRemove(t *testing.T) {
	withStore(t)
	if _, err := Put("meta/test/rm", []byte("bye")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if err := Remove("meta/test/rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, ok := Get("meta/test/rm"); ok {
		t.Error("key still present after Remove")
	}
	// Removing an absent key is a no-op, not an error.
	if err := Remove("meta/test/absent"); err != nil {
		t.Errorf("Remove on absent key errored: %v", err)
	}
}
