package scratch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gotoolchain"
)

// VALIDATES: the action empties the cache directory the toolchain names, and
// measures the device that directory sits on.
// PREVENTS: a clean run that reports free space for the checkout rather than
// for the filesystem cache/ is a symlink onto (plan/journal/full-disk-false-red.md).
func TestGoCleanCacheEmptiesTheNamedDirectory(t *testing.T) {
	cache := t.TempDir()
	entry := filepath.Join(cache, "aa")
	if err := os.Mkdir(entry, 0o750); err != nil {
		t.Fatalf("mkdir cache entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entry, "aaaa-d"), []byte("stale"), 0o600); err != nil {
		t.Fatalf("write cache entry: %v", err)
	}

	if err := goCleanCache(t.Context(), cache); err != nil {
		t.Fatalf("go clean -cache: %v", err)
	}
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Errorf("cache entry survived the clean: %v", err)
	}
}

// VALIDATES: the checkout cache path comes from the toolchain producer.
// PREVENTS: a second record of one fact, which drifts when the toolchain moves
// the cache.
func TestCleanCachesUsesTheToolchainCachePath(t *testing.T) {
	root := t.TempDir()
	if got, want := gotoolchain.GoCache(root), filepath.Join(root, "cache", "go-cache"); got != want {
		t.Fatalf("go cache = %q, want %q", got, want)
	}
}

// VALIDATES: free space is read for a cache directory the toolchain has not
// created yet.
// PREVENTS: a first run on a fresh checkout refusing because the cache is absent.
func TestFreeBytesWalksUpToAnExistingPath(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "cache", "go-cache")
	free, err := freeBytes(absent)
	if err != nil {
		t.Fatalf("free bytes: %v", err)
	}
	if free == 0 {
		t.Error("free bytes = 0 on a temporary directory's device")
	}
}

// VALIDATES: the ambient lookup runs with no inherited GOCACHE.
// PREVENTS: le's own override being reported as the machine default, which
// would empty one cache twice and leave the other full.
func TestWithoutGoCacheDropsEveryOverride(t *testing.T) {
	kept := withoutGoCache([]string{"HOME=/home/alice", "GOCACHE=/one", "PATH=/bin", "GOCACHE=/two"})
	want := []string{"HOME=/home/alice", "PATH=/bin"}
	if len(kept) != len(want) {
		t.Fatalf("environment = %#v, want %#v", kept, want)
	}
	for index, entry := range kept {
		if entry != want[index] {
			t.Errorf("entry %d = %q, want %q", index, entry, want[index])
		}
	}
}

// VALIDATES: the ambient cache is named by the go command itself.
// PREVENTS: a hardcoded per-platform default, which is wrong on the platform
// nobody tested.
func TestAmbientGoCacheAnswersAnAbsolutePath(t *testing.T) {
	path, err := ambientGoCache(t.Context())
	if err != nil {
		t.Fatalf("ambient cache: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("ambient cache = %q, want an absolute path", path)
	}
}

// VALIDATES: the default rendering states the freed and the remaining space for
// a cleaned cache, and the reason for one that was skipped or refused.
// PREVENTS: a refusal that reads as a success.
func TestCleanReportTextStatesEveryOutcome(t *testing.T) {
	report := CleanReport{Caches: []CacheClean{
		{Name: checkoutCache, Path: "/cache/go-cache", FreeBefore: 1 << 30, FreeAfter: 3 << 30, Freed: 2 << 30},
		{Name: ambientCache, Path: "/cache/go-cache", Skipped: "already emptied"},
		{Name: "third", Path: "/gone", Error: "statfs /gone: no such file or directory"},
	}}
	if code := report.verdict(); code != 1 {
		t.Errorf("verdict = %d, want 1 when a cache refused", code)
	}

	lines := strings.Split(strings.TrimSuffix(report.Text(), "\n"), "\n")
	if len(lines) != len(report.Caches) {
		t.Fatalf("text = %q, want one line per cache", report.Text())
	}
	for index, want := range []string{"freed 2.0G", "SKIP", "REFUSE"} {
		if !strings.Contains(lines[index], want) {
			t.Errorf("line %d = %q, want it to contain %q", index, lines[index], want)
		}
	}
	if !strings.Contains(lines[0], "free 3.0G") {
		t.Errorf("line 0 = %q, want the remaining space", lines[0])
	}
}

// VALIDATES: a clean run with no error answers 0.
// PREVENTS: a green run reported as a failure.
func TestCleanReportVerdictIsZeroWithoutAnError(t *testing.T) {
	report := CleanReport{Caches: []CacheClean{{Name: checkoutCache, Path: "/cache"}}}
	if code := report.verdict(); code != 0 {
		t.Errorf("verdict = %d, want 0", code)
	}
}
