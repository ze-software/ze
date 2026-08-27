// Design: docs/architecture/core-design.md -- the RFC engine proved against fixtures
// Overview: selftest.go -- fixture-suite orchestration and action answer
//
// selftest_core.go exercises the read-only parsers, scanners, checks, and HEAD
// comparison logic. Stateful artifacts and writers are in selftest_state.go.
package rfc

import (
	"os"
	"strings"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

const selftestWorkflow = `on:
  schedule:
    - cron: '0 3 * * *'
jobs:
  audit:
    steps:
      - run: make ze-interop-test
`

const selftestSummary = "# RFC 9999\n\n## Compliance Checklist\n\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2) {single-polarity: positive; no receiver input exists} {superseded: restated RFC10000-3-1; the successor states the same rule}\n- [ ] [RFC9999-2-2] [MUST NOT] A receiver MUST NOT drop the widget (§2)\n\nCorrection 2026-08-26: The row `RFC9999-2-1` quotes \"A speaker SHOULD send the widget and preserve its state.\".\n"

type summaryFixture struct {
	text        string
	expectedRID string
}

func summarySelftestFixture() summaryFixture {
	return summaryFixture{text: selftestSummary, expectedRID: "RFC9999-2-1"}
}

func runSummarySelftest(fixture summaryFixture) ([]leroot.SelftestResult, error) {
	requirements, err := ParseSummaryText(fixture.text, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		return nil, err
	}
	first := Requirement{}
	second := Requirement{}
	if len(requirements) > 0 {
		first = requirements[0]
	}
	if len(requirements) > 1 {
		second = requirements[1]
	}

	annotationOK := first.Annotation != nil
	if annotationOK {
		annotationOK = first.Annotation.Kind == annotationSinglePolarity
		annotationOK = annotationOK && first.Annotation.Polarity == "positive"
	}
	successorOK := first.Superseded != nil
	if successorOK {
		successorOK = first.Superseded.Disposition == successorRestated
		successorOK = successorOK && first.Superseded.Target == "RFC10000-3-1"
	}
	_, mismatch := ParseChecklistLine(
		"- [ ] [RFC9999-3-1] [MUST] A speaker MUST send the widget (§2)",
		"rfc9999", "rfc/short/rfc9999.md", 1,
	)
	corrections := parseCorrections(fixture.text)
	correctionOK := len(corrections) == 1
	if correctionOK {
		correctionOK = len(corrections[0].RIDs) == 1
		correctionOK = correctionOK && corrections[0].RIDs[0] == "RFC9999-2-1"
		correctionOK = correctionOK && len(corrections[0].Quotes) == 1
	}

	return []leroot.SelftestResult{
		selftestResult("summary/checklist-id", len(requirements) == 2 && first.RID == fixture.expectedRID,
			"the checklist id or requirement count changed"),
		selftestResult("summary/level-and-section", first.Level == "MUST" && first.Section == "2" && second.Level == "MUST NOT",
			"the level or trailing section anchor changed"),
		selftestResult("summary/annotation", annotationOK,
			"the single-polarity annotation did not retain its polarity and reason"),
		selftestResult("summary/successor", successorOK,
			"the successor marker did not compose with the coverage annotation"),
		selftestResult("summary/anchor-refusal", mismatch != nil,
			"a checklist id that disagrees with its section was accepted"),
		selftestResult("summary/correction", correctionOK,
			"the correction paragraph did not retain its id and quote"),
	}, nil
}

func runCarrierSelftest() ([]leroot.SelftestResult, error) {
	files := map[string]string{
		".github/workflows/nightly.yml":          selftestWorkflow,
		"internal/sample/widget_test.go":         "package sample\n// RFC requirement: RFC9999-2-1 positive\nfunc TestWidget() {}\n",
		"test/plugin/widget.ci":                  "# RFC requirement: RFC9999-2-2 negative\n",
		"test/editor/widget.et":                  "# RFC requirement: RFC9999-2-3 positive\n",
		"test/interop/scenarios/widget/check.py": "# RFC requirement: RFC9999-2-4 positive\nprint('widget')\n",
	}
	root, err := newSelftestTree("rfc-selftest-carriers-", files)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	carriers, err := Carriers(root)
	if err != nil {
		return nil, err
	}
	tags, err := ScanTree(root)
	if err != nil {
		return nil, err
	}

	wanted := map[string]string{
		"internal/sample/widget_test.go":         "unit",
		"test/plugin/widget.ci":                  "functional",
		"test/editor/widget.et":                  "editor",
		"test/interop/scenarios/widget/check.py": "interop",
	}
	found := map[string]string{}
	for _, tag := range tags {
		carrier, held := CarrierFor(tag.File, carriers)
		if held {
			found[tag.File] = carrier.Kind
		}
	}
	allKinds := len(found) == len(wanted)
	for path, kind := range wanted {
		allKinds = allKinds && found[path] == kind
	}
	interop, interopHeld := CarrierFor("test/interop/scenarios/widget/check.py", carriers)
	unrun, unrunHeld := CarrierFor("test/nosuite/widget.ci", carriers)
	_, draftHeld := CarrierFor("test/draft/widget.ci", carriers)

	return []leroot.SelftestResult{
		selftestResult("carriers/all-scanners", len(tags) == 4 && allKinds,
			"the Go, CI, ET, and Python scanners did not all contribute their carrier"),
		selftestResult("carriers/scheduled-interop", interopHeld && interop.Tier == tierNightly,
			"the scheduled workflow did not grant the interop carrier its nightly tier"),
		selftestResult("carriers/unrun-refusal", unrunHeld && unrun.Tier == tierUnrun,
			"an unmatched CI suite was not selected as unrun"),
		selftestResult("carriers/draft-skip", !draftHeld,
			"the draft incubator was selected as evidence"),
	}, nil
}

func runCoverageSelftest() ([]leroot.SelftestResult, error) {
	requirements, err := ParseSummaryText(selftestSummary, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		return nil, err
	}
	tags := make([]Tag, 0, 4)
	tags = append(tags,
		Tag{RID: "RFC9999-2-1", Polarity: "positive", File: "internal/sample/widget_test.go"},
		Tag{RID: "RFC9999-2-2", Polarity: "positive", File: "internal/sample/widget_test.go"},
		Tag{RID: "RFC9999-2-2", Polarity: "negative", File: "internal/sample/widget_test.go"},
	)
	enrolled := map[string]bool{"rfc9999": true}
	clean := evaluate(requirements, tags, enrolled)
	missing := evaluate(requirements, tags[:2], enrolled)
	unknown := evaluate(requirements, append(tags, Tag{RID: "RFC9999-9-9", Polarity: "positive"}), enrolled)
	rows := RFCCoverageRows(requirements, tags, carriersFor([]string{"plugin"}, map[string]string{}))
	rollupOK := len(rows) == 1
	if rollupOK {
		rollupOK = rows[0].Gated == 2 && rows[0].Both == 1 && rows[0].Annotated == 1
		rollupOK = rollupOK && rows[0].Outstanding() == 0
	}

	return []leroot.SelftestResult{
		selftestResult("coverage/evaluation-clean", len(clean) == 0,
			"complete polarity evidence produced a coverage violation"),
		selftestResult("coverage/missing-polarity", len(missing) == 1 && strings.Contains(missing[0], "negative"),
			"removing the negative test did not produce the named violation"),
		selftestResult("coverage/unknown-id", len(unknown) == 1 && strings.Contains(unknown[0], "RFC9999-9-9"),
			"a tag for an unknown requirement id was accepted"),
		selftestResult("coverage/rollup", rollupOK,
			"the annotated and both-polarity populations did not partition the rollup"),
	}, nil
}

func runStatusSelftest() ([]leroot.SelftestResult, error) {
	rows := ParseStatusLedger("| RFC 9999 | Widgets | Partial | unit tests | one MUST gap |\n")
	dispositions, err := ParseDispositions("rfc8888 backlog the extraction is owed\n")
	if err != nil {
		return nil, err
	}
	_, malformed := ParseDispositions("rfc7777 unknown not a disposition\n")
	gap := Requirement{
		RFC: "rfc9999", RID: "RFC9999-2-1", Level: "MUST", Text: "MUST send", Section: "2",
		Annotation: &Annotation{Kind: annotationGap, Reason: "not implemented"},
	}
	hidden := checkStatusAgreement(
		[]Requirement{gap},
		map[string]LedgerRow{"rfc9999": {Status: "Supported", Coverage: "complete"}},
		map[string]bool{"rfc9999": true},
	)
	declared := checkSummaryDisposition(
		map[string]bool{"rfc8888": true, "rfc9999": true},
		map[string]bool{"rfc9999": true}, dispositions, map[string]bool{},
	)

	return []leroot.SelftestResult{
		selftestResult("status/public-row", rows["rfc9999"].Status == "Partial" && rows["rfc9999"].Remaining == "one MUST gap",
			"the public status row did not retain its status and remaining work"),
		selftestResult("status/disposition", len(declared) == 0 && dispositions["rfc8888"].Kind == dispositionBacklog,
			"an enrolled-or-declared summary was reported as unowned"),
		selftestResult("status/disposition-refusal", malformed != nil,
			"an unknown disposition was accepted"),
		selftestResult("status/gap-disclosure", len(hidden) == 1 && strings.Contains(hidden[0], "RFC9999-2-1"),
			"a Supported row hid a requirement gap"),
	}, nil
}

func runBaselineSelftest() ([]leroot.SelftestResult, error) {
	root, err := newSelftestTree("rfc-selftest-baseline-", map[string]string{})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	suiteSource := `package functional
const (
	suiteParse = "parse"
	suiteUI = "ui"
)
var Gating = []string{suiteParse, suiteUI}
`
	suites, parseErr := functionalSuitesFromGo(suiteSource, "HEAD:internal/le/functional/suites.go")
	if parseErr != nil {
		return nil, parseErr
	}
	req := Requirement{RFC: "rfc9999", RID: "RFC9999-2-1", Level: "MUST", Source: "rfc/short/rfc9999.md", Line: 5}
	enrolled := map[string]bool{"rfc9999": true}
	baselineEnrolled := map[string]bool{"rfc9999": true}
	idLoss := checkIDAllocation([]Requirement{req}, map[string]bool{"RFC9999-2-2": true})
	coverageLoss := checkCoverageRatchet(
		[]Requirement{req}, nil, enrolled,
		map[string]map[string]bool{req.RID: {"negative": true}}, baselineEnrolled,
	)
	evidenceLoss := checkEvidenceRatchet(
		[]Requirement{req}, nil, enrolled, carriersFor([]string{"plugin"}, map[string]string{}),
		map[string]map[string]bool{req.RID: {"functional/verify": true}}, baselineEnrolled,
	)
	retired := checkRetiredRequirements(
		[]Requirement{req}, enrolled,
		map[string]bool{req.RID: true, "RFC9999-2-2": true}, baselineEnrolled,
		map[string]bool{"rfc9999": true}, map[string]bool{"rfc9999": true}, map[string]string{},
	)
	demoted := req
	demoted.Level = "SHOULD"
	levelLoss := checkLevelRatchet(root, []Requirement{demoted}, enrolled,
		map[string]string{req.RID: "MUST"}, baselineEnrolled)
	newSummary := checkNewSummaries(
		NewDeriver(root), map[string]bool{"rfc9999": true}, map[string]bool{"rfc8888": true},
		map[string]bool{}, []Requirement{req}, map[string]string{}, true,
	)
	emptyEnrolment := checkEnrolment(root, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{})

	return []leroot.SelftestResult{
		selftestResult("baseline/head-suite-parser", strings.Join(suites, ",") == "parse,ui",
			"the HEAD Go suite parser did not resolve the Gating constants"),
		selftestResult("baseline/id-allocation", len(idLoss) == 1 && strings.Contains(idLoss[0], "reuses a retired id"),
			"the high-water id ratchet accepted a retired ordinal"),
		selftestResult("baseline/coverage-ratchet", len(coverageLoss) == 1 && strings.Contains(coverageLoss[0], "negative"),
			"the polarity ratchet accepted lost evidence"),
		selftestResult("baseline/evidence-ratchet", len(evidenceLoss) == 1 && strings.Contains(evidenceLoss[0], "functional/verify"),
			"the non-unit evidence ratchet accepted a lost carrier tier"),
		selftestResult("baseline/retirement-ratchet", len(retired) == 1 && strings.Contains(retired[0], "RFC9999-2-2"),
			"the retired-requirement ratchet accepted a deleted id"),
		selftestResult("baseline/level-ratchet", len(levelLoss) == 1 && strings.Contains(levelLoss[0], "MUST"),
			"the level ratchet accepted an unauthorized demotion"),
		selftestResult("baseline/new-summary-ratchet", len(newSummary) == 1 && strings.Contains(newSummary[0], "not in rfc/enrolled.txt"),
			"a new gated summary remained unenrolled"),
		selftestResult("baseline/enrolment-ratchet", len(emptyEnrolment) == 1 && strings.Contains(emptyEnrolment[0], "nothing is enrolled"),
			"the empty enrolled set reported clean"),
	}, nil
}

func runCheckSelftest() ([]leroot.SelftestResult, error) {
	root, err := newSelftestTree("rfc-selftest-check-", map[string]string{
		".github/workflows/nightly.yml": selftestWorkflow,
		"test/plugin/broken.ci":         "# RFC requirement: RFC9999-2-1\n",
	})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	report, code := Check(root)
	refused := code == 2 && report.CannotRun != ""
	refused = refused && strings.Contains(report.CannotRun, "polarity")
	return []leroot.SelftestResult{
		selftestResult("check/public-driver", refused && len(report.Violations) == 0,
			"the public Check driver did not preserve the scanner refusal as cannot-run"),
	}, nil
}

// runRealTreeSelftest preserves the legacy suite's final property: the public
// check over the checkout must be clean before the selftest can succeed.
func runRealTreeSelftest() ([]leroot.SelftestResult, error) {
	root, err := lepath.Root()
	if err != nil {
		return nil, err
	}
	report, code := Check(root)
	return []leroot.SelftestResult{
		selftestResult("real-tree/public-check", code == 0, realTreeCheckDetail(report)),
	}, nil
}

func realTreeCheckDetail(report CheckReport) string {
	if report.CannotRun != "" {
		return report.CannotRun
	}
	if len(report.Violations) > 0 {
		return strings.Join(report.Violations, "; ")
	}
	return "the public Check returned a nonzero code without a diagnostic"
}
