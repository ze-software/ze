package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/version"
)

func TestScanStampRelease(t *testing.T) {
	raw := []byte("# ze-schema: 26.05.26\nset bgp router-id 1.2.3.4\n")
	if got := ScanStampRelease(raw); got != "26.05.26" {
		t.Errorf("ScanStampRelease = %q, want %q", got, "26.05.26")
	}
}

func TestScanStampReleaseMissing(t *testing.T) {
	raw := []byte("set bgp router-id 1.2.3.4\n")
	if got := ScanStampRelease(raw); got != "" {
		t.Errorf("ScanStampRelease = %q, want empty", got)
	}
}

func TestScanStampReleaseEmpty(t *testing.T) {
	if got := ScanStampRelease(nil); got != "" {
		t.Errorf("ScanStampRelease(nil) = %q, want empty", got)
	}
	if got := ScanStampRelease([]byte{}); got != "" {
		t.Errorf("ScanStampRelease([]) = %q, want empty", got)
	}
}

func TestScanStampReleaseInvalid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"wrong prefix", "# ze-stamp: 26.05.26\nset bgp\n"},
		{"comment only", "# this is a comment\nset bgp\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScanStampRelease([]byte(tc.input)); got != "" {
				t.Errorf("ScanStampRelease = %q, want empty", got)
			}
		})
	}
}

func TestFormatSchemaStamp(t *testing.T) {
	got := FormatSchemaStamp()
	want := "# ze-schema: " + version.Release() + "\n"
	if got != want {
		t.Errorf("FormatSchemaStamp() = %q, want %q", got, want)
	}
}

func TestScanStampReleaseOldIntegerFormat(t *testing.T) {
	raw := []byte("# ze-schema: 1\nset bgp\n")
	got := ScanStampRelease(raw)
	if got != "1" {
		t.Errorf("ScanStampRelease on old format = %q, want %q", got, "1")
	}
}

func TestScanStampReleaseNoNewline(t *testing.T) {
	raw := []byte("# ze-schema: 26.07.01")
	if got := ScanStampRelease(raw); got != "26.07.01" {
		t.Errorf("ScanStampRelease = %q, want %q", got, "26.07.01")
	}
}

func TestRecoverConfigFindsCompatibleRollback(t *testing.T) {
	version.Stamp("26.05.26", "2026-05-26")
	defer version.Stamp("dev", "unknown")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	futureConfig := "# ze-schema: 99.01.01\nset unknown-future-leaf value\n"
	err := os.WriteFile(configPath, []byte(futureConfig), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	store := storage.NewFilesystem()
	rollbackDir := filepath.Join(dir, "rollback")
	if mkErr := os.MkdirAll(rollbackDir, 0o700); mkErr != nil {
		t.Fatal(mkErr)
	}
	compatibleConfig := "# ze-schema: 26.05.26\nset bgp router-id 1.2.3.4\nset bgp session asn local 65000\n"
	rollbackName := "config-20260520-120000.000.conf"
	if writeErr := os.WriteFile(filepath.Join(rollbackDir, rollbackName), []byte(compatibleConfig), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}

	result, ok := RecoverConfig(store, configPath, []byte(futureConfig), nil)
	if !ok {
		t.Fatal("RecoverConfig should have found compatible rollback")
	}
	if result == nil {
		t.Fatal("RecoverConfig returned nil result")
	}

	written, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	writtenRelease := ScanStampRelease(written)
	if writtenRelease != "26.05.26" {
		t.Errorf("written config release = %q, want %q", writtenRelease, "26.05.26")
	}

	entries, dirErr := os.ReadDir(rollbackDir)
	if dirErr != nil {
		t.Fatal(dirErr)
	}
	foundBackup := false
	for _, e := range entries {
		if e.Name() == rollbackName {
			continue
		}
		backupData, rErr := os.ReadFile(filepath.Join(rollbackDir, e.Name()))
		if rErr != nil {
			continue
		}
		if ScanStampRelease(backupData) == "99.01.01" {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Error("future config should have been backed up to rollback dir")
	}
}

func TestRecoverConfigSkipsWhenStampCompatible(t *testing.T) {
	version.Stamp("26.05.26", "2026-05-26")
	defer version.Stamp("dev", "unknown")

	currentData := []byte("# ze-schema: 26.05.26\nset bgp router-id 1.2.3.4\n")
	result, ok := RecoverConfig(storage.NewFilesystem(), "/nonexistent", currentData, nil)
	if ok || result != nil {
		t.Error("RecoverConfig should return false when stamp <= binary release")
	}
}

func TestRecoverConfigNoStamp(t *testing.T) {
	version.Stamp("26.05.26", "2026-05-26")
	defer version.Stamp("dev", "unknown")

	currentData := []byte("set bgp router-id 1.2.3.4\n")
	result, ok := RecoverConfig(storage.NewFilesystem(), "/nonexistent", currentData, nil)
	if ok || result != nil {
		t.Error("RecoverConfig should return false when no stamp")
	}
}
