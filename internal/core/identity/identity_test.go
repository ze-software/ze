// Design: docs/architecture/core-design.md -- tests for machine identity resolution

package identity

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

type memStore struct {
	data map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{data: make(map[string][]byte)}
}

func (m *memStore) ReadFile(name string) ([]byte, error) {
	d, ok := m.data[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return d, nil
}

func (m *memStore) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.data[name] = append([]byte(nil), data...)
	return nil
}

func TestResolveFromZefs(t *testing.T) {
	store := newMemStore()
	store.data["meta/instance/machine-id"] = []byte("abc123\n")

	orig := readFile
	readFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	defer func() { readFile = orig }()

	got := Resolve(store)
	if got != "abc123" {
		t.Errorf("Resolve() = %q, want %q", got, "abc123")
	}
}

func TestResolveSeedsFromFilesystem(t *testing.T) {
	store := newMemStore()

	orig := readFile
	readFile = func(path string) ([]byte, error) {
		if path == "/etc/machine-id" {
			return []byte("fs-id-42\n"), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFile = orig }()

	got := Resolve(store)
	if got != "fs-id-42" {
		t.Errorf("Resolve() = %q, want %q", got, "fs-id-42")
	}

	persisted, err := store.ReadFile("meta/instance/machine-id")
	if err != nil {
		t.Fatal("expected machine-id to be persisted in store")
	}
	if string(persisted) != "fs-id-42" {
		t.Errorf("persisted = %q, want %q", string(persisted), "fs-id-42")
	}
}

func TestResolveNilStore(t *testing.T) {
	orig := readFile
	readFile = func(path string) ([]byte, error) {
		if path == "/etc/machine-id" {
			return []byte("fallback-id\n"), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFile = orig }()

	got := Resolve(nil)
	if got != "fallback-id" {
		t.Errorf("Resolve(nil) = %q, want %q", got, "fallback-id")
	}
}

func TestResolveHostnameFallback(t *testing.T) {
	store := newMemStore()

	orig := readFile
	readFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	defer func() { readFile = orig }()

	got := Resolve(store)
	if got == "" || got == "unknown" {
		t.Errorf("Resolve() should fall back to hostname, got %q", got)
	}

	persisted, err := store.ReadFile("meta/instance/machine-id")
	if err != nil {
		t.Fatal("expected identity to be persisted in store")
	}
	if string(persisted) != got {
		t.Errorf("persisted = %q, want %q", string(persisted), got)
	}
}

func TestResolvePersistFailureDoesNotPanic(t *testing.T) {
	store := &failWriteStore{}

	orig := readFile
	readFile = func(path string) ([]byte, error) {
		if path == "/etc/machine-id" {
			return []byte("persist-fail-id\n"), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFile = orig }()

	got := Resolve(store)
	if got != "persist-fail-id" {
		t.Errorf("Resolve() = %q, want %q", got, "persist-fail-id")
	}
}

type failWriteStore struct{}

func (f *failWriteStore) ReadFile(string) ([]byte, error) { return nil, os.ErrNotExist }
func (f *failWriteStore) WriteFile(string, []byte, fs.FileMode) error {
	return errors.New("disk full")
}

func TestResolveZefsPreferredOverFilesystem(t *testing.T) {
	store := newMemStore()
	store.data["meta/instance/machine-id"] = []byte("zefs-id")

	orig := readFile
	readFile = func(path string) ([]byte, error) {
		if path == "/etc/machine-id" {
			return []byte("fs-id"), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFile = orig }()

	got := Resolve(store)
	if got != "zefs-id" {
		t.Errorf("Resolve() = %q, want zefs-id (should prefer zefs over filesystem)", got)
	}
}
