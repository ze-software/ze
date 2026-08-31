package rfc

import (
	"maps"
	"os"
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

// VALIDATES: assumption A-1 of plan/spec-rfcgate-6-supported-extraction-signoff.md, at
// BOTH denominators the spec uses, re-derived from the page rather than retyped: the
// support-claiming STEM set parseStatusLedger answers, and the support-claiming ROW count
// of the eight RFC tables.
// PREVENTS: a scope carried forward from a table in a spec. The two denominators differ,
// and every earlier count of this page reported one of them without saying which.
//
// They differ for two reasons, and the numbers below are 53 = 50 - 1 + 4:
//   - RFC 2759 is stated TWICE, "Supported within PPP and IPsec EAP" in the access table
//     and "Partial" in the IPsec table. parseStatusLedger keys by stem and the later row
//     wins, so the support promise is invisible to every ledger check that reads the map.
//   - the ninth table (drafts and non-RFC standards) is outside the spec's scope cut, and
//     it carries four more rows that promise support.
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
	if len(mapped) != 53 || exact != 42 || qualified != 11 || yes != 0 {
		t.Errorf("the ledger keys %d support-promising stem(s) (%d exact, %d scope-qualified, %d 'Yes'), want 53 (42, 11, 0)",
			len(mapped), exact, qualified, yes)
	}
	if exact+qualified+yes != len(mapped) {
		t.Errorf("the three shapes sum to %d of %d accepted status cells, so a shape the producer accepts is uncounted",
			exact+qualified+yes, len(mapped))
	}

	// The eight RFC tables, row by row. The ninth table's heading ends them, and a row
	// keyed by anything other than an RFC number is not one of theirs.
	eightTables, _, _ := strings.Cut(page, "\n## Drafts")
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
	if len(rows) != 50 || rowExact != 38 || rowQualified != 12 || rowYes != 0 {
		t.Errorf("the eight RFC tables carry %d support-promising row(s) (%d exact, %d scope-qualified, %d 'Yes'), want 50 (38, 12, 0)",
			len(rows), rowExact, rowQualified, rowYes)
	}
}
