// Related: registryclean.go -- the job registry's row of a scratch clean run
// Related: ../job/prune_test.go -- every rule of the judgment this row counts
//
// VALIDATES: plan/journal/artifact-written-with-no-remover.md, 2026-11-13.
// `le scratch clean` prunes the job registry and reports what it removed and
// what it kept, in both modes, by the age floor the cache entries keep.
// PREVENTS: a registry nothing cleans, a clean that reports no count, and a
// registry failure that reads as a success.

package scratch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/procscan"
)

// registryFile writes one file of the job registry under root, last written
// age before now, and answers its path.
func registryFile(t *testing.T, root, name, body string, now time.Time, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(job.JobsDir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	written := now.Add(-age)
	if err := os.Chtimes(path, written, written); err != nil {
		t.Fatal(err)
	}
	return path
}

// noProcesses is a process table that lists nothing.
func noProcesses() ([]procscan.Process, error) { return nil, nil }

// registryPass is the pass of a default scratch clean with the clock and the
// process table a test fixes.
func registryPass(now time.Time, scan func() ([]procscan.Process, error)) cleanPass {
	return cleanPass{
		scan:    scan,
		holders: noHolders,
		self:    []int{1},
		entries: cachePass{now: now, floor: cacheEntryAgeFloor, remove: removeCacheEntry, run: cacheCleanRun},
	}
}

// VALIDATES: the default scratch clean removes the registry's dead files past
// the floor and reports the count, the size and what it kept.
// PREVENTS: the receipts staying for ever, and a removal nobody is told of.
// Method: the whole clean run over temp directories, with an old receipt, an
// old dead waiter record, a young receipt and a foreign file in the registry.
func TestCleanCachesPrunesTheJobRegistryAndReportsTheCount(t *testing.T) {
	manager, _ := isolatedManager(t)
	now := time.Now()
	old := 2 * cacheEntryAgeFloor
	receipt := registryFile(t, manager.Root, "lint.4001.7001.rc", "0\n", now, old)
	waiter := registryFile(t, manager.Root, "unit.4002.7002.wait", "LABEL=unit\nPID=4002\nWAITING=1\nCMD=go test\n", now, old)
	young := registryFile(t, manager.Root, "lint.4003.7003.rc", "1\n", now, time.Minute)
	foreign := registryFile(t, manager.Root, "notes.txt", "kept by hand\n", now, old)

	report, code := manager.cleanCaches(cleanOldEntries, registryPass(now, noProcesses))

	if code != 0 {
		t.Fatalf("exit %d, report:\n%s", code, report.Text())
	}
	for _, path := range []string{receipt, waiter} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s is still there (stat error %v), want it removed", filepath.Base(path), err)
		}
	}
	for _, path := range []string{young, foreign} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("%s: %v, want it kept", filepath.Base(path), err)
		}
	}

	row := report.Registry
	if row == nil {
		t.Fatalf("the report has no registry row:\n%s", report.Text())
	}
	if row.FilesRemoved != 2 || row.FilesKept != 2 {
		t.Errorf("row = %+v, want 2 files removed and 2 kept", *row)
	}
	wantBytes := int64(len("0\n") + len("LABEL=unit\nPID=4002\nWAITING=1\nCMD=go test\n"))
	if row.Removed != wantBytes {
		t.Errorf("removed %d bytes, want the %d of the two files", row.Removed, wantBytes)
	}

	var line string
	for candidate := range strings.SplitSeq(report.Text(), "\n") {
		if strings.HasPrefix(candidate, jobRegistry+" ") {
			line = candidate
		}
	}
	for _, want := range []string{
		row.Path, "(2 dead files older than 3h0m0s)", "kept 2 files",
		"1 " + job.PruneKeptYoung.String(), "1 " + job.PruneKeptForeignName.String(),
	} {
		if !strings.Contains(line, want) {
			t.Errorf("registry line = %q, want it to contain %q", line, want)
		}
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"registry":{`, `"files-removed":2`, `"files-kept":2`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("json = %s, want it to contain %s", encoded, want)
		}
	}
}

// VALIDATES: a checkout with no registry gets a skip row and exit 0.
// PREVENTS: a clean that fails, or creates the registry, where no job ran.
func TestCleanCachesSkipsAnAbsentJobRegistry(t *testing.T) {
	manager, _ := isolatedManager(t)

	report, code := manager.cleanCaches(cleanOldEntries, registryPass(time.Now(), noProcesses))

	if code != 0 {
		t.Fatalf("exit %d, report:\n%s", code, report.Text())
	}
	if report.Registry == nil || report.Registry.Skipped != absentRegistrySkip {
		t.Fatalf("registry row = %+v, want the absent-registry skip", report.Registry)
	}
	if !strings.Contains(report.Text(), "SKIP     "+report.Registry.Path) {
		t.Errorf("text = %q, want a SKIP row for the registry", report.Text())
	}
	if _, err := os.Lstat(report.Registry.Path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the clean created the registry (stat error %v)", err)
	}
}

// VALIDATES: a registry row that carries an error makes the run exit 1 and
// prints a refusal naming the command to run again.
// PREVENTS: a registry that was not cleaned reading as a clean one.
func TestRegistryErrorRefusesTheRun(t *testing.T) {
	root := t.TempDir()
	// A floor below the least bound the registry accepts is the one refusal a
	// test can cause without making a directory unreadable.
	pass := registryPass(time.Now(), noProcesses)
	pass.entries.floor = job.PruneAgeMin - time.Second
	receipt := registryFile(t, root, "lint.4001.7001.rc", "0\n", pass.entries.now, 2*cacheEntryAgeFloor)

	row := cleanJobRegistry(t.Context(), root, pass)

	if row.Error == "" {
		t.Fatalf("row = %+v, want an error for a refused prune", *row)
	}
	if _, err := os.Lstat(receipt); err != nil {
		t.Errorf("the receipt is gone after a refused prune: %v", err)
	}
	report := CleanReport{Registry: row}
	if code := report.verdict(); code != 1 {
		t.Errorf("verdict = %d, want 1 when the registry was not cleaned", code)
	}
	text := report.Text()
	for _, want := range []string{"REFUSE", row.Path, "`./le scratch clean` again"} {
		if !strings.Contains(text, want) {
			t.Errorf("text = %q, want it to contain %q", text, want)
		}
	}
}
