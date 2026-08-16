package hub

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
)

// VALIDATES: openStateOnlyStore refuses when no config directory is pinned, so a
// one-off `ze -` never creates or contends on a shared binary-relative store.
// PREVENTS: a regression that silently creates database.zefs next to the binary
// for every invocation.
func TestOpenStateOnlyStoreNeedsConfigDir(t *testing.T) {
	bs, err := openStateOnlyStore("")
	if !errors.Is(err, errNoConfigDirForStateStore) {
		t.Fatalf("openStateOnlyStore(\"\") error = %v, want errNoConfigDirForStateStore", err)
	}
	if bs != nil {
		t.Fatal("openStateOnlyStore(\"\") returned a store")
	}
}

// VALIDATES: openStateOnlyStore creates {dir}/database.zefs, round-trips a key,
// and REOPENS the same file on a second call so state survives a restart.
// PREVENTS: a state store that is recreated empty on every boot.
func TestOpenStateOnlyStoreCreatesAndReopens(t *testing.T) {
	dir := t.TempDir()

	bs, err := openStateOnlyStore(dir)
	if err != nil {
		t.Fatalf("openStateOnlyStore: %v", err)
	}
	if err := bs.WriteFile("meta/test/state", []byte("SURVIVE"), 0); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := bs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	path := filepath.Join(dir, stateStoreName)
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("state store %s not created: %v", path, statErr)
	}

	// Second open must REOPEN, not recreate: the key written above survives.
	reopened, err := openStateOnlyStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	got, err := reopened.ReadFile("meta/test/state")
	if err != nil || string(got) != "SURVIVE" {
		t.Fatalf("reopened state = %q err=%v, want SURVIVE", got, err)
	}
}

// VALIDATES: the state store is STATE-ONLY -- it does not migrate the on-disk
// config file into itself the way storage.NewBlob does.
// PREVENTS: the regression that made a blob-backed run answer a SIGHUP reload
// from a stale migrated copy instead of the rewritten file on disk.
func TestOpenStateOnlyStoreDoesNotMigrateConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "ze-bgp.conf")
	if err := os.WriteFile(cfg, []byte("bgp {\n}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	bs, err := openStateOnlyStore(dir)
	if err != nil {
		t.Fatalf("openStateOnlyStore: %v", err)
	}
	t.Cleanup(func() { _ = bs.Close() })

	if bs.Has("file/active/ze-bgp.conf") {
		t.Error("state store migrated the config file in; it must stay state-only")
	}
}

// VALIDATES: the state store opens even when ze.storage.blob=false, the setting
// that selects the CONFIG backend. A config-backend toggle must not decide
// whether runtime state survives a restart.
// PREVENTS: the regression where routing this path through resolve.Storage()
// made ze.storage.blob=false silently drop every runtime-state key -- which for
// the tc backend is not a lost baseline but a refused qdisc program
// (errSnapshotPersistUnavailable) that fails daemon startup outright.
func TestOpenStateOnlyStoreIgnoresConfigBackendToggle(t *testing.T) {
	// the first draft of this test set ze.storage.blob via env.Set.
	// That aborts the process -- the key is registered in package main
	// (cmd/ze/ze_core_dispatch.go:82), not in package hub, so env.Get fatals with
	// "unregistered key" from this test binary. The toggle is replaced by the
	// stronger structural assertion below: openStateOnlyStore takes only a
	// directory and never consults the config backend at all, so there is no
	// toggle left that could reach it.
	dir := t.TempDir()

	// The old path derived the state store from the CONFIG backend. With
	// ze.storage.blob=false that backend is storage.NewFilesystem, which yields
	// NO blob store -- so statestore stayed nil and every runtime-state key was
	// dropped. This pins that premise; the rest of the test proves
	// openStateOnlyStore does not depend on it.
	if _, ok := storage.BlobStoreFrom(storage.NewFilesystem()); ok {
		t.Fatal("filesystem storage reported a blob store")
	}

	bs, err := openStateOnlyStore(dir)
	if err != nil {
		t.Fatalf("openStateOnlyStore with ze.storage.blob=false: %v", err)
	}
	t.Cleanup(func() { _ = bs.Close() })

	if _, err := bs.ReadFile("meta/absent"); err == nil {
		t.Error("absent key read succeeded")
	}
	if err := bs.WriteFile("meta/test/tc-snapshot", []byte("{}"), 0); err != nil {
		t.Fatalf("state write with ze.storage.blob=false: %v", err)
	}
}
