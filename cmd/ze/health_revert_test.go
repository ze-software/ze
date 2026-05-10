// Design: plan/spec-appliance-4-device-config.md — auto-revert tests

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
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
	var savedPrev []byte
	writeConfigPrevious = func(data []byte) error {
		savedPrev = make([]byte, len(data))
		copy(savedPrev, data)
		return nil
	}
	readConfigPrevious = func() ([]byte, error) {
		if savedPrev == nil {
			return nil, fmt.Errorf("no previous config")
		}
		return savedPrev, nil
	}
	t.Cleanup(func() {
		writeConfigPrevious = defaultWriteConfigPrevious
		readConfigPrevious = defaultReadConfigPrevious
	})

	hr := NewHealthRevert(store, "ze.conf")
	hr.Start(prevConfig)

	hr.OnPeerClosed("connection reset")

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

	var lkgHash string
	writeLKGPushed = func(hash string) error {
		lkgHash = hash
		return nil
	}
	writeConfigPrevious = func(_ []byte) error { return nil }
	t.Cleanup(func() {
		writeLKGPushed = defaultWriteLKGPushed
		writeConfigPrevious = defaultWriteConfigPrevious
	})

	hr := NewHealthRevert(store, "ze.conf")
	hr.Start([]byte("set environment log level info\n"))

	hr.timer.Reset(10 * time.Millisecond)

	select {
	case <-hr.done:
	case <-time.After(2 * time.Second):
		t.Fatal("health check did not complete")
	}

	if hr.Reverted() {
		t.Fatal("should not revert when no flap occurs")
	}

	h := sha256.Sum256(newConfig)
	want := fmt.Sprintf("sha256:%x", h)
	if lkgHash != want {
		t.Errorf("LKG hash = %q, want %q", lkgHash, want)
	}
}

func TestRevertFallsBackToSeedConfig(t *testing.T) {
	store := setupHealthRevertTest(t)

	newConfig := []byte("set environment log level debug\n")
	if err := store.WriteFile(zefs.KeyFileActive.Key("ze.conf"), newConfig, 0); err != nil {
		t.Fatalf("write new config: %v", err)
	}

	readConfigPrevious = func() ([]byte, error) {
		return nil, fmt.Errorf("no previous config")
	}
	writeConfigPrevious = func(_ []byte) error { return nil }
	t.Cleanup(func() {
		readConfigPrevious = defaultReadConfigPrevious
		writeConfigPrevious = defaultWriteConfigPrevious
	})

	hr := NewHealthRevert(store, "ze.conf")
	hr.Start([]byte("set environment log level info\n"))

	hr.OnPeerClosed("connection reset")

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
