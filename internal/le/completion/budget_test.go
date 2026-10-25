// Related: completion.go -- the callback whose wall time this measures

package completion_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	lepath "github.com/ze-software/ze/internal/le/le/path"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// budget is the wall time a keystroke may wait for its candidates (AC-37).
const budget = 100 * time.Millisecond

// buildTagLE is the tag that links le into the ze program.
const buildTagLE = "ze_le"

// VALIDATES: AC-37 -- the callback answers within budget on a warm binary:
// the median of ten runs of `le completion words spec cl` is under 100 ms.
// It measures the le binary built from this tree, the way the functional
// fixtures build the subject they drive (go build with ze_le and the daemon
// tags into a private directory), so the verdict never depends on how fresh
// the shared bin/le is. One run before the ten warms the binary, as the AC
// states: a script calls a binary the shell has already paged in.
// PREVENTS: a callback slow enough that Tab feels broken.
func TestCompletionCallbackAnswersWithinBudget(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("no checkout: %v", err)
	}
	gates, err := repofeaturetags.DaemonTags(root)
	if err != nil {
		t.Fatalf("read the feature manifest: %v", err)
	}
	binary := filepath.Join(t.TempDir(), "le")
	tags := strings.Join(append([]string{buildTagLE}, gates...), ",")
	build := exec.CommandContext(t.Context(), "go", "build", "-tags", tags, "-o", binary, "./cmd/ze") //nolint:gosec // the test chooses the program and its arguments
	build.Dir = root
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		t.Fatalf("build the le personality: %v\n%s", err, buildOutput.String())
	}

	ask := func() time.Duration {
		started := time.Now()
		cmd := exec.CommandContext(t.Context(), binary, "completion", "words", "spec", "cl") //nolint:gosec // the le binary this test built
		cmd.Dir = root
		out, err := cmd.Output()
		elapsed := time.Since(started)
		if err != nil {
			t.Fatalf("le completion words spec cl: %v", err)
		}
		if !strings.HasPrefix(string(out), "claim\t") {
			t.Fatalf("le completion words spec cl answered %q, want the claim record", out)
		}
		return elapsed
	}
	ask()
	runs := make([]time.Duration, 0, 10)
	for range 10 {
		runs = append(runs, ask())
	}
	slices.Sort(runs)
	median := (runs[4] + runs[5]) / 2
	if median >= budget {
		t.Errorf("median %v over ten runs, want under %v (runs %v)", median, budget, runs)
	}
}
