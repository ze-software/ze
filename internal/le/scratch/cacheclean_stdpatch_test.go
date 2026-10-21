package scratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
)

// VALIDATES: no cache-clean target lies within, or holds, a directory the
// patched GOROOT trees live in (D2 design step 4b).
// PREVENTS: a clean emptying the GOROOT another session's build compiles in,
// which fails that build with "no such file" errors that never name the cause.
func TestCleanTargetsNeverReachThePatchedTrees(t *testing.T) {
	root := t.TempDir()
	perUser := filepath.Join(t.TempDir(), "per-user")
	for _, target := range cleanTargets(root, filepath.Join(t.TempDir(), "ambient"), perUser) {
		for _, dir := range gen.CacheDirs(root) {
			if pathWithin(target.path, dir) {
				t.Errorf("the %s cache %s lies within the patched GOROOT directory %s", target.name, target.path, dir)
			}
			if pathWithin(dir, target.path) {
				t.Errorf("the %s cache %s holds the patched GOROOT directory %s", target.name, target.path, dir)
			}
		}
	}
}

// VALIDATES: each patched GOROOT key, under cache/stdpatch or the tmp/stdpatch
// fallback, gets one KEEP row with its size, and stays on disk.
// PREVENTS: a kept tree the owner cannot see, so a key whose toolchain is gone
// is never removed.
func TestCleanReportKeepsAndSizesEachPatchedTree(t *testing.T) {
	root := t.TempDir()
	payload := strings.Repeat("x", 3<<20)
	var keys []string
	for index, dir := range gen.CacheDirs(root) {
		key := filepath.Join(dir, "key"+string(rune('a'+index)))
		source := filepath.Join(key, "goroot", "src", "crypto", "tls")
		if err := os.MkdirAll(source, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "zz_ze_server_session_id.go"), []byte(payload), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("/nonexistent/real/goroot/bin", filepath.Join(key, "goroot", "bin")); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}

	kept := patchedTrees(root, time.Now())
	if len(kept) != len(keys) {
		t.Fatalf("kept = %+v, want one row for each of %v", kept, keys)
	}
	for index, tree := range kept {
		if tree.Path != keys[index] {
			t.Errorf("row %d names %s, want %s", index, tree.Path, keys[index])
		}
		if tree.Error != "" {
			t.Errorf("row %d failed to measure: %s", index, tree.Error)
		}
		// The payload counts, and the dangling symlink costs only its own size.
		if tree.Bytes < int64(len(payload)) || tree.Bytes > int64(len(payload))+1<<16 {
			t.Errorf("row %d measures %d bytes, want about %d", index, tree.Bytes, len(payload))
		}
		if _, err := os.Stat(filepath.Join(keys[index], "goroot", "src")); err != nil {
			t.Errorf("the kept tree %s is gone: %v", keys[index], err)
		}
	}

	report := CleanReport{Trees: kept}
	text := report.Text()
	for _, key := range keys {
		if !strings.Contains(text, "KEEP     "+key) {
			t.Errorf("text = %q, want a KEEP row naming %s", text, key)
		}
	}
	if !strings.Contains(text, "3.0M") {
		t.Errorf("text = %q, want each key's 3.0M", text)
	}
	if code := report.verdict(); code != 0 {
		t.Errorf("verdict = %d, want 0: a kept tree is not a refusal", code)
	}
}

// VALIDATES: a checkout with no tree directory reports no kept row.
// PREVENTS: a missing cache/stdpatch read as an error.
func TestCleanReportKeepsNothingWithoutATree(t *testing.T) {
	if kept := patchedTrees(t.TempDir(), time.Now()); len(kept) != 0 {
		t.Errorf("kept = %+v, want none", kept)
	}
}

// patchedKey writes a tree at dir/name whose VERSION and src/go.mod are links,
// each naming a real file when its live flag is set and nothing otherwise.
func patchedKey(t *testing.T, dir, name string, versionLive, goModLive bool) string {
	t.Helper()
	real := t.TempDir()
	key := filepath.Join(dir, name)
	src := filepath.Join(key, "goroot", "src")
	if err := os.MkdirAll(src, 0o750); err != nil {
		t.Fatal(err)
	}
	for link, live := range map[string]bool{
		filepath.Join(key, "goroot", "VERSION"): versionLive,
		filepath.Join(src, "go.mod"):            goModLive,
	} {
		target := filepath.Join(real, "gone", filepath.Base(link))
		if live {
			target = filepath.Join(real, filepath.Base(link))
			if err := os.WriteFile(target, []byte("go1.27.2"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	return key
}

// VALIDATES: a tree whose VERSION or src/go.mod link dangles is dropped with a
// DROP row, a live tree is kept, a staging directory older than stagingIdleAge
// is dropped, and a young one, which a build may be writing, is kept.
// PREVENTS: the 2026-10-21 staticcheck reds. Homebrew removed go 1.27.2, the
// tree's src/go.mod link dangled, goroot/src stopped being a module, and
// `go list ./...` walked into it, while cache-clean printed KEEP for it
// (plan/journal/gate-verdict-depends-on-the-machine.md). Killed builds left
// tmp-* staging directories nobody reaped.
func TestPatchedTreesDropTheDeadAndKeepTheLive(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	dir := gen.CacheDirs(root)[0]
	live := patchedKey(t, dir, "live", true, true)
	deadVersion := patchedKey(t, dir, "dead-version", false, true)
	deadGoMod := patchedKey(t, dir, "dead-gomod", true, false)
	oldStaging := filepath.Join(dir, gen.StagingPrefix+"1")
	youngStaging := filepath.Join(dir, gen.StagingPrefix+"2")
	for path, age := range map[string]time.Duration{oldStaging: stagingIdleAge + time.Hour, youngStaging: time.Minute} {
		if err := os.MkdirAll(filepath.Join(path, "goroot"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}

	trees := patchedTrees(root, now)

	for _, kept := range []string{live, youngStaging} {
		if _, err := os.Lstat(kept); err != nil {
			t.Errorf("%s, which a build may use, was removed: %v", kept, err)
		}
	}
	for _, dropped := range []string{deadVersion, deadGoMod, oldStaging} {
		if _, err := os.Lstat(dropped); !os.IsNotExist(err) {
			t.Errorf("%s, which no build can use, survived: %v", dropped, err)
		}
	}
	report := CleanReport{Trees: trees}
	text := report.Text()
	for _, want := range []string{"KEEP     " + live, "DROP     " + deadVersion, "DROP     " + deadGoMod, "DROP     " + oldStaging, "KEEP     " + youngStaging} {
		if !strings.Contains(text, want) {
			t.Errorf("text = %q, want a row %q", text, want)
		}
	}
	if code := report.verdict(); code != 0 {
		t.Errorf("verdict = %d, want 0", code)
	}
}
