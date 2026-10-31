package site

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// rfcOutOfScopeFacts answers the at-a-glance out-of-scope row and the Out of
// scope card of one entry, as the page renders them.
func rfcOutOfScopeFacts(t *testing.T, entry *rfcLedgerStem) (glance, card string) {
	t.Helper()
	glance, card = "<no glance row>", "<no card>"
	for _, fact := range rfcGlanceFacts(entry) {
		if fact[0] == "Not applicable, so out of scope" {
			glance = rfcPlain(fact[1])
		}
	}
	for _, each := range rfcDetailCards(entry) {
		if each.Label == "Out of scope" {
			card = each.Value
		}
	}
	return glance, card
}

// VALIDATES: a `{not-applicable}` or `{feature-declined}` row whose evidence
// fails is never called out of scope on its RFC's page, and one whose evidence
// passes is.
//
// The figures leave such a row out of the rows Ze owes only once its evidence
// passes (figureEvidence.excluded), and count a bare one as a Ze gap. The page
// read the annotation counter (rfcLedgerCoverage.NotApplicable) instead, so a
// bare row was published as out of scope beside bars counting it as a gap.
// Method: one entry annotating two rows not applicable and one feature
// declined, of which the figures judged one not-applicable row excluded; the
// glance row and the card must name only that row, then none when the figures
// judged none, and say "not judged" when the figures answered nothing.
func TestABareNotApplicableRowIsNotCalledOutOfScope(t *testing.T) {
	entry := disclosureLedger().Stems[0]
	entry.Coverage.NotApplicable, entry.Coverage.FeatureDeclined = 2, 1
	entry.Figures.Mandatory.Excluded = map[string]int{rfc.AnnotationNotApplicable: 1}

	glance, card := rfcOutOfScopeFacts(t, &entry)
	if glance != "1" {
		t.Errorf("glance row = %q, want 1: two rows are annotated and only one passed its evidence", glance)
	}
	if card != "1" {
		t.Errorf("Out of scope card = %q, want 1: the bare not-applicable and feature-declined rows are Ze gaps", card)
	}

	entry.Figures.Mandatory.Excluded = map[string]int{rfc.AnnotationFeatureDeclined: 1}
	glance, card = rfcOutOfScopeFacts(t, &entry)
	if glance != "0" {
		t.Errorf("glance row = %q, want 0: no not-applicable row passed its evidence", glance)
	}
	if card != "1" {
		t.Errorf("Out of scope card = %q, want 1: the evidenced feature-declined row", card)
	}

	entry.Figures.Mandatory.Excluded = nil
	glance, card = rfcOutOfScopeFacts(t, &entry)
	if glance != "0" || card != "0" {
		t.Errorf("glance row %q and card %q, want 0 and 0: every annotated row is bare", glance, card)
	}

	entry.Figures = nil
	glance, card = rfcOutOfScopeFacts(t, &entry)
	if glance != rfcOutOfScopeUnjudged || card != rfcOutOfScopeUnjudged {
		t.Errorf("glance row %q and card %q, want %q: a zero would read as nothing out of scope",
			glance, card, rfcOutOfScopeUnjudged)
	}
}

// VALIDATES: the machine-readable ledger never publishes the annotation count
// under a key that says out of scope.
//
// rfcLedgerCoverage.NotApplicable counts annotations, bare ones included, and
// it was published as `out-of-scope`. Method: marshal a coverage holding two
// annotated rows and read the keys.
func TestTheLedgerNamesTheAnnotationCountForWhatItCounts(t *testing.T) {
	data, err := json.Marshal(rfcLedgerCoverage{NotApplicable: 2})
	if err != nil {
		t.Fatal(err)
	}
	published := string(data)
	if strings.Contains(published, `"out-of-scope":`) {
		t.Errorf("the ledger publishes the annotation count as out of scope: %s", published)
	}
	if !strings.Contains(published, `"annotated-never-bound":2`) {
		t.Errorf("the ledger does not publish the annotation count under its own key: %s", published)
	}
}

// VALIDATES: the notes beside the counts say that an annotation alone leaves
// nothing out of what Ze owes.
//
// The Not applicable and Optional feature declined bucket cards still count
// annotations, so their notes, and the Out of scope card's, must name the
// evidence condition. Method: read each note and refuse one that claims
// nothing is owed without it.
func TestTheOutOfScopeNotesNameTheEvidenceCondition(t *testing.T) {
	notes := map[string]string{"Out of scope card": rfcScopeCardNote}
	for _, group := range rfcStanding {
		if group.Label == rfcNotApplyLabel || group.Label == rfcFeatureDeclinedLabel {
			notes[group.Label] = group.Meaning
		}
	}
	if len(notes) != 3 {
		t.Fatalf("found %d notes, want the card and two bucket cards: %v", len(notes), notes)
	}
	for label, note := range notes {
		if !strings.Contains(note, "evidence") || !strings.Contains(note, "Ze gap") {
			t.Errorf("the %s note does not say a row whose evidence fails is a Ze gap: %q", label, note)
		}
	}
}
