// Design: docs/contributing/rfc-conformance-gates.md -- the intent marker
// Related: intent.go -- parseIntent, intentOnGatedRow, evaluateIntent, the code this stage proves
// Related: selftest.go -- the stage list this stage joins
//
// selftest_intent.go proves the {intent} marker through the production parser
// and the stale-marker check, as summary/updated-by proves {updated-by}.
package rfc

import (
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// selftestIntentRows are two optional fixture rows, one declared not ever with
// the RFC sentence it cites and one not yet, and a MUST row carrying a marker
// the parser refuses.
const (
	selftestIntentNotEverRow = "- [ ] [RFC9999-2-5] [MAY] A speaker MAY log every widget it discards (§2) " +
		`{intent: not-ever; "A speaker MAY log every widget it discards."; Ze keeps no discard log}` + "\n"
	selftestIntentNotYetRow = "- [ ] [RFC9999-2-6] [SHOULD] A speaker SHOULD count every widget (§2) " +
		"{not-applicable: the fixture's own reason} {intent: not-yet}\n"
	selftestIntentGatedRow = "- [ ] [RFC9999-2-7] [MUST] A speaker MUST send the widget (§2) {intent: not-yet}\n"
)

// runIntentSelftest parses the marker, refuses it on a gated row, and refuses
// it as stale on a row Ze does not owe and on a proven row.
func runIntentSelftest() ([]leroot.SelftestResult, error) {
	rows, err := ParseSummaryText(selftestIntentNotEverRow+selftestIntentNotYetRow, selftestStem, selftestSummaryRel)
	if err != nil {
		return nil, err
	}
	parsed := len(rows) == 2
	if parsed {
		parsed = rows[0].Intent != nil && rows[0].Intent.State == IntentNotEver && rows[0].Intent.Reason != ""
		parsed = parsed && rows[1].Intent != nil && rows[1].Intent.State == IntentNotYet
	}
	_, gatedErr := ParseSummaryText(selftestIntentGatedRow, selftestStem, selftestSummaryRel)
	stale := false
	if parsed {
		findings := evaluateIntent(rows, map[string]bool{rows[0].RID: true})
		stale = len(findings) == 2 && findings[0].RID == rows[0].RID && findings[1].RID == rows[1].RID
	}
	return []leroot.SelftestResult{
		selftestResult("intent/marker", parsed,
			"the not-ever or not-yet marker did not parse into the row's intent"),
		selftestResult("intent/gated-refusal", gatedErr != nil,
			"an intent marker on a MUST row was accepted"),
		selftestResult("intent/stale-marker", stale,
			"an intent marker on a proven row or a not-applicable row was not refused as stale"),
	}, nil
}
