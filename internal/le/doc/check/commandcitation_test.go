package doccheck

import (
	"strings"
	"testing"

	leretired "github.com/ze-software/ze/internal/le/le/retired"
)

// TestNoCitationNamesAVerbTheRegistryDoesNotHold proves the retired-name sweep
// judges every cited le command against what le registers (AC-25 of
// spec-le-cli-usability).
//
// VALIDATES: a cited area, namespace or first verb le does not hold fails the
// gate with its file and line, and a held one, a retired one and a record do not.
// PREVENTS: a page or a Go string keeping a command le never had, or lost
// without a rename row.
//
// Method: a fixture checkout cites a held command, a verb the `doc check` area
// does not hold, an area le does not hold, a shipped Go argv naming an absent verb, a
// retired spelling, and an absent verb in a historical record. The `doc check`
// area registers in this test binary, so the registry answers for it. The
// three citations le does not hold are refused with their file and line and
// the check answers 1; the held one is not, the retired one is left to the
// rename row that names its successor, and the record is not judged.
func TestNoCitationNamesAVerbTheRegistryDoesNotHold(t *testing.T) {
	retired := leretired.Renames()[0]
	retiredLine := "Run `le " + strings.Join(retired.Old(), " ") + "` here.\n"

	root := fixtureRepository(t, map[string]string{
		"docs/held.md":        "Run `le doc check links` to check the links.\n",
		"docs/verb.md":        "first line\nRun `le doc check no-such-verb` here.\n",
		"docs/area.md":        "Run `le no-such-area` here.\n",
		"internal/x/x.go":     `cmd := exec.Command("le", "doc", "check", "absent")` + "\n",
		"docs/retired.md":     retiredLine,
		"plan/learned/old.md": "We ran `le doc check no-such-verb` that day.\n",
	})

	report, code := runRetiredCheck(root)
	swept, ok := report.(retiredReport)
	if !ok {
		t.Fatalf("report is %T, want retiredReport: %v", report, report)
	}
	if code != 1 {
		t.Errorf("exit code=%d, want 1: a cited command le does not hold fails the gate\n%s", code, swept.Text())
	}

	want := []UnheldCitation{
		{File: "docs/area.md", Line: 1, Cited: "le no-such-area"},
		{File: "docs/verb.md", Line: 2, Cited: "le doc check no-such-verb"},
		{File: "internal/x/x.go", Line: 1, Cited: "le doc check absent"},
	}
	if len(swept.Unheld) != len(want) {
		t.Fatalf("unheld=%+v, want %d findings %+v", swept.Unheld, len(want), want)
	}
	for index, finding := range swept.Unheld {
		if finding.File != want[index].File || finding.Line != want[index].Line || finding.Cited != want[index].Cited {
			t.Errorf("finding %d=%+v, want %+v", index, finding, want[index])
		}
		if finding.Reason == "" {
			t.Errorf("finding %d names no reason: %+v", index, finding)
		}
	}
	if !strings.Contains(swept.Unheld[1].Reason, "holds no action no-such-verb") {
		t.Errorf("verb reason=%q, want it to name the verb le does not hold", swept.Unheld[1].Reason)
	}
	if !strings.Contains(swept.Unheld[1].Reason, "links") {
		t.Errorf("verb reason=%q, want it to list the verbs the area holds", swept.Unheld[1].Reason)
	}
	if !strings.Contains(swept.Unheld[0].Reason, "no-such-area") {
		t.Errorf("area reason=%q, want it to name the area le does not hold", swept.Unheld[0].Reason)
	}
	if !retiredFound(swept, "le "+strings.Join(retired.Old(), " "), "docs/retired.md", 1) {
		t.Errorf("the retired spelling is not reported by its rename row:\n%s", swept.Text())
	}
	if !strings.Contains(swept.Text(), "unheld docs/verb.md:2: le doc check no-such-verb") {
		t.Errorf("text does not name the unheld citation:\n%s", swept.Text())
	}

	// The retired row above fails the gate on its own, so a checkout whose
	// only finding is an unheld citation shows that the citation alone does.
	alone := fixtureRepository(t, map[string]string{
		"docs/held.md": "Run `le doc check links` to check the links.\n",
		"docs/verb.md": "first line\nRun `le doc check no-such-verb` here.\n",
	})
	if _, code := runRetiredCheck(alone); code != 1 {
		t.Errorf("exit code=%d with one unheld citation and no retired form, want 1", code)
	}
}

// TestUnheldCitationIsNotJudgedInTestCodeOrSpecs proves the two declared path
// rules of the unheld-citation judgement (citationUnjudgedRules): test code
// and plan/ specs are not judged for a command le does not hold, while the
// same citation in a doc and in shipped Go is, and a retired spelling in
// either skipped kind is still reported by its rename row.
//
// VALIDATES: a citation of a verb le does not hold is reported in docs/ and
// in a shipped .go string, and not in a _test.go, under internal/test/, in a
// test/**/*.ci, or in a plan/ spec; the retired judgement still reads a
// _test.go and a plan/ spec.
// PREVENTS: the path rules widening into shipped code or the instruction
// corpus, or silencing the retired-name sweep where they apply.
//
// Method: one fixture checkout writes the same unheld citation into each kind
// of file and a retired spelling into a _test.go and a plan/ spec, then
// compares the unheld findings with the two judged files exactly.
func TestUnheldCitationIsNotJudgedInTestCodeOrSpecs(t *testing.T) {
	retired := leretired.Renames()[0]
	retiredLine := "Run `le " + strings.Join(retired.Old(), " ") + "` here.\n"
	argv := `cmd := exec.Command("le", "doc", "check", "absent")` + "\n"
	span := "Run `le doc check no-such-verb` here.\n"

	root := fixtureRepository(t, map[string]string{
		"docs/verb.md":                   span,
		"internal/x/x.go":                argv,
		"internal/x/x_test.go":           argv,
		"internal/test/fixture/f.go":     argv,
		"test/plugin/t.ci":               span,
		"plan/release/spec-x.md":         span,
		"plan/release/spec-retired.md":   retiredLine,
		"internal/y/retired_use_test.go": "// " + retiredLine,
	})

	report, _ := runRetiredCheck(root)
	swept, ok := report.(retiredReport)
	if !ok {
		t.Fatalf("report is %T, want retiredReport: %v", report, report)
	}
	want := []UnheldCitation{
		{File: "docs/verb.md", Line: 1, Cited: "le doc check no-such-verb"},
		{File: "internal/x/x.go", Line: 1, Cited: "le doc check absent"},
	}
	if len(swept.Unheld) != len(want) {
		t.Fatalf("unheld=%+v, want exactly %+v", swept.Unheld, want)
	}
	for index, finding := range swept.Unheld {
		if finding.File != want[index].File || finding.Line != want[index].Line || finding.Cited != want[index].Cited {
			t.Errorf("finding %d=%+v, want %+v", index, finding, want[index])
		}
	}
	old := "le " + strings.Join(retired.Old(), " ")
	if !retiredFound(swept, old, "plan/release/spec-retired.md", 1) {
		t.Errorf("a retired spelling in a plan/ spec is not reported:\n%s", swept.Text())
	}
	if !retiredFound(swept, old, "internal/y/retired_use_test.go", 1) {
		t.Errorf("a retired spelling in a _test.go is not reported:\n%s", swept.Text())
	}
}
