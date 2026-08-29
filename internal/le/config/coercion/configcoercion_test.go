// Related: configcoercion.go -- the guard these tests drive from its entry point
//
// Every test here calls the tool as a function. The gate used to be reachable
// only as a subprocess, and the only case a subprocess could assert about this
// checkout was "the tree passes".

package configcoercion

import (
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/leroot"
)

// VALIDATES: each selftest fixture draws exactly the findings it declares.
// PREVENTS: the AST detection breaking silently. This is the case the script's
// --selftest carried, and it is asserted here PER FIXTURE rather than as one
// count, so a failure names which shape the guard stopped seeing.
func TestEachSelftestFixtureDrawsWhatItDeclares(t *testing.T) {
	dir := t.TempDir()
	fset := token.NewFileSet()

	for _, testCase := range selftestCases {
		t.Run(testCase.name, func(t *testing.T) {
			switches, asserts, err := testCase.countKinds(fset, dir)
			if err != nil {
				t.Fatalf("run the fixture: %v", err)
			}
			if switches != testCase.switches || asserts != testCase.asserts {
				t.Errorf("%s drew %d switch and %d assert finding(s), want %d and %d: %s",
					testCase.name, switches, asserts, testCase.switches, testCase.asserts, testCase.why)
			}
		})
	}
}

// VALIDATES: the selftest answers one result per case and passes over the
// fixtures it ships with.
// PREVENTS: a selftest that reports OK having run nothing, which is the same
// vacuity the guard itself exists to prevent.
func TestSelftestAnswersOneResultPerCase(t *testing.T) {
	report, err := Selftest()
	if err != nil {
		t.Fatalf("run the selftest: %v", err)
	}
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d results, want one per fixture (%d)", len(report.Results), len(selftestCases))
	}
	if failed := report.Failures(); len(failed) != 0 {
		t.Errorf("the selftest failed: %v", failed)
	}
	for i, result := range report.Results {
		if result.Case != selftestCases[i].name {
			t.Errorf("result %d names %q, want %q", i, result.Case, selftestCases[i].name)
		}
	}
}

// VALIDATES: a fixture that draws the wrong counts is scored as a FAILURE, and
// the right counts as a pass.
// PREVENTS: a selftest that reports every case as passed whatever the guard
// drew, which is the vacuity the selftest itself exists to catch, one level up.
func TestAWrongCountIsScoredAsAFailure(t *testing.T) {
	fixture := selftestCase{name: "buggy_switch", switches: 1, why: "not flagged"}

	if result := fixture.verdict(1, 0); !result.Passed || result.Detail != "" {
		t.Errorf("the declared counts score %#v, want a pass with no detail", result)
	}
	for _, wrong := range [][2]int{{0, 0}, {2, 0}, {1, 1}} {
		result := fixture.verdict(wrong[0], wrong[1])
		if result.Passed {
			t.Errorf("drawing %d switch and %d assert finding(s) scored a pass", wrong[0], wrong[1])
		}
		if !strings.Contains(result.Detail, "not flagged") {
			t.Errorf("the detail is %q, want it to say what the failure means", result.Detail)
		}
	}
}

// VALIDATES: a failing case carries a detail naming what it means, and the page
// prints it.
// PREVENTS: a selftest that says FAILED and not which fixture, which is what
// sends a reader to the source instead of the fix.
func TestAFailingSelftestCaseSaysWhat(t *testing.T) {
	report := leroot.NewSelftestReport(
		"config-string-coercion selftest OK",
		"config-string-coercion selftest FAILED:",
		leroot.Pass("good_switch"),
		leroot.Fail("buggy_switch", "numeric type switch without case string: not flagged"),
	)
	if len(report.Failures()) != 1 {
		t.Fatalf("the report answers %d failures, want 1", len(report.Failures()))
	}
	text := report.Text()
	if !strings.HasPrefix(text, "config-string-coercion selftest FAILED:\n") {
		t.Errorf("the page does not open with the verdict:\n%s", text)
	}
	if !strings.Contains(text, "  numeric type switch without case string: not flagged\n") {
		t.Errorf("the page does not name the failing case:\n%s", text)
	}
	passing := leroot.NewSelftestReport(
		"config-string-coercion selftest OK",
		"config-string-coercion selftest FAILED:",
		leroot.Pass("a"))
	if got := passing.Text(); got != "config-string-coercion selftest OK\n" {
		t.Errorf("a passing selftest renders %q", got)
	}
}

// VALIDATES: only a file NAMED config.go is scanned, an allowlisted one is
// skipped, and a test file is not scanned at all.
// PREVENTS: the guard widening to code it was never about, and the allowlist
// silently doing nothing.
func TestOnlyConfigFilesAreScanned(t *testing.T) {
	dir := t.TempDir()
	buggy := "package p\n\nfunc f(m map[string]any) bool { b, _ := m[\"x\"].(bool); return b }\n"
	for rel, body := range map[string]string{
		"internal/a/config.go":      buggy,
		"internal/b/parser.go":      buggy,
		"internal/c/config_test.go": buggy,
	} {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	findings, err := Check(dir)
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if len(findings) != 1 || findings[0].File != "internal/a/config.go" {
		t.Fatalf("the fixture draws %v, want the one finding in internal/a/config.go", findings)
	}
	if findings[0].Kind != KindTypeAssert {
		t.Errorf("the finding kind is %q, want %q", findings[0].Kind, KindTypeAssert)
	}

	allowlist["internal/a/config.go"] = "a test entry"
	t.Cleanup(func() { delete(allowlist, "internal/a/config.go") })
	findings, err = Check(dir)
	if err != nil {
		t.Fatalf("check the fixture: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("an allowlisted file still drew %v", findings)
	}
}

// VALIDATES: a file the guard cannot parse, and a tree it cannot walk, are each
// an error rather than a clean answer.
// PREVENTS: a coercion bug hiding behind a syntax error.
func TestAnUnreadableTreeIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "internal", "a", "config.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("package p\n\nfunc f() {\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if _, err := Check(dir); err == nil {
		t.Error("a config.go that will not parse did not stop the walk")
	}

	if _, err := Check(t.TempDir()); err == nil {
		t.Error("a tree holding no internal/ passed, so the guard answered clean over a tree it never read")
	}
}

// VALIDATES: both payloads are structured rows under the keys the script
// published.
// PREVENTS: a payload wrapping the rows, which would change what every caller
// of the JSON reads.
func TestBothAnswersAreStructuredRows(t *testing.T) {
	raw, err := json.Marshal(Findings{{File: "a.go", Line: 3, Kind: KindTypeSwitch, Code: "x"}})
	if err != nil {
		t.Fatalf("marshal the findings: %v", err)
	}
	if string(raw) != `[{"file":"a.go","line":3,"kind":"type-switch","code":"x"}]` {
		t.Errorf("the check payload is %s, want the script's array of four-key objects", raw)
	}

	raw, err = json.Marshal(leroot.NewSelftestReport("ok", "failed", leroot.Pass("a")))
	if err != nil {
		t.Fatalf("marshal the selftest report: %v", err)
	}
	if string(raw) != `[{"case":"a","passed":true}]` {
		t.Errorf("the selftest payload is %s, want a kebab-case row per case", raw)
	}
}

// VALIDATES: the two actions are reachable by their verbs, the bare command
// lists them, an unknown verb answers 2, and a value after a verb answers 1.
// PREVENTS: an area whose gates cannot be selected, and a caller that cannot
// tell a mistyped action from a gate that ran and failed.
func TestTheAreaDispatchesItsTwoGates(t *testing.T) {
	listing, code := Answer(nil)
	if code != 0 {
		t.Errorf("the bare command answers %d, want 0", code)
	}
	rows := Actions()
	if len(rows.Actions) != 2 || rows.Actions[0].Verb != "check" || rows.Actions[1].Verb != "selftest" {
		t.Fatalf("the area lists %v, want check and selftest", rows.Actions)
	}
	if listing == nil {
		t.Error("the bare command answered no listing")
	}

	if _, code = Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own fixtures, want 0", code)
	}
	if _, code = Answer([]string{"nonesuch"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code = Answer([]string{"check", "internal"}); code != 2 {
		t.Errorf("a value after an action answers %d, want 2", code)
	}
}

// VALIDATES: this checkout passes the gate, from the entry point a developer
// runs.
// PREVENTS: a config parser that silently ignores what an operator wrote. This
// is where TestNoNativeTypeConfigCoercion and TestConfigStringCoercionSelftest
// (internal/le/config/coercion/configcoercion_test.go) now live: the first forked the
// script and asserted the tree passes and the verdict reads OK, and the second
// forked --selftest for the same two facts. TestSelftestAnswersOneResultPerCase
// and TestEachSelftestFixtureDrawsWhatItDeclares carry the second one, per
// fixture rather than as one line of output.
func TestThisCheckoutPassesTheGate(t *testing.T) {
	payload, code := Answer([]string{"check"})
	findings, ok := payload.(Findings)
	if !ok {
		t.Fatalf("the check answered %T, want Findings", payload)
	}
	if code != 0 {
		t.Fatalf("the gate answers %d over this checkout:\n%s", code, findings.Text())
	}
	if got := findings.Text(); got != "config-string-coercion: OK\n" {
		t.Errorf("a passing run renders %q", got)
	}
}
