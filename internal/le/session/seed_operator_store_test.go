// Design: docs/contributing/running-commands.md -- session seed entry confinement.
// Related: seed_entry_test.go -- the same refusals through the outer guard
package session

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/job"
)

// TestSeederEntryCannotReachOperatorStore drives the seeder entry beneath its
// outer guard (validateSeedEntry is never called) with a recording child that
// never executes anything. The operator's own bin/ze and etc/ze sit beside the
// session tree, as they do in the checkout (journal
// mutating-a-guard-runs-the-unguarded-path, 2026-08-10). Every unsafe shape
// must be refused by the retained-root entry alone, start no child, and leave
// the operator store, a foreign session and every symlink target byte- and
// mode-identical. The owned session is the positive control: both init
// children run, each pinned to the session's own executable and config
// directory, and only that session's store appears.
//
// MUTATION: seed_root.go openSeedResources `if strings.TrimSpace(string(owner)) != id {`
// -> `if false && strings.TrimSpace(string(owner)) != id {` reddens "foreign owner".
// MUTATION: seed.go seedStoreWithOps `configOverride(ops.environ)` -> `configOverride(nil)`
// reddens the three "override" cases.
func TestSeederEntryCannotReachOperatorStore(t *testing.T) {
	for _, scenario := range []struct {
		name string
		want string
	}{
		{"owned", ""},
		{"foreign owner", "session owner changed"},
		{"session symlink", "is not a directory or is a symlink"},
		{"config symlink to operator store", "is not a directory or is a symlink"},
		{"binary symlink to operator binary", "cannot open session executable"},
		{"override uppercase", "ZE_CONFIG_DIR overrides the config directory"},
		{"override dotted", "ze.config.dir overrides the config directory"},
		{"override mixed", "Ze.Config_Dir overrides the config directory"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_SESSION_ID", "seed-operator-owner")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			root := t.TempDir()
			outside := t.TempDir()
			operatorConfig := filepath.Join(root, "etc", "ze")
			operatorBinary := filepath.Join(root, "bin", "ze")
			seedOperatorFile(t, filepath.Join(operatorConfig, "database", "sentinel"), "operator store\n", 0o640)
			seedOperatorFile(t, operatorBinary, "#!/bin/sh\nexit 99\n", 0o750)
			seedOperatorFile(t, filepath.Join(outside, "sentinel"), "outside\n", 0o640)

			rel := filepath.Join("tmp", "session", "2026-11-10-120000")
			session := filepath.Join(root, rel)
			binary := filepath.Join(rel, "bin", "ze")
			owner := "seed-operator-owner\n"
			if scenario.name == "foreign owner" {
				owner = "another-session\n"
			}
			seedOperatorFile(t, filepath.Join(session, ".session-id"), owner, 0o600)
			if scenario.name == "binary symlink to operator binary" {
				if err := os.MkdirAll(filepath.Join(session, "bin"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(operatorBinary, filepath.Join(root, binary)); err != nil {
					t.Fatal(err)
				}
			} else {
				seedOperatorFile(t, filepath.Join(root, binary), "#!/bin/sh\nexit 98\n", 0o700)
			}
			if scenario.name == "config symlink to operator store" {
				if err := os.MkdirAll(filepath.Join(session, "etc"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(operatorConfig, filepath.Join(session, "etc", "ze")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario.name == "session symlink" {
				moved := filepath.Join(outside, "session")
				if err := os.Rename(session, moved); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(moved, session); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario.name {
			case "override uppercase":
				t.Setenv("ZE_CONFIG_DIR", operatorConfig)
			case "override dotted":
				t.Setenv("ze.config.dir", operatorConfig)
			case "override mixed":
				t.Setenv("Ze.Config_Dir", operatorConfig)
			}

			protected := []string{filepath.Join(root, "etc"), filepath.Join(root, "bin"), outside}
			if scenario.name == "foreign owner" {
				protected = append(protected, session)
			}
			before := snapshotSeedTrees(t, protected)

			calls := 0
			run := func(argv []string, processIO job.ProcessIO) (int, error) {
				calls++
				if scenario.want != "" {
					t.Errorf("refused scenario started ze child %q", argv)
					return 1, nil
				}
				return recordSeedChild(t, root, session, operatorBinary, operatorConfig, argv, processIO)
			}
			report, code, err := seedRetainedEntry(root, binary, streams{Out: io.Discard, Err: io.Discard}, nil, run)

			if after := snapshotSeedTrees(t, protected); !equalSeedTrees(before, after) {
				t.Fatalf("seeder entry without its outer guard changed protected state:\nbefore %v\nafter  %v", before, after)
			}
			if scenario.want == "" {
				if err != nil {
					t.Fatalf("owned session refused: code=%d err=%v", code, err)
				}
				if code != 0 {
					t.Fatalf("owned session exit=%d", code)
				}
				if !report.Seeded {
					t.Fatalf("owned session not seeded: %+v", report)
				}
				if calls != 2 {
					t.Fatalf("owned session ran %d init children, want 2", calls)
				}
				info, statErr := os.Lstat(filepath.Join(session, "etc", "ze", "database"))
				if statErr != nil {
					t.Fatalf("owned store not created: %v", statErr)
				}
				if !info.IsDir() {
					t.Fatalf("owned store is not a directory: %v", info.Mode())
				}
				return
			}
			if err == nil {
				t.Fatalf("seeder entry accepted %s without its outer guard: code=%d calls=%d report=%+v", scenario.name, code, calls, report)
			}
			if code == 0 {
				t.Fatalf("refusal returned success: %v", err)
			}
			if calls != 0 {
				t.Fatalf("refusal started %d ze children: %v", calls, err)
			}
			if !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("refusal %q does not name %q", err, scenario.want)
			}
			if report.Seeded {
				t.Fatalf("refusal reported a seeded store: %+v", report)
			}
		})
	}
}

// recordSeedChild stands in for one ze init child. It proves the child would
// run the session's own executable inside the session's own config directory,
// then writes the artifact that child makes there.
func recordSeedChild(t *testing.T, root, session, operatorBinary, operatorConfig string, argv []string, processIO job.ProcessIO) (int, error) {
	t.Helper()
	if processIO.Executable == nil {
		t.Errorf("child %q has no held executable", argv)
		return 1, nil
	}
	if processIO.SeedDirectory == nil {
		t.Errorf("child %q has no held config directory", argv)
		return 1, nil
	}
	held, err := processIO.Executable.Stat()
	if err != nil {
		return 1, err
	}
	directory, err := processIO.SeedDirectory.Stat()
	if err != nil {
		return 1, err
	}
	sessionBinary, err := os.Lstat(filepath.Join(session, "bin", "ze"))
	if err != nil {
		return 1, err
	}
	sessionConfig := filepath.Join(session, "etc", "ze")
	ownConfig, err := os.Lstat(sessionConfig)
	if err != nil {
		return 1, err
	}
	operatorExecutable, err := os.Lstat(operatorBinary)
	if err != nil {
		return 1, err
	}
	operatorStore, err := os.Lstat(operatorConfig)
	if err != nil {
		return 1, err
	}
	if !os.SameFile(held, sessionBinary) {
		t.Errorf("child %q holds an executable other than the session's own", argv)
	}
	if os.SameFile(held, operatorExecutable) {
		t.Errorf("child %q holds the operator's bin/ze", argv)
	}
	if !os.SameFile(directory, ownConfig) {
		t.Errorf("child %q seeds a directory other than the session's own etc/ze", argv)
	}
	if os.SameFile(directory, operatorStore) {
		t.Errorf("child %q seeds the operator's etc/ze", argv)
	}
	if processIO.Dir != root {
		t.Errorf("child %q runs in %s, want the checkout %s", argv, processIO.Dir, root)
	}
	blob := filepath.Join(sessionConfig, "database.zefs")
	if strings.Join(argv[1:], " ") == "init --seed" {
		return 0, os.WriteFile(blob, []byte("seed blob\n"), 0o600)
	}
	if err := os.Mkdir(filepath.Join(sessionConfig, "database"), 0o700); err != nil {
		return 1, err
	}
	return 0, os.Remove(blob)
}

func seedOperatorFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// snapshotSeedTrees records every entry below each tree, without following a
// link: its type and permission bits, and a regular file's bytes or a link's
// target.
func snapshotSeedTrees(t *testing.T, trees []string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	for _, tree := range trees {
		err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			state := info.Mode().String()
			if info.Mode().IsRegular() {
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				state += " " + string(content)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				state += " -> " + target
			}
			snapshot[path] = state
			return nil
		})
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	return snapshot
}

func equalSeedTrees(before, after map[string]string) bool {
	if len(before) != len(after) {
		return false
	}
	for path, state := range before {
		if after[path] != state {
			return false
		}
	}
	return true
}
