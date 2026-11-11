// Design: docs/contributing/running-commands.md -- testing one package while you develop it
// Related: test/unit/pkg.go -- the action these cases reach through the dispatcher
//
// VALIDATES: `le test unit pkg` is reachable from the dispatcher the binary
// uses: its help page lists its four words, and a `path` line reaches the
// action's body, observed at the quiet-run seam with the label, the toolchain's
// argv, the checkout root and the race environment.
// PREVENTS: an action that exists in its package's table and unit tests but
// that no typed line reaches.

package le

import (
	"slices"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	testunit "github.com/ze-software/ze/internal/le/test/unit"
)

// TestTestUnitPkgIsReachableFromDispatch asks the dispatcher for the action's
// page with the help word typed after it.
//
// VALIDATES: `le test unit pkg help` answers 0 with a page naming the action
// and each of its keywords.
// PREVENTS: a `pkg` row missing from the area's registered table, which the
// dispatcher answers with the area's page or a refusal instead.
func TestTestUnitPkgIsReachableFromDispatch(t *testing.T) {
	restore := testunit.SetQuietRunForTest(func(label string, argv []string, _ string, _ []string) (job.Report, int) {
		t.Errorf("the help question ran %s: %q", label, argv)
		return job.Report{}, 0
	})
	defer restore()

	page, code := dispatchToStdout(t, "test", "unit", "pkg", "help")
	if code != 0 {
		t.Fatalf("le test unit pkg help answered %d, want 0; page:\n%s", code, page)
	}
	if !strings.Contains(page, "test unit pkg") {
		t.Errorf("the page does not name the action:\n%s", page)
	}
	// pageListsWord (actions_test.go) matches a line that opens with the word,
	// so "vet" inside the summary's prose does not count as listed.
	for _, keyword := range []string{"path", "run", "vet", "changed"} {
		if !pageListsWord(page, keyword) {
			t.Errorf("the page does not list the keyword %q:\n%s", keyword, page)
		}
	}
}

// TestDispatchReachesThePkgActionBody types a `path` line into the real
// dispatcher, over this checkout, with only the quiet-run seam replaced.
//
// VALIDATES: the line resolves to the unit area, parses through the action's
// table, and reaches the body, which hands the seam the unit-pkg label, the
// toolchain's `go test` argv for the named package under this checkout's
// feature tags with -race, the checkout root and CGO_ENABLED=1; the seam's code
// is the line's exit code.
// PREVENTS: a body that the dispatcher cannot reach, and one reached with a
// tag set or a directory other than the checkout's.
func TestDispatchReachesThePkgActionBody(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("locate the checkout: %v", err)
	}
	tc, err := gotoolchain.New(root)
	if err != nil {
		t.Fatalf("load the checkout's toolchain: %v", err)
	}

	type call struct {
		label   string
		argv    []string
		dir     string
		environ []string
	}
	calls := make([]call, 0, 1)
	restore := testunit.SetQuietRunForTest(func(label string, argv []string, dir string, environ []string) (job.Report, int) {
		calls = append(calls, call{label: label, argv: slices.Clone(argv), dir: dir, environ: slices.Clone(environ)})
		return job.Report{Label: label, Command: argv, Quiet: true, Code: 7}, 7
	})
	defer restore()

	_, code := dispatchToStdout(t, "test", "unit", "pkg", "path", "./internal/le/runlog/")
	if code != 7 {
		t.Fatalf("the pkg line answered %d, want the quiet run's 7", code)
	}
	if len(calls) != 1 {
		t.Fatalf("the quiet run was called %d times, want once", len(calls))
	}
	got := calls[0]
	if got.label != "unit-pkg" {
		t.Errorf("the job label is %q, want unit-pkg", got.label)
	}
	want := []string{"go", "test", "-timeout", tc.Timeout, "-tags", tc.TestTags(), "-race", "./internal/le/runlog/"}
	if !slices.Equal(got.argv, want) {
		t.Errorf("argv differs:\n got: %q\nwant: %q", got.argv, want)
	}
	if got.dir != root {
		t.Errorf("the child runs in %q, want the checkout root %q", got.dir, root)
	}
	cgo := ""
	for _, entry := range slices.Backward(got.environ) {
		value, found := strings.CutPrefix(entry, "CGO_ENABLED=")
		if found {
			cgo = value
			break
		}
	}
	if cgo != "1" {
		t.Errorf("the child's CGO_ENABLED is %q, want 1 for the race detector", cgo)
	}
}
