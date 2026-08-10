package appliance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/pkg/zefs"
)

func TestAssembleWritesLastKnownGood(t *testing.T) {
	dir := assembleTestAppliance(t, "lkg", nil)
	appDir := filepath.Join(dir, "lkg")
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("set environment log level info\n"), 0o644) //nolint:errcheck,gosec // test

	code := runAssemble([]string{"--keep", "lkg"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	store, err := zefs.Open(databasePath(dir, "lkg"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile(zefs.KeyConfigLastKnownGood.Pattern)
	if err != nil {
		t.Fatalf("last-known-good not written: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("last-known-good is empty")
	}
}

func TestLastKnownGoodHashMatchesSeedConfig(t *testing.T) {
	dir := assembleTestAppliance(t, "lkg-hash", nil)
	appDir := filepath.Join(dir, "lkg-hash")

	seedContent := "set environment log level info\n"
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte(seedContent), 0o644) //nolint:errcheck,gosec // test

	code := runAssemble([]string{"--keep", "lkg-hash"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	store, err := zefs.Open(databasePath(dir, "lkg-hash"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile(zefs.KeyConfigLastKnownGood.Pattern)
	if err != nil {
		t.Fatalf("last-known-good not written: %v", err)
	}

	cfg, _ := LoadConfig(ConfigPath(dir, "lkg-hash"))
	want := configHash(appendListenerOverrides(seedContent, cfg))
	got := string(data)
	if got != want {
		t.Errorf("last-known-good hash = %q, want %q", got, want)
	}
}

func TestBuildWritesLastKnownGood(t *testing.T) {
	dir := assembleTestAppliance(t, "build-lkg", nil)
	appDir := filepath.Join(dir, "build-lkg")
	os.WriteFile(filepath.Join(appDir, "ze.conf"), []byte("set environment log level debug\n"), 0o644) //nolint:errcheck,gosec // test

	code := runAssemble([]string{"--keep", "build-lkg"})
	if code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}

	store, err := zefs.Open(databasePath(dir, "build-lkg"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close() //nolint:errcheck // test

	data, err := store.ReadFile(zefs.KeyConfigLastKnownGood.Pattern)
	if err != nil {
		t.Fatalf("last-known-good missing after build: %v", err)
	}

	cfg, _ := LoadConfig(ConfigPath(dir, "build-lkg"))
	want := configHash(appendListenerOverrides("set environment log level debug\n", cfg))
	if string(data) != want {
		t.Errorf("hash = %q, want %q", string(data), want)
	}
}
