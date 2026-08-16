// Design: docs/architecture/appliance/device-config.md -- auto-revert tests

//go:build ze_core

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/pkg/zefs"
)

func setupHealthRevertTest(t *testing.T) storage.Storage {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "database.zefs")
	store, err := storage.NewBlob(dbPath, dir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck // test

	seed := []byte("set environment log level info\n")
	if err := store.WriteFile(zefs.KeyFileTemplate.Key("ze.conf"), seed, 0); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), seed, 0); err != nil {
		t.Fatalf("write active: %v", err)
	}
	return store
}

func TestAutoRevertOnRuntimeFailure(t *testing.T) {
	store := setupHealthRevertTest(t)

	newConfig := []byte("set environment log level debug\n")
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), newConfig, 0); err != nil {
		t.Fatalf("write new config: %v", err)
	}

	prevConfig := []byte("set environment log level info\n")

	hr := newHealthRevert(store, "ze.conf")
	hr.Start(prevConfig)

	// Start persists the pre-change snapshot into the shared zefs store.
	savedPrev, err := store.ReadFile(zefs.KeyConfigPreviousActive.Pattern)
	if err != nil {
		t.Fatalf("read previous config from store: %v", err)
	}
	if !bytes.Equal(savedPrev, prevConfig) {
		t.Errorf("stored previous config = %q, want %q", savedPrev, prevConfig)
	}

	hr.OnPeerClosed(nil, "connection reset")

	hr.Wait()

	if !hr.Reverted() {
		t.Fatal("expected revert after peer close")
	}

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active: %v", err)
	}
	if !bytes.Equal(got, prevConfig) {
		t.Errorf("active config = %q, want previous %q", got, prevConfig)
	}
}

func TestHealthCheckPassesWithoutFlap(t *testing.T) {
	store := setupHealthRevertTest(t)

	newConfig := []byte("set environment log level debug\n")
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), newConfig, 0); err != nil {
		t.Fatalf("write new config: %v", err)
	}

	hr := newHealthRevert(store, "ze.conf")
	hr.Start([]byte("set environment log level info\n"))

	hr.timer.Reset(10 * time.Millisecond)

	// wait for onHealthy to close done rather than racing a fixed 2s
	// wall-clock bound, which flaked under full-verify contention. A genuine hang
	// is caught by the go test framework timeout. Matches TestRevertFallsBackToSeedConfig.
	hr.Wait()

	if hr.Reverted() {
		t.Fatal("should not revert when no flap occurs")
	}

	h := sha256.Sum256(newConfig)
	want := fmt.Sprintf("sha256:%x", h)
	// The last-known-good hash is persisted to the shared zefs store.
	got, err := store.ReadFile(zefs.KeyConfigLastGoodPushed.Pattern)
	if err != nil {
		t.Fatalf("read last-known-good-pushed from store: %v", err)
	}
	if string(got) != want {
		t.Errorf("LKG hash = %q, want %q", got, want)
	}
}

func TestRevertFallsBackToSeedConfig(t *testing.T) {
	store := setupHealthRevertTest(t)

	newConfig := []byte("set environment log level debug\n")
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), newConfig, 0); err != nil {
		t.Fatalf("write new config: %v", err)
	}

	hr := newHealthRevert(store, "ze.conf")
	hr.Start([]byte("set environment log level info\n"))

	// Drop the previous-config snapshot so revert must fall back to the seed
	// template rather than the stored previous config.
	if err := store.Remove(zefs.KeyConfigPreviousActive.Pattern); err != nil {
		t.Fatalf("remove previous config: %v", err)
	}

	hr.OnPeerClosed(nil, "connection reset")

	hr.Wait()

	if !hr.Reverted() {
		t.Fatal("expected revert")
	}

	got, err := store.ReadFile(zefs.KeyFileActive.Key("ze.conf"))
	if err != nil {
		t.Fatalf("read active: %v", err)
	}
	seed, _ := store.ReadFile(zefs.KeyFileTemplate.Key("ze.conf"))
	if !bytes.Equal(got, seed) {
		t.Errorf("active config = %q, want seed %q", got, seed)
	}
}
