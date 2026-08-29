// VALIDATES: verify status freshness, moved-path certificates, summary formatting,
// and concurrent failure-index appends preserve the script contracts.
// PREVENTS: a full verify certifying a tree it did not read, or two failed
// stages interleaving one another's summary blocks.
package verifyengine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/job"
)

func TestSkippedSuitesRecognizesDottedEnvironmentSpellings(t *testing.T) {
	for _, name := range []string{"ZE_SKIP_SUITES", "ze.skip.suites", "Ze.Skip_Suites"} {
		if got := skippedSuites([]string{"HOME=/home/test", name + "=web,firewall"}); got != "web,firewall" {
			t.Errorf("%s resolved to %q", name, got)
		}
	}
	got := skippedSuites([]string{"ze.skip.suites=web", "ZE_SKIP_SUITES=firewall"})
	if got != "firewall" {
		t.Fatalf("shell spelling did not win: %q", got)
	}
}

func TestCertificateFixtureSupportsWholeTreeAndScopedFreshness(t *testing.T) {
	root := statusFixture(t)
	start := job.SnapshotTree(root)
	at := time.Date(2026, 8, 27, 11, 12, 13, 0, time.UTC)
	certificate, err := WriteCertificate(root, WriteRequest{
		Exit: 0, Mode: Mode, GitSHA: job.Head(root), Start: start, At: at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if certificate.TreeHash != start.Hash {
		t.Fatalf("certificate tree hash = %q, want start hash %q", certificate.TreeHash, start.Hash)
	}
	wantStatus := "exit=0\ntimestamp=2026-08-27T11:12:13Z\nmode=full\nskipped=\ngit_sha=" + job.Head(root) + "\ntree_hash=" + start.Hash + "\n"
	gotStatus, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(StatusPath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotStatus) != wantStatus {
		t.Fatalf("status file:\n%s\nwant:\n%s", gotStatus, wantStatus)
	}
	if got := CheckCertificate(root, nil); !got.Fresh {
		t.Fatalf("whole-tree check = %#v", got)
	}
	if got := CheckCertificate(root, []string{"changed.txt"}); !got.Fresh {
		t.Fatalf("scoped check = %#v", got)
	}

	writeVerifyFixture(t, root, "other.txt", "changed after verify\n")
	if got := CheckCertificate(root, nil); got.Fresh || !strings.Contains(got.Reason, "tree changed") {
		t.Fatalf("whole-tree check after unrelated edit = %#v", got)
	}
	if got := CheckCertificate(root, []string{"changed.txt"}); !got.Fresh {
		t.Fatalf("scoped check after unrelated edit = %#v", got)
	}
}

func TestCertificateKeepsAPathMovedDuringRunStaleAfterRestore(t *testing.T) {
	root := statusFixture(t)
	start := job.SnapshotTree(root)
	writeVerifyFixture(t, root, "changed.txt", "moved during run\n")
	_, err := WriteCertificate(root, WriteRequest{
		Exit: 0, Mode: Mode, GitSHA: job.Head(root), Start: start,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeVerifyFixture(t, root, "changed.txt", "dirty before run\n")

	got := CheckCertificate(root, []string{"changed.txt"})
	if got.Fresh || !strings.Contains(got.Reason, "moved while the run was in flight") {
		t.Fatalf("restored moved path check = %#v", got)
	}
	manifest, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ManifestPath)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), movedDuringRun+" changed.txt") {
		t.Fatalf("manifest did not record the moved path:\n%s", manifest)
	}
}

func TestFailureSummaryMatchesLintAndTestFixtures(t *testing.T) {
	dir := t.TempDir()
	lintLog := filepath.Join(dir, "lint.log")
	var lint strings.Builder
	for index := 1; index <= 105; index++ {
		fmt.Fprintf(&lint, "finding %03d\n", index)
	}
	if err := os.WriteFile(lintLog, []byte(lint.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	lintSummary, err := Summarize("lint", lintLog)
	if err != nil {
		t.Fatal(err)
	}
	if len(lintSummary.KeyLines) != 100 || lintSummary.KeyLines[99] != "finding 100" {
		t.Fatalf("lint key lines = %d, last %q", len(lintSummary.KeyLines), lintSummary.KeyLines[len(lintSummary.KeyLines)-1])
	}

	testLog := filepath.Join(dir, "test.log")
	content := "setup\n--- FAIL: TestOne (0.00s)\nnoise\npanic: broken\nFAIL\tpkg/name\n"
	if err := os.WriteFile(testLog, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	summary, err := Summarize("unit", testLog)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2:--- FAIL: TestOne (0.00s)", "4:panic: broken", "5:FAIL\tpkg/name"}
	if strings.Join(summary.KeyLines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("key lines = %q, want %q", summary.KeyLines, want)
	}
	wantText := "\n### Stage: unit\nFull log: " + testLog + "\n\nKey lines:\n" +
		"2:--- FAIL: TestOne (0.00s)\n4:panic: broken\n5:FAIL\tpkg/name\n"
	if got := summary.Text(); got != wantText {
		t.Fatalf("summary bytes:\n%q\nwant:\n%q", got, wantText)
	}
	missing, err := Summarize("unit", filepath.Join(dir, "missing.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !missing.Missing || len(missing.KeyLines) != 1 || !strings.Contains(missing.KeyLines[0], "stage log missing") {
		t.Fatalf("missing summary = %#v", missing)
	}
}

func TestConcurrentFailureSummaryAppendsKeepEachBlockWhole(t *testing.T) {
	dir := t.TempDir()
	failures := filepath.Join(dir, "failures.log")
	const writers = 24
	var wait sync.WaitGroup
	wait.Add(writers)
	for index := range writers {
		go func() {
			defer wait.Done()
			stage := fmt.Sprintf("stage-%02d", index)
			log := filepath.Join(dir, stage+".log")
			if err := os.WriteFile(log, []byte("panic: "+stage+"\n"), 0o600); err != nil {
				t.Errorf("write %s: %v", stage, err)
				return
			}
			if _, err := AppendSummary(failures, stage, log); err != nil {
				t.Errorf("append %s: %v", stage, err)
			}
		}()
	}
	wait.Wait()
	content, err := os.ReadFile(failures)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if got := strings.Count(text, "\n### Stage:"); got != writers {
		t.Fatalf("summary blocks = %d, want %d\n%s", got, writers, text)
	}
	for index := range writers {
		stage := fmt.Sprintf("stage-%02d", index)
		if strings.Count(text, "### Stage: "+stage+"\n") != 1 {
			t.Errorf("stage block %s was missing or split", stage)
		}
		if strings.Count(text, "1:panic: "+stage+"\n") != 1 {
			t.Errorf("stage key line %s was missing or split", stage)
		}
	}
}

func statusFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runVerifyGit(t, root, "init", "--quiet")
	runVerifyGit(t, root, "config", "user.email", "test@example.com")
	runVerifyGit(t, root, "config", "user.name", "test")
	writeVerifyFixture(t, root, ".gitignore", "tmp/\n")
	writeVerifyFixture(t, root, "changed.txt", "committed\n")
	writeVerifyFixture(t, root, "other.txt", "committed\n")
	runVerifyGit(t, root, "add", ".")
	runVerifyGit(t, root, "commit", "--quiet", "-m", "fixture")
	writeVerifyFixture(t, root, "changed.txt", "dirty before run\n")
	writeVerifyFixture(t, root, "untracked.txt", "untracked\n")
	return root
}

func runVerifyGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func writeVerifyFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
