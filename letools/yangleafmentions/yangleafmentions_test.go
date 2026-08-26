// VALIDATES: spec-le-is-a-ze-binary AC-5, AC-7 -- the leaf-mention report is
// called as a function and answers structured data.
// PREVENTS: a port whose heuristic silently stops telling a consumed leaf from
// an unconsumed one, which the report's own page cannot show: an always-silent
// check and a clean tree print the same thing.

package yangleafmentions

import (
	"encoding/json"
	"strings"
	"testing"
)

// scanFixture writes the selftest fixture into a temporary tree and scans it,
// which is the same tree and the same call the selftest action makes.
func scanFixture(t *testing.T) Report {
	t.Helper()
	dir := t.TempDir()
	if err := WriteFixture(dir); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	report, err := ScanTree(dir)
	if err != nil {
		t.Fatalf("scan the fixture: %v", err)
	}
	return report
}

// VALIDATES: the heuristic draws both polarities over a tree whose answer is
// known.
// PREVENTS: a scan that reports everything, or nothing, passing as a working
// heuristic.
func TestTheFixtureReportsOnlyTheUnreadLeaf(t *testing.T) {
	report := scanFixture(t)

	if report.Modules != 1 {
		t.Errorf("the scan found %d modules, want 1", report.Modules)
	}
	if report.Leaves != 3 {
		t.Errorf("the scan parsed %d leaves, want 3", report.Leaves)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("the scan reported %d findings, want 1: %s", len(report.Findings), summary(report))
	}
	if report.Findings[0].Leaf != "never-read-leaf" {
		t.Errorf("the scan reported %q, want never-read-leaf", report.Findings[0].Leaf)
	}
	if report.Findings[0].Path != "fixture/never-read-leaf" {
		t.Errorf("the finding carries path %q, want fixture/never-read-leaf", report.Findings[0].Path)
	}
	if report.Findings[0].Package != "internal/plugins/fixture" {
		t.Errorf("the finding names package %q, want the tree-relative path", report.Findings[0].Package)
	}
}

// VALIDATES: a list key leaf is never reported.
// PREVENTS: the noise the key rule exists to remove coming back, which no
// count-only assertion would notice.
func TestAListKeyLeafIsNotReported(t *testing.T) {
	leaves := parseLeaves(`module m {
    list peer {
        key "name";
        leaf name { type string; }
        leaf hold-time { type uint16; }
    }
}`)
	for _, leaf := range leaves {
		if leaf.name == "name" {
			t.Errorf("the key leaf was parsed as a reportable leaf: %+v", leaves)
		}
	}
	if len(leaves) != 1 || leaves[0].name != "hold-time" {
		t.Errorf("the parse answered %+v, want the one non-key leaf", leaves)
	}
}

// VALIDATES: the selftest table passes over its own fixture and answers 0.
// PREVENTS: a selftest that cannot pass, which would be discovered only by the
// gate that runs it.
func TestTheSelftestPassesOverItsOwnFixture(t *testing.T) {
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

// VALIDATES: every selftest case FAILS on a report that breaks the property it
// declares.
// PREVENTS: a case whose check can never fail, which is a selftest row that
// proves nothing and still counts as a pass.
func TestEverySelftestCaseFailsOnABrokenReport(t *testing.T) {
	// A report that breaks all seven at once: no module, no leaf, the unread
	// leaf missing, and both read leaves reported.
	broken := Report{
		Findings: []Finding{
			{Leaf: "read-leaf", Path: "fixture/read-leaf"},
			{Leaf: "also-read", Path: "fixture/inner/also-read"},
		},
	}
	for _, testCase := range selftestCases {
		if testCase.name == "leaf-path-carried" {
			// This one holds only when the leaf IS reported, so a report that
			// omits it is not a break of the property it declares.
			continue
		}
		if detail := testCase.check(broken); detail == "" {
			t.Errorf("case %s passed over a report that breaks it", testCase.name)
		}
	}

	wrongPath := Report{Modules: 1, Leaves: 3, Findings: []Finding{{Leaf: "never-read-leaf", Path: "elsewhere"}}}
	for _, testCase := range selftestCases {
		if testCase.name != "leaf-path-carried" {
			continue
		}
		if detail := testCase.check(wrongPath); detail == "" {
			t.Error("the path case passed over a finding carrying the wrong path")
		}
	}
}

// VALIDATES: AC-7 -- the payload is data a JSON encoder takes, with the
// script's own keys.
// PREVENTS: a port that answers a rendered page, which no pipe operator can act
// on.
func TestReportIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(Report{Modules: 2, Leaves: 9, Findings: []Finding{
		{Module: "ze-x-conf", Package: "internal/plugins/x", Leaf: "a", Path: "x/a"},
	}})
	if err != nil {
		t.Fatalf("the payload does not encode: %v", err)
	}
	for _, key := range []string{`"modules"`, `"leaves"`, `"findings"`, `"module"`, `"package"`, `"leaf"`, `"path"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("the payload has no %s key: %s", key, raw)
		}
	}
}

// VALIDATES: the page carries the counts and the advisory sentence whatever it
// found.
// PREVENTS: a reader acting on a row as a defect, which is what the advisory
// line exists to stop.
func TestThePageAlwaysCarriesItsAdvisory(t *testing.T) {
	clean := Report{Modules: 4}.Text()
	if !strings.Contains(clean, "4 config modules, 0 leaves, 0 never named") {
		t.Errorf("the clean page does not carry its counts:\n%s", clean)
	}
	if !strings.Contains(clean, "ADVISORY.") {
		t.Errorf("the clean page does not carry the advisory:\n%s", clean)
	}

	found := Report{Modules: 1, Leaves: 1, Findings: []Finding{
		{Module: "ze-x-conf", Package: "internal/plugins/x", Leaf: "a", Path: "x/a"},
	}}.Text()
	if !strings.Contains(found, "ze-x-conf") || !strings.Contains(found, "internal/plugins/x") {
		t.Errorf("the page does not carry its finding:\n%s", found)
	}
}

// VALIDATES: the area dispatches its two actions and refuses the two mistakes.
// PREVENTS: a verb that drifts from its gate name, which would leave the Make
// target pointing at nothing after the swap.
func TestTheAreaDispatchesItsActions(t *testing.T) {
	if _, code := Answer([]string{"selftest"}); code != 0 {
		t.Errorf("the selftest action answers %d over its own fixture, want 0", code)
	}
	if _, code := Answer([]string{"nope"}); code != 2 {
		t.Errorf("an unknown action answers %d, want 2", code)
	}
	if _, code := Answer([]string{"report", "value"}); code != 1 {
		t.Errorf("a value after an action answers %d, want 1", code)
	}

	verbs := Actions()
	if len(verbs.Actions) != 2 {
		t.Fatalf("the area holds %d actions, want 2", len(verbs.Actions))
	}
	if verbs.Actions[0].Verb != "report" || verbs.Actions[1].Verb != "selftest" {
		t.Errorf("the verbs are %q and %q, want report and selftest", verbs.Actions[0].Verb, verbs.Actions[1].Verb)
	}
}
