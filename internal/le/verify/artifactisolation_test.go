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

	for index, report := range reports {
		if report.Code != 0 || !report.Completed {
			t.Fatalf("run %d = %#v", index, report)
		}
		if report.LogDir == "" {
			t.Fatalf("run %d recorded no log directory, so nothing it wrote is attributable", index)
		}
	}
	if reports[0].LogDir == reports[1].LogDir {
		t.Fatalf("both runs wrote to %s: one run's artifacts overwrite the other's", reports[0].LogDir)
	}

	for index, report := range reports {
		body := readCombined(t, repo.root, report.LogDir)
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
