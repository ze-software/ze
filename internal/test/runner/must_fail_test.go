// VALIDATES: every .ci under testdata/mustfail/ still FAILS, for the reason it
//            names in its own header, at parse time or at run time.
// PREVENTS: the class this runner has produced four times in one week -- a .ci
//            that reads green while asserting nothing, because the runner
//            answered a directive it did not understand instead of refusing it.
//            No static check finds that in general: the file is well formed and
//            the assertion is real, and only the STIMULUS is wrong. The one
//            general detector is a forced red, and the only way to KEEP a
//            forced red is to store it, which is what this directory is.
//
// A fixture that starts passing fails this gate by name. That is the whole
// mechanism: each fixture is a refusal or a judgement the runner owes, and a
// runner that stops owing it turns its fixture green.
//
// The fixtures reach two layers and neither one needs a built ze. Four are
// parse-time refusals, which is where three of the four known instances of the
// class lived. Two run a real child from /bin, so they judge the run path: that
// a named stdin block reaches the child's standard input, and that a declared
// exit code is still compared. The `ze`-specific stdin routing (a daemon `-`
// becomes a file, a verb's `-` is piped) is proven by TestCIStdinPipesForNonDaemonZeCommand
// over routeStdinBlock, because reaching it from a .ci needs the binary.

package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// mustFailDir holds one .ci per refusal or judgement the runner owes.
const mustFailDir = "testdata/mustfail"

// mustFailMarker opens the line a fixture states its expected failure on. The
// text is compared against the failure, so a fixture that goes red for a NEW
// reason fails this gate as loudly as one that goes green: a must-fail suite
// that stops discriminating is the same defect one level up (R-4).
const mustFailMarker = "# must-fail: "

func TestCIMustFailFixturesAllFail(t *testing.T) {
	entries, err := os.ReadDir(mustFailDir)
	// Deliberately fatal, not a skip. A gate that disappears when its fixtures
	// move reads green forever (ai/rules/evidence.md).
	require.NoError(t, err, "the must-fail fixtures must be reachable")
	require.NotEmpty(t, entries, "an empty must-fail suite passes every assertion below and proves nothing")

	fixtures := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ci") {
			continue
		}
		fixtures++
		t.Run(entry.Name(), func(t *testing.T) {
			runMustFailFixture(t, filepath.Join(mustFailDir, entry.Name()))
		})
	}
	require.NotZero(t, fixtures, "no .ci in %s: the directory is there and the gate is reading nothing", mustFailDir)
}

// runMustFailFixture parses one fixture and, when it parses, runs it. Either
// step may produce the failure; neither producing one fails the gate.
func runMustFailFixture(t *testing.T, path string) {
	t.Helper()

	want := mustFailExpectation(t, path)

	ResetNickCounter()
	baseDir := t.TempDir()
	et := NewEncodingTests(baseDir)

	rec, parseErr := et.parseAndAdd(path)
	if parseErr != nil {
		require.Contains(t, parseErr.Error(), want,
			"%s failed to parse for a reason it does not name; its header says what it is FOR", path)
		return
	}

	runner, err := NewRunner(et, baseDir)
	require.NoError(t, err)
	defer runner.Cleanup()

	rec.Active = true
	passed := runner.runTest(t.Context(), rec, &RunOptions{})
	require.False(t, passed,
		"%s PASSED. It exists to fail: read its header for the refusal or the judgement the runner has stopped making", path)

	// The failure text, not the output the child produced. A fixture that
	// matched its expectation against stdout would accept a run that failed for
	// an unrelated reason while the needle sat in the output.
	require.Error(t, rec.Error, "%s failed with no error to read: the gate cannot tell why", path)
	got := rec.Error.Error() + " " + rec.FailureType
	require.Contains(t, got, want,
		"%s failed for a reason it does not name; a must-fail fixture that goes red for a new reason asserts nothing", path)
}

// mustFailExpectation reads the failure text a fixture names. A fixture without
// one fails the gate: the verdict alone is not the assertion, the reason is.
func mustFailExpectation(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path) //nolint:gosec // a fixture under this package's own testdata
	require.NoError(t, err)

	for line := range strings.Lines(string(body)) {
		text, ok := strings.CutPrefix(strings.TrimSpace(line), mustFailMarker)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		require.NotEmpty(t, text, "%s names an empty expectation, which every failure contains", path)
		return text
	}

	t.Fatalf("%s carries no %q line: a fixture that names no reason cannot be told from one that broke", path, mustFailMarker)
	return ""
}
