// Design: docs/architecture/core-design.md -- the RFC engine proved against fixtures
// Overview: selftest.go -- fixture-suite orchestration and action answer
//
// selftest_core.go exercises the read-only parsers, scanners, checks, and HEAD
// comparison logic. Stateful artifacts and writers are in selftest_state.go.
package rfc

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// The selftest's own fixture identity: one enrolled RFC, its two requirements,
// and the paths a fixture tree writes them to. Every selftest tree spells the
// same names, so a fixture that drifted in one file and not another would make
// the gate judge a tree it never built.
const (
	selftestStem           = "rfc9999"
	selftestSummaryRel     = "rfc/short/rfc9999.md"
	selftestWorkflowRel    = ".github/workflows/nightly.yml"
	selftestTestPath       = "internal/sample/widget_test.go"
	selftestRIDSend        = "RFC9999-2-1"
	selftestRIDDrop        = "RFC9999-2-2"
	selftestCorrectionDate = "2026-08-26"

	// selftestEnrolled is the body of the fixture tree's rfc/enrolled.txt: the
	// one stem, on its own line.
	selftestEnrolled = selftestStem + "\n"
)

const selftestWorkflow = `on:
  schedule:
    - cron: '0 3 * * *'
jobs:
  audit:
    steps:
      - run: ./le integration interop
`

const selftestSummary = "# RFC 9999\n\n## Compliance Checklist\n\n- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2) {single-polarity: positive; no receiver input exists} {superseded: restated RFC10000-3-1; the successor states the same rule}\n- [ ] [RFC9999-2-2] [MUST NOT] A receiver MUST NOT drop the widget (§2)\n\nCorrection 2026-08-26: The row `RFC9999-2-1` quotes \"A speaker SHOULD send the widget and preserve its state.\".\n"

type summaryFixture struct {
	text        string
	expectedRID string
}

func summarySelftestFixture() summaryFixture {
	return summaryFixture{text: selftestSummary, expectedRID: selftestRIDSend}
}

func runSummarySelftest(fixture summaryFixture) ([]leroot.SelftestResult, error) {
	requirements, err := parseSummaryText(fixture.text, selftestStem, selftestSummaryRel)
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
		annotationOK = first.Annotation.Kind == AnnotationSinglePolarity
		annotationOK = annotationOK && first.Annotation.Polarity == PolarityPositive
	}
	successorOK := first.Superseded != nil
	if successorOK {
		successorOK = first.Superseded.Disposition == successorRestated
		successorOK = successorOK && first.Superseded.Target == "RFC10000-3-1"
	}
	_, mismatch := parseChecklistLine(
		"- [ ] [RFC9999-3-1] [MUST] A speaker MUST send the widget (§2)",
		selftestStem, selftestSummaryRel, 1,
	)
	corrections := parseCorrections(fixture.text)
	correctionOK := len(corrections) == 1
	if correctionOK {
		correctionOK = len(corrections[0].RIDs) == 1
		correctionOK = correctionOK && corrections[0].RIDs[0] == selftestRIDSend
		correctionOK = correctionOK && len(corrections[0].Quotes) == 1
	}

	return []leroot.SelftestResult{
		selftestResult("summary/checklist-id", len(requirements) == 2 && first.RID == fixture.expectedRID,
			"the checklist id or requirement count changed"),
		selftestResult("summary/level-and-section", first.Level == levelMust && first.Section == "2" && second.Level == "MUST NOT",
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
		selftestWorkflowRel:                         selftestWorkflow,
		selftestTestPath:                            "package sample\n// RFC requirement: RFC9999-2-1 positive\nfunc TestWidget() {}\n",
		"test/plugin/widget.ci":                     "# RFC requirement: RFC9999-2-2 negative\n",
		"test/editor/widget.et":                     "# RFC requirement: RFC9999-2-3 positive\n",
		"internal/le/interoplab/bgp/widget_test.go": "package bgp\n// RFC requirement: RFC9999-2-4 positive\nfunc TestInteropWidget() {}\n",
	}
	root, err := newSelftestTree("rfc-selftest-carriers-", files)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	carriers, err := carriers(root)
	if err != nil {
		return nil, err
	}
	tags, err := ScanTree(root)
	if err != nil {
		return nil, err
	}

	wanted := map[string]string{
		selftestTestPath:                            kindUnit,
		"test/plugin/widget.ci":                     kindFunctional,
		"test/editor/widget.et":                     "editor",
		"internal/le/interoplab/bgp/widget_test.go": kindInterop,
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
	interop, interopHeld := CarrierFor("internal/le/interoplab/bgp/widget_test.go", carriers)
	unrun, unrunHeld := CarrierFor("test/nosuite/widget.ci", carriers)
	_, draftHeld := CarrierFor("test/draft/widget.ci", carriers)

	return []leroot.SelftestResult{
		selftestResult("carriers/all-scanners", len(tags) == 4 && allKinds,
			"the Go, CI, and ET scanners did not all contribute their carrier"),
		selftestResult("carriers/scheduled-interop", interopHeld && interop.Tier == tierNightly,
			"the scheduled workflow did not grant the interop carrier its nightly tier"),
		selftestResult("carriers/unrun-refusal", unrunHeld && unrun.Tier == tierUnrun,
			"an unmatched CI suite was not selected as unrun"),
		selftestResult("carriers/draft-skip", !draftHeld,
			"the draft incubator was selected as evidence"),
	}, nil
}

func runCoverageSelftest() ([]leroot.SelftestResult, error) {
	requirements, err := parseSummaryText(selftestSummary, selftestStem, selftestSummaryRel)
	if err != nil {
		return nil, err
	}
	tags := make([]Tag, 0, 4)
	tags = append(tags,
		Tag{RID: selftestRIDSend, Polarity: PolarityPositive, File: selftestTestPath},
		Tag{RID: selftestRIDDrop, Polarity: PolarityPositive, File: selftestTestPath},
		Tag{RID: selftestRIDDrop, Polarity: PolarityNegative, File: selftestTestPath},
	)
	enrolled := map[string]bool{selftestStem: true}
	clean := evaluate(requirements, tags, enrolled)
	missing := evaluate(requirements, tags[:2], enrolled)
	unknown := evaluate(requirements, append(tags, Tag{RID: "RFC9999-9-9", Polarity: PolarityPositive}), enrolled)
	rows := CoverageRows(requirements, tags, carriersFor([]string{"plugin"}, map[string]string{}))
	rollupOK := len(rows) == 1
	if rollupOK {
		rollupOK = rows[0].Gated == 2 && rows[0].Both == 1 && rows[0].Annotated == 1
		rollupOK = rollupOK && rows[0].Outstanding() == 0
	}

	return []leroot.SelftestResult{
		selftestResult("coverage/evaluation-clean", len(clean) == 0,
			"complete polarity evidence produced a coverage violation"),
		selftestResult("coverage/missing-polarity", len(missing) == 1 && strings.Contains(missing[0], PolarityNegative),
			"removing the negative test did not produce the named violation"),
		selftestResult("coverage/unknown-id", len(unknown) == 1 && strings.Contains(unknown[0], "RFC9999-9-9"),
			"a tag for an unknown requirement id was accepted"),
		selftestResult("coverage/rollup", rollupOK,
			"the annotated and both-polarity populations did not partition the rollup"),
	}, nil
}

func runStatusSelftest() ([]leroot.SelftestResult, error) {
	rows := parseStatusLedger("| RFC 9999 | Widgets | Partial | unit tests | one MUST gap |\n")
	dispositions, err := parseDispositions("rfc8888 backlog the extraction is owed\n")
	if err != nil {
		return nil, err
	}
	_, malformed := parseDispositions("rfc7777 unknown not a disposition\n")
	gap := Requirement{
		RFC: selftestStem, RID: selftestRIDSend, Level: levelMust, Text: "MUST send", Section: "2",
		Annotation: &Annotation{Kind: AnnotationGap, Reason: "not implemented"},
	}
	hidden := checkStatusAgreement(
		[]Requirement{gap},
		map[string]LedgerRow{selftestStem: {Status: "Supported", Coverage: "complete"}},
		map[string]bool{selftestStem: true},
	)
	declared := checkSummaryDisposition(
		map[string]bool{"rfc8888": true, selftestStem: true},
		map[string]bool{selftestStem: true}, dispositions, map[string]bool{},
	)

	return []leroot.SelftestResult{
		selftestResult("status/public-row", rows[selftestStem].Status == "Partial" && rows[selftestStem].Remaining == "one MUST gap",
			"the public status row did not retain its status and remaining work"),
		selftestResult("status/disposition", len(declared) == 0 && dispositions["rfc8888"].Kind == dispositionBacklog,
			"an enrolled-or-declared summary was reported as unowned"),
		selftestResult("status/disposition-refusal", malformed != nil,
			"an unknown disposition was accepted"),
		selftestResult("status/gap-disclosure", len(hidden) == 1 && strings.Contains(hidden[0], selftestRIDSend),
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
	req := Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust, Source: selftestSummaryRel, Line: 5}
	enrolled := map[string]bool{selftestStem: true}
	baselineEnrolled := map[string]bool{selftestStem: true}
	idLoss := checkIDAllocation([]Requirement{req}, map[string]bool{selftestRIDDrop: true})
	coverageLoss := checkCoverageRatchet(
		[]Requirement{req}, nil, enrolled,
		map[string]map[string]bool{req.RID: {PolarityNegative: true}}, baselineEnrolled,
	)
	evidenceLoss := checkEvidenceRatchet(
		[]Requirement{req}, nil, enrolled, carriersFor([]string{"plugin"}, map[string]string{}),
		map[string]map[string]bool{req.RID: {"functional/verify": true}}, baselineEnrolled,
	)
	retired := checkRetiredRequirements(
		[]Requirement{req}, enrolled,
		map[string]bool{req.RID: true, selftestRIDDrop: true}, baselineEnrolled,
		map[string]bool{selftestStem: true}, map[string]bool{selftestStem: true}, map[string]string{},
	)
	demoted := req
	demoted.Level = "SHOULD"
	levelLoss := checkLevelRatchet(root, []Requirement{demoted}, enrolled,
		map[string]string{req.RID: levelMust}, baselineEnrolled)
	newSummary := checkNewSummaries(
		NewDeriver(root), map[string]bool{selftestStem: true}, map[string]bool{"rfc8888": true},
		map[string]bool{}, []Requirement{req}, map[string]string{}, true,
	)
	emptyEnrolment := checkEnrolment(root, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{})

	return []leroot.SelftestResult{
		selftestResult("baseline/head-suite-parser", strings.Join(suites, ",") == "parse,ui",
			"the HEAD Go suite parser did not resolve the Gating constants"),
		selftestResult("baseline/id-allocation", len(idLoss) == 1 && strings.Contains(idLoss[0], "reuses a retired id"),
			"the high-water id ratchet accepted a retired ordinal"),
		selftestResult("baseline/coverage-ratchet", len(coverageLoss) == 1 && strings.Contains(coverageLoss[0], PolarityNegative),
			"the polarity ratchet accepted lost evidence"),
		selftestResult("baseline/evidence-ratchet", len(evidenceLoss) == 1 && strings.Contains(evidenceLoss[0], "functional/verify"),
			"the non-unit evidence ratchet accepted a lost carrier tier"),
		selftestResult("baseline/retirement-ratchet", len(retired) == 1 && strings.Contains(retired[0], selftestRIDDrop),
			"the retired-requirement ratchet accepted a deleted id"),
		selftestResult("baseline/level-ratchet", len(levelLoss) == 1 && strings.Contains(levelLoss[0], levelMust),
			"the level ratchet accepted an unauthorized demotion"),
		selftestResult("baseline/new-summary-ratchet", len(newSummary) == 1 && strings.Contains(newSummary[0], "not in rfc/enrolled.txt"),
			"a new gated summary remained unenrolled"),
		selftestResult("baseline/enrolment-ratchet", len(emptyEnrolment) == 1 && strings.Contains(emptyEnrolment[0], "nothing is enrolled"),
			"the empty enrolled set reported clean"),
	}, nil
}

func runCheckSelftest() ([]leroot.SelftestResult, error) {
	root, err := newSelftestTree("rfc-selftest-check-", map[string]string{
		selftestWorkflowRel:     selftestWorkflow,
		"test/plugin/broken.ci": "# RFC requirement: RFC9999-2-1\n",
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

// The discrimination fixture: one proof record and one escape, over the two
// requirements selftestSummary declares, plus the two sources they name. The
// sources are part of the fixture because a record is re-verified against the
// tree: a fixture with no tagged unit in it would prove only that an absent
// unit is refused.
const (
	selftestDiscriminationRel  = "rfc/discrimination/rfc9999.json"
	selftestDiscriminationUnit = selftestTestPath +
		"::TestWidget"
	selftestCIPath       = "test/plugin/widget.ci"
	selftestProducerPath = "internal/sample/widget.go"
	selftestProducerUnit = selftestProducerPath + "::SendWidget"
	// selftestTablePath is the declaration-only file the escape names. An
	// escape claims a fact about the code its claim rests on, and this is the
	// fact: a file with no function body has nothing to break.
	selftestTablePath = "internal/sample/table.go"

	// The tag carries a CLAIM, and a claim that runs onto a second comment
	// line, because that is what two thirds of the corpus looks like and the
	// claim fingerprint is taken over the whole paragraph.
	selftestTestSource = "package sample\n\n" +
		"// RFC requirement: RFC9999-2-1 positive -- SendWidget answers the count it\n" +
		"// was given, so a speaker that sends one widget sends exactly one.\n" +
		"func TestWidget() {\n\tif SendWidget(1) != 1 {\n\t\tpanic(\"BUG: the widget was not sent\")\n\t}\n}\n"
	selftestProducerSource = "package sample\n\n" +
		"// SendWidget answers the widget the speaker sends.\n" +
		"func SendWidget(count int) int {\n\treturn count\n}\n"
	// The claim NAMES Widgets, which selftestTableSource declares. An escape is
	// tied to its claim by that name: without it the record would say only "some
	// declaration-only file exists", which is true of every package.
	selftestCISource = "# RFC requirement: RFC9999-2-2 negative -- the daemon drops a widget that\n" +
		"# is not in the Widgets table it was given.\n"
	selftestTableSource = "package sample\n\nvar Widgets = []int{1, 2, 3}\n"
	// selftestCIDirective is the assertion a functional record cites. A .ci has
	// no assertion numbering, so the citation is the directive line itself.
	selftestCIDirective = "expect=widget sent"

	// selftestBreak is the mutated text one proof record stores. No gate parses
	// it: a reviewer reads it to judge whether the break engages the claim.
	selftestBreak = "return count -> return 0"

	selftestDiscriminationBroken = `{"rfc":"rfc9999","records":[
{"rid":"RFC9999-2-1","polarity":"sideways","unit":"internal/sample/widget_test.go::TestWidget",` +
		`"unit-sha":"0123456789abcdef","route":"mutant"}
]}`
)

// selftestDiscriminationSources is the tree a record is verified against.
func selftestDiscriminationSources() map[string]string {
	return map[string]string{
		selftestTestPath:     selftestTestSource,
		selftestProducerPath: selftestProducerSource,
		selftestCIPath:       selftestCISource,
		selftestTablePath:    selftestTableSource,
	}
}

// selftestDiscriminationRecords is the proof and the escape, sealed against the
// sources above.
//
// The fingerprints are MINTED by sealDiscrimination rather than spelled. A
// literal sha would be a second declaration of text this same file already
// declares, and it would go wrong the first time somebody edits the fixture
// (ai/rules/principles.md).
func selftestDiscriminationRecords(tree string) ([]DiscriminationRecord, error) {
	covers, err := tagCoversIn(tree)
	if err != nil {
		return nil, err
	}
	unsealed := []DiscriminationRecord{{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit:  selftestDiscriminationUnit,
		Route: RouteMutant, Producer: selftestProducerUnit,
		Break: selftestBreak,
	}, {
		RID: selftestRIDDrop, Polarity: PolarityNegative,
		Unit: selftestCIPath, Route: RouteNoBreak,
		Reason: escapeDeclaration, Producer: selftestTablePath,
	}}

	records := make([]DiscriminationRecord, 0, len(unsealed))
	// Indexed rather than ranged by value: a record is 176 bytes.
	for position := range unsealed {
		sealed, err := sealDiscrimination(tree, covers, unsealed[position])
		if err != nil {
			return nil, err
		}
		records = append(records, sealed)
	}
	return records, nil
}

// selftestDiscriminationText renders those records as the artifact file.
//
// It marshals the schema type itself, so a fixture that no longer matches the
// schema cannot exist: the two are one declaration.
func selftestDiscriminationText(records []DiscriminationRecord) (string, error) {
	raw, err := json.Marshal(discriminationFile{RFC: selftestStem, Records: records})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// runDiscriminationSelftest exercises the recorded-proof artifact: what loads,
// what is refused rather than skipped, and what the published figures count.
func runDiscriminationSelftest() ([]leroot.SelftestResult, error) {
	requirements, err := parseSummaryText(selftestSummary, selftestStem, selftestSummaryRel)
	if err != nil {
		return nil, err
	}
	// The sources land first, because a record is SEALED against the tree it will
	// be verified against. A fixture whose hashes were computed any other way
	// would be stale the moment it was written.
	root, err := newSelftestTree("rfc-selftest-discrimination-", selftestDiscriminationSources())
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout

	records, err := selftestDiscriminationRecords(root)
	if err != nil {
		return nil, err
	}
	artifact, err := selftestDiscriminationText(records)
	if err != nil {
		return nil, err
	}
	if err := writeSelftestFiles(root, map[string]string{selftestDiscriminationRel: artifact}); err != nil {
		return nil, err
	}

	broken, err := newSelftestTree("rfc-selftest-discrimination-broken-", map[string]string{
		selftestDiscriminationRel: selftestDiscriminationBroken,
	})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(broken) //nolint:errcheck // temporary fixture checkout

	absent, err := newSelftestTree("rfc-selftest-discrimination-absent-", map[string]string{})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(absent) //nolint:errcheck // temporary fixture checkout

	loaded, err := loadDiscrimination(root)
	if err != nil {
		return nil, err
	}
	loadOK := len(loaded) == 2
	if loadOK {
		loadOK = loaded[0].RID == selftestRIDSend && loaded[0].Route == RouteMutant
		loadOK = loadOK && loaded[0].Unit == selftestDiscriminationUnit
		loadOK = loadOK && loaded[0].Source == selftestDiscriminationRel
	}
	covers, err := tagCoversIn(root)
	if err != nil {
		return nil, err
	}
	verdicts, err := verifyDiscrimination(root, loaded, covers)
	if err != nil {
		return nil, err
	}
	proven, escaped := discriminationRouteCounts(verdicts)

	_, refusal := loadDiscrimination(broken)
	empty, emptyErr := loadDiscrimination(absent)

	proof := verdicts[0]
	unknown := proof
	unknown.Record.RID = "RFC9999-9-9"
	fixtureFiles := selftestDiscriminationSources()
	unknownErrs := selftestRecordRatchet(fixtureFiles, requirements, unknown)
	duplicateErrs := selftestRecordRatchet(fixtureFiles, requirements, proof, proof)
	cleanErrs := selftestRecordRatchet(fixtureFiles, requirements, verdicts...)

	stale, err := discriminationStaleVerdicts(records)
	if err != nil {
		return nil, err
	}
	// One verdict per call: several verdicts of ONE record share a cover key, so
	// a single call would refuse the second as a duplicate and never reach the
	// staleness each one is here to prove.
	refusedNames := []string{staleUnit, staleProducer, staleRenamed, staleTagless, staleReworded}
	staleErrs := map[string][]string{}
	staleProven := 0
	for _, name := range refusedNames {
		staleErrs[name] = selftestRecordRatchet(stale[name].files, requirements, stale[name].verdict)
		proven, _ := discriminationRouteCounts([]DiscriminationVerdict{stale[name].verdict})
		staleProven += proven
	}

	// The obligation half. A cover is (requirement, polarity, tagged unit), so
	// these rows are built from the fixture's own cover rather than from a tag
	// list: the file key this ratchet used until 2026-08-31 billed nothing for a
	// second unit in a file the requirement was already proven in.
	sendPositive := Cover{RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestDiscriminationUnit}
	tagged := Tag{RID: selftestRIDSend, Polarity: PolarityPositive,
		File: selftestTestPath, Line: 3}
	gated := map[string]bool{selftestRIDSend: true}
	treeCovers := map[Cover][]Tag{sendPositive: {tagged}}
	otherAtPrior := map[Cover]bool{{RID: selftestRIDDrop, Polarity: PolarityNegative,
		Unit: selftestCIPath}: true}
	sendAtPrior := map[Cover]bool{sendPositive: true}

	owedNew := len(discriminationOwedTags(treeCovers, otherAtPrior, true, nil, gated))
	owedCommitted := len(discriminationOwedTags(treeCovers, sendAtPrior, true, nil, gated))
	owedNoBaseline := len(discriminationOwedTags(treeCovers, nil, false, nil, gated))
	owedProven := len(discriminationOwedTags(treeCovers, otherAtPrior, true, verdicts, gated))
	owedUngated := len(discriminationOwedTags(treeCovers, otherAtPrior, true, nil, nil))
	// The backlog reads the same predicate with the pushed branch as its
	// baseline, and answers nothing when that ref does not resolve.
	backlogCount := discriminationBacklog(discriminationInput{Gated: gated,
		HeadCovers: treeCovers, BacklogCovers: otherAtPrior, BacklogRef: backlogRevision})
	backlogSilent := discriminationBacklog(discriminationInput{Gated: gated,
		HeadCovers: treeCovers, BacklogCovers: otherAtPrior})

	owedErrs := checkDiscriminationRatchet(discriminationInput{Requirements: requirements,
		Gated: gated, Covers: treeCovers, HeadCovers: treeCovers,
		PriorCovers: otherAtPrior, PriorKnown: true})
	owedUncommitted := checkDiscriminationRatchet(discriminationInput{Requirements: requirements,
		Gated: gated, Covers: treeCovers, HeadCovers: nil,
		PriorCovers: otherAtPrior, PriorKnown: true})
	withdrawnErrs := checkDiscriminationRatchet(discriminationInput{Requirements: requirements,
		Gated: gated, Covers: treeCovers, HeadCovers: treeCovers,
		PriorCovers: sendAtPrior, PriorKnown: true,
		HeadRecords: sendAtPrior, HeadKnown: true})
	withdrawnWithTagGone := checkDiscriminationRatchet(discriminationInput{Requirements: requirements,
		Gated: gated, HeadRecords: sendAtPrior, HeadKnown: true})
	withdrawnNoBaseline := checkDiscriminationRatchet(discriminationInput{Requirements: requirements,
		Gated: gated, Covers: treeCovers, HeadCovers: treeCovers,
		PriorCovers: sendAtPrior, PriorKnown: true, HeadRecords: sendAtPrior})

	return []leroot.SelftestResult{
		selftestResult("discrimination/record-load", loadOK,
			"a well-formed record did not load with its requirement, route, unit and source"),
		selftestResult("discrimination/escape-counted-apart", proven == 1 && escaped == 1,
			"the no-break escape was counted as a proof"),
		selftestResult("discrimination/malformed-refusal", refusal != nil,
			"a record with an unknown polarity was skipped instead of refused"),
		selftestResult("discrimination/absent-tree", emptyErr == nil && len(empty) == 0,
			"an absent artifact tree was read as a tree the gate cannot read"),
		selftestResult("discrimination/unknown-requirement", len(unknownErrs) == 1 &&
			strings.Contains(unknownErrs[0], "RFC9999-9-9"),
			"a record naming an undeclared requirement was counted as a proof"),
		selftestResult("discrimination/duplicate-record", len(duplicateErrs) == 1 &&
			strings.Contains(duplicateErrs[0], selftestRIDSend),
			"two records claiming one tagged unit were both counted"),
		selftestResult("discrimination/clean-corpus", len(cleanErrs) == 0,
			"declared and distinct records produced a violation"),
		selftestResult("discrimination/owed-is-change-scoped",
			owedNew == 1 && owedCommitted == 0 && owedNoBaseline == 0 && owedProven == 0 &&
				owedUngated == 0,
			"the owed count did not follow the gated tagged units the tip commit added"),
		selftestResult("discrimination/new-tag-owes-a-proof",
			len(owedErrs) == 1 && strings.Contains(owedErrs[0], selftestTestPath) &&
				strings.Contains(owedErrs[0], selftestRIDSend) &&
				strings.Contains(owedErrs[0], PolarityPositive) &&
				strings.Contains(owedErrs[0], RouteMutant) && len(owedUncommitted) == 0,
			"a tagged unit the tip commit added carried no proof and was not refused, or a tag "+
				"present only in somebody's working tree was billed"),
		selftestResult("discrimination/backlog-is-measured-not-billed",
			backlogCount != nil && *backlogCount == 1 && backlogSilent == nil,
			"the unpushed backlog was not measured against the pushed branch, or it was "+
				"counted against a ref that does not resolve"),
		selftestResult("discrimination/withdrawn-record-refused",
			len(withdrawnErrs) == 1 && strings.Contains(withdrawnErrs[0], selftestDiscriminationUnit) &&
				len(withdrawnWithTagGone) == 0 && len(withdrawnNoBaseline) == 0,
			"deleting a committed proof beside a standing tag was allowed, or a deletion was billed where git could not answer"),
		selftestResult("discrimination/stale-proof-refused",
			staleProven == 0 &&
				selftestStaleRefused(staleErrs[staleUnit], ProofUnitChanged) &&
				selftestStaleRefused(staleErrs[staleProducer], ProofProducerChanged) &&
				selftestStaleRefused(staleErrs[staleReworded], ProofClaimChanged),
			"a record whose stored fingerprints no longer match the tree was published as proven"),
		selftestResult("discrimination/reworded-claim-stales-the-proof",
			stale[staleReworded].verdict.State == ProofClaimChanged,
			"widening a tag's claim left the old red published as a proof of the new sentence"),
		selftestResult("discrimination/orphan-is-removable",
			stale[staleRenamed].verdict.State == ProofUnitGone && stale[staleTagless].verdict.State == ProofTagGone &&
				len(staleErrs[staleRenamed]) == 0 && len(staleErrs[staleTagless]) == 0,
			"a record whose tag is gone was refused rather than reported for removal"),
		selftestResult("discrimination/comment-edit-keeps-proof", stale[staleCommented].verdict.Verified(),
			"a comment above the producer voided a proof; the key hashes BEHAVIOR, not text"),
		selftestResult("discrimination/half-written-refusal", discriminationHalfWrittenRefused(records[0], records[1]),
			"a record missing half of what its route needs, or an escape wearing a proof's fields, was accepted"),
	}, nil
}

// discriminationHalfWrittenRefused reports whether every half-written record is
// refused at load time.
//
// Each row removes or adds ONE field, so a rule that refused everything and a
// rule that refused nothing both fail. The escape rows matter as much as the
// proof rows: a no-break record carrying a break reads as evidence to a human
// and is counted as debt by the gate, which is the confusion the routes exist to
// keep apart, and an escape with no reason, or a reason outside the closed
// vocabulary, is the blanket opt-out the escape exists to refuse (R-9).
func discriminationHalfWrittenRefused(proof, escape DiscriminationRecord) bool {
	noProducer := proof
	noProducer.Producer = ""
	fileProducer := proof
	fileProducer.Producer = selftestProducerPath
	noBreakText := proof
	noBreakText.Break = "   "
	badUnitSHA := proof
	badUnitSHA.UnitSHA = "not-a-fingerprint"
	badProducerSHA := proof
	badProducerSHA.ProducerSHA = ""
	linedUnit := proof
	linedUnit.Unit = selftestTestPath + ":42"
	escapeNoReason := escape
	escapeNoReason.Reason = ""
	escapeUnknownReason := escape
	escapeUnknownReason.Reason = "there is nothing to break here"
	escapeForeignWithProducer := escape
	escapeForeignWithProducer.Reason = escapeForeign
	// foreign-producer names no code here, so the citation is the only field that
	// says WHICH claim it discharges. Without it the reason is true of every
	// interop tag at once (R-9).
	escapeForeignNoCitation := escape
	escapeForeignNoCitation.Reason = escapeForeign
	escapeForeignNoCitation.Producer = ""
	escapeNoProducer := escape
	escapeNoProducer.Producer = ""
	escapeWithCitation := escape
	escapeWithCitation.Citation = selftestCIDirective
	escapeWithBreak := escape
	escapeWithBreak.Break = selftestBreak
	proofWithReason := proof
	proofWithReason.Reason = escapeDeclaration

	// The intact pair is checked in the same walk, so "refuses everything"
	// cannot pass this row.
	if validateDiscrimination(proof, selftestDiscriminationRel, 1) != nil ||
		validateDiscrimination(escape, selftestDiscriminationRel, 2) != nil {
		return false
	}
	halfWritten := []DiscriminationRecord{noProducer, fileProducer, noBreakText,
		badUnitSHA, badProducerSHA, linedUnit, escapeNoReason, escapeUnknownReason,
		escapeForeignWithProducer, escapeForeignNoCitation, escapeNoProducer, escapeWithCitation, escapeWithBreak,
		proofWithReason}
	// Indexed rather than ranged by value: a record is 176 bytes.
	for position := range halfWritten {
		if validateDiscrimination(halfWritten[position], selftestDiscriminationRel, 1) == nil {
			return false
		}
	}
	return true
}

// selftestRecordRatchet runs the ninth ratchet over verdicts alone, which is
// where every refusal that judges a RECORD lives. The two that judge a CHANGE
// take their baselines by name at the call site.
//
// The fixture's own files stand for the COMMIT, because a stale record is
// refused only where the drift is committed (owner decision, 2026-08-31). A
// fixture with no committed side would prove that the ratchet judges nothing,
// which is the other half of that rule and is proven by its own row.
func selftestRecordRatchet(files map[string]string, requirements []Requirement,
	verdicts ...DiscriminationVerdict) []string {
	return checkDiscriminationRatchet(discriminationInput{Verdicts: verdicts,
		Requirements: requirements, Sources: newTextReader(files), Index: newScopeIndex(),
		HeadSources: newTextReader(files), HeadBlobsKnown: true})
}

// selftestStaleRefused reports whether one moved fingerprint produced exactly
// one violation naming the state it moved into.
func selftestStaleRefused(errs []string, state string) bool {
	return len(errs) == 1 && strings.Contains(errs[0], state)
}

// The trees one proof is re-verified over, named by what each one moved.
const (
	staleUnit      = "the tagged unit's assertion inverted"
	staleProducer  = "the producer rewritten"
	staleRenamed   = "the tagged unit renamed"
	staleTagless   = "the tag deleted, the unit kept"
	staleReworded  = "the claim reworded, the unit kept"
	staleCommented = "a comment added above and inside both units"
)

// discriminationStaleVerdicts re-verifies one proof against six trees, and
// answers what each one made of it.
//
// The pairing is what makes the property a property. A rule that refuses
// everything would satisfy the refusals on its own, and the comment tree is
// what says the record survives an edit that changed no behavior (R-6, A-7).
// The reworded tree is the pair to it: a claim is a comment too, and that one
// MUST void the proof while the doc comment beside it does not.
func discriminationStaleVerdicts(records []DiscriminationRecord) (map[string]staleReplay, error) {
	proof := records[0]
	proof.Source = selftestDiscriminationRel
	trees := map[string]map[string]string{
		staleUnit:     {selftestTestPath: strings.Replace(selftestTestSource, "!= 1", "== 1", 1)},
		staleProducer: {selftestProducerPath: strings.Replace(selftestProducerSource, "return count", "return 0", 1)},
		staleRenamed:  {selftestTestPath: strings.Replace(selftestTestSource, "TestWidget", "TestGadget", 1)},
		staleTagless: {selftestTestPath: strings.Replace(selftestTestSource,
			"// RFC requirement: RFC9999-2-1 positive -- SendWidget answers the count it\n"+
				"// was given, so a speaker that sends one widget sends exactly one.\n", "", 1)},
		staleReworded: {selftestTestPath: strings.Replace(selftestTestSource,
			"sends exactly one.", "sends exactly one, and rejects a second.", 1)},
		staleCommented: {
			selftestTestPath: "// Package sample is the fixture.\n\n" +
				strings.Replace(selftestTestSource, "func TestWidget() {\n", "func TestWidget() {\n\t// Send it.\n\n", 1),
			selftestProducerPath: "// Package sample is the fixture.\n\n" + selftestProducerSource,
		},
	}

	out := make(map[string]staleReplay, len(trees))
	for name, overlay := range trees {
		files := selftestDiscriminationSources()
		maps.Copy(files, overlay)
		root, err := newSelftestTree("rfc-selftest-discrimination-stale-", files)
		if err != nil {
			return nil, err
		}
		covers, coverErr := tagCoversIn(root)
		if coverErr != nil {
			removeErr := os.RemoveAll(root)
			return nil, errors.Join(coverErr, removeErr)
		}
		verdicts, err := verifyDiscrimination(root, []DiscriminationRecord{proof}, covers)
		removeErr := os.RemoveAll(root)
		if err != nil || removeErr != nil {
			return nil, errors.Join(err, removeErr)
		}
		out[name] = staleReplay{verdict: verdicts[0], files: files}
	}
	return out, nil
}

// staleReplay is one re-verification and the tree it was taken over.
//
// The tree is kept because the ratchet needs a COMMITTED side to judge a stale
// record against, and the fixture's own files are that side. Without it the
// replay would only show what the verifier answered, never what the gate does
// with the answer.
type staleReplay struct {
	verdict DiscriminationVerdict
	files   map[string]string
}
