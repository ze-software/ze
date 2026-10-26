package scratch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gostaticcheck "github.com/ze-software/ze/internal/le/go/staticcheck"
	"github.com/ze-software/ze/internal/le/go/stdpatch/gen"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// errWriterRace is what `go clean -cache` returns when a concurrent build
// writes into a subdirectory the walk has already passed.
var errWriterRace = errors.New("go clean -cache: unlinkat /cache/go-cache/18: directory not empty")

// errCacheUnwritable is a refusal that is NOT a race: the cache is still
// there, the disk is as full as it was, and the next build fails the same way.
var errCacheUnwritable = errors.New("go clean -cache: unlinkat /cache/go-cache/18: permission denied")

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

// VALIDATES: a clean run empties the golangci-lint cache as well as the two Go
// caches, and takes its path from the toolchain producer.
// PREVENTS: the cache no `go clean` reaches growing unbounded while scratch clean
// reports success. It held 9.5G on 2026-09-13, more than both Go caches
// together, on a volume with 1G left (plan/journal/full-disk-false-red.md).
func TestCacheTargetsCoverTheLintCache(t *testing.T) {
	root := t.TempDir()
	targets := cleanTargets(root, "/machine/default")

	want := []struct{ name, path string }{
		{checkoutCache, gotoolchain.GoCache(root)},
		{ambientCache, "/machine/default"},
		{bootstrapCache, gotoolchain.BootstrapCache(root)},
		{lintCache, gotoolchain.LintCache(root)},
		{staticcheckCache, gostaticcheck.CacheDir(root)},
	}
	if len(targets) != len(want) {
		t.Fatalf("targets = %d, want %d", len(targets), len(want))
	}
	for index, expected := range want {
		if targets[index].name != expected.name || targets[index].path != expected.path {
			t.Errorf("target %d = %q at %q, want %q at %q",
				index, targets[index].name, targets[index].path, expected.name, expected.path)
		}
		if targets[index].skipped != "" {
			t.Errorf("target %d skipped: %s", index, targets[index].skipped)
		}
	}
}

// VALIDATES: an ambient cache that IS the checkout cache is emptied once.
// PREVENTS: one cache reported twice, which reads as both caches being clean
// while the other is still full.
func TestCacheTargetsSkipAnAmbientThatIsTheCheckout(t *testing.T) {
	root := t.TempDir()
	targets := cleanTargets(root, gotoolchain.GoCache(root))

	for _, target := range targets {
		if target.name == ambientCache && target.skipped == "" {
			t.Error("the ambient row was not skipped when it names the checkout cache")
		}
		if target.name != ambientCache && target.skipped != "" {
			t.Errorf("the %s row was skipped: %s", target.name, target.skipped)
		}
	}
}

// VALIDATES: the lint cache is emptied by deleting it, and an absent one is not
// an error.
// PREVENTS: a fresh checkout, which has never run the linter, reporting a
// refusal for a cache that was never created.
func TestRemoveCacheEmptiesAndToleratesAbsence(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "golangci-lint-cache")
	if err := os.MkdirAll(filepath.Join(cache, "aa"), 0o750); err != nil {
		t.Fatalf("mkdir lint cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cache, "aa", "facts"), []byte("stale"), 0o600); err != nil {
		t.Fatalf("write lint cache entry: %v", err)
	}

	if err := removeCache(cache); err != nil {
		t.Fatalf("remove lint cache: %v", err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("lint cache survived the clean: %v", err)
	}
	if err := removeCache(cache); err != nil {
		t.Errorf("remove of an absent cache: %v", err)
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

	// Every name a target carries is followed by at least one space, so the
	// longest one does not run into its path ("bootstrap/path/...").
	for _, target := range cleanTargets("/checkout", "/ambient") {
		line := CleanReport{Caches: []CacheClean{{Name: target.name, Path: target.path}}}.Text()
		if !strings.HasPrefix(line, target.name+" ") {
			t.Errorf("row %q does not separate the name from the path", line)
		}
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

// VALIDATES: a sweep that freed space and then lost a race with a live writer
// is reported as CONTENDED, carries the bytes it reclaimed, and does not make
// the run exit 1.
// PREVENTS: the shape measured on 2026-09-19. `go clean -cache` walks the cache
// removing each subdirectory; a concurrent `go` command writing into one it has
// already walked leaves it non-empty and the run ends `unlinkat .../18:
// directory not empty`. The cache had gone from 41G to 114M and the command
// still printed REFUSE and exited 1, which an operator reads as "the clean
// failed" before reaching for `rm -rf` on a cache directory
// (plan/journal/full-disk-false-red.md).
func TestMeasureCleanReportsAWriterRaceAsContendedNotRefused(t *testing.T) {
	cache := t.TempDir()

	// The emptier frees the cache and then fails, which is the order the race
	// produces: the walk removes what it reaches, then trips on what a writer
	// put back behind it.
	raced := func() error {
		if err := os.RemoveAll(filepath.Join(cache, "18")); err != nil {
			return err
		}
		return errWriterRace
	}
	if err := os.MkdirAll(filepath.Join(cache, "18"), 0o750); err != nil {
		t.Fatalf("seed cache subdirectory: %v", err)
	}

	result := measureClean("checkout", cache, raced)

	if result.Error != "" {
		t.Errorf("a sweep that freed space was reported as a refusal: %s", result.Error)
	}
	if result.Contended == "" {
		t.Error("the writer race was not named, so the operator cannot tell it from a clean run")
	}
	report := CleanReport{Caches: []CacheClean{result}}
	if code := report.verdict(); code != 0 {
		t.Errorf("verdict %d for a cache that freed its space and met a writer, want 0", code)
	}
	if !strings.Contains(report.Text(), "concurrent build") {
		t.Errorf("the report does not say a concurrent build kept writing:\n%s", report.Text())
	}
}

// VALIDATES: an emptier that frees nothing and fails is still a refusal, and
// still exits 1.
// PREVENTS: the fix above swallowing a real failure. A cache that could not be
// emptied leaves the disk as full as it was, and the next build fails the same
// way, so the operator has to be told.
func TestMeasureCleanStillRefusesWhenNothingWasFreed(t *testing.T) {
	cache := t.TempDir()

	result := measureClean("checkout", cache, func() error { return errCacheUnwritable })

	if result.Error == "" {
		t.Error("an emptier that freed nothing and failed was not reported as a refusal")
	}
	if result.Contended != "" {
		t.Errorf("a failure that freed nothing was excused as contention: %s", result.Contended)
	}
	report := CleanReport{Caches: []CacheClean{result}}
	if code := report.verdict(); code != 1 {
		t.Errorf("verdict %d for a cache that refused, want 1", code)
	}
}

// VALIDATES: the bootstrap cache is one of the caches this action empties.
// PREVENTS: the published claim outrunning the behavior. The action states it
// empties every build cache this checkout fills and walked past tmp/go-cache,
// which held 1.3G when an operator found it by hand
// (plan/journal/full-disk-false-red.md). The `le` launcher still writes it when
// cache/ cannot be created, so the row stays on a linked host too: AC-13 counts
// the one Go cache a linked host fills, and AC-13a keeps this fallback's row
// (spec-le-one-go-cache-per-host).
func TestCacheTargetsCoverTheBootstrapCache(t *testing.T) {
	root := t.TempDir()
	want := gotoolchain.BootstrapCache(root)

	var found bool
	for _, target := range cleanTargets(root, filepath.Join(root, "ambient")) {
		if target.path == want {
			found = true
		}
	}
	if !found {
		t.Errorf("no target empties %s, the cache the le bootstrap and the deployment builds write", want)
	}
}

// VALIDATES: AC-13. On a checkout laid out as every le run leaves it, cache/ a
// link to the per-user target, cleanTargets answers one Go cache row, one lint
// row and one staticcheck row besides the ambient and the launcher's fallback,
// each with the kind and budget the trim reads, and no two rows land in one
// directory. A stale cache copy is a row of its own, removed whole past the
// floor.
// PREVENTS: a per-user row beside the checkout row that names the same
// directory through the link, which cleaned one cache twice and counted it
// twice against the budget.
func TestCleanTargetsOneRowPerCache(t *testing.T) {
	root := t.TempDir()
	perUser := t.TempDir()
	if err := os.Symlink(perUser, filepath.Join(root, cacheName)); err != nil {
		t.Fatalf("link cache/: %v", err)
	}

	want := []struct {
		name   string
		kind   cacheKind
		budget budgetGroup
	}{
		{checkoutCache, goBuildCache, goCachesBudget},
		{ambientCache, goBuildCache, notTrimmed},
		{bootstrapCache, goBuildCache, goCachesBudget},
		{lintCache, copiedFormatCache, lintCacheBudget},
		{staticcheckCache, copiedFormatCache, staticcheckCacheBudget},
	}
	targets := cleanTargets(root, "/machine/default")
	if len(targets) != len(want) {
		t.Fatalf("targets = %d, want %d: %+v", len(targets), len(want), targets)
	}
	landed := map[string]string{}
	for index, expected := range want {
		target := targets[index]
		if target.name != expected.name || target.kind != expected.kind || target.budget != expected.budget {
			t.Errorf("target %d = %s kind %d budget %d, want %s kind %d budget %d", index,
				target.name, target.kind, target.budget, expected.name, expected.kind, expected.budget)
		}
		if target.skipped != "" {
			t.Errorf("the %s row was skipped: %s", target.name, target.skipped)
		}
		where := gen.Landing(root, target.path)
		if first, ok := landed[where]; ok {
			t.Errorf("the %s and %s rows both land in %s", first, target.name, where)
		}
		landed[where] = target.name
	}
	for _, name := range []string{checkoutCache, lintCache, staticcheckCache} {
		path := rowTarget(t, targets, name).path
		if where := gen.Landing(root, path); !pathWithin(where, perUser) {
			t.Errorf("the %s row %s lands in %s, outside the per-user target %s", name, path, where, perUser)
		}
	}

	stale := filepath.Join(root, cacheName+staleCacheInfix+"20261024T101500Z-42")
	if err := os.Mkdir(stale, 0o750); err != nil {
		t.Fatal(err)
	}
	withStale := cleanTargets(root, "/machine/default")
	if len(withStale) != len(want)+1 {
		t.Fatalf("targets with one stale copy = %d, want %d", len(withStale), len(want)+1)
	}
	last := withStale[len(withStale)-1]
	if last.name != staleCopyName(0) || last.path != stale || last.kind != staleCopy || last.budget != pastFloor {
		t.Errorf("stale row = %+v, want %s at %s, kind staleCopy, budget pastFloor", last, staleCopyName(0), stale)
	}
}

// VALIDATES: AC-13. The ambient row is a skip when `go env GOCACHE` resolves
// to the checkout cache, compared where both land, and is cleaned when it is
// another directory.
// PREVENTS: a host whose `go env -w GOCACHE` names the shared cache by its
// per-user path, which differs as a string from cache/go-cache, cleaning one
// directory twice under two rows.
func TestAmbientSkippedWhenSameDirectory(t *testing.T) {
	root := t.TempDir()
	perUser := t.TempDir()
	if err := os.Symlink(perUser, filepath.Join(root, cacheName)); err != nil {
		t.Fatalf("link cache/: %v", err)
	}
	shared := filepath.Join(perUser, "go-cache")
	if err := os.MkdirAll(shared, 0o750); err != nil {
		t.Fatal(err)
	}

	if got := rowTarget(t, cleanTargets(root, shared), ambientCache); !strings.Contains(got.skipped, "checkout cache") {
		t.Errorf("ambient %s, which is cache/go-cache through the link: skipped = %q, want a skip naming the checkout cache", shared, got.skipped)
	}
	elsewhere := t.TempDir()
	if got := rowTarget(t, cleanTargets(root, elsewhere), ambientCache); got.skipped != "" {
		t.Errorf("ambient %s, another directory: skipped = %q, want it cleaned", elsewhere, got.skipped)
	}
}

// VALIDATES: the stale copy rows follow the checkout, whatever its path holds.
// Method: a checkout under a directory whose name carries `[`, which glob
// syntax reads as an unclosed class, holds two stale copies and a file with
// the same prefix; both copies are listed in name order and the file is not.
// PREVENTS: a checkout path built into a glob pattern panicking scratch clean
// and the store trim, or matching names that are not stale copies.
func TestStaleCopiesListedUnderAGlobMetacharacterRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "work[1", "ze")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(root, cacheName+staleCacheInfix+"20261024T090000Z-7")
	newer := filepath.Join(root, cacheName+staleCacheInfix+"20261024T101500Z-42")
	for _, copied := range []string{newer, older} {
		if err := os.Mkdir(copied, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	decoy := filepath.Join(root, cacheName+staleCacheInfix+"20261024T110000Z-9")
	if err := os.WriteFile(decoy, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var stale []cleanTarget
	for _, target := range cleanTargets(root, "/machine/default") {
		if target.kind == staleCopy {
			stale = append(stale, target)
		}
	}
	if len(stale) != 2 {
		t.Fatalf("stale rows = %+v, want the two copies %s and %s and not the file %s", stale, older, newer, decoy)
	}
	for index, want := range []string{older, newer} {
		if stale[index].path != want || stale[index].name != staleCopyName(index) || stale[index].failed != "" {
			t.Errorf("stale row %d = %+v, want %s at %s, not failed", index, stale[index], staleCopyName(index), want)
		}
	}
}

// VALIDATES: a checkout that cannot be listed answers one failed stale row.
// Method: cleanTargets over a root that does not exist; the row names the
// root and carries the listing error, and the store trim reports it as an
// error rather than trimming or skipping it.
// PREVENTS: an unlisted checkout reading as "no stale copy".
func TestStaleCopiesUnlistedRootAnswersAFailedRow(t *testing.T) {
	root := filepath.Join(t.TempDir(), "gone")

	targets := cleanTargets(root, "/machine/default")
	failed := rowTarget(t, targets, staleCacheCopy)
	if failed.path != root || failed.kind != staleCopy || !strings.Contains(failed.failed, "cannot list") {
		t.Fatalf("unlisted row = %+v, want %s at %s, kind staleCopy, failed naming the listing", failed, staleCacheCopy, root)
	}
	_, rows := trimCaches(t.Context(), targets, budgetsOf(1<<40, 1<<40, 1<<40), testPass(cacheEntryAgeFloor))
	for _, row := range rows {
		if row.Name != staleCacheCopy {
			continue
		}
		if row.Error != failed.failed || row.EntriesRemoved != 0 || row.Skipped != "" {
			t.Errorf("trim row = %+v, want error %q, nothing removed, no skip", row, failed.failed)
		}
	}
}

// rowTarget answers the target named name, failing the test when none is.
func rowTarget(t *testing.T, targets []cleanTarget, name string) cleanTarget {
	t.Helper()
	for _, target := range targets {
		if target.name == name {
			return target
		}
	}
	t.Fatalf("no %s target in %+v", name, targets)
	return cleanTarget{}
}

// VALIDATES: `all` empties the Staticcheck matrix cache by deleting it,
// because no `go clean` owns another tool's copy of Go's layout.
// PREVENTS: a full empty that leaves the matrix cache, which Staticcheck trims
// only daily, as the one cache still full.
// Method: seed each matrix cache row cleanTargets answers, run its emptier.
func TestStaticcheckCachesEmptiedByDeletion(t *testing.T) {
	root := t.TempDir()
	emptied := 0
	for _, target := range cleanTargets(root, "/machine/default") {
		if target.budget != staticcheckCacheBudget {
			continue
		}
		entry := filepath.Join(target.path, "ab", "ab01-a")
		if err := os.MkdirAll(filepath.Dir(entry), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(entry, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := target.emptier(t.Context())(); err != nil {
			t.Fatalf("%s: %v", target.name, err)
		}
		if _, err := os.Lstat(target.path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: %s still exists after the full empty (%v)", target.name, target.path, err)
		}
		emptied++
	}
	if emptied != 1 {
		t.Errorf("emptied %d staticcheck caches, want the one matrix cache", emptied)
	}
}
