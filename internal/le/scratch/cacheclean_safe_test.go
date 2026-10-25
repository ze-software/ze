package scratch

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	gostaticcheck "github.com/ze-software/ze/internal/le/go/staticcheck"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/procscan"
)

// seededCache is one Go-format cache a test wrote: an entry older than the age
// floor, one younger than it, and a name that is no entry at all.
type seededCache struct {
	old, young, other string
}

// seedCache writes a seededCache under dir, dated against now.
func seedCache(t *testing.T, dir string, now time.Time) seededCache {
	t.Helper()
	sub := filepath.Join(dir, "aa")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	seeded := seededCache{
		old:   filepath.Join(sub, "aa01-d"),
		young: filepath.Join(sub, "aa02-a"),
		other: filepath.Join(dir, "README"),
	}
	for path, age := range map[string]time.Duration{
		seeded.old:   cacheEntryAgeFloor + time.Hour,
		seeded.young: cacheEntryAgeFloor - time.Hour,
		seeded.other: 30 * 24 * time.Hour,
	} {
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 8192)), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, now.Add(-age), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	return seeded
}

// assertSafeClean fails unless only the old entry is gone.
func (seeded seededCache) assertSafeClean(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Lstat(seeded.old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s: the entry older than the floor survived: %v", name, err)
	}
	if _, err := os.Lstat(seeded.young); err != nil {
		t.Errorf("%s: the entry younger than the floor, which a running build may be about to read, was removed: %v", name, err)
	}
	if _, err := os.Lstat(seeded.other); err != nil {
		t.Errorf("%s: a name that is no cache entry was removed: %v", name, err)
	}
}

// assertUntouched fails unless every seeded file is still there.
func (seeded seededCache) assertUntouched(t *testing.T, name string) {
	t.Helper()
	for _, path := range []string{seeded.old, seeded.young, seeded.other} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s: %s was removed by a run that should have touched nothing: %v", name, path, err)
		}
	}
}

// VALIDATES: the default clean removes an entry older than the store trim's
// floor and keeps one younger, and reports what it removed and kept.
// PREVENTS: the 2026-10-21 staticcheck reds, where a clean emptied the caches
// under another session's running lint and the lint failed with "cache entry
// not found" (plan/journal/gate-verdict-depends-on-the-machine.md).
// Method: one temp cache with an old, a young and a non-entry file, cleaned
// with the real clock, floor and remover.
func TestCleanOldCacheEntriesKeepsTheYoung(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	seeded := seedCache(t, dir, now)

	pass := cachePass{now: now, floor: cacheEntryAgeFloor, remove: removeCacheEntry, run: cacheCleanRun}
	row := cleanOldCacheEntries(t.Context(), cleanTarget{name: checkoutCache, path: dir, kind: goBuildCache}, pass)

	seeded.assertSafeClean(t, checkoutCache)
	if row.Error != "" {
		t.Fatalf("row refused: %s", row.Error)
	}
	if row.EntriesRemoved != 1 {
		t.Errorf("entries removed = %d, want 1", row.EntriesRemoved)
	}
	if row.Removed <= 0 {
		t.Errorf("removed = %d bytes, want the old entry's size", row.Removed)
	}
	if row.KeptYoung <= 0 {
		t.Errorf("kept young = %d bytes, want the young entry's size", row.KeptYoung)
	}
	text := CleanReport{Caches: []CacheClean{row}}.Text()
	for _, want := range []string{"removed ", "(1 entries)", "younger than " + cacheEntryAgeFloor.String(), "free "} {
		if !strings.Contains(text, want) {
			t.Errorf("text = %q, want it to contain %q", text, want)
		}
	}
}

// VALIDATES: a cache that does not exist is a skip, not a refusal.
// PREVENTS: a fresh checkout's first clean exiting 1.
func TestCleanOldCacheEntriesSkipsAnAbsentCache(t *testing.T) {
	pass := cachePass{now: time.Now(), floor: cacheEntryAgeFloor, remove: removeCacheEntry, run: cacheCleanRun}
	row := cleanOldCacheEntries(t.Context(), cleanTarget{name: lintCache, path: filepath.Join(t.TempDir(), "none")}, pass)
	if row.Skipped != absentStoreSkip {
		t.Errorf("row = %+v, want the absent-store skip", row)
	}
}

// isolatedManager answers a Manager over a temp checkout whose every cache,
// the ambient one included, resolves under the test's temp directories, and
// fails the test before anything is cleaned if the ambient cache does not.
// The refusal tests use it too, so a refusal that regresses empties temp
// directories rather than the developer's real caches.
func isolatedManager(t *testing.T) (*Manager, string) {
	t.Helper()
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("GOENV", "off")
	// The `go env` below would otherwise write telemetry counters under the
	// temp home, and start a detached upload child that outlives it and writes
	// there while t.TempDir removes it ("directory not empty", seen 2026-10-21).
	// Go reads the telemetry mode from <user config dir>/go/telemetry/mode.
	config, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	telemetry := filepath.Join(config, "go", "telemetry")
	if err := os.MkdirAll(telemetry, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(telemetry, "mode"), []byte("off"), 0o600); err != nil {
		t.Fatal(err)
	}
	ambient, err := ambientGoCache(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !pathWithin(ambient, home) {
		t.Fatalf("the ambient cache resolved to %s, outside the test's home %s: refusing to clean a real cache", ambient, home)
	}
	return New(root, os.Environ()), ambient
}

// VALIDATES: the default mode reaches every cache target through the action's
// own entry point, keeping the young entries of each.
// PREVENTS: one target (the lint cache is not Go's) still emptied outright.
// Method: every target redirected to temp directories, then CleanCaches.
func TestCleanCachesDefaultKeepsYoungEntriesInEveryTarget(t *testing.T) {
	manager, ambient := isolatedManager(t)
	now := time.Now()
	caches := map[string]string{
		checkoutCache:    gotoolchain.GoCache(manager.Root),
		ambientCache:     ambient,
		bootstrapCache:   gotoolchain.BootstrapCache(manager.Root),
		lintCache:        gotoolchain.LintCache(manager.Root),
		staticcheckCache: gostaticcheck.CacheDir(manager.Root),
	}
	seeded := map[string]seededCache{}
	for name, dir := range caches {
		seeded[name] = seedCache(t, dir, now)
	}

	report, code := manager.CleanCaches(cleanOldEntries)

	if code != 0 {
		t.Fatalf("exit %d, report:\n%s", code, report.Text())
	}
	if report.Mode != oldEntriesMode {
		t.Errorf("mode = %q, want %q", report.Mode, oldEntriesMode)
	}
	for name, cache := range seeded {
		cache.assertSafeClean(t, name)
	}
	if len(report.Caches) != len(caches) {
		t.Errorf("report has %d rows, want one per cache:\n%s", len(report.Caches), report.Text())
	}
}

// compileUnderAnotherSession is a compile another session's build runs.
var compileUnderAnotherSession = procscan.Process{
	PID: 4242, Argv: []string{"/goroot/pkg/tool/darwin_arm64/compile", "-o", "/work/b001/_pkg_.a"},
}

// noHolders is an empty job registry.
func noHolders(string) ([]job.Holder, error) { return nil, nil }

// VALIDATES: `all` refuses while another build runs, names the process and the
// fix, exits 1, and leaves every cache untouched.
// PREVENTS: the full empty failing another session's build.
// Method: a fake process lister answering one compile.
func TestCleanCachesAllRefusesWhileABuildRuns(t *testing.T) {
	manager, _ := isolatedManager(t)
	root := manager.Root
	cache := seedCache(t, gotoolchain.GoCache(root), time.Now())
	pass := cleanPass{
		scan:    func() ([]procscan.Process, error) { return []procscan.Process{compileUnderAnotherSession}, nil },
		holders: noHolders,
		self:    []int{1},
	}

	report, code := manager.cleanCaches(cleanEverything, pass)

	if code != 1 {
		t.Errorf("exit %d, want 1 for a refused full empty", code)
	}
	cache.assertUntouched(t, checkoutCache)
	if len(report.Users) != 1 || report.Users[0].PID != compileUnderAnotherSession.PID {
		t.Fatalf("users = %+v, want the one compile", report.Users)
	}
	text := report.Text()
	for _, want := range []string{"REFUSE", "pid 4242", "compile", "cache-clean all", "./le scratch cache-clean`"} {
		if !strings.Contains(text, want) {
			t.Errorf("text = %q, want it to contain %q", text, want)
		}
	}
}

// VALIDATES: a le job or verify run holding a registry slot refuses the full
// empty even while no toolchain process runs under it, and the row names its label.
// PREVENTS: an empty landing between two stages of a verify run.
func TestCleanCachesAllRefusesWhileAJobHoldsASlot(t *testing.T) {
	manager, _ := isolatedManager(t)
	pass := cleanPass{
		scan: func() ([]procscan.Process, error) {
			return []procscan.Process{{PID: 777, Argv: []string{"/bin/le-darwin-arm64/le", "verify", "current"}}}, nil
		},
		holders: func(string) ([]job.Holder, error) { return []job.Holder{{PID: 777, Label: "verify-worktree"}}, nil },
		self:    []int{1},
	}
	report, code := manager.cleanCaches(cleanEverything, pass)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(report.Text(), "(le job verify-worktree)") {
		t.Errorf("text = %q, want the job label", report.Text())
	}
}

// VALIDATES: a process table that cannot be read refuses the full empty and
// says why, rather than reading as "nothing runs".
func TestCleanCachesAllRefusesWithoutAProcessTable(t *testing.T) {
	manager, _ := isolatedManager(t)
	root := manager.Root
	cache := seedCache(t, gotoolchain.GoCache(root), time.Now())
	pass := cleanPass{
		scan:    func() ([]procscan.Process, error) { return nil, errors.New("ps: operation not permitted") },
		holders: noHolders,
	}
	report, code := manager.cleanCaches(cleanEverything, pass)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	cache.assertUntouched(t, checkoutCache)
	if !strings.Contains(report.Refusal, "process table could not be read") {
		t.Errorf("refusal = %q, want the reason", report.Refusal)
	}
}

// VALIDATES: the cache users are toolchain programs and live registry holders,
// never this run's own pids, an unrelated process, or a holder whose pid is gone.
func TestCacheUsersJudgesEachProcess(t *testing.T) {
	processes := []procscan.Process{
		{PID: 10, Argv: []string{"go", "test", "./..."}},
		{PID: 11, Argv: []string{"/tmp/go-build1/b001/scratch.test", "-test.v"}},
		{PID: 12, Argv: []string{"golangci-lint", "run"}},
		{PID: 13, Argv: []string{"/opt/homebrew/bin/staticcheck", "./..."}},
		{PID: 14, Argv: []string{"vim", "go.mod"}},
		{PID: 15, Argv: []string{"gopls"}},
		{PID: 16, Argv: []string{"go", "env", "GOCACHE"}},
		{PID: 17},
	}
	holders := []job.Holder{{PID: 99, Label: "gone"}}
	users := cacheUsers(processes, holders, []int{16})
	var pids []int
	for _, user := range users {
		pids = append(pids, user.PID)
	}
	want := []int{10, 11, 12, 13}
	if len(pids) != len(want) {
		t.Fatalf("users = %v, want %v", pids, want)
	}
	for index := range want {
		if pids[index] != want[index] {
			t.Errorf("users = %v, want %v", pids, want)
		}
	}
}

// VALIDATES: a command line longer than commandShownMax is cut.
func TestShownCommandIsBounded(t *testing.T) {
	long := []string{"compile", strings.Repeat("a", 2*commandShownMax)}
	if got := shownCommand(long); len(got) != commandShownMax+len("...") {
		t.Errorf("shown command has %d bytes, want %d", len(got), commandShownMax+len("..."))
	}
}

// VALIDATES: `./le scratch cache-clean` with no keyword runs the safe default,
// and only the `all` keyword selects the full empty.
// PREVENTS: the dispatch swapped, so the bare command empties every cache
// under other sessions' builds. Method: the mode runCacheClean dispatches on is
// read through cleanModeOf, the function it calls.
func TestCacheCleanKeywordSelectsTheMode(t *testing.T) {
	if got := cleanModeOf(leaction.Arguments{}); got != cleanOldEntries {
		t.Errorf("no keyword selected %s, want %s", got, cleanOldEntries)
	}
	if got := cleanModeOf(leaction.Arguments{everythingMode: {""}}); got != cleanEverything {
		t.Errorf("`%s` selected %s, want %s", everythingMode, got, cleanEverything)
	}
}

// VALIDATES: a job registry that exists and cannot be listed refuses the full
// empty, end to end through job.Holders, and touches no cache.
// PREVENTS: an unreadable registry reading as "no le job runs", so the empty
// lands under a live verify run. Method: tmp/.ze-jobs made mode 000; root
// reads such a directory, so the case skips as root.
func TestCleanCachesAllRefusesOnAnUnreadableRegistry(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 000 directory, so the unreadable state cannot be built")
	}
	manager, _ := isolatedManager(t)
	cache := seedCache(t, gotoolchain.GoCache(manager.Root), time.Now())
	jobs := filepath.Join(manager.Root, job.JobsDir)
	if err := os.MkdirAll(jobs, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(jobs, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(jobs, 0o750); err != nil { //nolint:gosec // restores the test's own directory so t.TempDir can remove it
			t.Error(err)
		}
	})
	pass := cleanPass{
		scan:    func() ([]procscan.Process, error) { return nil, nil },
		holders: job.Holders,
	}

	report, code := manager.cleanCaches(cleanEverything, pass)

	if code != 1 {
		t.Fatalf("exit %d, want 1 for an unreadable registry", code)
	}
	cache.assertUntouched(t, checkoutCache)
	if !strings.Contains(report.Refusal, "job registry could not be read") {
		t.Errorf("refusal = %q, want the registry reason", report.Refusal)
	}
}

// VALIDATES: the refusal counts its processes in plain English: one process,
// two processes.
func TestRefusalCountsProcessesInEnglish(t *testing.T) {
	manager, _ := isolatedManager(t)
	for _, tc := range []struct {
		processes []procscan.Process
		want      string
	}{
		{[]procscan.Process{compileUnderAnotherSession}, "that 1 running process may be reading"},
		{[]procscan.Process{compileUnderAnotherSession, {PID: 4343, Argv: []string{"go", "vet"}}}, "that 2 running processes may be reading"},
	} {
		pass := cleanPass{
			scan:    func() ([]procscan.Process, error) { return tc.processes, nil },
			holders: noHolders,
			self:    []int{1},
		}
		refusal, _ := manager.refuseFullEmpty(pass)
		if !strings.Contains(refusal, tc.want) {
			t.Errorf("refusal = %q, want it to contain %q", refusal, tc.want)
		}
	}
}

// VALIDATES: the JSON keys of a cache row are kebab-case, as the le command
// grammar requires of every JSON answer (ai/rules/cli.md).
// PREVENTS: a key the spot checks below do not name going out snake_case, as
// free_before and free_after did until 2026-10-22: every tagged field of the
// row and of the report is checked.
func TestCacheCleanJSONKeysAreKebabCase(t *testing.T) {
	encoded, err := json.Marshal(CacheClean{Name: "n", EntriesRemoved: 2, KeptYoung: 3, Removed: 4})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"entries-removed":2`, `"kept-young":3`, `"removed":4`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("json = %s, want it to contain %s", encoded, want)
		}
	}
	for _, kind := range []reflect.Type{reflect.TypeFor[CacheClean](), reflect.TypeFor[CleanReport]()} {
		for field := range kind.Fields() {
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				continue
			}
			if name != strings.ToLower(name) || strings.Contains(name, "_") {
				t.Errorf("%s.%s has JSON key %q, want lowercase kebab-case", kind.Name(), field.Name, name)
			}
		}
	}
}

// VALIDATES: a walk its time limit stops names the command the operator ran
// and the fix, for cache-clean and for the store trim alike.
// PREVENTS: a cache-clean run reporting "the trim" stopped, which names an
// operation the operator did not start and gives no next step.
// Method: a context already canceled stops the walk at its first check.
func TestWalkStoppedByItsLimitNamesTheCommandAndTheFix(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, run := range []trimRun{cacheCleanRun, storeTrimRun} {
		_, err := walkGoFormatCache(ctx, t.TempDir(), 0, run)
		if err == nil {
			t.Fatalf("%s: a canceled walk answered no error", run.command)
		}
		for _, want := range []string{"`" + run.command + "` reached its " + run.limit.String() + " limit", "df -h", "then run `" + run.command + "` again"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: error = %q, want it to contain %q", run.command, err, want)
			}
		}
	}
}
