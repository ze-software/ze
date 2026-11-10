// VALIDATES: the error-wrap gate refuses a changed fmt.Errorf whose literal
// segments repeat one the wrapped error already carries, one call away, two
// calls away and through local function literals, names both sites, leaves a
// wrap that adds new context alone, judges only changed lines, and counts the
// wrap whose producer arrives as a function value instead of passing it.
// PREVENTS: an operator reading "reload: parse config: parse config: ...".
package archerrorwrap

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// fixtureModule is the module path every tree under testdata is read as, so
// a fixture needs no go.mod of its own.
const fixtureModule = "example.com/fix"

// fixtureTree answers the absolute path of one fixture tree.
func fixtureTree(t *testing.T, name string) string {
	t.Helper()
	tree, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return tree
}

// checkHub runs the check over a fixture with the given lines of hub/hub.go
// changed.
func checkHub(t *testing.T, name string, spans []repochanged.LineSpan) CheckReport {
	t.Helper()
	report, err := check(fixtureTree(t, name), fixtureModule, repochanged.ChangedLines{"hub/hub.go": spans})
	if err != nil {
		t.Fatalf("check(%s): %v", name, err)
	}
	return report
}

// wholeFile marks every line of a fixture file changed.
var wholeFile = []repochanged.LineSpan{{From: 1, To: 1000}}

// TestErrorWrapRefusesRepeatedPrefix proves the owned shape: loader.Parse
// returns "parse config: %w" and the hub wraps it in "reload: parse config:
// %w". The finding names the wrap and the constructor that first wrote the
// segment, the text names both file:line sites and the fix, and the exit code
// is 1.
func TestErrorWrapRefusesRepeatedPrefix(t *testing.T) {
	report := checkHub(t, "repeated", wholeFile)
	want := Findings{{
		File: "hub/hub.go", Line: 13, Fn: "Reload", Segment: "parse config",
		OriginFile: "loader/loader.go", OriginLine: 13, OriginFn: "Parse",
	}}
	if !slices.Equal(report.Findings, want) {
		t.Fatalf("findings = %+v, want %+v", report.Findings, want)
	}
	if report.exitCode() != 1 {
		t.Fatalf("exit code = %d, want 1", report.exitCode())
	}
	if report.Wraps != 1 {
		t.Fatalf("wraps judged = %d, want 1", report.Wraps)
	}
	text := report.Text()
	for _, part := range []string{"hub/hub.go:13", "loader/loader.go:13", `"parse config"`, "twice", "Fix: drop the segment"} {
		if !strings.Contains(text, part) {
			t.Errorf("text lacks %q:\n%s", part, text)
		}
	}
}

// TestErrorWrapAcceptsNewContext proves a wrap whose segment the wrapped error
// does not carry is no finding, while it is still judged and traced.
func TestErrorWrapAcceptsNewContext(t *testing.T) {
	report := checkHub(t, "newcontext", wholeFile)
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", report.Findings)
	}
	if report.Wraps != 1 {
		t.Fatalf("wraps judged = %d, want 1", report.Wraps)
	}
	if report.Untraced != 0 {
		t.Fatalf("untraced = %d, want 0: loader.Parse is a named callee", report.Untraced)
	}
	if report.exitCode() != 0 {
		t.Fatalf("exit code = %d, want 0", report.exitCode())
	}
}

// TestErrorWrapJudgesOnlyChangedLines proves the change-set scope: with only
// the hub's final return changed, the repeated wrap on line 13 is not due.
func TestErrorWrapJudgesOnlyChangedLines(t *testing.T) {
	report := checkHub(t, "repeated", []repochanged.LineSpan{{From: 15, To: 15}})
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %+v, want none: line 13 did not change", report.Findings)
	}
	if report.Wraps != 0 {
		t.Fatalf("wraps judged = %d, want 0", report.Wraps)
	}
}

// TestErrorWrapRefusesTwoLevelChain is the A-3 fixture: loader.Load passes up
// unwrapped what loader.parse built with "parse config: %w", so the segment
// sits two calls below the wrap. The trace follows the pass-through return.
func TestErrorWrapRefusesTwoLevelChain(t *testing.T) {
	report := checkHub(t, "twolevel", wholeFile)
	want := Findings{{
		File: "hub/hub.go", Line: 13, Fn: "Reload", Segment: "parse config",
		OriginFile: "loader/loader.go", OriginLine: 21, OriginFn: "parse",
	}}
	if !slices.Equal(report.Findings, want) {
		t.Fatalf("findings = %+v, want %+v", report.Findings, want)
	}
}

// TestErrorWrapRefusesThroughLocalLiterals proves the shape diskConfigLoaders
// builds: the wrapped error comes from a local function literal that calls
// another local literal, which calls the loader.
func TestErrorWrapRefusesThroughLocalLiterals(t *testing.T) {
	report := checkHub(t, "closure", wholeFile)
	want := Findings{{
		File: "hub/hub.go", Line: 20, Fn: "Reload", Segment: "parse config",
		OriginFile: "loader/loader.go", OriginLine: 13, OriginFn: "Parse",
	}}
	if !slices.Equal(report.Findings, want) {
		t.Fatalf("findings = %+v, want %+v", report.Findings, want)
	}
}

// TestErrorWrapCountsAFunctionValueItCannotTrace proves the check says so
// when it cannot look: Reload's loader arrives as a parameter, so the syntax
// names no producer, and the wrap is counted as untraced rather than
// reported clean in silence. This is the shape of the hub's real reload,
// whose load function is a parameter four calls from its construction.
func TestErrorWrapCountsAFunctionValueItCannotTrace(t *testing.T) {
	report := checkHub(t, "funcparam", wholeFile)
	if report.Wraps != 1 {
		t.Fatalf("wraps judged = %d, want 1", report.Wraps)
	}
	if report.Untraced != 1 {
		t.Fatalf("untraced = %d, want 1", report.Untraced)
	}
	if !strings.Contains(report.Text(), "1 whose wrapped error could not be traced") {
		t.Fatalf("text does not count the untraced wrap:\n%s", report.Text())
	}
}

// TestErrorWrapReportListsTheTree proves `report` judges every file under its
// roots, changed or not, and names its scope.
func TestErrorWrapReportListsTheTree(t *testing.T) {
	answer, err := report(fixtureTree(t, "repeated"), fixtureModule, []string{"hub", "loader"})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if answer.Files != 2 {
		t.Fatalf("files = %d, want 2", answer.Files)
	}
	if len(answer.Findings) != 1 {
		t.Fatalf("findings = %+v, want the one repeated wrap", answer.Findings)
	}
	if !strings.Contains(answer.Text(), "every Go file under hub, loader") {
		t.Fatalf("text does not name the scope:\n%s", answer.Text())
	}
}

// TestModulePathReadsGoMod proves Check reads the module path a fixture
// bypasses, and refuses a tree with no module line.
func TestModulePathReadsGoMod(t *testing.T) {
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "go.mod"), []byte("module example.com/m\n\ngo 1.27\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	module, err := modulePath(tree)
	if err != nil {
		t.Fatalf("modulePath: %v", err)
	}
	if module != "example.com/m" {
		t.Fatalf("module = %q, want example.com/m", module)
	}
	if _, err := modulePath(t.TempDir()); err == nil {
		t.Fatal("modulePath of a tree with no go.mod answered no error")
	}
}

// TestFormatSegments proves which parts of a format string are segments: the
// literal colon-terminated parts before the first %w, and for a format with
// no %w its verb-free parts.
func TestFormatSegments(t *testing.T) {
	cases := []struct {
		format   string
		produced []string
		wrapped  []int
	}{
		{"reload: parse config: %w", []string{"reload", "parse config"}, []int{0}},
		{"parse config: %w\n\n%s", []string{"parse config"}, []int{0}},
		{"peer %s: parse config: %w", []string{"parse config"}, []int{1}},
		{"failed %w", nil, []int{0}},
		{"%w", nil, []int{0}},
		{"line %d: unknown keyword %q", nil, nil},
		{"read config: no file", []string{"read config", "no file"}, nil},
		{"100%% done: %w", []string{"100% done"}, []int{0}},
		{"%*d of %d: %w", nil, []int{3}},
	}
	for _, tc := range cases {
		form, parsed := parseFormat(tc.format)
		if !parsed {
			t.Errorf("%q: not parsed", tc.format)
			continue
		}
		if !slices.Equal(form.produced(), tc.produced) {
			t.Errorf("%q: segments = %q, want %q", tc.format, form.produced(), tc.produced)
		}
		if !slices.Equal(form.wrapped, tc.wrapped) {
			t.Errorf("%q: wrapped operands = %v, want %v", tc.format, form.wrapped, tc.wrapped)
		}
	}
	if _, parsed := parseFormat("%[2]w: %[1]s"); parsed {
		t.Error("an explicit argument index was parsed: its operand would be guessed")
	}
}
