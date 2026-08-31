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
		"rfc/enrolled.txt":  selftestEnrolled,
		selftestSummaryRel: "# RFC 9999\n\n## Compliance Checklist\n\n" +
			"- [ ] [" + selftestRIDSend + "] [MUST] A speaker MUST send the widget (§2)\n",
		"rfc/full/rfc9999.txt": "A speaker MUST send the widget.\n",
		"rfc/drain-budget.txt": "start 2026-07-29\nrate 0\n",
		// The tag scanner type-checks the packages carrying tags under every feature gate,
		// so the manifest must name at least one. This fixture's one gated package holds
		// nothing.
		"feature-gates.txt": "ze_widget  internal/widget\n",
		"docs/features/rfc-status.md": "| RFC | Feature | Status | Supported | Gaps |\n" +
			"|-----|---------|--------|-----------|------|\n" +
			"| RFC 9999 | Widgets | Partial | The widget sender. | Zero MUST gaps. |\n",
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
		map[string]bool{selftestStem: true},
		map[string]Extraction{},
	)

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

// VALIDATES: a row that promises support for an RFC with NO rfc/short/ summary at all is
// refused, and the message names the missing summary.
// PREVENTS: the hole checkUnprovenSupport discloses in its own error text -- it iterates
// the summary stems, so "Rows naming an RFC with no summary at all are outside this
// check". Ten such rows sit on the public page today, each one a support promise no check
// in this package can see.
func TestCheckSupportedSignoffRefusesSupportedRowWithNoSummary(t *testing.T) {
	errs := checkSupportedSignoff(
		map[string]LedgerRow{selftestStem: {Status: "Supported on Linux"}},
		map[string]bool{},
		map[string]Extraction{},
	)

	if len(errs) != 1 {
		t.Fatalf("a Supported row with no summary answered %d violation(s): %v", len(errs), errs)
	}
	for _, want := range []string{selftestStem, "'Supported on Linux'", "rfc/short/rfc9999.md"} {
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
		map[string]bool{"rfc1000": true, "rfc1001": true, "rfc1002": true},
		map[string]Extraction{
			"rfc1000": {Stem: "rfc1000", Register: registerRFC2119},
			"rfc1001": {Stem: "rfc1001", Register: registerProse},
			"rfc1002": {Stem: "rfc1002", Register: registerRFC2119},
		},
	)

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

	errs := checkSupportedSignoff(rows, map[string]bool{}, map[string]Extraction{})

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
		map[string]bool{selftestStem: true},
		signed,
	)

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
		map[string]bool{selftestStem: true},
		signed,
	)

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
		map[string]bool{selftestStem: true},
		signed,
	)

	if len(errs) != 1 {
		t.Fatalf("a skeleton artifact answered %d violation(s): %v", len(errs), errs)
	}
	for _, want := range []string{selftestStem, "rfc/extraction/rfc9999.json"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("the violation does not name %q:\n%s", want, errs[0])
		}
	}
}

// VALIDATES: assumption A-1 of plan/spec-rfcgate-6-supported-extraction-signoff.md, at
// BOTH denominators the spec uses, re-derived from the page rather than retyped: the
// support-claiming STEM set parseStatusLedger answers, and the support-claiming ROW count
// of the eight RFC tables.
// PREVENTS: a scope carried forward from a table in a spec. The two denominators differ,
// and every earlier count of this page reported one of them without saying which.
//
// They differ for one reason now, and the numbers below are 52 = 48 + 4: the ninth table
// (drafts and non-RFC standards) is outside the spec's scope cut, and it carries four more
// rows that promise support. That term is DERIVED from the page at the end of this test
// rather than retyped, so the bridge between the two denominators is checked and not
// merely described.
//
// A second term stood here until 2026-08-31, written `- 1` for a stem the page stated
// TWICE: RFC 2759 read "Supported within PPP and IPsec EAP" in the access table and
// "Partial" in the IPsec table, and parseStatusLedger keys by stem and keeps the LAST row,
// so the promise was invisible to every ledger check that reads the map. Commit 460fdc0f8
// closed the three MUST gaps the IPsec row disclosed and merged the two rows into one
// "Supported" row, so no stem is stated twice and the term has nothing to subtract. Do not
// restore it: a duplicate that hides a promise again makes the derived bridge below fail,
// which is where a reader should meet it.
//
// A number that moves fails this test with both counts printed, which is the point: the
// scope is re-derived at the start of a phase and at closure, and a delta is recorded in
// the spec's Risks & Assumptions rather than absorbed.
func TestSupportedRowsHaveDerivableScope(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(statusRel)))
	if err != nil {
		t.Fatalf("read the public ledger: %v", err)
	}
	page := string(raw)

	ledger := parseStatusLedger(page)
	var mapped []string
	for _, stem := range sortedKeysOf(ledger) {
		if statusPromisesSupport(ledger[stem].Status) {
			mapped = append(mapped, ledger[stem].Status)
		}
	}
	exact, qualified, yes := supportClaimSplit(mapped)
	// No stem reads 'Yes' any more: that cell was RFC 1997's and the page's own
	// vocabulary paragraph never defined the word, so it now reads 'Supported'.
	// The shape is kept in the split rather than dropped, because the predicate
	// still accepts it and a future row could reintroduce it.
	if len(mapped) != 52 || exact != 41 || qualified != 11 || yes != 0 {
		t.Errorf("the ledger keys %d support-promising stem(s) (%d exact, %d scope-qualified, %d 'Yes'), want 52 (41, 11, 0)",
			len(mapped), exact, qualified, yes)
	}
	if exact+qualified+yes != len(mapped) {
		t.Errorf("the three shapes sum to %d of %d accepted status cells, so a shape the producer accepts is uncounted",
			exact+qualified+yes, len(mapped))
	}

	// The eight RFC tables, row by row. The ninth table's heading ends them, and a row
	// keyed by anything other than an RFC number is not one of theirs.
	eightTables, ninthTable, _ := strings.Cut(page, "\n## Drafts")
	var rows []string
	for line := range strings.SplitSeq(eightTables, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 3 || !statusRFCRE.MatchString(cells[0]) {
			continue
		}
		if statusPromisesSupport(cells[2]) {
			rows = append(rows, cells[2])
		}
	}
	rowExact, rowQualified, rowYes := supportClaimSplit(rows)
	if len(rows) != 48 || rowExact != 37 || rowQualified != 11 || rowYes != 0 {
		t.Errorf("the eight RFC tables carry %d support-promising row(s) (%d exact, %d scope-qualified, %d 'Yes'), want 48 (37, 11, 0)",
			len(rows), rowExact, rowQualified, rowYes)
	}

	// The ninth table, by the two key shapes parseStatusLedger accepts there: a draft stem
	// and a non-RFC stem. It holds no RFC-keyed row, so its support promises are exactly
	// the stems the eight-table count above cannot reach.
	var draftRows []string
	for line := range strings.SplitSeq(ninthTable, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 3 {
			continue
		}
		if !statusDraftRE.MatchString(cells[0]) && !statusStemRE.MatchString(cells[0]) {
			continue
		}
		if statusPromisesSupport(cells[2]) {
			draftRows = append(draftRows, cells[2])
		}
	}

	// The bridge between the two denominators, asserted rather than described. Every
	// support-promising row the page carries reaches the ledger under its own key, so the
	// stem count is the eight-table count plus the ninth table's. A stem stated twice
	// whose later row withdraws the promise drops the left side and leaves the right one
	// standing, which is the shape RFC 2759 held until 460fdc0f8 merged its two rows.
	if len(mapped) != len(rows)+len(draftRows) {
		t.Errorf("the ledger keys %d support-promising stem(s) but the page carries %d row(s) (%d in the eight RFC tables, %d in the ninth), so a promised row is hidden by a repeated key",
			len(mapped), len(rows)+len(draftRows), len(rows), len(draftRows))
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
	}
	// One proof and one escape over the one requirement this fixture declares, so
	// the two figures cannot be read off a single row.
	proof := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: selftestCIPath, Route: routeMutant, Citation: selftestCIDirective,
		Producer: selftestProducerUnit, Break: selftestBreak,
	})
	escape := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: polarityNegative,
		Unit: selftestCIPath, Route: routeNoBreak,
		Reason: escapeDeclaration, Producer: selftestTablePath,
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: selftestCIPath, Route: routeMutant, Citation: selftestCIDirective,
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: selftestCIPath, Route: routeMutant, Citation: selftestCIDirective,
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
	for _, want := range []string{selftestRIDSend, polarityPositive, selftestCIPath,
		selftestBreak, selftestProducerUnit, proofProducerChanged} {
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
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=fixture@example.invalid", "-c", "user.name=rfc-fixture",
			"commit", "-q", "-m", "fixture"},
	} {
		command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...) //nolint:gosec // this test's own throwaway fixture repository
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}

	writeFixtureFiles(t, root, working)
	if _, err := IndexUpdate(root); err != nil {
		t.Fatalf("rewrite the fixture ledger pages: %v", err)
	}
	return root
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

// VALIDATES: AC-2 -- a tagged unit present in the tree and absent at HEAD, on an enrolled
// RFC's gated requirement, reds the gate with exit 2, and the violation names the file,
// the line, the requirement, the polarity and the proof route its carrier kind admits.
// METHOD: one corpus committed, then one carrier added on top of it. The forced red is
// restored by putting the tree back to what was committed, which is the pair that says
// the ratchet followed the CHANGE rather than the corpus.
// PREVENTS: an inert ratchet. A floor that starts at zero and only forbids going below
// zero proves nothing (R-2), so the obligation is what a change adds.
func TestCheckDiscriminationRequiresProofForNewTag(t *testing.T) {
	unchanged := commitFixtureTree(t, fixtureCorpus(), nil)
	if report, code := Check(unchanged); code != 0 || len(report.Violations) != 0 {
		t.Fatalf("the unchanged corpus answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	} else if report.DiscriminationOwed != 0 {
		t.Fatalf("owed is %d on a corpus no change touched, want 0: every tag in it is its own HEAD's",
			report.DiscriminationOwed)
	}

	added := commitFixtureTree(t, fixtureCorpus(),
		map[string]string{fixtureGadgetCIPath: fixtureGadgetCI})
	report, code := Check(added)
	if code != 2 || len(report.Violations) != 1 {
		t.Fatalf("a tag new against HEAD answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	for _, want := range []string{fixtureGadgetCIPath + ":1", selftestRIDSend, polarityPositive,
		routeRevert, kindFunctional} {
		if !strings.Contains(report.Violations[0], want) {
			t.Errorf("the violation omits %q: %s", want, report.Violations[0])
		}
	}
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: fixtureGadgetCIPath, Route: routeRevert, Citation: selftestCIDirective,
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
	for _, want := range []string{fixtureGadgetCIPath, selftestRIDSend, proofUnitChanged} {
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: fixtureGadgetCIPath, Route: routeRevert, Citation: selftestCIDirective,
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: fixtureGadgetCIPath, Route: routeRevert, Citation: selftestCIDirective,
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
	for _, want := range []string{selftestRIDSend, fixtureGadgetCIPath, proofClaimChanged} {
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: fixtureGadgetCIPath, Route: routeRevert, Citation: selftestCIDirective,
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
	for _, want := range []string{selftestRIDSend, polarityPositive, fixtureGadgetCIPath} {
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: fixtureGadgetCIPath, Route: routeRevert, Citation: selftestCIDirective,
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
		report.UnscannedTags[0].Polarity != polarityPositive {
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
		RID: selftestRIDSend, Polarity: polarityPositive,
		Unit: fixtureGadgetCIPath, Route: routeRevert, Citation: selftestCIDirective,
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
	for _, want := range []string{selftestRIDSend, fixtureGadgetCIPath, proofProducerChanged} {
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
	if !strings.Contains(refused.Violations[0], proofProducerChanged) {
		t.Errorf("the violation does not name the fingerprint that moved: %s", refused.Violations[0])
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
	key := cover{rid: selftestRIDSend, polarity: polarityPositive, unit: unit}
	head := map[string]string{selftestTestPath: selftestTestSource}
	measure := func(working map[string]string) (int, int) {
		return discriminationChangedUnits(discriminationInput{
			Gated:      map[string]bool{selftestRIDSend: true},
			Covers:     map[cover][]Tag{key: {{RID: selftestRIDSend, Polarity: polarityPositive}}},
			HeadCovers: map[cover]bool{key: true},
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
