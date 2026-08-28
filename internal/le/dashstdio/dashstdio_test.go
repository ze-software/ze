// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the dash-stdio guard is
// called as a function, answers structured rows, and keeps its exit codes apart.
// PREVENTS: a port whose taint analysis silently narrowed. This checkout draws
// zero findings, so a guard that stopped following a path through a helper and
// a clean tree print the same page.

package dashstdio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// fixtureTree writes every selftest fixture into a temporary tree and answers
// its root.
func fixtureTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := WriteFixture(dir); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	return dir
}

// VALIDATES: this checkout passes the gate and the walk actually read it.
// PREVENTS: a port whose walk roots resolve to nothing, which looks identical to
// a clean tree.
func TestTheRealCheckoutPasses(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	findings, err := Check(tree, scanFloor)
	if err != nil {
		t.Fatalf("the gate could not read this checkout: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("this checkout fails the dash-stdio gate:\n%s", findings.Text())
	}
}

// VALIDATES: a tree too small to be the one asked about is an ERROR.
// PREVENTS: a run whose walk roots exist but hold nothing reporting OK, which is
// the same false green as a walk that never happened.
func TestATreeTooSmallToBeTheOneAskedAboutIsAnError(t *testing.T) {
	if _, err := Check(t.TempDir(), scanFloor); err == nil {
		t.Fatal("a tree holding none of the scan roots passed the floor")
	}
}

// VALIDATES: both polarities over one tree -- each violating shape is drawn and
// each legitimate shape is left alone.
// PREVENTS: the taint analysis losing a hop, an alias, or the allow marker,
// which no clean-tree run can show.
func TestEveryFixtureDrawsWhatItDeclares(t *testing.T) {
	dir := fixtureTree(t)
	findings, err := scan(dir, []string{dir}, 0)
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}

	drawn := map[string]int{}
	for _, finding := range findings {
		drawn[strings.Split(finding.File, "/")[0]]++
	}

	flagged, silent := 0, 0
	for _, testCase := range selftestCases {
		if (drawn[testCase.name] > 0) != testCase.flagged {
			t.Errorf("fixture %s drew %d findings, want flagged=%v: %s",
				testCase.name, drawn[testCase.name], testCase.flagged, testCase.why)
		}
		if testCase.flagged {
			flagged++
			continue
		}
		silent++
	}
	if flagged < 9 || silent < 5 {
		t.Errorf("the table holds %d must-flag and %d must-not-flag fixtures; both polarities are what make a silent gate meaningful", flagged, silent)
	}
}

// VALIDATES: a file the parser cannot read stops the run.
// PREVENTS: a raw path call nobody parsed being counted as no raw path call at
// all.
func TestAFileThatWillNotParseStopsTheRun(t *testing.T) {
	dir := fixtureTree(t)
	if err := os.WriteFile(filepath.Join(dir, "direct", "bad.go"), []byte("package p\n\nfunc broken( {\n"), 0o600); err != nil {
		t.Fatalf("write the broken file: %v", err)
	}

	_, err := scan(dir, []string{dir}, 0)
	if err == nil {
		t.Fatal("a file that will not parse was walked past")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("the error is %v, want one naming the parse", err)
	}
}

// VALIDATES: the finding carries the SOURCE LINE it was drawn from.
// PREVENTS: the port reading the file a second time for its code column, which
// is what the script did while dropping that read's error: a file that became
// unreadable in between yielded findings with a blank column.
func TestEveryFindingCarriesItsSourceLine(t *testing.T) {
	dir := fixtureTree(t)
	findings, err := scan(dir, []string{dir}, 0)
	if err != nil {
		t.Fatalf("the gate failed over the fixture: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("the fixture drew no finding at all")
	}
	for _, finding := range findings {
		if finding.Code == "" {
			t.Errorf("%s:%d carries no code line", finding.File, finding.Line)
		}
		if finding.Fn == "" {
			t.Errorf("%s:%d names no os function", finding.File, finding.Line)
		}
	}
}

// VALIDATES: the selftest table passes over its own fixtures and answers 0.
// PREVENTS: a selftest that cannot pass, which would be discovered only by the
// gate that runs it.
func TestTheSelftestPassesOverItsOwnFixtures(t *testing.T) {
	report, err := Selftest()
	if err != nil {
		t.Fatalf("the selftest could not run: %v", err)
	}
	if failures := report.Failures(); len(failures) != 0 {
		t.Errorf("the selftest failed: %v", failures)
	}
	if code := report.Code(1); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d rows for %d cases", len(report.Results), len(selftestCases))
	}
	for i, result := range report.Results {
		if result.Case != selftestCases[i].name {
			t.Errorf("row %d names %q, want %q", i, result.Case, selftestCases[i].name)
		}
	}
}

// VALIDATES: AC-7 -- the payload is data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestFindingsAreStructuredRows(t *testing.T) {
	raw, err := json.Marshal(Findings{{File: "a.go", Line: 3, Fn: "ReadFile", Code: "os.ReadFile(fs.Arg(0))"}})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	if !strings.HasPrefix(string(raw), "[") {
		t.Errorf("the payload is not an array: %s", raw)
	}
	for _, key := range []string{`"file"`, `"line"`, `"fn"`, `"code"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// VALIDATES: the page says OK when there is nothing, and carries the site and
// the remedy when there is.
// PREVENTS: a failing gate whose page still reads OK, which is what a reader
// acts on.
func TestThePageCarriesItsVerdictAndItsRemedy(t *testing.T) {
	if clean := (Findings{}).Text(); clean != "cli-dash-stdio: OK\n" {
		t.Errorf("the clean page is %q", clean)
	}

	found := Findings{{File: "a.go", Line: 3, Fn: "ReadFile", Code: "os.ReadFile(fs.Arg(0))"}}.Text()
	if strings.Contains(found, "OK") {
		t.Errorf("a page holding a finding still says OK:\n%s", found)
	}
	if !strings.Contains(found, "a.go:3 (os.ReadFile)") {
		t.Errorf("the page does not carry its site:\n%s", found)
	}
	if !strings.Contains(found, "internal/core/cliio") {
		t.Errorf("the page does not carry the remedy:\n%s", found)
	}
}

// VALIDATES: the area dispatches its two actions and refuses the two mistakes.
// PREVENTS: a verb that drifts from its registered native action and becomes
// unreachable.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	if _, code := Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own fixtures, want 0", code)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "value"}); code != 2 {
		t.Errorf("a value after an action answers %d, want 2", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want 2", len(verbs.Actions))
	}
	if verbs.Actions[0].Verb != "check" || verbs.Actions[1].Verb != "selftest" {
		t.Errorf("the verbs are %q and %q, want check and selftest", verbs.Actions[0].Verb, verbs.Actions[1].Verb)
	}
}
