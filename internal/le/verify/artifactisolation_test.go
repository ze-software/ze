// VALIDATES: two verification runs of one checkout keep their logs and their
// failure index apart, so a reader can attribute an artifact to the run that
// wrote it (spec-shared-machine-job-admission, AC-9).
package verify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestConcurrentRunsDoNotShareArtifactPaths is AC-9. Admission lets more than
// one heavy job run at once, so two verification runs of one checkout overlap
// by design. If both wrote to one set of paths, the failure index would name
// stages the reader's own run never ran, and a red would be charged to the
// wrong tree.
//
// The two runs are given DIFFERENT stage output, which is what makes the
// assertion discriminating: a shared directory would put one run's output
// under the other's name, and each run is checked for its own.
func TestConcurrentRunsDoNotShareArtifactPaths(t *testing.T) {
	repo := newFixtureRepo(t)
	repo.commit(t, "fixture", "one")

	reports := make([]verifyengine.Report, 2)
	marks := [2]string{"FIRST-RUN-MARK", "SECOND-RUN-MARK"}

	var group sync.WaitGroup
	for index := range reports {
		group.Go(func() {
			mark := marks[index]
			runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
				return verifyengine.ActionResult{
					Identity: identity, Registered: true, Completed: true,
					Output: mark + " " + identity.Name + "\n",
				}
			}
			reports[index] = runCurrent(context.Background(), repo.root, "changed", runner)
		})
	}
	group.Wait()

	// Admission is entitled to answer the second run with the first one's
	// verdict instead of running the same work over the same tree twice, and
	// which run wins that race is the scheduler's business. So the isolation
	// claim is about the runs that JUDGED, and the run that attached is held to
	// the other half of the promise: that it says it attached and names where
	// the holder's evidence is. Reading the attached report as a broken run is
	// what made this case fail under whole-tree load while proving nothing.
	var judged []int
	for index, report := range reports {
		if report.Attached != nil {
			if report.Attached.Log == "" {
				t.Errorf("run %d attached and named no log, so the holder's evidence is unreachable", index)
			}
			continue
		}
		if report.Code != 0 || !report.Completed {
			t.Fatalf("run %d = %#v", index, report)
		}
		if report.LogDir == "" {
			t.Fatalf("run %d recorded no log directory, so nothing it wrote is attributable", index)
		}
		judged = append(judged, index)
	}
	if len(judged) == 0 {
		t.Fatal("both reports attached, so neither run judged the tree and no artifact was written")
	}
	t.Logf("%d of 2 runs judged the tree; the rest took the holder's verdict", len(judged))

	if len(judged) == 2 && reports[0].LogDir == reports[1].LogDir {
		t.Fatalf("both runs wrote to %s: one run's artifacts overwrite the other's", reports[0].LogDir)
	}

	for _, index := range judged {
		body := readCombined(t, repo.root, reports[index].LogDir)
		if !strings.Contains(body, marks[index]) {
			t.Errorf("run %d's log holds %q, want its own output", index, body)
		}
		if other := marks[1-index]; strings.Contains(body, other) {
			t.Errorf("run %d's log holds %s, which is the other run's output", index, other)
		}
	}
}

// readCombined answers the combined log one run wrote inside its own directory.
func readCombined(t *testing.T, root, logDir string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(logDir), "ze-verify.log")
	body, err := os.ReadFile(path) //nolint:gosec // a path this test's own run reported
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// TestARunThatAttachedSaysSoAndNamesTheHolder is the other half of AC-9, and it
// is the half nothing covered.
//
// VALIDATES: a report that took another run's verdict carries Attached, naming
// the registry entry and the log where that run's evidence is, and carries the
// holder's code.
// PREVENTS: a green nobody earned. An attached run judges nothing, so it has no
// stages, no log directory and Completed false; with a zero Code and no marker
// it is indistinguishable from a run that broke before it started, which is the
// reading admissionFailure refuses to publish for the other no-verdict path.
// `le verify worktree` has said it attached since it was written, and this
// entry point said nothing.
//
// The ticket is the input rather than a real race: attaching needs a second run
// to admit while a first still holds the label, and a test that waits for that
// window asserts on state the scheduler owns
// (plan/journal/assertion-reads-state-the-scheduler-owns.md).
func TestARunThatAttachedSaysSoAndNamesTheHolder(t *testing.T) {
	const commit = "a2fb7acb1a01b38e3b7dffcc51766d1fb08044a5"
	report := attachedReport("changed", commit, &job.Ticket{
		Label: "verify", Kind: job.KindAttached, Code: 1,
		Entry: "tmp/job/verify/1234", Log: "tmp/job/verify/1234/log",
	})

	if report.Attached == nil {
		t.Fatalf("the report does not say it attached: %#v", report)
	}
	if report.Attached.Entry == "" || report.Attached.Log == "" {
		t.Errorf("attached = %+v, and a reader cannot reach the holder's evidence", report.Attached)
	}
	if report.Code != 1 {
		t.Errorf("code = %d, want the holder's verdict", report.Code)
	}
	if report.LogDir != "" || len(report.Stages) != 0 || report.Completed {
		t.Errorf("the attached run reports work of its own: %#v", report)
	}
	if !strings.Contains(report.Console, "shared the verification already running") {
		t.Errorf("the operator is told nothing: %q", report.Console)
	}
}
