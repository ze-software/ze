// VALIDATES: AC-6 (auto-init fallback), AC-8 (no fallback off-gokrazy), AC-9 (read-only diagnostic)
// PREVENTS: bricked gokrazy appliance when /perm/ze missing

//go:build ze_core

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
)

func setAutoInitConfigDir(t *testing.T, dir string) {
	t.Helper()
	if err := env.Set("ze.config.dir", dir); err != nil {
		t.Fatalf("env.Set ze.config.dir: %v", err)
	}
	t.Cleanup(func() {
		_ = env.Set("ze.config.dir", "")
	})
}

func TestGokrazyAutoInitCreatesDB(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "ze")

	setAutoInitConfigDir(t, configDir)

	store, err := gokrazyAutoInit()
	if err != nil {
		t.Fatalf("gokrazyAutoInit: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	if !storage.IsBlobStorage(store) {
		t.Fatal("expected blob storage after auto-init")
	}

	dbPath := filepath.Join(configDir, "database.zefs")
	if _, statErr := os.Stat(dbPath); statErr != nil {
		t.Fatalf("database.zefs should exist at %s: %v", dbPath, statErr)
	}
}

func TestGokrazyAutoInitReadOnlyFails(t *testing.T) {
	// skip as root rather than assert a condition root cannot be in.
	// The test provokes the failure with a permission BIT (0555), but root holds
	// CAP_DAC_OVERRIDE and MkdirAll succeeds regardless, so gokrazyAutoInit
	// correctly returns nil and the assertion is simply unprovokable here.
	//
	// This is not the production condition either way. gokrazyAutoInit's only
	// barrier is os.MkdirAll (ze_core_autoinit.go:30-34) and its own message names
	// the real cause -- "read-only /perm? check ext4 mountability" -- which is
	// EROFS from a read-only MOUNT. Root does NOT bypass EROFS, so the appliance
	// behavior this guards is intact; only the simulation is uid-sensitive.
	//
	// Coverage is preserved where it is meaningful: every non-root run (the dev
	// host, CI) still executes the assertion. It went unnoticed until the QEMU
	// unit phase, which runs as root, was repaired and executed for the first time.
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the 0555 permission bit (CAP_DAC_OVERRIDE); the read-only /perm this guards is EROFS, which root does not bypass")
	}

	dir := t.TempDir()
	readonlyDir := filepath.Join(dir, "readonly")
	configDir := filepath.Join(readonlyDir, "ze")

	if err := os.MkdirAll(readonlyDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(readonlyDir, 0o755) }) //nolint:errcheck // cleanup

	setAutoInitConfigDir(t, configDir)

	_, err := gokrazyAutoInit()
	if err == nil {
		t.Fatal("gokrazyAutoInit should fail on read-only parent directory")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("error should mention read-only, got: %v", err)
	}
}
