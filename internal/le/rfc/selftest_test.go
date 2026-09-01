package rfc

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// VALIDATES: The RFC selftest runs every declared in-process fixture stage.
// PREVENTS: A registered selftest whose report silently omits an RFC engine concern.
func TestRFCSelftestEveryStageContributesAResult(t *testing.T) {
	stages := selftestStages()
	if len(stages) < 10 {
		t.Fatalf("selftest has %d stages, want at least 10", len(stages))
	}

	report, err := Selftest()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{}
	for _, stage := range stages {
		expected[stage.name] = true
	}
	contributed := map[string]bool{}
	seen := map[string]bool{}
	for _, row := range report.Results {
		stage, _, held := strings.Cut(row.Case, "/")
		if !held || !expected[stage] {
			t.Errorf("unowned selftest case %q", row.Case)
		}
		contributed[stage] = true
		if seen[row.Case] {
			t.Errorf("duplicate selftest case %q", row.Case)
		}
		seen[row.Case] = true
	}
	for _, stage := range stages {
		if !contributed[stage.name] {
			t.Errorf("stage %s contributed no result", stage.name)
		}
	}
	for _, failure := range report.Failures() {
		if failure.Case != "real-tree/public-check" {
			t.Errorf("fixture property failed: %+v", failure)
		}
	}
}

// VALIDATES: The legacy live-tree property and the Go selftest read the same public Check result.
// PREVENTS: Fixture success hiding a red RFC gate over the checkout.
func TestRFCSelftestRealTreeRowMirrorsPublicCheck(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	check, checkCode := Check(root)
	report, err := Selftest()
	if err != nil {
		t.Fatal(err)
	}
	var realTree leroot.SelftestResult
	for _, row := range report.Results {
		if row.Case == "real-tree/public-check" {
			realTree = row
			break
		}
	}
	if realTree.Case == "" {
		t.Fatal("real-tree/public-check row is absent")
	}
	if realTree.Passed != (checkCode == 0) {
		t.Fatalf("real-tree row passed=%v, public Check code=%d", realTree.Passed, checkCode)
	}
	if checkCode == 0 {
		if code := report.Code(1); code != 0 {
			t.Fatalf("green public Check produced selftest code %d", code)
		}
		if text := report.Text(); text != "rfc_requirements selftest OK\n" {
			t.Fatalf("green selftest output %q", text)
		}
		return
	}
	if code := report.Code(1); code != 1 {
		t.Fatalf("red public Check produced selftest code %d", code)
	}
	if check.CannotRun != "" && !strings.Contains(realTree.Detail, check.CannotRun) {
		t.Fatalf("real-tree detail %q omits cannot-run %q", realTree.Detail, check.CannotRun)
	}
	for _, violation := range check.Violations {
		if !strings.Contains(realTree.Detail, violation) {
			t.Errorf("real-tree detail omits violation %q", violation)
		}
	}
}

// VALIDATES: A false property becomes a named failed row and a nonzero report code.
// PREVENTS: A broken RFC fixture being collapsed into the stable success line.
func TestRFCSelftestBrokenFixtureYieldsNamedFailure(t *testing.T) {
	fixture := summarySelftestFixture()
	fixture.expectedRID = "RFC9999-2-99"

	rows, err := runSummarySelftest(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, row := range rows {
		if row.Case != "summary/checklist-id" {
			continue
		}
		found = true
		if row.Passed {
			t.Fatalf("broken property passed: %+v", row)
		}
		if !strings.Contains(row.Detail, "checklist-id") {
			t.Fatalf("failure detail does not name the property: %q", row.Detail)
		}
	}
	if !found {
		t.Fatal("summary/checklist-id row is absent")
	}

	report := leroot.NewSelftestReport(
		"rfc_requirements selftest OK",
		"rfc_requirements selftest FAILED:",
		rows...,
	)
	if code := report.Code(1); code != 1 {
		t.Fatalf("broken fixture code %d, want 1", code)
	}
	if text := report.Text(); !strings.Contains(text, "summary/checklist-id") {
		t.Fatalf("failure output does not name the property: %q", text)
	}
}

// VALIDATES: the selftest carries the discrimination stage, and every property it declares
// holds against the production loader, ratchet and counters.
// PREVENTS: a refusal that exists in code and is proven by nothing, which is how every
// other RFC engine concern is kept honest.
func TestSelftestCoversDiscriminationProperties(t *testing.T) {
	var stage selftestStage
	for _, one := range selftestStages() {
		if one.name == "discrimination" {
			stage = one
			break
		}
	}
	if stage.name == "" {
		t.Fatal("the selftest declares no discrimination stage")
	}

	rows, err := stage.run()
	if err != nil {
		t.Fatalf("the discrimination stage could not run: %v", err)
	}
	wanted := map[string]bool{
		"discrimination/record-load":           false,
		"discrimination/malformed-refusal":     false,
		"discrimination/absent-tree":           false,
		"discrimination/unknown-requirement":   false,
		"discrimination/duplicate-record":      false,
		"discrimination/escape-counted-apart":  false,
		"discrimination/owed-is-change-scoped": false,
		"discrimination/clean-corpus":          false,
		// The two re-verification rows. A record that passes the schema and the
		// corpus check is not yet a proof: the fingerprints have to still match.
		"discrimination/stale-proof-refused":      false,
		"discrimination/comment-edit-keeps-proof": false,
		"discrimination/half-written-refusal":     false,
	}
	if len(rows) < len(wanted) {
		t.Fatalf("the discrimination stage contributed %d row(s), want one per refusal and count", len(rows))
	}
	for _, row := range rows {
		if !strings.HasPrefix(row.Case, "discrimination/") {
			t.Errorf("row %q is not owned by the discrimination stage", row.Case)
		}
		if !row.Passed {
			t.Errorf("property failed: %+v", row)
		}
		if _, held := wanted[row.Case]; held {
			wanted[row.Case] = true
		}
	}
	for name, covered := range wanted {
		if !covered {
			t.Errorf("the stage declares no %s row", name)
		}
	}
}
