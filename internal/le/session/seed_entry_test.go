// Design: docs/contributing/running-commands.md -- session seed entry confinement.
package session

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestSeedEntryConfinement calls the same producer as the native seed-store
// action, without its outer command guards. Every outside target is test-owned.
func TestSeedEntryConfinement(t *testing.T) {
	for _, scenario := range []string{"foreign owner", "session symlink", "config symlink", "binary symlink", "override existing", "override uppercase", "override dotted empty", "password symlink", "owner symlink", "owned existing"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SESSION_ID", "seed-entry-owner")
			root := t.TempDir()
			rel := filepath.Join("tmp", "session", "2026-11-03-120000")
			dir := filepath.Join(root, rel)
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "operator-sentinel")
			if err := os.WriteFile(sentinel, []byte("operator state\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
				t.Fatal(err)
			}
			owner := "seed-entry-owner\n"
			if scenario == "foreign owner" {
				owner = "another-session\n"
			}
			if err := os.WriteFile(filepath.Join(dir, ".session-id"), []byte(owner), 0o600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, "etc", "ze")
			if err := os.MkdirAll(filepath.Join(config, "database"), 0o700); err != nil {
				t.Fatal(err)
			}
			binary := filepath.Join(rel, "bin", "ze")
			if scenario != "binary symlink" {
				if err := os.WriteFile(filepath.Join(root, binary), []byte("#!/bin/sh\nexit 99\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "session symlink" {
				moved := filepath.Join(outside, "session")
				if err := os.Rename(dir, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, dir); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "config symlink" {
				moved := filepath.Join(outside, "config")
				if err := os.Rename(config, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, config); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "binary symlink" {
				if err := os.Symlink(sentinel, filepath.Join(root, binary)); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "password symlink" {
				if err := os.Symlink(sentinel, filepath.Join(config, ".dev-password")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "owner symlink" {
				marker := filepath.Join(dir, ".session-id")
				moved := filepath.Join(outside, "owner")
				if err := os.Rename(marker, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, marker); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "override existing" {
				t.Setenv("Ze.Config_Dir", outside)
			}
			if scenario == "override uppercase" {
				t.Setenv("ZE_CONFIG_DIR", outside)
			}
			if scenario == "override dotted empty" {
				t.Setenv("ze.config.dir", "")
			}
			report, code, err := seedStore(root, binary, streams{Out: io.Discard, Err: io.Discard})
			// Exercise the registered native action too, without a shell hook.
			t.Setenv("ze.repo.root", root)
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			_, actionCode := Answer([]string{"seed-store", "binary", filepath.ToSlash(binary)})
			if actionCode != code {
				t.Fatalf("native action exit=%d, seed producer exit=%d", actionCode, code)
			}
			content, readErr := os.ReadFile(sentinel)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(content) != "operator state\n" {
				t.Fatalf("operator sentinel changed: %q", content)
			}
			info, statErr := os.Stat(sentinel)
			if statErr != nil {
				t.Fatal(statErr)
			}
			if info.Mode().Perm() != 0o640 {
				t.Fatalf("operator sentinel mode changed: %v", info.Mode())
			}
			if scenario == "owned existing" {
				if err != nil {
					t.Fatal(err)
				}
				if code != 0 {
					t.Fatalf("owned existing store failed: %d", code)
				}
				if !report.Existing {
					t.Fatal("owned existing store not reused")
				}
				return
			}
			if err == nil {
				t.Fatalf("unsafe seed entry accepted: code=%d report=%+v", code, report)
			}
			if code == 0 {
				t.Fatalf("refusal returned success: %v", err)
			}
			want := "symlink"
			if scenario == "foreign owner" {
				want = "owned by another-session"
			}
			if strings.HasPrefix(scenario, "override") {
				want = "overrides the config directory"
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("refusal %q does not name %q", err, want)
			}
		})
	}
}

// TestSeedEntryUnavailableInputs proves an existing store cannot hide an
// unreadable path, unavailable owner, invalid identity, or unusable binary.
// Real filesystem failures reach the entry; no validation operation is mocked.
func TestSeedEntryUnavailableInputs(t *testing.T) {
	for _, scenario := range []struct {
		name string
		want string
	}{
		{"lexical escape", "not a binary in a session bin directory"},
		{"inspection failure", "inspect session seed path"},
		{"invalid identity", "unsafe session id"},
		{"missing owner", "read seed session owner"},
		{"empty owner", "empty session id"},
		{"malformed owner", "malformed session id"},
		{"missing binary", "open session binary"},
		{"directory binary", "not a regular file"},
		{"nonexecutable binary", "not executable"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SESSION_ID", "seed-error-owner")
			root := t.TempDir()
			rel := filepath.Join("tmp", "session", "2026-11-03-140000")
			dir := filepath.Join(root, rel)
			config := filepath.Join(dir, "etc", "ze")
			if err := os.MkdirAll(filepath.Join(config, "database"), 0o700); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(config, "database", "sentinel")
			if err := os.WriteFile(sentinel, []byte("existing store\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			owner := "seed-error-owner\n"
			if scenario.name == "empty owner" {
				owner = ""
			}
			if scenario.name == "malformed owner" {
				owner = "invalid/owner\n"
			}
			if scenario.name != "missing owner" {
				if err := os.WriteFile(filepath.Join(dir, ".session-id"), []byte(owner), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			binDir := filepath.Join(dir, "bin")
			if scenario.name == "inspection failure" {
				// Lstat(bin/ze) must return ENOTDIR, not be treated as absent.
				if err := os.WriteFile(binDir, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(binDir, 0o700); err != nil {
					t.Fatal(err)
				}
				seedErrorBinary(t, filepath.Join(binDir, "ze"), scenario.name)
			}
			binary := filepath.Join(rel, "bin", "ze")
			if scenario.name == "lexical escape" {
				binary = filepath.Join(root, binary)
			}
			if scenario.name == "invalid identity" {
				t.Setenv("CLAUDE_CODE_SESSION_ID", "../foreign")
			}
			report, code, err := seedStore(root, binary, streams{Out: io.Discard, Err: io.Discard})
			if err == nil {
				t.Fatalf("unavailable input reused store: %+v", report)
			}
			if code != 1 {
				t.Fatalf("exit=%d, want refusal 1", code)
			}
			if !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("error=%q, want %q", err, scenario.want)
			}
			if strings.HasPrefix(scenario.name, "missing") {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("missing input lost filesystem cause: %v", err)
				}
			}
			if report != (seedReport{}) {
				t.Fatalf("refusal published a seed report: %+v", report)
			}
			got, err := os.ReadFile(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "existing store\n" {
				t.Fatalf("refusal changed store: %q", got)
			}
			info, err := os.Stat(sentinel)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("refusal changed store mode: %v", info.Mode())
			}
			for _, name := range []string{".dev-password", ".seed-lock", "database.zefs"} {
				if _, err := os.Lstat(filepath.Join(config, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("refusal left seed artifact %s: %v", name, err)
				}
			}
		})
	}
}

func seedErrorBinary(t *testing.T, path, scenario string) {
	t.Helper()
	if scenario == "missing binary" {
		return
	}
	if scenario == "directory binary" {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		return
	}
	mode := os.FileMode(0o700)
	if scenario == "nonexecutable binary" {
		mode = 0o600
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 99\n"), mode); err != nil {
		t.Fatal(err)
	}
}
