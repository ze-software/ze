// VALIDATES: a level correction is read from rfc/corrections/<stem>.md, and a
// paragraph left in the summary no longer authorizes anything.
// PREVENTS: the move silently keeping both homes. A reader that still accepted
// the summary would let the narrative drift back in one paragraph at a time,
// and a reader that accepted neither would fire checkLevelRatchet on every
// correction recorded tonight.

package rfc

import (
	"os"
	"path/filepath"
	"testing"
)

// correctionParagraph is the shape checkLevelRatchet demands: the opening
// words, the id in backticks, and at least 24 characters of the RFC quoted.
const correctionParagraph = "Correction 2026-09-21: `RFC9999-2-1` was published [MUST]. " +
	"RFC 9999 states it once, at SHOULD: \"A speaker SHOULD send the widget when it can\".\n"

func TestACorrectionIsReadFromTheRecordAndNotFromTheSummary(t *testing.T) {
	for _, one := range []struct {
		name  string
		rel   string
		found int
	}{
		{"record", filepath.Join(correctionRel, "rfc9999.md"), 1},
		{"summary", filepath.Join(summaryRel, "rfc9999.md"), 0},
	} {
		t.Run(one.name, func(t *testing.T) {
			tree := t.TempDir()
			path := filepath.Join(tree, one.rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatalf("the fixture directory: %v", err)
			}
			if err := os.WriteFile(path, []byte(correctionParagraph), 0o600); err != nil {
				t.Fatalf("the fixture record: %v", err)
			}
			got := loadCorrections(tree, "rfc9999")
			if len(got) != one.found {
				t.Fatalf("a correction in %s read %d record(s), want %d: the reader takes "+
					"rfc/corrections/ and nothing else", one.rel, len(got), one.found)
			}
			if one.found == 0 {
				return
			}
			if len(got[0].RIDs) != 1 || got[0].RIDs[0] != "RFC9999-2-1" {
				t.Errorf("the record names %v, want the one id it carries", got[0].RIDs)
			}
			if got[0].Date != "2026-09-21" {
				t.Errorf("the record is dated %q", got[0].Date)
			}
		})
	}
}

// VALIDATES: an RFC with no correction record is not an error.
// PREVENTS: a reader that treated the ordinary case as a failure. Almost no
// summary has ever had a level corrected, so an absent file is the norm, and
// the ratchet's own refusal is what reports a correction that is owed.
func TestAnAbsentCorrectionRecordIsNotAnError(t *testing.T) {
	if got := loadCorrections(t.TempDir(), "rfc9999"); got != nil {
		t.Fatalf("an absent record answered %v, want nothing", got)
	}
}
