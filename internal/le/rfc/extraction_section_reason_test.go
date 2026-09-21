// VALIDATES: a WALKED section keeps the reason its decision carried.
// PREVENTS: the prose behind an `unsourced-ids` list vanishing at apply time.
// A section that declares an obligation the extractor cannot see owes the
// sentence it was read from, and that prose has nowhere else to live: the
// requirement row states the obligation, not where the reviewer found it.
// Until 2026-09-21 the reason was kept only for a SKIPPED section, so every
// walk that recorded an unsourced id wrote its justification and lost it.

package rfc

import "testing"

func TestAWalkedSectionKeepsItsReason(t *testing.T) {
	const reason = "the obligation is stated in indicative prose with no capitalised keyword"
	derived := extractionDocumentSection{ID: "4", Sites: 2}

	for _, one := range []struct {
		name        string
		disposition string
		skipKind    string
		wantSkip    bool
	}{
		{"walked", dispositionWalked, "", false},
		{"skipped", dispositionSkipped, "references", true},
	} {
		t.Run(one.name, func(t *testing.T) {
			got := sectionFromDecision(derived, classifySectionDecision{
				ID:           "4",
				Disposition:  one.disposition,
				SkipKind:     one.skipKind,
				Reason:       reason,
				UnsourcedIDs: []string{"RFC9999-4-1"},
			})
			if got.Reason != reason {
				t.Errorf("a %s section answered reason %q, want it carried: the sentence an "+
					"unsourced id was read from has nowhere else to live", one.name, got.Reason)
			}
			if (got.SkipKind != "") != one.wantSkip {
				t.Errorf("a %s section answered skip kind %q; only a skipped section owes one",
					one.name, got.SkipKind)
			}
			if len(got.UnsourcedIDs) != 1 {
				t.Errorf("the unsourced ids did not survive: %v", got.UnsourcedIDs)
			}
		})
	}
}
