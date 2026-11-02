//go:build ze_bgp

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// sweepSourceFiles uses the trackableFiles population from le/feature:
// tracked files plus untracked files Git's ignore rules permit. Git owns the
// artifact boundary, including tracked exceptions and nested negations.
// The runner's draft tree is not a regression corpus, even when tracked.
func sweepSourceFiles(ctx context.Context, root string, keep sweepKeep) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others",
		"--exclude-standard", "--", "*.conf", "*.ci", "*.tape")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	listing, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list config sources in %s: %w: %s", root, err, stderr.String())
	}
	// Git can exit zero after an unreadable directory warning. An incomplete
	// source population must not produce a passing validation verdict.
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("list config sources in %s: %s", root, stderr.String())
	}
	names := strings.Split(string(listing), "\x00")
	slices.Sort(names)
	names = slices.Compact(names)
	var files []string
	for _, name := range names {
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, filepath.ToSlash(runner.SuiteDir("", "", true))+"/") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config source %s: %w", path, err)
		}
		if keep.keeps(path, text) {
			files = append(files, path)
		}
	}
	return files, nil
}

// TestSweepSourceDiscovery exercises Git's tracked and untracked populations
// against ignored artifacts, re-included source directories and read failures.
func TestSweepSourceDiscovery(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write(".gitignore", "/bin/\nbuild/\n!/internal/le/build/\n")
	write("source/ordinary.conf", "ordinary")
	write("source/untracked.conf", "untracked")
	write("source/draft/source.conf", "not the test incubator")
	write("internal/le/build/source.conf", "re-included")
	write("bin/builds/.building/export.conf", "artifact")
	write("bin/tracked.conf", "tracked exception")
	write(filepath.Join(runner.SuiteDir("", "", true), "plugin", "tracked.ci"), "draft demonstration")
	for _, name := range []string{"source/ordinary.conf", "bin/tracked.conf", "test/draft/plugin/tracked.ci"} {
		blob := git("hash-object", "-w", name)
		git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+name)
	}
	if err := os.Symlink("missing-export", filepath.Join(root, "bin", "builds", ".building", "broken.conf")); err != nil {
		t.Fatal(err)
	}
	want := []string{"bin/tracked.conf", "internal/le/build/source.conf", "source/draft/source.conf", "source/ordinary.conf", "source/untracked.conf"}
	for i := range want {
		want[i] = filepath.Join(root, want[i])
	}
	files, err := sweepSourceFiles(t.Context(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, want) {
		t.Errorf("discovered %q, want %q", files, want)
	}
	t.Run("missing_tracked_source", func(t *testing.T) {
		if err := os.Remove(filepath.Join(root, "source", "ordinary.conf")); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { write("source/ordinary.conf", "ordinary") })
		_, err := sweepSourceFiles(t.Context(), root, nil)
		if err == nil {
			t.Fatal("missing tracked source silently reduced the population")
		}
		if !strings.Contains(err.Error(), "ordinary.conf") {
			t.Errorf("failure does not name the missing source: %v", err)
		}
	})
	t.Run("unreadable_directory", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can traverse mode-000 directories")
		}
		write("source/locked/hidden.conf", "unreadable")
		locked := filepath.Join(root, "source", "locked")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(locked, 0o700); err != nil {
				t.Error(err)
			}
		})
		if _, err := sweepSourceFiles(t.Context(), root, nil); err == nil {
			t.Error("unreadable directory silently reduced the source population")
		}
	})
	// A broken source is not an absent source, even for a narrowed sweep.
	if err := os.Symlink("missing-target", filepath.Join(root, "source", "broken.conf")); err != nil {
		t.Fatal(err)
	}
	if _, err := sweepSourceFiles(t.Context(), root, sweepKeep{"never-matches"}); err == nil {
		t.Error("unreadable source was silently excluded")
	}
}

// TestSweepSourceDiscoveryRequiresRepository refuses a missing discovery root.
func TestSweepSourceDiscoveryRequiresRepository(t *testing.T) {
	if _, err := sweepSourceFiles(t.Context(), t.TempDir(), nil); err == nil {
		t.Fatal("discovery without repository metadata succeeded")
	}
}
