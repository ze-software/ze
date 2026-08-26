// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7, AC-8 -- the sensitivity ratchet
// is called as a function, answers structured data, and keeps 1 (the ratchet
// fired) apart from 2 (the tree could not be read).
// PREVENTS: the guard whose only enforcement path could never deny. Valid used
// to be a field the scan always set to true, so the JSON half reported findings
// and exited 0.

package testsensitivity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/letools/lepath"
)

// VALIDATES: this checkout passes the ratchet and the scan actually read it.
// PREVENTS: a port whose walk roots resolve to nothing, which looks identical to
// a tree in which every test asserts.
func TestTheRealCheckoutPassesTheRatchet(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}

	result, err := Scan(tree, WorkingTree)
	if err != nil {
		t.Fatalf("the gate could not read this checkout: %v", err)
	}
	if result.FilesScanned == 0 || result.TestsScanned == 0 {
		t.Fatalf("the scan read %d files and %d tests: a population it never walked",
			result.FilesScanned, result.TestsScanned)
	}
	if len(result.TagUniverse) == 0 {
		t.Error("the tag universe is empty, so every gated test would read as an orphan")
	}

	baseline, err := ReadBaseline(tree)
	if err != nil {
		t.Fatalf("read the baseline: %v", err)
	}
	verdict := Judge(result, baseline)
	if !verdict.Result.Valid {
		t.Errorf("this checkout fails the sensitivity ratchet:\n%s", verdict.Breach())
	}
}

// VALIDATES: a tree missing a test root, or holding no test file, is an ERROR.
// PREVENTS: a mis-set root shrinking the count to something the ratchet happily
// accepts, which the floor writer would then bake in permanently.
func TestATreeThatWasNotReadIsAnError(t *testing.T) {
	if _, err := Scan(t.TempDir(), WorkingTree); err == nil {
		t.Error("a tree holding none of the test roots was scanned")
	}

	dir := t.TempDir()
	for _, root := range testRoots {
		if err := os.MkdirAll(filepath.Join(dir, root), 0o750); err != nil {
			t.Fatalf("create %s: %v", root, err)
		}
	}
	if _, err := Scan(dir, WorkingTree); err == nil {
		t.Error("a tree holding no test file at all was scanned")
	}
}

// VALIDATES: the ratchet DENIES when a count exceeds its floor, and passes when
// it does not.
// PREVENTS: the enforcement path that could never fail. A verdict is what the
// payload carries, so `| json` sees the same denial the page does.
func TestTheRatchetCanActuallyDeny(t *testing.T) {
	result := Result{
		AssertNothing: []Finding{{File: "a_test.go", Test: "TestA", Line: 3, Reason: ReasonAssertNothing}},
		TagOrphan:     []Finding{{File: "b_test.go", Line: 1, Reason: ReasonTagOrphan, Detail: "ze_nowhere"}},
		FilesScanned:  2,
	}

	fired := Judge(result, Baseline{})
	if fired.Result.Valid {
		t.Error("a count above both floors passed the ratchet")
	}
	if fired.Text() != "" {
		t.Errorf("a firing ratchet rendered a verdict line: %q", fired.Text())
	}
	for _, want := range []string{"assert-nothing count 1 exceeds baseline 0", "tag-orphan count 1 exceeds baseline 0", "a_test.go:3 TestA", "make ze-test-health-update"} {
		if !strings.Contains(fired.Breach(), want) {
			t.Errorf("the breach has no %q:\n%s", want, fired.Breach())
		}
	}

	passed := Judge(result, Baseline{AssertNothing: 1, TagOrphan: 1})
	if !passed.Result.Valid {
		t.Error("a count sitting exactly on both floors failed the ratchet")
	}
	if !strings.Contains(passed.Text(), "test-sensitivity: OK (assert-nothing 1/1, tag-orphan 1/1, 2 test files)") {
		t.Errorf("the passing verdict is %q", passed.Text())
	}
	if passed.Breach() != "" {
		t.Errorf("a passing ratchet on its floors reported a breach: %q", passed.Breach())
	}

	slack := Judge(result, Baseline{AssertNothing: 5, TagOrphan: 5})
	if !strings.Contains(slack.Breach(), "baseline is slack") {
		t.Errorf("a slack floor was not reported:\n%s", slack.Breach())
	}
}

// VALIDATES: a baseline that cannot be read or believed is an ERROR.
// PREVENTS: a missing or negative floor being read as zero, which would either
// fail every tree or pass every one.
func TestABadBaselineIsRefused(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadBaseline(dir); err == nil {
		t.Error("a tree with no baseline was accepted")
	}

	path := filepath.Join(dir, filepath.FromSlash(BaselinePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	for name, body := range map[string]string{
		"a document that is not JSON": "not json",
		"a negative floor":            `{"assert-nothing": -1, "tag-orphan": 0}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write the baseline: %v", err)
		}
		if _, err := ReadBaseline(dir); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}

	if err := os.WriteFile(path, []byte(`{"assert-nothing": 3, "tag-orphan": 1}`), 0o600); err != nil {
		t.Fatalf("write the baseline: %v", err)
	}
	baseline, err := ReadBaseline(dir)
	if err != nil || baseline.AssertNothing != 3 || baseline.TagOrphan != 1 {
		t.Errorf("a good baseline answered (%+v, %v)", baseline, err)
	}
}

// VALIDATES: every assert-nothing case draws exactly the count it declares, and
// the table holds both polarities.
// PREVENTS: a detector that credits everything, or nothing, passing as a working
// heuristic.
func TestEveryAssertCaseDrawsWhatItDeclares(t *testing.T) {
	inert, asserting := 0, 0
	for _, testCase := range assertCases {
		got, err := CountInert(testCase.src, nil)
		if err != nil {
			t.Fatalf("case %q: %v", testCase.name, err)
		}
		if got != testCase.want {
			t.Errorf("case %q drew %d findings, want %d", testCase.name, got, testCase.want)
		}
		if testCase.want > 0 {
			inert++
			continue
		}
		asserting++
	}
	if inert < 5 || asserting < 5 {
		t.Errorf("the table holds %d inert and %d asserting fixtures; both polarities are what make a silent gate meaningful", inert, asserting)
	}
}

// VALIDATES: the one-level follow into another first-party package works in both
// directions.
// PREVENTS: the follow becoming a blanket pardon for every function that happens
// to take a *testing.T.
func TestTheCrossPackageFollowCreditsOnlyAnAssertingHelper(t *testing.T) {
	root, err := WriteCrossFixture()
	if err != nil {
		t.Fatalf("write the cross-package fixture: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(root) }) //nolint:errcheck // temp fixture

	for _, testCase := range crossCases {
		got, err := CountInert(testCase.src, NewIndex(root))
		if err != nil {
			t.Fatalf("case %q: %v", testCase.name, err)
		}
		if got != testCase.want {
			t.Errorf("case %q drew %d findings, want %d", testCase.name, got, testCase.want)
		}
	}
}

// VALIDATES: every tag case is judged as it declares, over a two-tag universe.
// PREVENTS: the single "everything available is on" evaluation, which condemns
// every negated constraint.
func TestEveryTagCaseIsJudgedAsDeclared(t *testing.T) {
	orphans, reachable := 0, 0
	for _, testCase := range tagCases {
		file, err := parseSource(testCase.src)
		if err != nil {
			t.Fatalf("case %q: %v", testCase.name, err)
		}
		got, detail := TagOrphan(file, tagUniverseFixture)
		if got != testCase.orphan {
			t.Errorf("case %q answered %v, want %v", testCase.name, got, testCase.orphan)
		}
		if got && len(detail) == 0 {
			t.Errorf("case %q was called an orphan with nothing named", testCase.name)
		}
		if testCase.orphan {
			orphans++
			continue
		}
		reachable++
	}
	if orphans < 4 || reachable < 4 {
		t.Errorf("the table holds %d orphan and %d reachable fixtures; both polarities are what make a silent gate meaningful", orphans, reachable)
	}
}

// VALIDATES: the tag universe is derived from the make files and is not empty.
// PREVENTS: a universe seeded straight from the manifest, which would make the
// guard unable to fail: deleting $(ZE_FEATURES) from GO_TEST_TAGS would strand
// every gated test and the gate would still report zero orphans.
func TestTheTagUniverseComesFromTheMakeFiles(t *testing.T) {
	tree, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve the repository root: %v", err)
	}
	universe, err := TagUniverse(tree)
	if err != nil {
		t.Fatalf("derive the tag universe: %v", err)
	}
	if len(universe) < 2 {
		t.Fatalf("the universe holds %d tags: the make files did not parse", len(universe))
	}
	if !universe["ze_core"] {
		t.Error("the universe does not hold ze_core, which every unit target passes")
	}

	if _, err := TagUniverse(t.TempDir()); err == nil {
		t.Error("a tree with no Makefile answered a universe")
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
	if code := report.Code(2); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	if len(report.Results) != SelftestCaseCount() {
		t.Errorf("the selftest answered %d rows for %d cases", len(report.Results), SelftestCaseCount())
	}
}

// VALIDATES: AC-7 -- both payloads encode as the SAME document, with the
// script's own keys.
// PREVENTS: a caller of `| json` getting a different shape from the check than
// from the report, which would break the generated test-health page.
func TestBothAnswersEncodeAsOneDocument(t *testing.T) {
	result := Result{
		AssertNothing: []Finding{{File: "a_test.go", Test: "TestA", Line: 3, Reason: ReasonAssertNothing}},
		TagOrphan:     []Finding{},
		FilesScanned:  2, TestsScanned: 4, TagUniverse: []string{"ze_core"}, Valid: true,
	}
	fromResult, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("the result does not encode: %v", err)
	}
	fromVerdict, err := json.Marshal(Judge(result, Baseline{AssertNothing: 1}))
	if err != nil {
		t.Fatalf("the verdict does not encode: %v", err)
	}
	if !bytes.Equal(fromResult, fromVerdict) {
		t.Errorf("the two payloads differ.\nresult:  %s\nverdict: %s", fromResult, fromVerdict)
	}
	for _, key := range []string{
		`"assert-nothing"`, `"tag-orphan"`, `"files-scanned"`, `"tests-scanned"`,
		`"test-tag-universe"`, `"valid"`, `"file"`, `"test"`, `"line"`, `"reason"`,
	} {
		if !strings.Contains(string(fromResult), key) {
			t.Errorf("the payload has no %s key: %s", key, fromResult)
		}
	}
}

// VALIDATES: the report page carries both counts and both lists.
// PREVENTS: a page a reader cannot act on, which is the only rendering the
// report action has.
func TestTheReportPageCarriesBothLists(t *testing.T) {
	page := Result{
		AssertNothing: []Finding{{File: "a_test.go", Test: "TestA", Line: 3}},
		TagOrphan:     []Finding{{File: "b_test.go", Line: 1, Detail: "ze_nowhere"}},
		FilesScanned:  2, TestsScanned: 4, TagUniverse: []string{"ze_core", "ze_web"},
	}.Text()
	for _, want := range []string{
		"Test files scanned: 2", "Test functions scanned: 4",
		"Test tag universe: ze_core ze_web",
		"## Assert-nothing (1)", "  a_test.go:3 TestA",
		"## Tag-orphan (1)", "  b_test.go:1 requires ze_nowhere",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page has no %q:\n%s", want, page)
		}
	}
}

// VALIDATES: the area dispatches its four actions and refuses the two mistakes.
// PREVENTS: a verb that drifts from its gate name, which would leave the Make
// target pointing at nothing after the swap.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	if _, code := Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own fixtures, want 0", code)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"check", "value"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 4 {
		t.Fatalf("the area holds %d actions, want 4", len(verbs.Actions))
	}
	for i, want := range []string{"check", "selftest", "report", "tracked"} {
		if verbs.Actions[i].Verb != want {
			t.Errorf("action %d is %q, want %q", i, verbs.Actions[i].Verb, want)
		}
	}
}
