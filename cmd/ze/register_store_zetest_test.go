//go:build zetest && ze_core

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
)

// VALIDATES: a DUT build given ze.test.storage=memory and no store runs on the
// memory store and writes nothing under the config folder; unset, it creates
// the disk store; an existing disk store is opened, never replaced; an unknown
// value is refused with the values that work.
// PREVENTS: a disk-store test silently losing its disk store, and a typo
// silently falling back to either store.
func TestStoreSelectorMemoryWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	setTestStorage(t, testStorageMemory)
	store, err := openExplicitStore(filepath.Join(dir, "router.conf"))
	if err != nil {
		t.Fatalf("openExplicitStore: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	if _, err := os.Stat(filepath.Join(dir, "database")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("memory selection created a disk tree: %v", err)
	}
}

func TestStoreSelectorDiskWhenUnset(t *testing.T) {
	dir := t.TempDir()
	setTestStorage(t, "")
	store, err := openExplicitStore(filepath.Join(dir, "router.conf"))
	if err != nil {
		t.Fatalf("openExplicitStore: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	if _, err := os.Stat(filepath.Join(dir, "database")); err != nil {
		t.Fatalf("unset selector did not create the disk store: %v", err)
	}
}

func TestStoreSelectorKeepsExistingDiskStore(t *testing.T) {
	dir := t.TempDir()
	seeded, err := storage.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := seeded.WriteKey("meta/instance/name", []byte("seeded")); err != nil {
		t.Fatal(err)
	}
	if err := seeded.Close(); err != nil {
		t.Fatal(err)
	}
	setTestStorage(t, testStorageMemory)
	store, err := openExplicitStore(filepath.Join(dir, "router.conf"))
	if err != nil {
		t.Fatalf("openExplicitStore: %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	got, err := store.ReadKey("meta/instance/name")
	if err != nil || string(got) != "seeded" {
		t.Fatalf("existing store not opened: %q, %v", got, err)
	}
}

func TestStoreSelectorRefusesUnknownValue(t *testing.T) {
	setTestStorage(t, "tmpfs")
	_, err := openExplicitStore(filepath.Join(t.TempDir(), "router.conf"))
	if err == nil || !strings.Contains(err.Error(), "memory") {
		t.Fatalf("unknown value = %v, want a refusal naming memory", err)
	}
}

func setTestStorage(t *testing.T, value string) {
	t.Helper()
	original := env.Get(testStorageKey)
	t.Cleanup(func() { _ = env.Set(testStorageKey, original) })
	if err := env.Set(testStorageKey, value); err != nil {
		t.Fatal(err)
	}
}
