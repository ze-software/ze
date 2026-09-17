// Design: docs/architecture/hub-architecture.md -- one daemon store owns state
package hub

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	internalresolve "github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

func newTestStore(t *testing.T, dirs ...string) storage.Storage {
	t.Helper()
	var dir string
	if len(dirs) != 0 {
		dir = dirs[0]
	} else {
		dir = t.TempDir()
	}
	store, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return internalresolve.BindConfigSource(store, "", internalresolve.ConfigSourceStored)
}

func newTestFileStore(t *testing.T, path string) storage.Storage {
	t.Helper()
	return internalresolve.BindConfigSource(newTestStore(t, filepath.Dir(path)), path, internalresolve.ConfigSourceFile)
}

// State and configuration share ownership and survive the same close/reopen.
func TestDaemonStateSharesConfigStore(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir)
	statestore.SetStore(store)
	t.Cleanup(func() { statestore.SetStore(nil) })
	if saved, err := statestore.Put(zefs.KeyNTPLastTime.Pattern, []byte("SURVIVE")); err != nil || !saved {
		t.Fatalf("persist state: saved=%v err=%v", saved, err)
	}
	if err := store.WriteFile("router.conf", []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	statestore.SetStore(nil)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close() //nolint:errcheck // test cleanup
	got, err := reopened.ReadFile(zefs.KeyNTPLastTime.Pattern)
	if err != nil || string(got) != "SURVIVE" {
		t.Fatalf("state after config write and restart = %q, %v", got, err)
	}
}

// An absent stdin store is identified without creating a side database or
// migrating the loose config. Blob, corrupt and permission failures differ.
func TestStdinAbsentStoreCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "router.conf")
	if err := os.WriteFile(config, []byte("bgp {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(dir)
	if !errors.Is(err, storage.ErrNoStore) || store != nil {
		t.Fatalf("open absent store = %v, %v", store, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "router.conf" {
		t.Fatalf("absent open changed the config folder: %v", entries)
	}
}
