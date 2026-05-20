package config

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config/storage"
)

func TestScanSchemaStamp(t *testing.T) {
	raw := []byte("# ze-schema: 3\nset bgp router-id 1.2.3.4\n")
	if got := ScanSchemaStamp(raw); got != 3 {
		t.Errorf("ScanSchemaStamp = %d, want 3", got)
	}
}

func TestScanSchemaStampMissing(t *testing.T) {
	raw := []byte("set bgp router-id 1.2.3.4\n")
	if got := ScanSchemaStamp(raw); got != 0 {
		t.Errorf("ScanSchemaStamp = %d, want 0", got)
	}
}

func TestScanSchemaStampEmpty(t *testing.T) {
	if got := ScanSchemaStamp(nil); got != 0 {
		t.Errorf("ScanSchemaStamp(nil) = %d, want 0", got)
	}
	if got := ScanSchemaStamp([]byte{}); got != 0 {
		t.Errorf("ScanSchemaStamp([]) = %d, want 0", got)
	}
}

func TestScanSchemaStampInvalid(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"non-numeric", "# ze-schema: abc\nset bgp\n"},
		{"negative", "# ze-schema: -1\nset bgp\n"},
		{"empty value", "# ze-schema: \nset bgp\n"},
		{"wrong prefix", "# ze-stamp: 3\nset bgp\n"},
		{"comment only", "# this is a comment\nset bgp\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScanSchemaStamp([]byte(tc.input)); got != 0 {
				t.Errorf("ScanSchemaStamp = %d, want 0", got)
			}
		})
	}
}

func TestFormatSchemaStamp(t *testing.T) {
	got := FormatSchemaStamp(1)
	want := "# ze-schema: 1\n"
	if got != want {
		t.Errorf("FormatSchemaStamp(1) = %q, want %q", got, want)
	}
}

func TestScanSchemaStampOnly(t *testing.T) {
	raw := []byte("# ze-schema: 1\n")
	if got := ScanSchemaStamp(raw); got != 1 {
		t.Errorf("ScanSchemaStamp = %d, want 1", got)
	}
}

func TestSchemaStampRoundTrip(t *testing.T) {
	for _, v := range []int{0, 1, 5, 42, 100} {
		stamp := FormatSchemaStamp(v)
		got := ScanSchemaStamp([]byte(stamp))
		if got != v {
			t.Errorf("round-trip(%d): got %d", v, got)
		}
	}
}

func TestScanSchemaStampNoNewline(t *testing.T) {
	raw := []byte("# ze-schema: 7")
	if got := ScanSchemaStamp(raw); got != 7 {
		t.Errorf("ScanSchemaStamp = %d, want 7", got)
	}
}

func TestRecoverConfigFindsCompatibleRollback(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.conf")

	// Write a "future" config that this binary can't parse (stamp > SchemaStamp).
	futureStamp := SchemaStamp + 1
	futureConfig := FormatSchemaStamp(futureStamp) + "set unknown-future-leaf value\n"
	err := os.WriteFile(configPath, []byte(futureConfig), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Create a rollback version with a compatible config (stamp = SchemaStamp).
	store := storage.NewFilesystem()
	rollbackDir := filepath.Join(dir, "rollback")
	if mkErr := os.MkdirAll(rollbackDir, 0o700); mkErr != nil {
		t.Fatal(mkErr)
	}
	compatibleConfig := FormatSchemaStamp(SchemaStamp) + "set bgp router-id 1.2.3.4\nset bgp session asn local 65000\n"
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

	// Verify the recovered config was written back to config.conf.
	written, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	writtenStamp := ScanSchemaStamp(written)
	if writtenStamp != SchemaStamp {
		t.Errorf("written config stamp = %d, want %d", writtenStamp, SchemaStamp)
	}

	// Verify the future-version config was backed up before overwrite.
	entries, dirErr := os.ReadDir(rollbackDir)
	if dirErr != nil {
		t.Fatal(dirErr)
	}
	foundBackup := false
	for _, e := range entries {
		if e.Name() == rollbackName {
			continue
		}
		data, rErr := os.ReadFile(filepath.Join(rollbackDir, e.Name()))
		if rErr != nil {
			continue
		}
		if ScanSchemaStamp(data) == futureStamp {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Error("future-version config should have been backed up to rollback dir")
	}
}

func TestRecoverConfigSkipsWhenStampCompatible(t *testing.T) {
	currentData := []byte(FormatSchemaStamp(SchemaStamp) + "set bgp router-id 1.2.3.4\n")
	result, ok := RecoverConfig(storage.NewFilesystem(), "/nonexistent", currentData, nil)
	if ok || result != nil {
		t.Error("RecoverConfig should return false when stamp <= SchemaStamp")
	}
}

func TestRecoverConfigNoStamp(t *testing.T) {
	currentData := []byte("set bgp router-id 1.2.3.4\n")
	result, ok := RecoverConfig(storage.NewFilesystem(), "/nonexistent", currentData, nil)
	if ok || result != nil {
		t.Error("RecoverConfig should return false when no stamp (version 0)")
	}
}
