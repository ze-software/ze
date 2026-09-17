// VALIDATES: DefaultConfig derives "<instance>.conf" from the stored instance
// name, falling back to "ze.conf" when the name is missing, empty, or fails the
// validInstanceName guard (which blocks path traversal in blob keys).
// PREVENTS: an unsanitized instance name flowing into a config filename, and a
// regression in the missing/empty/invalid fallbacks.

package resolve

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/pkg/zefs"
)

// errStubUnused is returned by fakeStore methods that DefaultConfig never calls.
var errStubUnused = errors.New("fakeStore: method not used by these tests")

// fakeStore implements storage.Storage; only ReadFile carries behavior.
type fakeStore struct {
	storage.Storage
	data map[string][]byte
}

func (f fakeStore) ReadFile(name string) ([]byte, error) {
	if b, ok := f.data[name]; ok {
		return b, nil
	}
	return nil, os.ErrNotExist
}
func (fakeStore) WriteFile(string, []byte, fs.FileMode) error        { return nil }
func (fakeStore) Remove(string) error                                { return nil }
func (fakeStore) Exists(string) bool                                 { return false }
func (fakeStore) List(string) ([]string, error)                      { return nil, nil }
func (fakeStore) AcquireLock(string) (storage.WriteGuard, error)     { return nil, errStubUnused }
func (fakeStore) Stat(string) (storage.FileMeta, error)              { return storage.FileMeta{}, nil }
func (fakeStore) Rename(string, string) error                        { return nil }
func (fakeStore) Close() error                                       { return nil }
func (fakeStore) WriteVersion(string, []byte, time.Time) error       { return nil }
func (fakeStore) ListVersions(string) ([]storage.VersionInfo, error) { return nil, nil }

func TestDefaultConfig(t *testing.T) {
	key := zefs.KeyInstanceName.Pattern

	for _, tc := range []struct {
		name   string
		stored map[string][]byte
		want   string
	}{
		{"valid name", map[string][]byte{key: []byte("edge-01\n")}, "edge-01.conf"},
		{"missing key", map[string][]byte{}, "ze.conf"},
		{"empty value", map[string][]byte{key: []byte("  \n")}, "ze.conf"},
		{"path traversal blocked", map[string][]byte{key: []byte("../evil")}, "ze.conf"},
		{"leading hyphen blocked", map[string][]byte{key: []byte("-bad")}, "ze.conf"},
	} {
		if got := DefaultConfig(fakeStore{data: tc.stored}); got != tc.want {
			t.Errorf("%s: DefaultConfig = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Explicit paths own their folder regardless of a pinned default. Empty and
// stdin inputs share the environment/default resolution and never auto-create.
func TestStorageForResolution(t *testing.T) {
	prior := env.Get("ze.config.dir")
	t.Cleanup(func() {
		if err := env.Set("ze.config.dir", prior); err != nil {
			t.Error(err)
		}
	})
	pinned, explicit := t.TempDir(), t.TempDir()
	if err := env.Set("ze.config.dir", pinned); err != nil {
		t.Fatal(err)
	}
	if got := StoreDir(filepath.Join(explicit, "router.conf")); got != explicit {
		t.Fatalf("explicit dir=%q", got)
	}
	for _, path := range []string{"", "-"} {
		if got := StoreDir(path); got != pinned {
			t.Fatalf("StoreDir(%q)=%q", path, got)
		}
		if store, err := StorageFor(path); !errors.Is(err, storage.ErrNoStore) || store != nil {
			t.Fatalf("absent open=%v,%v", store, err)
		}
	}
	created, err := storage.Create(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if err := created.WriteKey("meta/test/resolution", []byte("explicit")); err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := StorageFor(filepath.Join(explicit, "router.conf"))
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close() //nolint:errcheck // test cleanup
	value, err := opened.ReadKey("meta/test/resolution")
	if err != nil || string(value) != "explicit" {
		t.Fatalf("resolved value=%q,%v", value, err)
	}
	if err := env.Set("ze.config.dir", ""); err != nil {
		t.Fatal(err)
	}
	if got := StoreDir("-"); got != paths.DefaultConfigDir() {
		t.Fatalf("default dir=%q", got)
	}
}
