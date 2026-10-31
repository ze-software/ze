// Design: docs/contributing/rfc-conformance-gates.md -- the intent marker
// Related: intent.go -- parseIntent, intentOnGatedRow, evaluateIntent, the code this stage proves
// Related: selftest.go -- the stage list this stage joins
//
// selftest_intent.go proves the {intent} marker through the production parser
// and the stale-marker check, as summary/updated-by proves {updated-by}.
package rfc

import (
	"os"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// selftestIntentRows are three optional fixture rows and a MUST row carrying a
// marker the parser refuses. The first is declared not ever with the RFC
// sentence it cites, and the proof set names it. The second is
// {not-applicable} with the evidence its kind owes: a whole sentence of its
// section, binding a role the fixture's Meta does not give Ze. The third is
// {not-applicable} with a bare reason, which the evidence judge refuses, so
// the row stays owed and its marker stands.
const (
	selftestIntentNotEverRow = "- [ ] [RFC9999-2-5] [MAY] A speaker MAY log every widget it discards (§2) " +
		`{intent: not-ever; "A speaker MAY log every widget it discards."; Ze keeps no discard log}` + "\n"
	selftestIntentRelaySentence = "A relay SHOULD count every widget."
	selftestIntentEvidencedRow  = "- [ ] [RFC9999-2-6] [SHOULD] " + selftestIntentRelaySentence + " (§2) " +
		`{not-applicable: "` + selftestIntentRelaySentence + `"; binds a relay; Ze relays no widget} {intent: not-yet}` + "\n"
	selftestIntentBareRow = "- [ ] [RFC9999-2-8] [SHOULD] A speaker SHOULD count every widget (§2) " +
		"{not-applicable: the fixture's own reason} {intent: not-yet}\n"
	selftestIntentGatedRow = "- [ ] [RFC9999-2-7] [MUST] A speaker MUST send the widget (§2) {intent: not-yet}\n"
)

// selftestIntentSource is the fixture RFC with the relay's sentence in
// section 2, so the evidenced row's quote is a whole sentence there.
const selftestIntentSource = selftestRFCSource + "\n    " + selftestIntentRelaySentence + "\n"

// runIntentSelftest parses the marker, refuses it on a gated row, refuses it
// as stale on a proven row and on a row whose not-owed evidence passes the
// production judge, and accepts it on a row whose evidence fails.
func runIntentSelftest() ([]leroot.SelftestResult, error) {
	summary := "# RFC 9999\n\n" + selftestMeta + "\n## Compliance Checklist\n\n" +
		selftestIntentNotEverRow + selftestIntentEvidencedRow + selftestIntentBareRow
	rows, err := ParseSummaryText(summary, selftestStem, selftestSummaryRel)
	if err != nil {
		return nil, err
	}
	parsed := len(rows) == 3
	if parsed {
		parsed = rows[0].Intent != nil && rows[0].Intent.State == IntentNotEver && rows[0].Intent.Reason != ""
		parsed = parsed && rows[1].Intent != nil && rows[1].Intent.State == IntentNotYet
		parsed = parsed && rows[2].Intent != nil && rows[2].Intent.State == IntentNotYet
	}
	_, gatedErr := ParseSummaryText(selftestIntentGatedRow, selftestStem, selftestSummaryRel)
	root, err := newSelftestTree("rfc-selftest-intent-", map[string]string{selftestSourceRel: selftestIntentSource})
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root) //nolint:errcheck // temporary fixture checkout
	stale, owed := false, false
	if parsed {
		findings := evaluateIntent(rows, map[string]bool{rows[0].RID: true}, newEvidenceReader(root))
		stale = len(findings) == 2 && findings[0].RID == rows[0].RID && findings[1].RID == rows[1].RID
		owed = true
		for _, finding := range findings {
			if finding.RID == rows[2].RID {
				owed = false
			}
		}
	}
	return []leroot.SelftestResult{
		selftestResult("intent/marker", parsed,
			"the not-ever or not-yet marker did not parse into the row's intent"),
		selftestResult("intent/gated-refusal", gatedErr != nil,
			"an intent marker on a MUST row was accepted"),
		selftestResult("intent/stale-marker", stale,
			"an intent marker on a proven row or an evidenced not-applicable row was not refused as stale"),
		selftestResult("intent/owed-marker", owed,
			"an intent marker on a not-applicable row whose evidence fails was refused, though the figures keep that row owed"),
	}, nil
}
