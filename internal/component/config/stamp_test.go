package config

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

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

	for _, reject := range []bool{false, true} {
		name := "publish"
		if reject {
			name = "publication failure"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.conf")
			store, err := storage.Create(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close() //nolint:errcheck // test cleanup
			futureConfig := "# ze-schema: 99.01.01\nset unknown-future-leaf value\n"
			if err := store.WriteFile(configPath, []byte(futureConfig), 0o600); err != nil {
				t.Fatal(err)
			}
			compatibleConfig := "# ze-schema: 26.05.26\nset bgp router-id 1.2.3.4\nset bgp session asn local 65000\n"
			if err := store.WriteVersion(configPath, []byte(compatibleConfig), time.Date(2026, 5, 20, 12, 0, 0, 0, time.Local)); err != nil {
				t.Fatal(err)
			}
			result, ok := RecoverConfig(store, configPath, []byte(futureConfig), nil, func(content []byte) error {
				if reject {
					return errors.New("publication refused")
				}
				return store.WriteFile(configPath, content, 0o600)
			})
			if reject {
				if ok || result != nil {
					t.Fatal("failed publication reported successful recovery")
				}
			} else if !ok || result == nil {
				t.Fatal("compatible rollback was not recovered")
			}
			written, readErr := store.ReadFile(configPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			wantRelease := "26.05.26"
			if reject {
				wantRelease = "99.01.01"
			}
			if got := ScanStampRelease(written); got != wantRelease {
				t.Fatalf("published release=%q want %q", got, wantRelease)
			}
			versions, listErr := store.ListVersions(configPath)
			if listErr != nil {
				t.Fatal(listErr)
			}
			foundBackup := false
			for _, entry := range versions {
				backup, readErr := store.ReadFile(entry.Path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if string(backup) == futureConfig {
					foundBackup = true
				}
			}
			if !foundBackup {
				t.Fatal("unsupported config was not preserved in history")
			}
		})
	}
}

func TestRecoverConfigSkipsWhenStampCompatible(t *testing.T) {
	version.Stamp("26.05.26", "2026-05-26")
	defer version.Stamp("dev", "unknown")

	currentData := []byte("# ze-schema: 26.05.26\nset bgp router-id 1.2.3.4\n")
	result, ok := RecoverConfig(nil, "/nonexistent", currentData, nil, nil)
	if ok || result != nil {
		t.Error("RecoverConfig should return false when stamp <= binary release")
	}
}

func TestRecoverConfigNoStamp(t *testing.T) {
	version.Stamp("26.05.26", "2026-05-26")
	defer version.Stamp("dev", "unknown")

	currentData := []byte("set bgp router-id 1.2.3.4\n")
	result, ok := RecoverConfig(nil, "/nonexistent", currentData, nil, nil)
	if ok || result != nil {
		t.Error("RecoverConfig should return false when no stamp")
	}
}
