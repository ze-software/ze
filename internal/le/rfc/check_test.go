package rfc

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// checkFixtureTree writes the smallest checkout Check can judge: one enrolled RFC whose
// summary declares one gated MUST, the nightly workflow the carrier scanner reads, and the
// public status row enrolment obliges. The caller adds or replaces files to plant what the
// test is about.
func checkFixtureTree(t *testing.T, extra map[string]string) string {
	t.Helper()

	files := map[string]string{
		selftestWorkflowRel: selftestWorkflow,
		selftestSummaryRel: "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
			"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n",
		"rfc/full/rfc9999.txt": "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt": "start 2026-07-29\nrate 0\n",
		// The tag scanner type-checks the packages carrying tags under every feature gate,
		// so the manifest must name at least one. This fixture's one gated package holds
		// nothing.
		"feature-gates.txt": "ze_widget  internal/widget\n",
	}
	maps.Copy(files, extra)

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}

	// The two generated ledger pages are written by the production writer rather than typed
	// here, so the fixture cannot drift from what the freshness check derives, and so a
	// planted violation is the only violation the tree carries.
	if _, err := IndexUpdate(root); err != nil {
		t.Fatalf("write the fixture ledger pages: %v", err)
	}
	return root
}

// VALIDATES: the public Check entry point reports the SHAPE of a violation -- exit 2, no
// cannot-run diagnostic, and one violation naming the requirement it planted and that
// requirement's level.
// PREVENTS: a partial driver that omits a check or invents a diagnostic.
//
// This test used to assert against the real checkout: that Check answered exactly five
// violations, and that each one matched a hardcoded RFC 9552 id, positionally. Its subject
// is the CHECKER, so making the working tree's conformance state its oracle meant every
// RFC anyone closed broke a test about `internal/le/rfc`'s plumbing, and the repair was
// always to retype the tree's new failure list here. Closing RFC9552-8.2.2-9 on 2026-08-28
// was the last time. The planted fixture asserts the same properties and costs a future
// conformance fix nothing. The real tree keeps the assertions below that survive it.
func TestRFCCheckReportsThePlantedViolation(t *testing.T) {
	root := checkFixtureTree(t, nil)

	report, code := Check(root)
	if code != 2 {
		t.Fatalf("a gated MUST with no test and no annotation answered %d, want 2:\n%s", code, report.Text())
	}
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if len(report.Violations) != 1 {
		t.Fatalf("the fixture reported %d violations, want 1:\n%s", len(report.Violations), report.Text())
	}
	violation := report.Violations[0]
	for _, want := range []string{selftestSummaryRel, selftestRIDSend, "[MUST]", "no test and no annotation"} {
		if !strings.Contains(violation, want) {
			t.Errorf("the violation does not name %q:\n%s", want, violation)
		}
	}
}

// VALIDATES: the same requirement stops being a violation once a tagged test covers both
// polarities, so the driver's verdict follows the evidence rather than the file list.
// PREVENTS: a checker that counts rows and never reads the tags, which would report the
// same number whatever the tests say.
func TestRFCCheckClearsTheViolationWhenBothPolaritiesAreProven(t *testing.T) {
	// A functional carrier rather than a Go one: a Go tag sends the driver to type-check the
	// package that holds it, and a fixture tree is not a Go module.
	root := checkFixtureTree(t, map[string]string{
		"test/plugin/widget.ci": "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend + " negative\n",
	})

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("a proven requirement answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
}

// VALIDATES: AC-1 -- an armed drain schedule reds `./le rfc check` at its public entry
// point, and the one violation it answers names the floor, the rate, the start date, the
// enrolled cap, the credited total and the remaining backlog.
// PREVENTS: a schedule that computes a floor and never compares it. The rate ships at 0
// (owner decision D5), so every other test in this package runs against an inert budget,
// and nothing anywhere showed that a non-zero rate can fail a tree.
//
// The armed budget is planted BESIDE the inert one checkFixtureTree writes, never in place
// of it: re-pointing the shared fixture would arm the schedule under every other check
// test. The tagged carrier clears the planted requirement violation, so the drain floor is
// the only thing left that can red this tree, and "exactly one violation" is then an
// assertion about the drain floor rather than about a count.
func TestRFCCheckReportsTheDrainFloorViolation(t *testing.T) {
	// Check reads the wall clock, so the start date is old enough that the verdict cannot
	// depend on the day the test runs: at rate 1 the floor is min(1, ceil(1 x months)),
	// which is 1 from 2020-02-01 onward. The fixture carries no sign-off, so the credited
	// total is 0 and the tree owes one walk it has not done.
	root := checkFixtureTree(t, map[string]string{
		"rfc/drain-budget.txt": "start 2020-01-01\nrate 1\n",
		"test/plugin/widget.ci": "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend + " negative\n",
	})

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 2 {
		t.Fatalf("an under-quota tree answered %d, want 2:\n%s", code, report.Text())
	}
	if len(report.Violations) != 1 {
		t.Fatalf("the fixture reported %d violation(s), want 1:\n%s",
			len(report.Violations), report.Text())
	}
	violation := report.Violations[0]
	for _, want := range []string{
		"rfc/drain-budget.txt",
		"requires 1 extraction sign-off(s) by now",
		"rate 1.0/calendar month",
		"since 2020-01-01",
		"capped at the 1 enrolled RFC(s)",
		"and there are 0 (rfc2119 0, prose 0, manual-walk 0",
		"leaving 1 unsigned",
		"./le rfc extraction-create stem <stem>",
	} {
		if !strings.Contains(violation, want) {
			t.Errorf("the drain violation does not name %q:\n%s", want, violation)
		}
	}
}

// VALIDATES: Check runs end to end over the real checkout and answers about it, whatever
// the tree's conformance state is.
// PREVENTS: a driver that only works on a fixture: the real corpus is two orders of
// magnitude larger, and a parser that refuses one of its summaries would show up here as a
// cannot-run rather than as a verdict.
//
// It asserts nothing about HOW MANY requirements are unproven, because that number is what
// the rest of the repository is meant to change.
func TestRFCCheckRunsOverTheRealCheckout(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the real tree could not run: %s", report.CannotRun)
	}
	if code != 0 && code != 2 {
		t.Fatalf("the real tree answered %d, want 0 or 2:\n%s", code, report.Text())
	}
	if (code == 2) != (len(report.Violations) > 0) {
		t.Fatalf("exit code %d disagrees with %d violation(s)", code, len(report.Violations))
	}
	for _, violation := range report.Violations {
		if strings.TrimSpace(violation) == "" {
			t.Error("a violation carries no diagnostic")
		}
	}
	if code != 0 {
		// The summary counts are populated only on the clean path, so a tree with an open
		// violation has nothing more to assert than the diagnostics above.
		return
	}
	if report.Enrolled == 0 || report.Gated == 0 || report.Tags == 0 {
		t.Fatalf("a clean real tree reported %d enrolled RFC(s), %d gated requirement(s) and %d tag(s)",
			report.Enrolled, report.Gated, report.Tags)
	}
}

// VALIDATES: `ze-rfc-check` is a claimed read-only action reached as `le rfc check`.
// PREVENTS: A gate claim with no action, or a check incorrectly marked as writing.
func TestRFCCheckActionIsClaimedReadOnly(t *testing.T) {
	for _, action := range Actions().Actions {
		if action.Verb != "check" {
			continue
		}
		if action.Writes {
			t.Error("the check action is marked as writing")
		}
		return
	}
	t.Error("check is not claimed by the rfc action table")
}

// VALIDATES: HEAD's functional Gating slice is resolved from Go constants.
// PREVENTS: A baseline reader that silently falls back because it cannot parse the real source shape.
func TestFunctionalSuitesFromGoResolvesTheGatingConstants(t *testing.T) {
	source := `package functional
const (
	suiteParse = "parse"
	suiteUi = "ui"
)
var Gating = []string{suiteParse, suiteUi}
`
	suites, err := functionalSuitesFromGo(source, "HEAD:internal/le/functional/suites.go")
	if err != nil {
		t.Fatalf("parsing the suite source: %v", err)
	}
	if strings.Join(suites, ",") != "parse,ui" {
		t.Fatalf("the parsed Gating list is %v", suites)
	}
}

// supportClaimSplit counts a set of Status cells by the three SHAPES a public support
// promise takes: the bare word, the word with a scope after it, and the legacy `Yes`.
//
// It is a test-local census and never a second copy of statusPromisesSupport: the
// producer answers one yes/no question, and this answers which of its three arms fired.
// The tests below assert that the three arms sum to what the producer accepts, so a
// shape that stopped being counted here cannot pass unnoticed.
func supportClaimSplit(statuses []string) (exact, qualified, yes int) {
	for _, status := range statuses {
		status = strings.TrimSpace(status)
		switch {
		case status == "Supported":
			exact++
		case status == "Yes":
			yes++
		case strings.HasPrefix(status, "Supported "):
			qualified++
		}
	}
	return exact, qualified, yes
}

// signedExtractionFixture answers what evaluateExtractions returned over the smallest
// checkout it will sign off: the selftest summary, the source text its requirements were
// extracted from, and the artifact the selftest writer derives from that source.
//
// The map comes from the producer rather than being typed here, because the property
// under test is that checkSupportedSignoff and the sign-off evaluator agree on what
// "signed" means. When skeleton is true every site carries a null disposition, which is
// the state `./le rfc extraction-create` writes and the state R-1 predicts a session will
// generate in bulk. The evaluator refuses it, so the stem is absent from the map.
func signedExtractionFixture(t *testing.T, skeleton bool) map[string]Extraction {
	t.Helper()

	root := t.TempDir()
	if err := writeSelftestFiles(root, map[string]string{
		selftestSummaryRel:     selftestSummary,
		"rfc/full/rfc9999.txt": selftestRFCSource,
	}); err != nil {
		t.Fatalf("fixture files: %v", err)
	}
	requirements, err := parseSummaryText(selftestSummary, selftestStem, selftestSummaryRel)
	if err != nil {
		t.Fatalf("parse the fixture summary: %v", err)
	}
	inventory, err := NewDeriver(root).Inventory(selftestStem, gatedCounts(requirements)[selftestStem])
	if err != nil {
		t.Fatalf("derive the fixture inventory: %v", err)
	}
	if inventory == nil {
		t.Fatal("the fixture source text derived no inventory")
	}

	document := extractionSelftestArtifact(inventory)
	if skeleton {
		sites, isSites := document[keySites].([]map[string]any)
		if !isSites {
			t.Fatalf("the selftest artifact holds %T sites", document[keySites])
		}
		for _, site := range sites {
			delete(site, "mapped-to")
			delete(site, "excluded-kind")
			delete(site, keyReason)
			site[keyDisposition] = nil
		}
	}
	body, err := marshalSelftestJSON(document)
	if err != nil {
		t.Fatalf("marshal the fixture artifact: %v", err)
	}
	if err := writeSelftestFiles(root, map[string]string{"rfc/extraction/rfc9999.json": body}); err != nil {
		t.Fatalf("write the fixture artifact: %v", err)
	}

	signed, _, err := evaluateExtractions(NewDeriver(root), requirements)
	if err != nil {
		t.Fatalf("evaluate the fixture artifact: %v", err)
	}
	return signed
}

// VALIDATES: a row that promises support, over a stem with a summary and no valid
// extraction sign-off, is refused, and the message names the stem, the Status cell
// verbatim and the artifact path the reviewer must write.
// PREVENTS: a public promise resting on a checklist nothing bounds. `./le rfc check`
// proves every requirement a summary LISTS; an obligation nobody extracted is owed no
// test, so the gate is green for it and stays green forever.
//
// The check is called leaf-directly with hand-built maps because it is not yet wired
// into check(): the real tree carries 40-odd unsigned support claims, so calling it from
// the driver would red the repository gate for every session sharing this checkout until
// the spec's data phases land.
func TestCheckSupportedSignoffRefusesUnsignedSupportedRow(t *testing.T) {
	errs := checkSupportedSignoff(
		map[string]LedgerRow{selftestStem: {Status: "Supported"}},
		map[string]Extraction{})

	if len(errs) != 1 {
		t.Fatalf("an unsigned Supported row answered %d violation(s): %v", len(errs), errs)
	}
	for _, want := range []string{selftestStem, "'Supported'", "rfc/extraction/rfc9999.json",
		"./le rfc extraction-create stem rfc9999"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("the violation does not name %q:\n%s", want, errs[0])
		}
	}
}

// VALIDATES: the intended end state is clean -- every support-promising row whose stem
// carries a summary and a valid sign-off passes, scope-qualified rows included.
// PREVENTS: a check that refuses its whole population, which would be red for a reason
// nobody could act on and would never go green however much work landed.
func TestCheckSupportedSignoffPassesWhenEverySupportedRowIsSigned(t *testing.T) {
	errs := checkSupportedSignoff(
		map[string]LedgerRow{
			"rfc1000": {Status: "Supported"},
			"rfc1001": {Status: "Supported on Linux"},
			"rfc1002": {Status: "Yes"},
		},
		map[string]Extraction{
			"rfc1000": {Stem: "rfc1000", Register: registerRFC2119},
			"rfc1001": {Stem: "rfc1001", Register: registerProse},
			"rfc1002": {Stem: "rfc1002", Register: registerRFC2119},
		})

	if len(errs) != 0 {
		t.Errorf("the intended end state answered %d violation(s): %v", len(errs), errs)
	}
}

// VALIDATES: a status that does NOT promise conformance is outside the population, with
// no summary and no sign-off: 'Unsupported', 'Future', 'Partial', 'Experimental' and the
// page's one 'Not supported' cell.
// PREVENTS: the wrong predicate. statusIsSupportClaim, the neighbor in check_status.go,
// returns true for everything except the literals 'Unsupported' and 'Future', so it
// accepts 'Partial' (66 rows on the page today), 'Experimental' (30) and 'Not supported'
// (1). A test covering only the two literals would pass against it and the check would
// demand a sign-off from 144 of 158 rows.
func TestCheckSupportedSignoffIgnoresUnsupportedAndFuture(t *testing.T) {
	rows := map[string]LedgerRow{
		"rfc2000": {Status: "Unsupported"},
		"rfc2001": {Status: "Future"},
		"rfc2002": {Status: "Partial"},
		"rfc2003": {Status: "Experimental"},
		"rfc2004": {Status: "Not supported"},
	}

	errs := checkSupportedSignoff(
		rows, map[string]Extraction{})

	if len(errs) != 0 {
		t.Errorf("a status that discloses rather than promises answered %d violation(s): %v",
			len(errs), errs)
	}
}

// VALIDATES: an artifact that EXISTS but is an unclassified skeleton does not satisfy the
// check, because the population it reads is the set evaluateExtractions ACCEPTED.
// PREVENTS: risk R-1 of the spec -- a session generating skeletons in bulk to show
// progress. The skeleton is built through the real producer over a fixture tree rather
// than by omitting a map key, so the test proves the two halves agree: what
// evaluateExtractions refuses is what this check refuses to credit.
func TestCheckSupportedSignoffRefusesSkeletonArtifact(t *testing.T) {
	tree := fixtureTree(t, map[string]string{
		"rfc/full/rfc9999.txt": derivedFixture,
		"rfc/extraction/rfc9999.json": `{"schema-version": 1, "stem": "rfc9999",
 "register": "rfc2119", "source-path": "rfc/full/rfc9999.txt",
 "source-sha": "` + RequirementSHA(derivedFixture) + `",
 "signed-off": "", "reviewer": "",
 "sections": [{"id": "front", "sites": 0, "disposition": null},
              {"id": "1", "sites": 0, "disposition": null},
              {"id": "2", "sites": 1, "disposition": null}],
 "sites": [{"id": "2:1", "quote": "A speaker MUST send the widget.", "disposition": null}]}`,
	})

	signed, extractionErrors, err := evaluateExtractions(NewDeriver(tree), nil)
	if err != nil {
		t.Fatalf("evaluate the fixture sign-off: %v", err)
	}
	if len(extractionErrors) == 0 {
		t.Fatal("the skeleton produced no extraction violation, so this test would prove nothing")
	}

	errs := checkSupportedSignoff(
		map[string]LedgerRow{selftestStem: {Status: "Supported"}},
		signed)

	if len(errs) != 1 {
		t.Fatalf("a skeleton satisfied the check: %d violation(s): %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "rfc/extraction/rfc9999.json") {
		t.Errorf("the violation does not name the artifact path:\n%s", errs[0])
	}
}

// VALIDATES: a sign-off the evaluator itself produced is credited, so checkSupportedSignoff
// and evaluateExtractions agree on what "signed" means, and a scope-qualified row is the
// one it is asked about.
// PREVENTS: a check that passes against a hand-built map while the real evaluator hands it
// a shape it does not credit. The sibling test above states the same end state over the
// three status shapes with typed Extraction values; this one derives its single value from
// the producer.
func TestCheckSupportedSignoffCreditsTheEvaluatorsOwnSignoff(t *testing.T) {
	signed := signedExtractionFixture(t, false)
	if _, held := signed[selftestStem]; !held {
		t.Fatal("the fixture artifact earned no sign-off, so this test asserts nothing")
	}

	errs := checkSupportedSignoff(
		map[string]LedgerRow{selftestStem: {Status: "Supported on Linux", Coverage: "the widget sender"}},
		signed)

	if len(errs) != 0 {
		t.Fatalf("a signed support claim answered %d violation(s): %v", len(errs), errs)
	}
}

// VALIDATES: the skeleton shape `./le rfc extraction-create` writes, every site disposition
// null, earns no credit when it is produced by the real artifact writer rather than typed
// as JSON.
// PREVENTS: risk R-1 reached by its second route. The sibling test above plants the null
// dispositions as artifact text; this one strips them from the document the selftest writer
// derives, so a generator that started emitting a creditable shape fails here.
func TestCheckSupportedSignoffRefusesTheGeneratedSkeleton(t *testing.T) {
	signed := signedExtractionFixture(t, true)
	if _, held := signed[selftestStem]; held {
		t.Fatal("a skeleton artifact earned a sign-off from evaluateExtractions")
	}

	errs := checkSupportedSignoff(
		map[string]LedgerRow{selftestStem: {Status: "Supported"}},
		signed)

	if len(errs) != 1 {
		t.Fatalf("a skeleton artifact answered %d violation(s): %v", len(errs), errs)
	}
	for _, want := range []string{selftestStem, "rfc/extraction/rfc9999.json"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("the violation does not name %q:\n%s", want, errs[0])
		}
	}
}

// VALIDATES: assumption A-1 of plan/spec-rfcgate-6-supported-extraction-signoff.md,
// re-derived from the summaries rather than retyped, and AC-4 of
// plan/spec-rfc-ledger-single-declaration.md: one summary renders exactly one row.
// PREVENTS: a scope carried forward from a table in a spec.
//
// The two denominators this test carried until 2026-09-01 -- the STEM set the page's
// parser answered, and the ROW count of the eight RFC tables -- were two numbers because
// the page was authored and a stem stated twice lost its earlier row. RFC 2759 held that
// shape until 460fdc0f8. They are one number now: a row IS a summary's `| Support |`
// declaration, so a stem cannot be stated twice and the bridge has nothing left to check.
// What survives is the split by section, which is the term the spec's scope cut turns on.
//
// A number that moves fails this test with both counts printed, which is the point: the
// scope is re-derived at the start of a phase and at closure, and a delta is recorded in
// the spec's Risks & Assumptions rather than absorbed.
func TestSupportedRowsHaveDerivableScope(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	metas, metaProblems, err := summaryMetas(root, nil)
	if err != nil {
		t.Fatalf("read the summaries: %v", err)
	}
	if len(metaProblems) > 0 {
		t.Fatalf("the corpus holds %d unparsable Meta table(s): %v", len(metaProblems), metaProblems)
	}

	var mapped, rfcTables, draftTable []string
	for _, stem := range sortMetaStems(metas) {
		meta := metas[stem]
		if !meta.HasRow() || !statusPromisesSupport(meta.Status) {
			continue
		}
		mapped = append(mapped, meta.Status)
		if meta.Support == "drafts" {
			draftTable = append(draftTable, meta.Status)
			continue
		}
		rfcTables = append(rfcTables, meta.Status)
	}
	exact, qualified, yes := supportClaimSplit(mapped)
	// No stem reads 'Yes' any more: that cell was RFC 1997's and the page's own
	// vocabulary paragraph never defined the word, so it now reads 'Supported'.
	// The shape is kept in the split rather than dropped, because the predicate
	// still accepts it and a future row could reintroduce it.
	if len(mapped) != 51 || exact != 40 || qualified != 11 || yes != 0 {
		t.Errorf("the summaries declare %d support-promising row(s) (%d exact, %d scope-qualified, %d 'Yes'), want 51 (40, 11, 0)",
			len(mapped), exact, qualified, yes)
	}
	if exact+qualified+yes != len(mapped) {
		t.Errorf("the three shapes sum to %d of %d accepted status cells, so a shape the producer accepts is uncounted",
			exact+qualified+yes, len(mapped))
	}

	rowExact, rowQualified, rowYes := supportClaimSplit(rfcTables)
	if len(rfcTables) != 48 || rowExact != 37 || rowQualified != 11 || rowYes != 0 {
		t.Errorf("the eight RFC sections carry %d support-promising row(s) (%d exact, %d scope-qualified, %d 'Yes'), want 48 (37, 11, 0)",
			len(rfcTables), rowExact, rowQualified, rowYes)
	}
	if len(mapped) != len(rfcTables)+len(draftTable) {
		t.Errorf("%d support-promising row(s) split into %d + %d, so a section outside the nine holds one",
			len(mapped), len(rfcTables), len(draftTable))
	}
}

// VALIDATES: AC-4 -- every summary renders at most one row, and no two summaries claim
// one place on the page.
// PREVENTS: the duplicate-key defect the authored page could hold and its parser hid. A
// stem stated twice kept only its LAST row, so a support promise could sit in public with
// no check in this package able to see it.
func TestOneSummaryRendersExactlyOneStatusRow(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	metas, metaProblems, err := summaryMetas(root, nil)
	if err != nil {
		t.Fatalf("read the summaries: %v", err)
	}
	if len(metaProblems) > 0 {
		t.Fatalf("the corpus holds %d unparsable Meta table(s): %v", len(metaProblems), metaProblems)
	}
	placed, err := placeRows(metas)
	if err != nil {
		t.Fatalf("place the rows: %v", err)
	}
	rows := 0
	for _, section := range placed {
		for range section {
			rows++
		}
	}

	// The duplicate a summary CAN still write is a shared rank, and it is the
	// one placeRows refuses. Two rows with one rank have no order, and a
	// tie-break invented in the renderer would move the page's reading order
	// out of the summaries that own it. Asserting "no stem appears twice" over
	// the real corpus proves nothing: placeRows appends each stem once under
	// its single Support key, so that loop cannot fail whatever the corpus
	// says.
	clash := map[string]Meta{
		"rfc1000": {Enrolment: enrolmentEnrolled, EnrolmentReason: "gated",
			Support: "bgp-base", Rank: 10, Area: "a", Status: "Partial",
			Coverage: "c", Remaining: "r"},
		"rfc1001": {Enrolment: enrolmentEnrolled, EnrolmentReason: "gated",
			Support: "bgp-base", Rank: 10, Area: "a", Status: "Partial",
			Coverage: "c", Remaining: "r"},
	}
	if _, err := placeRows(clash); err == nil {
		t.Error("two summaries claiming one rank were placed, so the page's row order is undefined")
	} else if !strings.Contains(err.Error(), "both claim rank 10") {
		t.Errorf("the refusal does not name the clash: %v", err)
	}
	declared := 0
	for _, meta := range metas {
		if meta.HasRow() {
			declared++
		}
	}
	if rows != declared {
		t.Errorf("%d summaries declare a row and %d were placed", declared, rows)
	}
}

// VALIDATES: AC-10 -- a clean tree publishes the three discrimination figures, in the
// report envelope and on the summary line, and each one counts what its name says.
// PREVENTS: a line that prints three constants. The fixture carries one proof record and
// one escape, so a counter that ignored the route would report 2 proven or 0 escaped.
func TestCheckReportCarriesDiscriminationCounters(t *testing.T) {
	// A functional carrier rather than a Go one for the TAGS: a Go tag sends the
	// driver to type-check the package that holds it, and a fixture tree is not a
	// Go module. The producer is a plain Go file carrying no tag, which the driver
	// never type-checks.
	// The negative tag CLAIMS what the escape's producer file declares. An escape
	// is tied to its claim by that name, so a claim naming nothing in the file it
	// points at would escape every tag equally (R-9).
	files := map[string]string{
		selftestCIPath: "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend +
			" negative -- a widget outside the Widgets table is dropped\n" +
			selftestCIDirective + "\n",
		selftestProducerPath: selftestProducerSource,
		selftestTablePath:    selftestTableSource,
		selftestCITablePath:  selftestCITableSource,
	}
	// One proof and one escape over the one requirement this fixture declares, so
	// the two figures cannot be read off a single row.
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestCIPath, Route: RouteMutant, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	escape := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityNegative,
		Unit: selftestCIPath, Route: RouteNoBreak,
		Reason: escapeDeclaration, Producer: selftestCITablePath,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof, escape)
	root := checkFixtureTree(t, files)

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("a proven fixture answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if report.DiscriminationProven != 1 {
		t.Errorf("proven is %d, want the 1 mutant record the fixture carries", report.DiscriminationProven)
	}
	if report.DiscriminationEscaped != 1 {
		t.Errorf("escaped is %d, want the 1 no-break record the fixture carries", report.DiscriminationEscaped)
	}
	if report.DiscriminationOwed != 0 {
		t.Errorf("owed is %d, want 0: a fixture tree has no HEAD, and a baseline that cannot be read accuses nobody", report.DiscriminationOwed)
	}
	if line := "discrimination: 1 proven, 0 owed, 1 escaped"; !strings.Contains(report.Text(), line) {
		t.Errorf("the summary omits %q:\n%s", line, report.Text())
	}
}

// VALIDATES: a record naming a requirement no summary declares reds the public gate, and
// the violation names the record and the file it was read from.
// PREVENTS: an artifact that is loaded, counted as proven, and never judged.
func TestCheckRefusesADiscriminationRecordForAnUndeclaredRequirement(t *testing.T) {
	files := map[string]string{
		selftestCIPath: "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend + " negative\n" +
			selftestCIDirective + "\n",
		selftestProducerPath: selftestProducerSource,
	}
	// Sealed against the requirement the fixture DOES declare, then re-pointed:
	// a record for an id nothing declares has no tag to take a claim
	// fingerprint from, so the minter refuses it before the ratchet can. What is
	// under test here is the ratchet's own refusal, reached with fingerprints
	// that are otherwise sound.
	undeclared := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestCIPath, Route: RouteMutant, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	undeclared.RID = "RFC9999-9-9"
	files[selftestDiscriminationRel] = discriminationArtifact(t, undeclared)
	root := checkFixtureTree(t, files)

	report, code := Check(root)
	if report.CannotRun != "" {
		t.Fatalf("the fixture tree could not run: %s", report.CannotRun)
	}
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("an undeclared requirement answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if !strings.Contains(report.Violations[0], "RFC9999-9-9") ||
		!strings.Contains(report.Violations[0], selftestDiscriminationRel) {
		t.Errorf("the violation names neither the record nor its file: %q", report.Violations[0])
	}
}

// VALIDATES: AC-1 -- a record whose stored break no longer reddens its tagged unit reds
// the public gate with exit 2, and the violation names the requirement, the polarity, the
// tagged unit key, the mutated text and the producer function.
// METHOD: one record recorded against the fixture's producer, then the producer rewritten
// under it. The proven count is checked to stay at zero, so the figure a reader trusts
// cannot be derived from a claim the gate refused.
// PREVENTS: the hole Phase 1 left standing -- a record that passes the schema and the
// corpus check counted as proven, so a hand-written record publishes a red nobody observed
// (ai/rules/principles.md).
func TestCheckDiscriminationRatchetReportsBrokenProof(t *testing.T) {
	files := map[string]string{
		selftestCIPath: "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend + " negative\n" +
			selftestCIDirective + "\n",
		selftestProducerPath: selftestProducerSource,
	}
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: selftestCIPath, Route: RouteMutant, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof)

	if report, code := Check(checkFixtureTree(t, files)); code != 0 || len(report.Violations) > 0 {
		t.Fatalf("the intact proof answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}

	// The producer is rewritten under the record, which is exactly the state a
	// hand-written record starts in: a fingerprint nothing was observed against.
	// COMMITTED, because that is the only drift the ratchet refuses since the owner
	// decision of 2026-08-31; the uncommitted half is its own test.
	files[selftestProducerPath] = strings.Replace(selftestProducerSource, "return count", "return 0", 1)
	report, code := Check(commitFixtureTree(t, files, nil))
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a broken proof answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{selftestRIDSend, PolarityPositive, selftestCIPath,
		selftestBreak, selftestProducerUnit, ProofProducerChanged} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the violation omits %q: %s", want, report.Violations[0])
		}
	}
	if report.DiscriminationProven != 0 {
		t.Errorf("proven is %d beside a refused record, want 0", report.DiscriminationProven)
	}
}

// VALIDATES: a record file that exists and cannot be parsed refuses the whole gate, naming
// the file, rather than reading as a corpus with nothing proven.
// PREVENTS: the fail-open shape a corrupt artifact takes -- zero records, zero violations,
// and a green bar (ai/rules/principles.md).
func TestCheckRefusesAnUnreadableDiscriminationRecord(t *testing.T) {
	root := checkFixtureTree(t, map[string]string{
		"test/plugin/widget.ci": "# RFC requirement: " + selftestRIDSend + " positive\n" +
			"# RFC requirement: " + selftestRIDSend + " negative\n",
	})
	// Planted AFTER the fixture's generated pages are written. The ledger
	// render reads the record corpus too, so a corrupt file present during the
	// write refuses the writer instead of the gate, and this test would prove
	// nothing about Check.
	writeFixtureFiles(t, root, map[string]string{
		selftestDiscriminationRel: `{"rfc":"rfc9999","records":[{"rid":"RFC9999-2-1","polarity":"positive",` +
			`"unit":"internal/sample/widget_test.go::TestWidget","rout":"mutant"}]}`,
	})

	report, code := Check(root)
	if code != 2 {
		t.Fatalf("a malformed record answered %d, want 2:\n%s", code, report.Text())
	}
	if !strings.Contains(report.CannotRun, selftestDiscriminationRel) {
		t.Fatalf("the refusal does not name the file: %q", report.CannotRun)
	}
}

// commitFixtureTree writes the fixture, commits it as HEAD, then lays the
// working-tree overlay on top and rewrites the generated ledger pages.
//
// The discrimination obligation is HEAD-relative, so a fixture with no commit
// proves only that the ratchet judges nothing where git cannot answer. What
// needs a commit is the other half: that a tagged unit this change ADDED is
// billed and one the tree has carried all along is not.
//
// The fixture repository is cut off from the developer's own git configuration,
// because a global commit.gpgsign would ask this test for a passphrase.
func commitFixtureTree(t *testing.T, committed, working map[string]string) string {
	t.Helper()

	root := checkFixtureTree(t, committed)
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "fixture")
	layFixture(t, root, working)
	return root
}

// commitFixtureTip commits base, commits tip on top of it, then lays working
// over the result without committing it.
//
// Two commits are what an obligation scoped to the TIP commit needs: base is
// HEAD^, tip is HEAD, and working is the edit nobody committed. A fixture with
// one commit has no HEAD^, so the obligation judges nothing there -- which is
// its own case and has its own row.
func commitFixtureTip(t *testing.T, base, tip, working map[string]string) string {
	t.Helper()

	root := checkFixtureTree(t, base)
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "base")
	layFixture(t, root, tip)
	commitFixture(t, root, "tip")
	layFixture(t, root, working)
	return root
}

// layFixture writes files over a fixture tree and rewrites the generated ledger
// pages, which the freshness check compares byte for byte against the corpus.
func layFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()

	writeFixtureFiles(t, root, files)
	if _, err := IndexUpdate(root); err != nil {
		t.Fatalf("rewrite the fixture ledger pages: %v", err)
	}
}

// commitFixture stages and commits whatever the fixture tree holds.
func commitFixture(t *testing.T, root, message string) {
	t.Helper()

	gitFixture(t, root, []string{"add", "-A"})
	gitFixture(t, root, []string{"-c", "user.email=fixture@example.invalid",
		"-c", "user.name=rfc-fixture", "commit", "-q", "-m", message})
}

// gitFixture runs one git command in the fixture's own throwaway repository.
func gitFixture(t *testing.T, root string, args []string) {
	t.Helper()

	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...) //nolint:gosec // this test's own throwaway fixture repository
	command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// writeFixtureFiles lays files over a fixture tree, deleting the ones whose
// body is the removal marker.
func writeFixtureFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if body == fixtureRemoved {
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove the fixture file %s: %v", rel, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
}

// fixtureRemoved is the body that says "this committed file is gone from the
// working tree", which is the state both halves of AC-4 are about.
const fixtureRemoved = "\x00removed"

// The fixture corpus these tests share. widget.ci carries both polarities of
// the one gated requirement, so it alone satisfies the coverage ratchet and a
// test may add or remove gadget.ci without moving any figure but this one's.
const (
	fixtureWidgetCI = "# RFC requirement: " + selftestRIDSend + " positive -- the daemon sends\n" +
		"# the widget when it is asked for one.\n" +
		"# RFC requirement: " + selftestRIDSend + " negative -- it sends none otherwise.\n" +
		selftestCIDirective + "\n"
	fixtureGadgetCIPath = "test/plugin/gadget.ci"
	fixtureGadgetClaim  = "-- a second carrier sends the widget too."
	fixtureGadgetCI     = "# RFC requirement: " + selftestRIDSend + " positive " + fixtureGadgetClaim + "\n" +
		selftestCIDirective + "\nexpect=stdout:contains=gadget\n"
)

// fixtureCorpus is the committed side every test below starts from.
func fixtureCorpus() map[string]string {
	return map[string]string{
		selftestCIPath:       fixtureWidgetCI,
		selftestProducerPath: selftestProducerSource,
	}
}

// VALIDATES: AC-2 -- a tagged unit the TIP COMMIT added, on an enrolled RFC's gated
// requirement, reds the gate with exit 2, and the violation names the file, the line, the
// requirement, the polarity and the proof route its carrier kind admits.
// VALIDATES: owner decision of 2026-09-01 -- the same tag left UNCOMMITTED is nobody's
// violation. Several sessions share this checkout, and `./le verify worktree` checks the
// commit under test out detached, where a tag that commit added IS the tip. Billing the
// working tree instead fires for every bystander and never in the one place it must.
// METHOD: three trees over one corpus. Two commits with the tag in the second, two commits
// with nothing added on top, and two commits with the tag laid over them uncommitted. The
// three together say the ratchet follows the tip COMMIT, rather than the corpus or the
// working tree.
// PREVENTS: an inert ratchet on one side (R-2: a floor that starts at zero and only forbids
// going below zero proves nothing) and, on the other, the cross-session red that gets a
// ratchet removed rather than obeyed (R-8).
func TestCheckDiscriminationRequiresProofForNewTag(t *testing.T) {
	unchanged := commitFixtureTip(t, fixtureCorpus(), fixtureCorpusNudge(), nil)
	if report, code := Check(unchanged); code != 0 || len(report.Violations) != 0 {
		t.Fatalf("the unchanged corpus answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	} else if report.DiscriminationOwed != 0 {
		t.Fatalf("owed is %d on a tip commit that added no tag, want 0", report.DiscriminationOwed)
	}

	added := commitFixtureTip(t, fixtureCorpus(),
		map[string]string{fixtureGadgetCIPath: fixtureGadgetCI}, nil)
	report, code := Check(added)
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a tag the tip commit added answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{fixtureGadgetCIPath + ":1", selftestRIDSend, PolarityPositive,
		RouteRevert, kindFunctional} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the violation omits %q: %s", want, report.Violations[0])
		}
	}

	// The same tag, in the working tree and committed by nobody. It bills nobody
	// until whoever wrote it commits it, which is the whole point of the rule:
	// every other session sharing this checkout would meet it as its own.
	uncommitted := commitFixtureTip(t, fixtureCorpus(), fixtureCorpusNudge(),
		map[string]string{fixtureGadgetCIPath: fixtureGadgetCI})
	quiet, code := Check(uncommitted)
	if code != 0 || len(quiet.Violations) != 0 {
		t.Fatalf("a tag nobody committed answered %d with %d violation(s):\n%s",
			code, len(quiet.Violations), quiet.Text())
	}
	if quiet.DiscriminationOwed != 0 {
		t.Errorf("owed is %d over an uncommitted tag, want 0", quiet.DiscriminationOwed)
	}
}

// fixtureCorpusNudge is a tip commit that adds no tag.
//
// git refuses an empty commit, so a two-commit fixture whose subject is "the tip
// added nothing tagged" still has to put SOMETHING in that commit. An untagged
// note under docs/ is the smallest thing no carrier scans.
func fixtureCorpusNudge() map[string]string {
	return map[string]string{"docs/fixture-note.md": "# Fixture\n\nThe tip commit added no tag.\n"}
}

// VALIDATES: AC-3, positive half -- a tagged unit whose behavior changed under a record
// reds the gate, and the violation names the unit and the record that went stale.
// VALIDATES: AC-3, negative half -- a comment added inside that same unit produces NO
// violation, because behaviorBytes strips comments and the record hashes behavior.
// METHOD: one sealed record, two edits to the unit it names. The pair is what makes it a
// property: a rule that fired on every edit would satisfy the first half alone.
func TestCheckDiscriminationFiresOnChangedUnit(t *testing.T) {
	files := fixtureCorpus()
	files[fixtureGadgetCIPath] = fixtureGadgetCI
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof)

	if report, code := Check(checkFixtureTree(t, files)); code != 0 || len(report.Violations) != 0 {
		t.Fatalf("the intact proof answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}

	changed := maps.Clone(files)
	changed[fixtureGadgetCIPath] = strings.Replace(fixtureGadgetCI,
		"expect=stdout:contains=gadget", "expect=stdout:contains=widget", 1)
	// COMMITTED, because the ratchet judges the drift against HEAD: an edit
	// nobody has committed is reported instead (owner decision, 2026-08-31).
	report, code := Check(commitFixtureTree(t, changed, nil))
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a changed tagged unit answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{fixtureGadgetCIPath, selftestRIDSend, ProofUnitChanged} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the violation omits %q: %s", want, report.Violations[0])
		}
	}
}

// VALIDATES: AC-3, negative half -- a comment edit inside a tagged unit that is not the
// tag's own claim leaves its record verifying, and the gate stays green.
// PREVENTS: the false-stale class the audit artifact already pays for, where a mechanical
// edit voided a verdict and cost two paragraphs of re-stamping (R-6).
func TestCheckDiscriminationIgnoresCommentOnlyEdit(t *testing.T) {
	files := fixtureCorpus()
	files[fixtureGadgetCIPath] = fixtureGadgetCI
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof)

	// The comment goes UNDER a directive rather than under the tag: a comment
	// line directly below a tag continues that tag's claim, and this test is
	// about the other kind of comment.
	files[fixtureGadgetCIPath] = fixtureGadgetCI + "# The gadget carrier also prints its name.\n"
	report, code := Check(checkFixtureTree(t, files))
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("a comment-only edit answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if report.DiscriminationProven != 1 {
		t.Errorf("proven is %d after a comment-only edit, want the 1 record the fixture carries",
			report.DiscriminationProven)
	}
}

// VALIDATES: AC-13 -- rewording a tag's CLAIM stales its record, even though the unit's
// behavior is untouched, so a proof of a modest claim cannot be widened into a proof of a
// larger one with no code edit at all (owner decision, 2026-08-31).
// METHOD: one sealed record, then the claim sentence widened and nothing else. The green
// pair is TestCheckDiscriminationIgnoresCommentOnlyEdit above: an unrelated comment inside
// the same unit leaves the same record verifying.
// PREVENTS: the exact over-claim this spec exists to stop, in its cheapest form. It needs
// no test edit, so every mechanism that watches for a changed assertion misses it.
func TestCheckDiscriminationStalesOnRewordedClaim(t *testing.T) {
	files := fixtureCorpus()
	files[fixtureGadgetCIPath] = fixtureGadgetCI
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof)

	widened := maps.Clone(files)
	widened[fixtureGadgetCIPath] = strings.Replace(fixtureGadgetCI, fixtureGadgetClaim,
		fixtureGadgetClaim+" and never sends a second one.", 1)
	// COMMITTED, for the reason TestCheckDiscriminationFiresOnChangedUnit states.
	report, code := Check(commitFixtureTree(t, widened, nil))
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a reworded claim answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{selftestRIDSend, fixtureGadgetCIPath, ProofClaimChanged} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the violation omits %q: %s", want, report.Violations[0])
		}
	}
	if report.DiscriminationProven != 0 {
		t.Errorf("proven is %d beside a refused record, want 0", report.DiscriminationProven)
	}

	// The claim put back, byte for byte: the record verifies again, so what the
	// gate followed was the sentence and not the edit.
	if restored, code := Check(checkFixtureTree(t, files)); code != 0 || len(restored.Violations) != 0 {
		t.Fatalf("the restored claim answered %d with %d violation(s):\n%s",
			code, len(restored.Violations), restored.Text())
	}
}

// VALIDATES: AC-4, first half -- a record committed and then deleted while the tag it
// proved is still in the tree reds the gate: the proven set only goes up.
// VALIDATES: AC-4, second half -- the same deletion beside the tag's own removal is legal,
// because a record dies with the tag it proves.
// PREVENTS: a proof withdrawn from the published ledger with the claim left standing.
func TestCheckDiscriminationProvenSetIsMonotonic(t *testing.T) {
	committed := fixtureCorpus()
	committed[fixtureGadgetCIPath] = fixtureGadgetCI
	proof := sealFixture(t, committed, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	committed[selftestDiscriminationRel] = discriminationArtifact(t, proof)

	withdrawn := commitFixtureTree(t, committed,
		map[string]string{selftestDiscriminationRel: fixtureRemoved})
	report, code := Check(withdrawn)
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a withdrawn record answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{selftestRIDSend, PolarityPositive, fixtureGadgetCIPath} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the violation omits %q: %s", want, report.Violations[0])
		}
	}

	// The tag deleted with its record. widget.ci still carries both polarities,
	// so no coverage figure moves and this branch is the only thing under test.
	both := commitFixtureTree(t, committed, map[string]string{
		selftestDiscriminationRel: fixtureRemoved,
		fixtureGadgetCIPath:       fixtureRemoved,
	})
	if restored, code := Check(both); code != 0 || len(restored.Violations) != 0 {
		t.Fatalf("a record deleted beside its own tag answered %d with %d violation(s):\n%s",
			code, len(restored.Violations), restored.Text())
	}
}

// VALIDATES: AC-4 -- a record whose TAG is gone while its unit stands is REPORTED as
// removable rather than refused, and it is counted as proven by nothing.
// METHOD: the tag removed from a carrier the record names, the carrier kept. Phase 2 made
// every unresolvable record a violation; this is the exemption AC-4 asks for.
// PREVENTS: a session that deleted a tag being billed for the record it correctly orphaned.
func TestCheckDiscriminationOrphanRecordIsRemovable(t *testing.T) {
	files := fixtureCorpus()
	files[fixtureGadgetCIPath] = fixtureGadgetCI
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, proof)
	files[fixtureGadgetCIPath] = selftestCIDirective + "\nexpect=stdout:contains=gadget\n"

	report, code := Check(checkFixtureTree(t, files))
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("an orphaned record answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if len(report.DiscriminationRemovable) != 1 ||
		!strings.Contains(report.DiscriminationRemovable[0], fixtureGadgetCIPath) {
		t.Fatalf("the orphan was not reported as removable: %v", report.DiscriminationRemovable)
	}
	if report.DiscriminationProven != 0 {
		t.Errorf("proven is %d beside an orphaned record, want 0", report.DiscriminationProven)
	}
	if !strings.Contains(report.Text(), "record(s) can be removed") {
		t.Errorf("the summary says nothing about the removable record:\n%s", report.Text())
	}
}

// VALIDATES: AC-9 -- an `RFC requirement:` comment in a non-test Go file that no carrier
// claims is reported, with the ones no scanner could read counted apart.
// METHOD: two production tags, one well formed and one with no polarity, in a file the
// carrier table refuses. The gate stays GREEN: these predate the check, and a ratchet that
// reds the tree over standing debt gets removed rather than obeyed (R-8).
// PREVENTS: what this spec found while measuring its own population -- ten such tags in
// the checkout, read by no scanner and counted by no gate, that look like evidence to a
// person opening the file.
func TestCheckReportsUnscannedProductionTags(t *testing.T) {
	files := fixtureCorpus()
	files[selftestProducerPath] = selftestProducerSource +
		"\n// RFC requirement: " + selftestRIDSend + " positive -- the sender emits it.\n" +
		"func SendAgain(count int) int { return SendWidget(count) }\n" +
		"\n// RFC requirement: " + selftestRIDSend + " -- no polarity at all.\n" +
		"func SendOnce(count int) int { return SendWidget(count) }\n"

	report, code := Check(checkFixtureTree(t, files))
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("production tags answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if len(report.UnscannedTags) != 2 {
		t.Fatalf("the report names %d unscanned tag(s), want the 2 the fixture plants: %+v",
			len(report.UnscannedTags), report.UnscannedTags)
	}
	if report.UnscannedTags[0].File != selftestProducerPath || report.UnscannedTags[0].Refusal != "" ||
		report.UnscannedTags[0].Polarity != PolarityPositive {
		t.Errorf("the well-formed production tag was misread: %+v", report.UnscannedTags[0])
	}
	if report.UnscannedTags[1].Refusal == "" {
		t.Errorf("the production tag with no polarity carries no refusal: %+v", report.UnscannedTags[1])
	}
	if !strings.Contains(report.Text(), "unscanned: 2 ") ||
		!strings.Contains(report.Text(), "1 of those would be refused") {
		t.Errorf("the summary does not publish the unscanned population:\n%s", report.Text())
	}
}

// VALIDATES: owner decision of 2026-08-31 -- a record staled by an edit NOBODY HAS
// COMMITTED is reported, never refused, and the same edit committed reds the gate.
// METHOD: one sealed record and one producer rewrite, run twice over the same fixture:
// once as a working-tree overlay on a commit that still holds the original, and once as
// the commit itself. The pair is the property. A rule that reported everything would
// satisfy the first half alone, and the rule this replaced refused both.
// PREVENTS: one session's uncommitted edit redding `./le rfc check` for every session
// sharing this checkout, at a 576-second re-record to clear an interop one. A rule that
// reds the tree on unrelated work gets removed rather than obeyed.
func TestCheckDiscriminationDriftIsJudgedAgainstHead(t *testing.T) {
	committed := fixtureCorpus()
	committed[fixtureGadgetCIPath] = fixtureGadgetCI
	proof := sealFixture(t, committed, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	committed[selftestDiscriminationRel] = discriminationArtifact(t, proof)
	broken := strings.Replace(selftestProducerSource, "return count", "return 0", 1)

	report, code := Check(commitFixtureTree(t, committed,
		map[string]string{selftestProducerPath: broken}))
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("an uncommitted edit under a record answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if len(report.DiscriminationDrifted) != 1 {
		t.Fatalf("the drifted list carries %d record(s), want the 1 the edit staled:\n%s",
			len(report.DiscriminationDrifted), report.Text())
	}
	for _, want := range []string{selftestRIDSend, fixtureGadgetCIPath, ProofProducerChanged} {
		if !strings.Contains(report.DiscriminationDrifted[0], want) {
			t.Errorf("the drifted line omits %q: %s", want, report.DiscriminationDrifted[0])
		}
	}
	if report.DiscriminationProven != 0 {
		t.Errorf("proven is %d beside a record the working tree contradicts, want 0",
			report.DiscriminationProven)
	}

	// The same edit, committed. It is now the author's, and it reds.
	committed[selftestProducerPath] = broken
	refused, code := Check(commitFixtureTree(t, committed, nil))
	if code != 2 || len(refused.Violations) != 1 {
		t.Fatalf("a committed edit under a record answered %d with %d violation(s):\n%s",
			code, len(refused.Violations), refused.Text())
	}
	if !strings.Contains(refused.Violations[0], ProofProducerChanged) {
		t.Errorf("the violation does not name the fingerprint that moved: %s", refused.Violations[0])
	}
}

// VALIDATES: owner decision of 2026-09-01, second half -- the WIDER obligation is measured
// against the pushed branch and REPORTED, never billed: it moves no violation, changes no
// exit code, and its report line says in its own words that it enforces nothing.
// METHOD: three commits and a remote-tracking ref. origin/main is the first, the second
// adds one tagged carrier, and the third adds nothing tagged. The tip commit therefore
// owes nothing and the tree is green, while the branch is one tagged unit ahead of what
// was pushed. A tree whose ref does not resolve is the pair, and it prints no line at all
// rather than a figure taken against nothing.
// PREVENTS: billing a backlog nobody can clear inside the change in hand, which is the R-8
// failure at the scale that gets a ratchet removed rather than obeyed, and its opposite --
// publishing a count against a baseline that was never read.
func TestCheckDiscriminationBacklogIsMeasuredNotBilled(t *testing.T) {
	root := checkFixtureTree(t, fixtureCorpus())
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "pushed")
	layFixture(t, root, map[string]string{fixtureGadgetCIPath: fixtureGadgetCI})
	commitFixture(t, root, "unpushed, one tagged carrier added")
	layFixture(t, root, fixtureCorpusNudge())
	commitFixture(t, root, "unpushed, nothing tagged")

	// Before the ref exists the branch is measured against nothing, and the
	// report says nothing rather than guessing a baseline.
	silent, code := Check(root)
	if code != 0 || len(silent.Violations) != 0 {
		t.Fatalf("a tip commit that added no tag answered %d with %d violation(s):\n%s",
			code, len(silent.Violations), silent.Text())
	}
	if silent.DiscriminationBacklog != nil {
		t.Fatalf("an unresolvable ref measured %d, want no measurement at all",
			*silent.DiscriminationBacklog)
	}
	if strings.Contains(silent.Text(), "added since") {
		t.Errorf("the report published a backlog line with no baseline to take it against:\n%s",
			silent.Text())
	}

	gitFixture(t, root, []string{"update-ref", "refs/remotes/origin/main", "HEAD~2"})
	report, code := Check(root)
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("the measured backlog answered %d with %d violation(s), and it enforces "+
			"nothing:\n%s", code, len(report.Violations), report.Text())
	}
	if report.DiscriminationOwed != 0 {
		t.Errorf("owed is %d, want 0: the tip commit added no tag, so the backlog sits "+
			"behind the obligation rather than inside it", report.DiscriminationOwed)
	}
	if report.DiscriminationBacklog == nil || *report.DiscriminationBacklog != 1 {
		t.Fatalf("the backlog did not measure the 1 tagged unit the unpushed commits added "+
			"against origin/main:\n%s", report.Text())
	}
	want := "discrimination: 1 tagged unit(s) carry a tag added since origin/main with no proof recorded"
	if !strings.Contains(report.Text(), want) {
		t.Errorf("the report omits %q:\n%s", want, report.Text())
	}
}

// fixtureProducerPair is the producer file with a SECOND function beside the one
// a record fingerprints. Two functions are what tells a file-level comparison
// from a unit-level one: an edit to the second says nothing about the first.
const fixtureProducerPair = "package sample\n\n" +
	"// SendWidget answers the widget the speaker sends.\n" +
	"func SendWidget(count int) int {\n\treturn count\n}\n\n" +
	"// WidgetName answers the name this speaker gives a widget.\n" +
	"func WidgetName() string {\n\treturn \"widget\"\n}\n"

// VALIDATES: the drift a record went stale against is judged at the granularity the record
// FINGERPRINTS -- the producer FUNCTION -- so an unrelated uncommitted edit elsewhere in
// the producer's file cannot downgrade a committed drift from a violation to a report.
// METHOD: one record sealed against SendWidget, that function rewritten and COMMITTED, and
// a second function in the same file rewritten and left uncommitted. The pair beside
// TestCheckDiscriminationDriftIsJudgedAgainstHead is what makes it a property: an
// uncommitted edit to the fingerprinted function itself still only reports.
// PREVENTS: the fail-open shape the file-level comparison had. Any session editing any
// line of a producer's file silenced the author's own violation, and the record went on
// being published as unproven with nobody billed for re-recording it.
func TestCheckDiscriminationDriftIsJudgedAtUnitGranularity(t *testing.T) {
	sealed := fixtureCorpus()
	sealed[fixtureGadgetCIPath] = fixtureGadgetCI
	sealed[selftestProducerPath] = fixtureProducerPair
	proof := sealFixture(t, sealed, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive,
		Unit: fixtureGadgetCIPath, Route: RouteRevert, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	sealed[selftestDiscriminationRel] = discriminationArtifact(t, proof)

	// The fingerprinted function rewritten, and COMMITTED. The drift is the
	// commit's, so the author owes a re-record and the gate says so.
	committed := maps.Clone(sealed)
	committed[selftestProducerPath] = strings.Replace(fixtureProducerPair, "return count", "return 0", 1)
	// A DIFFERENT function in the same file, edited and left uncommitted. It
	// explains none of the drift above it.
	elsewhere := strings.Replace(committed[selftestProducerPath], `return "widget"`, `return "gadget"`, 1)

	report, code := Check(commitFixtureTree(t, committed,
		map[string]string{selftestProducerPath: elsewhere}))
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a committed drift beside an unrelated uncommitted edit answered %d with "+
			"%d violation(s), want exit 2 and the one violation the commit owes:\n%s",
			code, len(report.Violations), report.Text())
	}
	if !strings.Contains(report.Violations[0], ProofProducerChanged) {
		t.Errorf("the violation does not name the fingerprint that moved: %s", report.Violations[0])
	}
	if len(report.DiscriminationDrifted) != 0 {
		t.Errorf("the drifted list carries %d record(s), want 0: the drift was committed, so "+
			"it is a violation rather than somebody's working copy:\n%s",
			len(report.DiscriminationDrifted), report.Text())
	}
}

// VALIDATES: owner decision of 2026-08-31 -- the WIDE reading of the obligation is
// MEASURED and published, and enforces nothing: a grandfathered tagged unit whose behavior
// changed since HEAD with no record counts on the report line and adds no violation.
// METHOD: one committed tagged unit with no record, changed in the working tree, beside a
// comment-only edit to a second one. The pair is what proves the predicate is ChangedTags
// and not "the file was touched".
// PREVENTS: publishing a number nobody can act on, and the opposite failure of enforcing a
// backlog before anyone has measured it.
func TestCheckDiscriminationMeasuresChangedGrandfatheredUnits(t *testing.T) {
	committed := fixtureCorpus()
	committed[fixtureGadgetCIPath] = fixtureGadgetCI

	clean, code := Check(commitFixtureTree(t, committed, nil))
	if code != 0 || len(clean.Violations) != 0 {
		t.Fatalf("the committed corpus answered %d with %d violation(s):\n%s",
			code, len(clean.Violations), clean.Text())
	}
	if clean.DiscriminationChanged != 0 {
		t.Fatalf("an unchanged corpus measured %d changed unit(s), want 0", clean.DiscriminationChanged)
	}

	changed, code := Check(commitFixtureTree(t, committed, map[string]string{
		fixtureGadgetCIPath: strings.Replace(fixtureGadgetCI,
			"expect=stdout:contains=gadget", "expect=stdout:contains=widget", 1),
	}))
	if code != 0 || len(changed.Violations) != 0 {
		t.Fatalf("a changed grandfathered unit answered %d with %d violation(s), and the "+
			"measurement enforces nothing:\n%s", code, len(changed.Violations), changed.Text())
	}
	if changed.DiscriminationChanged != 1 {
		t.Errorf("the measurement is %d, want the 1 grandfathered unit whose behavior changed:\n%s",
			changed.DiscriminationChanged, changed.Text())
	}

	commented, code := Check(commitFixtureTree(t, committed, map[string]string{
		fixtureGadgetCIPath: fixtureGadgetCI + "# The gadget carrier also prints its name.\n",
	}))
	if code != 0 || commented.DiscriminationChanged != 0 {
		t.Errorf("a comment-only edit measured %d changed unit(s) and answered %d, want 0 and 0",
			commented.DiscriminationChanged, code)
	}
}

// VALIDATES: owner decision B's measurement over the path 3,802 of the in-scope tags
// take -- a unit key naming a FUNCTION, resolved through resolveKeyText and judged by
// ChangedTags over the function body alone.
// VALIDATES: a unit the walk cannot read at both revisions is counted apart and published,
// never dropped into the changed count's zero.
// METHOD: one Go function tagged at HEAD, then four working trees: unchanged, its body
// inverted, a comment added inside it, and the function renamed away. Driven through
// discriminationChangedUnits directly, because a Go tag inside the Check fixture would
// send checkTagPackagesCompile to a `go vet` over a tree that carries no go.mod.
// PREVENTS: a measurement that reports a clean corpus it never opened. Every failure on
// this path was a silent continue, so a resolver answering nothing looked exactly like a
// backlog of zero (ai/rules/principles.md).
func TestDiscriminationMeasuresChangedGoFunctionUnits(t *testing.T) {
	unit := selftestTestPath + "::TestWidget"
	key := Cover{RID: selftestRIDSend, Polarity: PolarityPositive, Unit: unit}
	head := map[string]string{selftestTestPath: selftestTestSource}
	measure := func(working map[string]string) (int, int) {
		return discriminationChangedUnits(discriminationInput{
			Gated:      map[string]bool{selftestRIDSend: true},
			Covers:     map[Cover][]Tag{key: {{RID: selftestRIDSend, Polarity: PolarityPositive}}},
			HeadCovers: map[Cover][]Tag{key: {{RID: selftestRIDSend, Polarity: PolarityPositive}}},
			Sources:    newTextReader(working), Index: newScopeIndex(), HeadTagBlobs: head})
	}

	for name, working := range map[string]map[string]string{
		"an unchanged function": head,
		"a comment added inside the function": {selftestTestPath: strings.Replace(selftestTestSource,
			"func TestWidget() {\n", "func TestWidget() {\n\t// The widget count is one.\n", 1)},
	} {
		if changed, unresolved := measure(working); changed != 0 || unresolved != 0 {
			t.Errorf("%s measured %d changed and %d unresolved, want 0 and 0", name, changed, unresolved)
		}
	}

	inverted := map[string]string{selftestTestPath: strings.Replace(selftestTestSource, "!= 1", "== 1", 1)}
	if changed, unresolved := measure(inverted); changed != 1 || unresolved != 0 {
		t.Errorf("an inverted assertion measured %d changed and %d unresolved, want 1 and 0",
			changed, unresolved)
	}

	renamed := map[string]string{selftestTestPath: strings.Replace(selftestTestSource,
		"TestWidget", "TestGadget", 1)}
	if changed, unresolved := measure(renamed); changed != 0 || unresolved != 1 {
		t.Errorf("a unit the working tree no longer holds measured %d changed and %d unresolved, "+
			"want 0 and 1: a unit nobody could read is not a unit nobody changed", changed, unresolved)
	}
}

// VALIDATES: exactly ONE disposition excuses a public support claim, and it is
// the one that states a property of the DOCUMENT.
// PREVENTS: the escape widening into a blanket. Every other kind says the
// extraction is owed or unreachable, and a claim resting on either is what this
// check exists to refuse -- so an escape keyed on "is not enrolled" would exempt
// the whole un-enrolled population. `source-restricted` sat on the excusing side
// until 2026-09-02 and is on this side now: being unable to bound a claim is a
// reason to stop making it, not a reason to be excused from proving it.
func TestOnlyANonNormativeDispositionExcusesASupportClaim(t *testing.T) {
	rows := map[string]LedgerRow{selftestStem: {Status: "Experimental"}}
	stems := map[string]bool{selftestStem: true}

	for _, one := range []struct {
		kind    string
		excused bool
	}{
		{dispositionNonNormative, true},
		{dispositionSourceRestricted, false},
		{dispositionBacklog, false},
		{dispositionBlocked, false},
	} {
		t.Run(one.kind, func(t *testing.T) {
			errs := checkUnprovenSupport(nil, rows, stems,
				map[string]Disposition{selftestStem: {Kind: one.kind, Reason: "why"}},
				map[string]Extraction{}, map[string]string{})
			if one.excused && len(errs) != 0 {
				t.Errorf("%s did not excuse the claim: %v", one.kind, errs)
			}
			if !one.excused && len(errs) != 1 {
				t.Errorf("%s answered %d violation(s), want 1", one.kind, len(errs))
			}
		})
	}
}

// VALIDATES: the remedy this refusal prints offers only a disposition that
// ACTUALLY clears it.
// PREVENTS: a refusal that sends its reader to a fix which does not work. The
// message closed by telling the author that a standard whose text may not be
// redistributed "declares `| Enrolment | source-restricted |` instead", and the
// round-2 fix on 2026-09-02 stopped that kind excusing a support claim. An
// author who followed the instruction met the same violation on the next run,
// with nothing naming the contradiction. A refusal's remedy is part of the
// guard: a wrong one costs the reader the round trip the guard exists to save.
func TestTheUnprovenSupportRemedyOffersOnlyAnExcusingDisposition(t *testing.T) {
	rows := map[string]LedgerRow{selftestStem: {Status: "Partial"}}
	stems := map[string]bool{selftestStem: true}

	errs := checkUnprovenSupport(nil, rows, stems, map[string]Disposition{},
		map[string]Extraction{}, map[string]string{})
	if len(errs) != 1 {
		t.Fatalf("an unproven support claim answered %d violation(s), want 1: %v", len(errs), errs)
	}
	offered := 0
	for _, kind := range enrolmentKindNames() {
		if kind == enrolmentEnrolled {
			continue
		}
		cell := "`| Enrolment | " + kind + " |`"
		if !strings.Contains(errs[0], cell) {
			continue
		}
		offered++
		if !supportClaimExcused(kind) {
			t.Errorf("the remedy offers %s, which does not excuse a support claim:\n%s", cell, errs[0])
		}
	}
	if offered == 0 {
		t.Errorf("the remedy names no disposition at all, so the assertion above proves nothing:\n%s", errs[0])
	}
}

// VALIDATES: a `source-restricted` reason must name what stops the text being copied, and
// must not judge what Ze owes.
// PREVENTS: an unfalsifiable escape. This kind excuses a public support promise, so its
// reason carries the weight a non-normative reason carries and is held to the same
// discipline: a reviewer can check "published by ISO" and cannot check "we do not need it".
func TestSourceRestrictedReasonMustNameWhatStopsTheCopy(t *testing.T) {
	for _, one := range []struct {
		name   string
		reason string
		want   string
	}{
		{"names the publisher", "Published by ISO/IEC and not freely redistributable.", ""},
		{"names the license", "The copyright holder forbids redistribution.", ""},
		{"cites nothing", "We never got round to it.", "names nothing a reviewer can check"},
		{"judges what ze owes", "Ze does not need this standard.", "judges what ZE owes"},
	} {
		t.Run(one.name, func(t *testing.T) {
			errs := checkSummaryDisposition("", map[string]Meta{"iso-iec-10589": {
				Enrolment: dispositionSourceRestricted, EnrolmentReason: one.reason,
			}}, gatedFixture())
			if one.want == "" {
				if len(errs) != 0 {
					t.Fatalf("a checkable reason was refused: %v", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("answered %d violation(s), want 1: %v", len(errs), errs)
			}
			if !strings.Contains(errs[0], one.want) {
				t.Errorf("the refusal does not say %q:\n%s", one.want, errs[0])
			}
		})
	}
}

// preMigrationSummary is the fixture summary in the shape it had before
// enrolment moved into the Meta table: a checklist and no `## Meta` at all.
const preMigrationSummary = "# RFC 9999\n\n## Compliance Checklist\n\n" +
	"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n"

// postMigrationSummary is the same document after the move, declaring its own
// enrolment and its own public row.
const postMigrationSummary = "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
	"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n"

// VALIDATES: AC-6 and risk R-1 -- the four gated ratchets keep running across
// the commit that moves enrolment into the summaries, so a coverage regression
// carried by that commit is still reported.
// PREVENTS: the hazard that made this the dangerous half of the change. `check`
// runs checkRetiredRequirements, checkLevelRatchet, checkCoverageRatchet and
// checkEvidenceRatchet only where the current enrolled set INTERSECTS the
// baseline one. A baseline read from HEAD's summaries is EMPTY at the migration
// commit, because HEAD's summaries predate the Meta field, so all four would
// stop running over exactly the change they exist to judge -- and a run that
// judges nothing looks identical to a run that found nothing.
func TestRatchetsFireWhenEnrolmentMovesToMeta(t *testing.T) {
	const carrier = "test/plugin/widget.ci"
	both := "# RFC requirement: " + selftestRIDSend + " positive\n" +
		"# RFC requirement: " + selftestRIDSend + " negative\n"

	// HEAD carries the FILE shape: a summary with no Meta table, and the
	// enrolment in rfc/enrolled.txt beside it. Written by hand rather than
	// through checkFixtureTree, because that helper regenerates the ledger
	// pages and a summary with no Meta table does not parse.
	root := t.TempDir()
	writeFixtureFiles(t, root, map[string]string{
		selftestWorkflowRel:    selftestWorkflow,
		selftestSummaryRel:     preMigrationSummary,
		"rfc/enrolled.txt":     "# the retired shape\nrfc9999\tthe fixture RFC\n",
		"rfc/full/rfc9999.txt": "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt": "start 2026-07-29\nrate 0\n",
		"feature-gates.txt":    "ze_widget  internal/widget\n",
		carrier:                both,
	})
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "before the migration")

	// The baseline reads that shape rather than answering empty, which is the
	// one fact the ratchets stand on here.
	baseline, known := baselineMetas(root)
	if !known || !baseline[selftestStem].Enrolled() {
		t.Fatalf("the baseline read %d stem(s) from HEAD's retired ledger, known=%v",
			len(baseline), known)
	}

	// The working tree carries the META shape, and drops the negative tag with
	// it. That is the regression the ratchet exists to catch.
	writeFixtureFiles(t, root, map[string]string{
		selftestSummaryRel: postMigrationSummary,
		carrier:            "# RFC requirement: " + selftestRIDSend + " positive\n",
	})
	if _, err := IndexUpdate(root); err != nil {
		t.Fatalf("regenerate the fixture ledger pages: %v", err)
	}

	report, code := Check(root)
	if code == 0 {
		t.Fatalf("the migration commit hid a coverage regression:\n%s", report.Text())
	}
	var found string
	for _, violation := range report.Violations {
		if strings.Contains(violation, selftestRIDSend) && strings.Contains(violation, PolarityNegative) {
			found = violation
		}
	}
	if found == "" {
		t.Fatalf("no violation names %s losing its %s polarity:\n%s",
			selftestRIDSend, PolarityNegative, report.Text())
	}
}

// VALIDATES: AC-10 -- every refusal that states a property of ONE document
// still fires after the move, on a fixture of its own.
// PREVENTS: a migration that retires more than it meant to. Five refusals died
// with the second copy they compared, and it would be easy to lose a sixth that
// was never about agreement at all. Each case here is a property of a single
// summary, so none of them could have been retired by having one copy.
func TestSurvivingDispositionRefusalsStillFire(t *testing.T) {
	gap := Requirement{
		RFC: selftestStem, RID: selftestRIDSend, Level: levelMust, Text: "MUST send",
		Section: "2", Annotation: &Annotation{Kind: AnnotationGap, Reason: "not implemented"},
	}

	for _, one := range []struct {
		name string
		errs []string
		want string
	}{
		{
			"a non-normative reason judging what ze owes",
			checkSummaryDisposition("", map[string]Meta{selftestStem: {
				Enrolment: dispositionNonNormative, EnrolmentReason: "ze does not implement it",
			}}, gatedFixture()),
			"judges what ZE owes",
		},
		{
			"a non-normative reason citing nothing about the document",
			checkSummaryDisposition("", map[string]Meta{selftestStem: {
				Enrolment: dispositionNonNormative, EnrolmentReason: "nobody got to it",
			}}, gatedFixture()),
			"cites nothing about the DOCUMENT",
		},
		{
			"a source-restricted reason naming no publisher",
			checkSummaryDisposition("", map[string]Meta{selftestStem: {
				Enrolment: dispositionSourceRestricted, EnrolmentReason: "we never fetched it",
			}}, gatedFixture()),
			"names nothing a reviewer can check",
		},
		{
			"an out-of-scope summary claiming public support",
			checkSummaryDisposition("", map[string]Meta{selftestStem: {
				Enrolment: dispositionOutOfScope, EnrolmentReason: "declined 2026-09-01",
				Support: "bgp-base", Status: "Supported",
			}}, gatedFixture()),
			"cannot be advertised as supported",
		},
		{
			"a gap the public row does not disclose",
			checkStatusAgreement([]Requirement{gap},
				map[string]LedgerRow{selftestStem: {Status: "Supported", Coverage: "complete"}},
				map[string]bool{selftestStem: true}),
			"cannot be advertised as clean support",
		},
		{
			"a gap on a summary that declares no public row",
			checkStatusAgreement([]Requirement{gap}, map[string]LedgerRow{},
				map[string]bool{selftestStem: true}),
			"the public ledger must disclose it",
		},
		{
			"a support claim over a summary declaring no MUST",
			checkUnprovenSupport(nil, map[string]LedgerRow{selftestStem: {Status: "Partial"}},
				map[string]bool{selftestStem: true}, map[string]Disposition{},
				map[string]Extraction{}, map[string]string{}),
			"declares no MUST-level requirement",
		},
		{
			"a supported row no extraction sign-off bounds",
			checkSupportedSignoff(map[string]LedgerRow{selftestStem: {Status: "Supported"}},
				map[string]Extraction{}),
			"is not a valid extraction sign-off",
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			if len(one.errs) != 1 {
				t.Fatalf("answered %d violation(s), want 1: %v", len(one.errs), one.errs)
			}
			if !strings.Contains(one.errs[0], one.want) {
				t.Errorf("the refusal does not say %q:\n%s", one.want, one.errs[0])
			}
		})
	}
}

// VALIDATES: AC-10 at the three counts that behave differently -- zero gaps, one
// gap, and many.
// PREVENTS: an off-by-one nobody would see at a single count. The spelled reader
// maps a word to a number, so "one" and "twenty-one" take different branches of
// spelledNumbers, and a row with no spelled number at all must leave the check
// rather than be read as zero.
func TestGapCountAgreementAcrossZeroOneAndMany(t *testing.T) {
	gaps := func(count int) []Requirement {
		var out []Requirement
		for range count {
			out = append(out, Requirement{
				RFC: selftestStem, RID: selftestRIDSend, Level: levelMust,
				Annotation: &Annotation{Kind: AnnotationGap, Reason: "not implemented"},
			})
		}
		return out
	}

	for _, one := range []struct {
		name      string
		remaining string
		gaps      int
		refused   bool
	}{
		{"no spelled number leaves the check", "No tracked gap in current source anchors.", 3, false},
		{"zero gaps and no claim", "No tracked gap in current source anchors.", 0, false},
		{"one gap, agreeing", "One MUST-level gap remains.", 1, false},
		{"one gap, disagreeing", "One MUST-level gap remains.", 0, true},
		{"many gaps, agreeing", "Twenty-one MUSTs remain.", 21, false},
		{"many gaps, disagreeing by one", "Twenty-one MUSTs remain.", 20, true},
		{"a digit count is outside the check", "21 MUSTs remain.", 3, false},
	} {
		t.Run(one.name, func(t *testing.T) {
			errs := checkGapCountAgreement(gaps(one.gaps),
				map[string]LedgerRow{selftestStem: {Remaining: one.remaining}})
			if one.refused && len(errs) != 1 {
				t.Fatalf("answered %d violation(s), want 1: %v", len(errs), errs)
			}
			if !one.refused && len(errs) != 0 {
				t.Fatalf("answered %d violation(s), want none: %v", len(errs), errs)
			}
		})
	}
}

// gatedFixture answers one MUST-level requirement for the fixture stem, so a
// disposition test that is not ABOUT the empty-checklist refusal does not trip
// it. checkOutOfScope reds an out-of-scope summary declaring nothing.
func gatedFixture() []Requirement {
	return []Requirement{{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust}}
}

// VALIDATES: the guard an independent review found missing on 2026-09-01 -- a
// public row that DISAPPEARS while its RFC stays enrolled is refused, and so is
// a newly enrolled RFC that arrives with none.
// PREVENTS: the cheapest way to silence a ledger check. A row is a summary's
// `Support` declaration now, so deleting one is a single cell's edit; the stem
// then leaves rowsFrom and every check reading the public page stops seeing it,
// including the unsigned-support-claim refusal an author under pressure would
// most want gone. checkRetiredRequirements does NOT cover it: that ratchet
// compares requirement ids and never reads a Support cell, which the commit
// message wrongly claimed it did.
func TestAPublicRowCannotBeDeletedWhileItsRFCStaysEnrolled(t *testing.T) {
	rowed := Meta{Enrolment: enrolmentEnrolled, EnrolmentReason: "gated",
		Support: "bgp-base", Rank: 10, Status: "Partial"}
	unrowed := Meta{Enrolment: enrolmentEnrolled, EnrolmentReason: "gated"}

	deleted := checkPublicRowMonotonic(
		map[string]Meta{selftestStem: unrowed}, map[string]Meta{selftestStem: rowed},
		true, map[string]bool{})
	if len(deleted) != 1 {
		t.Fatalf("deleting a row answered %d violation(s), want 1: %v", len(deleted), deleted)
	}
	if !strings.Contains(deleted[0], "rendered a row") {
		t.Errorf("the refusal does not name what was lost:\n%s", deleted[0])
	}

	// The same edit on a stem that never had a row is the grandfathered state,
	// and stays silent: 32 enrolled RFCs have no public row by decision.
	grandfathered := checkPublicRowMonotonic(
		map[string]Meta{selftestStem: unrowed}, map[string]Meta{selftestStem: unrowed},
		true, map[string]bool{})
	if len(grandfathered) != 0 {
		t.Errorf("an RFC that never had a row was billed: %v", grandfathered)
	}

	// A NEW enrolment owes one.
	arriving := checkPublicRowMonotonic(
		map[string]Meta{selftestStem: unrowed}, map[string]Meta{}, true,
		map[string]bool{selftestStem: true})
	if len(arriving) != 1 || !strings.Contains(arriving[0], "newly enrolled") {
		t.Fatalf("a new enrolment with no row answered %d violation(s): %v",
			len(arriving), arriving)
	}

	// Where git cannot answer, the HEAD comparison judges nothing rather than
	// judging everything, exactly as the other ratchets do.
	unknown := checkPublicRowMonotonic(
		map[string]Meta{selftestStem: unrowed}, map[string]Meta{selftestStem: rowed},
		false, map[string]bool{})
	if len(unknown) != 0 {
		t.Errorf("an unreadable baseline accused somebody: %v", unknown)
	}
}

// VALIDATES: the two premises `out-of-scope` rests on are checked, in the
// FAILING direction.
// PREVENTS: an escape that costs nothing to write. checkNewSummaries asks an
// out-of-scope summary for no enrolment, which is only right where the
// obligations are written down, so a summary declaring none under it records
// nothing and bills nobody. The date is the other half: a scope decision nobody
// can age reads forever as one somebody still stands behind.
func TestOutOfScopeMustCarryItsExtractionAndItsDate(t *testing.T) {
	for _, one := range []struct {
		name   string
		reason string
		gated  int
		want   string
	}{
		{"dated, extracted", "declined by the owner 2026-09-01", 1, ""},
		{"no date", "declined by the owner", 1, "carrying no date"},
		{"no extraction", "declined by the owner 2026-09-01", 0, "NO MUST-level requirement"},
		{"neither", "declined by the owner", 0, "NO MUST-level requirement"},
	} {
		t.Run(one.name, func(t *testing.T) {
			errs := checkOutOfScope("rfc/short/rfc9999.md: ", Meta{
				Enrolment: dispositionOutOfScope, EnrolmentReason: one.reason,
			}, one.gated)
			if one.want == "" {
				if len(errs) != 0 {
					t.Fatalf("a complete declaration was refused: %v", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("%s was accepted", one.name)
			}
			if !strings.Contains(strings.Join(errs, "\n"), one.want) {
				t.Errorf("the refusal does not say %q:\n%v", one.want, errs)
			}
		})
	}
}

// VALIDATES: the row guard is WIRED -- it fires through Check over a real
// checkout, not only when a test calls it directly.
// PREVENTS: the shape an independent review found on 2026-09-02. The guard
// itself had four unit cases and one production call site, and deleting those
// two lines in `check` left the whole suite green. `ai/rules/evidence.md` asks
// for a guard to be driven from its entry point precisely because a guard
// nothing reaches is indistinguishable from one that never fires.
//
// The fixture is deliberately NOT enrolled. checkSupportedSignoff bills any row
// whose Status promises conformance, enrolled or not, so the population this
// guard has to cover is the ROW population -- which is the half the first fix
// got wrong, and rfc/short/rfc9384.md is the live instance of it.
func TestCheckReportsAPublicRowDeletedThroughItsEntryPoint(t *testing.T) {
	const stem = "rfc9999"
	declared := "# RFC 9999\n\n## Meta\n\n| Field | Value |\n|-------|-------|\n" +
		"| Title | Widgets |\n| Enrolment | non-normative |\n" +
		"| Enrolment reason | Informational, and it invokes no RFC 2119 key-words machinery |\n" +
		"| Support | bgp-base 10 |\n| Support area | Widgets |\n" +
		"| Support status | Partial |\n| Support coverage | unit tests |\n" +
		"| Support remaining | Zero MUST gaps. |\n\n## Compliance Checklist\n\n" +
		"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n"
	retired := strings.Replace(declared, "| Support | bgp-base 10 |", "| Support | - |", 1)
	for _, cell := range []string{"| Support area | Widgets |\n",
		"| Support status | Partial |\n", "| Support coverage | unit tests |\n",
		"| Support remaining | Zero MUST gaps. |\n"} {
		retired = strings.Replace(retired, cell, "", 1)
	}

	root := commitFixtureTree(t,
		map[string]string{selftestSummaryRel: declared},
		map[string]string{selftestSummaryRel: retired})

	report, code := Check(root)
	if code == 0 {
		t.Fatalf("a deleted public row was not reported:\n%s", report.Text())
	}
	var found string
	for _, violation := range report.Violations {
		if strings.Contains(violation, "rendered a row") {
			found = violation
		}
	}
	if found == "" {
		t.Fatalf("no violation names the retired row:\n%s", report.Text())
	}
	if !strings.Contains(found, stem) {
		t.Errorf("the violation does not name %s:\n%s", stem, found)
	}
}

// VALIDATES: `source-restricted` cannot be written over a source text that IS in
// the tree, exercised against a real file rather than an empty tree path.
// PREVENTS: a refusal that has never executed. The first test of this guard
// passed an empty tree, so `treePath("", ...)` resolved beside the package, the
// file never existed, and the branch was never entered -- and the corpus reaches
// it in zero of 198 summaries, so nothing else did either.
func TestSourceRestrictedIsRefusedWhenTheTextIsPresent(t *testing.T) {
	root := t.TempDir()
	writeFixtureFiles(t, root, map[string]string{
		"rfc/full/rfc9999.txt": "A speaker MUST send the widget.\n",
	})
	meta := Meta{Enrolment: dispositionSourceRestricted,
		EnrolmentReason: "Published by ISO/IEC and not freely redistributable."}

	present := checkSummaryDisposition(root, map[string]Meta{"rfc9999": meta}, gatedFixture())
	if len(present) != 1 {
		t.Fatalf("a source-restricted claim over a text in the tree answered %d violation(s): %v",
			len(present), present)
	}
	if !strings.Contains(present[0], "that text IS in this repository") {
		t.Errorf("the refusal does not name the contradiction:\n%s", present[0])
	}

	// The same declaration over a stem whose text is genuinely absent is the
	// state the disposition exists for, and stays silent.
	absent := checkSummaryDisposition(root, map[string]Meta{"iso-iec-10589": meta}, gatedFixture())
	if len(absent) != 0 {
		t.Errorf("a standard whose text is absent was refused: %v", absent)
	}
}
