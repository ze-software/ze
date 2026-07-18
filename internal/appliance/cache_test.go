package appliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := resolveCacheDir()
	want := filepath.Join(home, ".cache", cacheSubdir)
	if got != want {
		t.Errorf("resolveCacheDir() = %q, want %q", got, want)
	}
}

func TestResolveCacheDirCustom(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-cache")
	t.Setenv("XDG_CACHE_HOME", custom)
	got := resolveCacheDir()
	want := filepath.Join(custom, cacheSubdir)
	if got != want {
		t.Errorf("resolveCacheDir() = %q, want %q", got, want)
	}
}

func TestCopyTreePreservesSymlinkAndFiles(t *testing.T) {
	// VALIDATES: copyTree preserves regular files, nested dirs, AND symlinks
	// (matching the `cp -R` Make path), rather than silently dropping symlinks.
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "lib", "modules", "7.1.1-ze"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "vmlinuz"), []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "lib", "modules", "7.1.1-ze", "modules.dep"), []byte("dep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("modules.dep", filepath.Join(src, "lib", "modules", "7.1.1-ze", "modules.alias")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "cached")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "vmlinuz")); err != nil || string(data) != "kernel" {
		t.Errorf("vmlinuz not copied: %v %q", err, data)
	}
	if _, err := os.Stat(filepath.Join(dst, "lib", "modules", "7.1.1-ze", "modules.dep")); err != nil {
		t.Errorf("nested file not copied: %v", err)
	}
	linkPath := filepath.Join(dst, "lib", "modules", "7.1.1-ze", "modules.alias")
	info, err := os.Lstat(linkPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink not preserved: %v mode=%v", err, info.Mode())
	}
	if target, err := os.Readlink(linkPath); err != nil || target != "modules.dep" {
		t.Errorf("symlink target = %q (err %v), want modules.dep", target, err)
	}
}

func TestEvictKeepN(t *testing.T) {
	// VALIDATES AC-4 (a namespace is bounded to keep-N most-recent entries; older ones are
	// reclaimed on a key change) and R-1/AC-8 (an entry touched within the grace window is
	// never removed, so eviction cannot race a concurrent materialize/boot).
	ns := t.TempDir()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)

	mk := func(name string, ageMin int) {
		dir := filepath.Join(ns, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-time.Duration(ageMin) * time.Minute)
		if err := os.Chtimes(dir, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	mk("newest2", 2)   // rank 1 -> kept by keep-N
	mk("newest1", 4)   // rank 2 -> kept by keep-N
	mk("fresh_old", 6) // rank 3 (beyond keep-2) BUT within grace -> protected
	mk("stale", 60)    // rank 4, beyond keep-2 and beyond grace -> evicted

	origNow, origGrace := evictNow, evictGrace
	t.Cleanup(func() { evictNow, evictGrace = origNow, origGrace })
	evictNow = func() time.Time { return now }
	evictGrace = 10 * time.Minute

	evictKeepN(ns)

	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(ns, name))
		return err == nil
	}
	for _, keep := range []string{"newest2", "newest1"} {
		if !exists(keep) {
			t.Errorf("keep-N should retain %q", keep)
		}
	}
	if !exists("fresh_old") {
		t.Error("entry within grace must be protected even beyond keep-N (R-1/AC-8)")
	}
	if exists("stale") {
		t.Error("stale entry beyond keep-N and grace must be evicted (AC-4)")
	}
}

func TestCopyTreeKeepsExistingDstWhenSourceFails(t *testing.T) {
	// VALIDATES atomicity (R-1/AC-8): a failed copy must NOT destroy the existing
	// dst. The old in-place copyTree RemoveAll'd dst first, so a mid-copy failure
	// left a hole a concurrent reader on the shared cache key could observe. The
	// staged copy only swaps at the very end, so the old tree survives a failure.
	parent := t.TempDir()
	dst := filepath.Join(parent, "cached")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "vmlinuz"), []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A non-existent source makes the walk fail.
	if err := copyTree(filepath.Join(t.TempDir(), "does-not-exist"), dst); err == nil {
		t.Fatal("expected copyTree to fail on a missing source")
	}

	data, err := os.ReadFile(filepath.Join(dst, "vmlinuz"))
	if err != nil || string(data) != "OLD" {
		t.Errorf("existing dst was destroyed by a failed copy: err=%v data=%q", err, data)
	}
	// No staging dir must be left behind in the parent.
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".copytree-") {
			t.Errorf("staging dir leaked after a failed copy: %s", e.Name())
		}
	}
}

func TestCopyTreeLeavesNoStagingDirOnSuccess(t *testing.T) {
	// The sibling staging dir must be renamed into place, not left beside dst.
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "vmlinuz"), []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	dst := filepath.Join(parent, "cached")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "cached" {
			t.Errorf("unexpected leftover beside dst: %s", e.Name())
		}
	}
}

func TestEvictKeepNStagingDirs(t *testing.T) {
	// A .copytree-* staging dir is never counted toward keep-N. A FRESH one
	// (within grace: a live populate) survives; a STALE one (orphaned by a crash
	// or a lost rename race) is reaped, so leaked staging cannot accumulate.
	ns := t.TempDir()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	mk := func(name string, ageMin int) {
		dir := filepath.Join(ns, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-time.Duration(ageMin) * time.Minute)
		if err := os.Chtimes(dir, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	mk("newest1", 2)
	mk("newest2", 4)
	mk("stale", 60)            // real, beyond keep-N + grace -> evicted
	mk(".copytree-fresh", 3)   // live populate, within grace -> survives
	mk(".copytree-orphan", 90) // orphaned staging, past grace -> reaped

	origNow, origGrace := evictNow, evictGrace
	t.Cleanup(func() { evictNow, evictGrace = origNow, origGrace })
	evictNow = func() time.Time { return now }
	evictGrace = 10 * time.Minute

	evictKeepN(ns)

	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(ns, name))
		return err == nil
	}
	if !exists(".copytree-fresh") {
		t.Error("a fresh staging dir (within grace) must survive")
	}
	if exists(".copytree-orphan") {
		t.Error("an orphaned staging dir (past grace) must be reaped")
	}
	if exists("stale") {
		t.Error("stale real entry must be evicted (staging dirs not counted toward keep-N)")
	}
	for _, keep := range []string{"newest1", "newest2"} {
		if !exists(keep) {
			t.Errorf("keep-N should retain %q", keep)
		}
	}
}

func TestEvictKeepNReapsStagingBelowKeepN(t *testing.T) {
	// Even when real keys are within keep-N (so the keep-N loop would early-return),
	// an orphaned staging dir is still reaped: the reap runs during the entry scan,
	// before that early return.
	ns := t.TempDir()
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	mk := func(name string, ageMin int) {
		dir := filepath.Join(ns, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-time.Duration(ageMin) * time.Minute)
		if err := os.Chtimes(dir, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	mk("only-key", 5)          // single real key, within keep-N -> early-return path
	mk(".copytree-orphan", 90) // orphaned staging, past grace -> reaped anyway

	origNow, origGrace := evictNow, evictGrace
	t.Cleanup(func() { evictNow, evictGrace = origNow, origGrace })
	evictNow = func() time.Time { return now }
	evictGrace = 10 * time.Minute

	evictKeepN(ns)

	if _, err := os.Stat(filepath.Join(ns, ".copytree-orphan")); err == nil {
		t.Error("orphaned staging must be reaped even when real keys are within keep-N")
	}
	if _, err := os.Stat(filepath.Join(ns, "only-key")); err != nil {
		t.Error("the single real key must be retained")
	}
}
