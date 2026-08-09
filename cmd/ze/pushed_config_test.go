// Design: docs/architecture/appliance/device-config.md -- pushed config loading tests

//go:build ze_core

package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

func setupPushedConfigTest(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(dbPath, dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test

	seedConfig := []byte("set environment log level info\n")
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), seedConfig, 0); err != nil {
		t.Fatalf("write seed config: %v", err)
	}
	return store
}

func TestBootWithSeedConfigOnly(t *testing.T) {
	store := setupPushedConfigTest(t)
	seedData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))

	readPushedConfig = func() ([]byte, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { readPushedConfig = defaultReadPushedConfig })

	checkPushedConfig(store, "ze.conf")

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if !bytes.Equal(got, seedData) {
		t.Errorf("active config changed: got %q, want %q", got, seedData)
	}
}

func TestBootWithValidPushedConfig(t *testing.T) {
	store := setupPushedConfigTest(t)

	pushedData := []byte("set environment log level debug\n")
	readPushedConfig = func() ([]byte, error) {
		return pushedData, nil
	}
	t.Cleanup(func() { readPushedConfig = defaultReadPushedConfig })

	checkPushedConfig(store, "ze.conf")

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if !bytes.Equal(got, pushedData) {
		t.Errorf("active config = %q, want pushed %q", got, pushedData)
	}
}

func TestBootWithInvalidPushedConfigFallsBack(t *testing.T) {
	store := setupPushedConfigTest(t)
	seedData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))

	readPushedConfig = func() ([]byte, error) {
		return []byte("this is { not { valid config syntax !!!"), nil
	}
	removed := false
	removePushedConfig = func() error {
		removed = true
		return nil
	}
	t.Cleanup(func() {
		readPushedConfig = defaultReadPushedConfig
		removePushedConfig = defaultRemovePushedConfig
	})

	checkPushedConfig(store, "ze.conf")

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active config: %v", err)
	}
	if !bytes.Equal(got, seedData) {
		t.Errorf("active config should be seed: got %q, want %q", got, seedData)
	}
	if !removed {
		t.Error("invalid pushed config should have been removed")
	}
}

func TestConfigActiveHashWritten(t *testing.T) {
	store := setupPushedConfigTest(t)

	writeConfigActiveHash(store, "ze.conf")

	// The active-config hash is ze's own output state, persisted in the shared
	// zefs store under KeyConfigActiveHash (not a loose file).
	got, err := store.ReadFile(zefs.KeyConfigActiveHash.Pattern)
	if err != nil {
		t.Fatalf("config-active-hash not written to store: %v", err)
	}

	activeData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	h := sha256.Sum256(activeData)
	want := fmt.Sprintf("sha256:%x", h)
	if string(got) != want {
		t.Errorf("hash = %q, want %q", got, want)
	}
}

func TestLastKnownGoodHashVerification(t *testing.T) {
	store := setupPushedConfigTest(t)

	seedConfig := "set environment log level info\n"
	h := sha256.Sum256([]byte(seedConfig))
	lkgHash := fmt.Sprintf("sha256:%x", h)
	if err := store.WriteFile(zefs.KeyConfigLastKnownGood.Pattern, []byte(lkgHash), 0); err != nil {
		t.Fatalf("write LKG: %v", err)
	}

	got, err := store.ReadFile(zefs.KeyConfigLastKnownGood.Pattern)
	if err != nil {
		t.Fatalf("read LKG: %v", err)
	}

	activeData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	activeH := sha256.Sum256(activeData)
	activeHash := fmt.Sprintf("sha256:%x", activeH)

	if string(got) != activeHash {
		t.Errorf("LKG hash %q does not match active config hash %q", got, activeHash)
	}
}

func TestCheckPushedConfigRemoveError(t *testing.T) {
	store := setupPushedConfigTest(t)
	seedData, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))

	readPushedConfig = func() ([]byte, error) {
		return []byte("this is { not { valid config syntax !!!"), nil
	}
	removePushedConfig = func() error {
		return errors.New("permission denied")
	}
	t.Cleanup(func() {
		readPushedConfig = defaultReadPushedConfig
		removePushedConfig = defaultRemovePushedConfig
	})

	checkPushedConfig(store, "ze.conf")

	got, _ := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if !bytes.Equal(got, seedData) {
		t.Errorf("active config should be seed even when remove fails")
	}
}
