// The Python-lint port's contract.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- the same checkers run over the
// same scopes, in the same order, and the ratchet reads the same number from
// the same file.
// PREVENTS: a lint gate that reads green because its checker is absent. That is
// the shape this repository has been bitten by: it reads as "checked" when it
// means "not checked".

package pylint

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// recorder is a Linter whose forks are recorded rather than made. Every case
// below drives the real Run through it.
type recorder struct {
	calls   [][]string
	answers map[string]result
	missing map[string]bool
}

type result struct {
	out  string
	err  string
	code int
}

// linter answers a Linter wired to this recorder, over a tree carrying the
// given ceiling.
func (r *recorder) linter(root string) *Linter {
	return &Linter{
		Root: root,
		Which: func(name string) bool {
			return !r.missing[name]
		},
		Exec: func(argv []string, _ string) (string, string, int) {
			r.calls = append(r.calls, argv)
			if answer, ok := r.answers[strings.Join(argv, " ")]; ok {
				return answer.out, answer.err, answer.code
			}
			return "", "", 0
		},
	}
}

// labels answers the first two words of each recorded call, which is enough to
// say which checker ran in which mode.
func (r *recorder) labels() []string {
	out := make([]string, 0, len(r.calls))
	for _, call := range r.calls {
		out = append(out, strings.Join(call, " "))
	}
	return out
}

func newRecorder() *recorder {
	return &recorder{answers: map[string]result{}, missing: map[string]bool{}}
}

// ceilingTree writes a pyproject.toml declaring the given ceiling and answers
// its root.
func ceilingTree(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "pyproject.toml"), body)
	return root
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// --- The ceiling is data ----------------------------------------------------

func TestTheCeilingIsReadFromPyproject(t *testing.T) {
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 113\n")
	got, err := LegacyCeiling(root)
	if err != nil {
		t.Fatalf("LegacyCeiling: %v", err)
	}
	if got != 113 {
		t.Errorf("ceiling = %d, want 113", got)
	}
}

// TestTheCeilingIsReadFromItsOwnTable is what stops a key of the same name
// under another tool from being taken for this one.
func TestTheCeilingIsReadFromItsOwnTable(t *testing.T) {
	root := ceilingTree(t, "[tool.other]\nlegacy-max = 9\n\n[tool.le.lint]\nlegacy-max = 113\n\n[tool.third]\nlegacy-max = 4\n")
	got, err := LegacyCeiling(root)
	if err != nil {
		t.Fatalf("LegacyCeiling: %v", err)
	}
	if got != 113 {
		t.Errorf("ceiling = %d, want 113: a key from another table was read", got)
	}
}

// TestAMissingCeilingIsAnError verifies that the parser has no silent default.
// Zero makes the ratchet impossible to pass, while a large value disables it.
func TestAMissingCeilingIsAnError(t *testing.T) {
	for name, body := range map[string]string{
		"no table":       "[tool.other]\nlegacy-max = 9\n",
		"no key":         "[tool.le.lint]\nsomething = 1\n",
		"not an integer": "[tool.le.lint]\nlegacy-max = \"many\"\n",
		"empty file":     "",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := LegacyCeiling(ceilingTree(t, body)); err == nil {
				t.Errorf("a pyproject with %s answered %d rather than refusing", name, got)
			}
		})
	}
}

func TestAnAbsentPyprojectIsAnError(t *testing.T) {
	if _, err := LegacyCeiling(t.TempDir()); err == nil {
		t.Error("a tree with no pyproject.toml answered a ceiling")
	}
}

// --- What runs, and in what order -------------------------------------------

func TestACheckRunsEveryStageInOrder(t *testing.T) {
	rec := newRecorder()
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	report, code := rec.linter(root).Run(Options{})
	if code != 0 {
		t.Fatalf("a clean run exited %d\n%s", code, report.Text())
	}

	want := []string{
		"ruff check scripts/le le",
		"ruff format scripts/le le --check",
		"mypy",
		"ruff check --statistics --exclude scripts/le",
	}
	if got := rec.labels(); !slices.Equal(got, want) {
		t.Errorf("ran\n%v\nwant\n%v", got, want)
	}
}

// TestEveryStageRunsEvenAfterOneFails verifies the Python behavior.
// A lint pass must report the complete problem list instead of stopping at the first error.
func TestEveryStageRunsEvenAfterOneFails(t *testing.T) {
	rec := newRecorder()
	rec.answers["ruff check scripts/le le"] = result{out: "E501 line too long", code: 1}
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	report, code := rec.linter(root).Run(Options{})
	if code != 1 {
		t.Errorf("a failing stage exited %d, want 1", code)
	}
	if len(rec.calls) != 4 {
		t.Errorf("%d stages ran after a failure, want 4", len(rec.calls))
	}
	if !slices.Contains(report.Failed, "ruff check") {
		t.Errorf("the failure is not named: %v", report.Failed)
	}
}

func TestFixTurnsTheTwoRuffStagesIntoWriters(t *testing.T) {
	rec := newRecorder()
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	if _, code := rec.linter(root).Run(Options{Fix: true}); code != 0 {
		t.Fatalf("a clean fix run exited %d", code)
	}
	labels := rec.labels()
	if !slices.Contains(labels, "ruff check scripts/le le --fix") {
		t.Errorf("the linter did not apply its fixes: %v", labels)
	}
	if !slices.Contains(labels, "ruff format scripts/le le") {
		t.Errorf("the formatter still ran in check mode: %v", labels)
	}
}

func TestStrictSkipsTheLegacyRatchet(t *testing.T) {
	rec := newRecorder()
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	if _, code := rec.linter(root).Run(Options{StrictOnly: true}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, label := range rec.labels() {
		if strings.Contains(label, "--statistics") {
			t.Errorf("the legacy ratchet ran under strict: %v", rec.labels())
		}
	}
}

func TestTypesOnlyRunsTheTypeCheckerAlone(t *testing.T) {
	rec := newRecorder()
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	if _, code := rec.linter(root).Run(Options{TypesOnly: true}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if got := rec.labels(); !slices.Equal(got, []string{"mypy"}) {
		t.Errorf("ran %v, want mypy alone", got)
	}
}

func TestLintOnlyRunsTheLinterAlone(t *testing.T) {
	rec := newRecorder()
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	if _, code := rec.linter(root).Run(Options{LintOnly: true}); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, label := range rec.labels() {
		if label == "mypy" {
			t.Errorf("the type checker ran under lint-only: %v", rec.labels())
		}
	}
	if len(rec.calls) != 3 {
		t.Errorf("%d stages ran, want the three ruff stages", len(rec.calls))
	}
}

// --- An absent checker is a failure, not a skip ------------------------------

// TestAnAbsentTypeCheckerFailsTheRun is the whole reason this gate exists in
// the shape it does. mypy is required, and a gate that reports green because
// its checker is absent reads as "checked" when it means "not checked".
func TestAnAbsentTypeCheckerFailsTheRun(t *testing.T) {
	rec := newRecorder()
	rec.missing["mypy"] = true
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	report, code := rec.linter(root).Run(Options{})
	if code == 0 {
		t.Fatal("an absent mypy passed the run")
	}
	if !slices.Contains(report.Failed, "mypy (not installed)") {
		t.Errorf("the absence is not named: %v", report.Failed)
	}
}

func TestAnAbsentLinterFailsTheRun(t *testing.T) {
	rec := newRecorder()
	rec.missing["ruff"] = true
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	report, code := rec.linter(root).Run(Options{})
	if code == 0 {
		t.Fatal("an absent ruff passed the run")
	}
	if !slices.Contains(report.Failed, "ruff (not installed)") {
		t.Errorf("the absence is not named: %v", report.Failed)
	}
	// The legacy ratchet needs the same binary, so it contributes no second
	// failure for one missing tool.
	if slices.Contains(report.Failed, "ruff check (legacy)") {
		t.Errorf("one missing tool produced two failures: %v", report.Failed)
	}
}

// --- The ratchet ------------------------------------------------------------

// statistics is what `ruff check --statistics` prints: a count, a rule code and
// a description per line.
const statistics = "  100\tQ000\t[*] Single quotes found\n" +
	"   13\tANN001\tMissing type annotation\n"

func TestTheRatchetFailsWhenTheCountRises(t *testing.T) {
	rec := newRecorder()
	rec.answers["ruff check --statistics --exclude scripts/le"] = result{out: statistics, code: 1}
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 100\n")

	report, code := rec.linter(root).Run(Options{})
	if code == 0 {
		t.Fatal("113 findings passed a ceiling of 100")
	}
	if !slices.Contains(report.Failed, "ruff check (legacy)") {
		t.Errorf("the ratchet failure is not named: %v", report.Failed)
	}
	if report.Findings != 113 {
		t.Errorf("Findings = %d, want 113", report.Findings)
	}
}

// TestTheRatchetFailsWhenTheCountFalls is the half that keeps the ceiling
// meaningful. A ceiling nobody lowers stops saying anything.
func TestTheRatchetFailsWhenTheCountFalls(t *testing.T) {
	rec := newRecorder()
	rec.answers["ruff check --statistics --exclude scripts/le"] = result{out: statistics, code: 1}
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 200\n")

	report, code := rec.linter(root).Run(Options{})
	if code == 0 {
		t.Fatal("a stale ceiling passed")
	}
	if !slices.Contains(report.Failed, "ruff check (legacy ceiling is stale)") {
		t.Errorf("the stale ceiling is not named: %v", report.Failed)
	}
}

func TestTheRatchetPassesAtTheCeiling(t *testing.T) {
	rec := newRecorder()
	rec.answers["ruff check --statistics --exclude scripts/le"] = result{out: statistics, code: 1}
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 113\n")

	report, code := rec.linter(root).Run(Options{})
	if code != 0 {
		t.Fatalf("a count at the ceiling exited %d: %v", code, report.Failed)
	}
}

func TestTheFindingCountSumsTheStatisticsTable(t *testing.T) {
	for body, want := range map[string]int{
		"":                                0,
		"  100\tQ000\tSingle quotes\n":    100,
		"1\tA\tone\n2\tB\ttwo\n3\tC\tc\n": 6,
		"Found 3 errors.\n":               0,
	} {
		if got := countFindings(body); got != want {
			t.Errorf("countFindings(%q) = %d, want %d", body, got, want)
		}
	}
}

// --- The rendering -----------------------------------------------------------

func TestACleanRunSaysSo(t *testing.T) {
	rec := newRecorder()
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")
	report, _ := rec.linter(root).Run(Options{})

	if !strings.Contains(report.Text(), "Python lint and types clean.") {
		t.Errorf("a clean run rendered:\n%s", report.Text())
	}
}

func TestAFailedRunNamesEveryFailedStage(t *testing.T) {
	rec := newRecorder()
	rec.answers["ruff check scripts/le le"] = result{code: 1}
	rec.answers["mypy"] = result{code: 1}
	root := ceilingTree(t, "[tool.le.lint]\nlegacy-max = 0\n")

	report, _ := rec.linter(root).Run(Options{})
	if got := report.Text(); !strings.Contains(got, "Failed: ruff check, mypy") {
		t.Errorf("rendered:\n%s", got)
	}
}
