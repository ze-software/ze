package rfc

import (
	"strings"
	"testing"
)

// The demonstrated-gap fixture: a Go module holding its own copy of the helper
// under the import path suffix the gate matches, and one test file whose body
// the case chooses. A fixture module cannot import Ze's internal package, and
// the gate matches the suffix for exactly that reason (rfcgapImportSuffix).
const (
	gapFixtureTestPath = "cmd/widget/widget_test.go"
	gapFixtureHelper   = "package rfcgap\n\nimport \"testing\"\n\n" +
		"// Demonstrate runs body against tb.\n" +
		"func Demonstrate(tb testing.TB, id string, body func(testing.TB)) {\n\ttb.Helper()\n\t_ = id\n\tbody(tb)\n}\n"
	gapFixtureTestHead = "package widget\n\nimport (\n\t\"testing\"\n\n" +
		"\t\"example.com/widget/internal/test/rfcgap\"\n)\n\n"
)

// gapFixtureTest answers a test file whose one function carries a gap tag for
// selftestRIDSend and calls the helper with id.
func gapFixtureTest(id string) string {
	return gapFixtureTestHead +
		"// RFC requirement: " + selftestRIDSend + " gap -- a speaker sends the widget, which Ze does not yet.\n" +
		"func TestSendsWidget(t *testing.T) {\n" +
		"\trfcgap.Demonstrate(t, \"" + id + "\", func(tb testing.TB) { tb.Error(\"no widget sent\") })\n}\n"
}

// gapFixtureSummary is the fixture summary with its one row annotated {gap}, and
// its public row's remaining cell counting that gap, so the gap count agreement
// check holds.
func gapFixtureSummary() string {
	meta := strings.Replace(selftestMeta, "Zero MUST gaps.", "One MUST gap.", 1)
	return "# RFC 9999\n\n" + meta + "\n## Compliance Checklist\n\n" +
		"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2) {gap: nothing sends the widget yet}\n"
}

// gapFixtureFiles answers the module, the helper and the test, plus the {gap}
// summary when gapRow is set.
func gapFixtureFiles(gapRow bool, test string) map[string]string {
	files := map[string]string{
		"go.mod":                         "module example.com/widget\n\ngo 1.27.0\n",
		"internal/test/rfcgap/rfcgap.go": gapFixtureHelper,
		gapFixtureTestPath:               test,
	}
	if gapRow {
		files[selftestSummaryRel] = gapFixtureSummary()
	}
	return files
}

// violationWith answers the first violation holding every want, or "".
func violationWith(report *CheckReport, wants ...string) string {
	for _, violation := range report.Violations {
		held := true
		for _, want := range wants {
			if !strings.Contains(violation, want) {
				held = false
			}
		}
		if held {
			return violation
		}
	}
	return ""
}

// VALIDATES: AC-5 and AC-11 -- a gap tag on a {gap} row, inside the test that
// calls the helper with the tag's own id, passes the whole gate at its entry
// point, and the report counts the row as demonstrated rather than described.
// METHOD: full Check over a Go module fixture, so the tag is scanned, tied to
// its unit, type-checked with the proof tags' packages and counted.
func TestCheckAcceptsDemonstratedGap(t *testing.T) {
	root := checkFixtureTree(t, gapFixtureFiles(true, gapFixtureTest(selftestRIDSend)))
	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("cannot check the gap fixture: %s", report.CannotRun)
	}
	if code != 0 {
		t.Fatalf("a demonstrated gap answered %d:\n%s", code, report.Text())
	}
	if report.GapsDemonstrated != 1 || report.GapsDescribed != 0 {
		t.Fatalf("gaps demonstrated %d, described %d, want 1 and 0", report.GapsDemonstrated, report.GapsDescribed)
	}
	if got := report.GapsByStem["rfc9999"]; got != (GapCount{Demonstrated: 1}) {
		t.Fatalf("rfc9999 gap split %+v, want one demonstrated", got)
	}
}

// VALIDATES: AC-6 -- a gap tag on a row that is not annotated {gap} is refused,
// and the violation names the tag, the row and the retag owed.
// PREVENTS: a gap tag that outlives its annotation (R-3).
func TestCheckRefusesGapTagOnNonGapRow(t *testing.T) {
	root := checkFixtureTree(t, gapFixtureFiles(false, gapFixtureTest(selftestRIDSend)))
	report, code := Check(root)
	if code != 2 {
		t.Fatalf("a gap tag on a non-gap row answered %d:\n%s", code, report.Text())
	}
	if violationWith(&report, gapFixtureTestPath, selftestSummaryRel, selftestRIDSend,
		"not annotated {gap}", "retag the test positive or negative") == "" {
		t.Fatalf("no violation names the gap tag on a non-gap row:\n%s", report.Text())
	}
}

// VALIDATES: AC-7 -- a gap tag whose test calls the helper with ANOTHER id is
// refused, so the tie is to the tag's own id and never to any call.
// PREVENTS: a gap tag that reads as demonstrated with nothing running it (R-4).
func TestCheckRefusesGapTagWithoutHelper(t *testing.T) {
	root := checkFixtureTree(t, gapFixtureFiles(true, gapFixtureTest("RFC9999-9-9")))
	report, code := Check(root)
	if code != 2 {
		t.Fatalf("a gap tag without its helper call answered %d:\n%s", code, report.Text())
	}
	if violationWith(&report, gapFixtureTestPath, selftestRIDSend,
		"does not call rfcgap.Demonstrate with this id") == "" {
		t.Fatalf("no violation names the undemonstrated gap tag:\n%s", report.Text())
	}
	if report.GapsDemonstrated != 0 {
		t.Fatalf("a refused gap tag was counted as demonstrated")
	}
}

// VALIDATES: AC-8 -- a positive and a negative tag on a {gap} row stay refused
// as a stale annotation, unchanged by the gap marker.
func TestCheckStillRefusesPolarityTagOnGapRow(t *testing.T) {
	files := fixtureCorpus()
	files[selftestSummaryRel] = gapFixtureSummary()
	report, code := Check(checkFixtureTree(t, files))
	if code != 2 {
		t.Fatalf("polarity tags on a {gap} row answered %d:\n%s", code, report.Text())
	}
	if violationWith(&report, selftestRIDSend, "is annotated {gap} but IS tested") == "" {
		t.Fatalf("no violation names the stale {gap}:\n%s", report.Text())
	}
}

// VALIDATES: AC-9 -- a gap tag in a .ci carrier is refused: a gap is
// demonstrated in a Go test, which is the only shape that can invert its own
// result.
func TestCheckRefusesGapTagInCI(t *testing.T) {
	files := map[string]string{
		selftestSummaryRel: gapFixtureSummary(),
		selftestCIPath: "# RFC requirement: " + selftestRIDSend + " gap -- the daemon sends no widget yet.\n" +
			selftestCIDirective + "\n",
	}
	report, code := Check(checkFixtureTree(t, files))
	if code != 2 {
		t.Fatalf("a gap tag in a .ci file answered %d:\n%s", code, report.Text())
	}
	if violationWith(&report, selftestCIPath, selftestRIDSend, "outside a Go test") == "" {
		t.Fatalf("no violation names the .ci gap tag:\n%s", report.Text())
	}
}

// VALIDATES: AC-10 -- a gap tag the tip commit added owes no discrimination
// record, where a polarity tag added the same way owes one
// (TestCheckDiscriminationRequiresProofForNewTag).
// METHOD: two commits, the gap test in the second, so the tag is new at HEAD.
func TestGapTagOwesNoDiscrimination(t *testing.T) {
	base := gapFixtureFiles(true, gapFixtureTestHead)
	delete(base, gapFixtureTestPath)
	tip := map[string]string{gapFixtureTestPath: gapFixtureTest(selftestRIDSend)}
	root := commitFixtureTip(t, base, tip, nil)
	report, code := Check(root)
	if code != 0 {
		t.Fatalf("a gap tag the tip added answered %d:\n%s", code, report.Text())
	}
	if report.DiscriminationOwed != 0 {
		t.Fatalf("a gap tag was billed %d discrimination record(s)", report.DiscriminationOwed)
	}
	if report.GapsDemonstrated != 1 {
		t.Fatalf("the committed gap tag was not counted as demonstrated")
	}
}

// VALIDATES: AC-11 -- the report text prints demonstrated and described gaps
// beside the other figures, and names the summaries that hold a demonstrated
// one.
func TestCheckReportsDemonstratedGaps(t *testing.T) {
	report := CheckReport{GapsDemonstrated: 1, GapsDescribed: 4, GapsByStem: map[string]GapCount{
		"rfc7606": {Demonstrated: 1, Described: 1}, "rfc4271": {Described: 3},
	}}
	text := report.Text()
	want := "gaps: 1 demonstrated by a test, 4 described by the annotation alone, of 5 {gap} row(s) in every summary; demonstrated in rfc7606 1.\n"
	if !strings.Contains(text, want) {
		t.Fatalf("report text omits the gap figures:\n%s", text)
	}
	demonstrated := demonstratedGaps(nil, []Tag{{RID: "RFC1-1-1", Gap: true, File: "a_test.go", Demonstration: "a_test.go::T"}})
	if len(demonstrated) != 0 {
		t.Fatalf("a gap tag naming no requirement was counted: %v", demonstrated)
	}
	counts := gapCounts(nil, demonstrated)
	if len(counts) != 0 {
		t.Fatalf("no requirement produced gap counts %v", counts)
	}
}
