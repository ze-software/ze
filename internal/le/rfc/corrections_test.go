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

// retirementParagraph is the shape checkRetiredRequirements accepts: the
// dated opener, the id in backticks, and the sections read, as § references.
const retirementParagraph = "Retired 2026-09-26: `RFC9999-2-3` states no obligation RFC 9999 carries.\n" +
	"Read §2 and §3, then the whole text: no sentence says a speaker counts widgets.\n" +
	"It carried no tag.\n"

// VALIDATES: D-2 -- a retirement paragraph retires the id it names, and a
// malformed one retires nothing.
// PREVENTS: the retirement route turning into a silent delete. A paragraph
// with no date, no backticked id or no section reference records no search,
// and a paragraph that names a neighbor says nothing about this row. A
// paragraph retires the first id it names only: the ids after it are where
// its tags moved, and reading them as retired would free a live row's id.
func TestARetirementParagraphRetiresOnlyWhatItNames(t *testing.T) {
	for _, one := range []struct {
		name    string
		text    string
		retires bool
	}{
		{"well formed", retirementParagraph, true},
		{"quoted, after a blank line", "\n> Retired 2026-09-26: `RFC9999-2-3` read §2.1.\n", true},
		{"no date", "Retired: `RFC9999-2-3` read §2 and §3.\n", false},
		{"malformed date", "Retired 26-09-2026: `RFC9999-2-3` read §2 and §3.\n", false},
		{"no backticked id", "Retired 2026-09-26: RFC9999-2-3 read §2 and §3.\n", false},
		{"no section reference", "Retired 2026-09-26: `RFC9999-2-3` read the whole text.\n", false},
		{"a neighbor", "Retired 2026-09-26: `RFC9999-2-2` read §2 and §3.\n", false},
		{"a longer id sharing the prefix", "Retired 2026-09-26: `RFC9999-2-33` read §2.\n", false},
		{"a level correction", "Correction 2026-09-26: `RFC9999-2-3` read §2: \"A speaker SHOULD count widgets\".\n", false},
		{"opener not first", "Note.\nRetired 2026-09-26: `RFC9999-2-3` read §2.\n", false},
		{"named first, the tag's destination after", "Retired 2026-09-26: `RFC9999-2-3` read §2. Its tag moved to `RFC9999-2-1`.\n", true},
		{"named as another retirement's destination", "Retired 2026-09-26: `RFC9999-2-2` read §2. Its tag moved to `RFC9999-2-3`.\n", false},
	} {
		t.Run(one.name, func(t *testing.T) {
			retires := false
			for _, correction := range parseCorrections(one.text) {
				retires = retires || correction.retires("RFC9999-2-3")
			}
			if retires != one.retires {
				t.Fatalf("%q retires RFC9999-2-3: %v, want %v", one.text, retires, one.retires)
			}
		})
	}
}

// VALIDATES: D-2 -- a retirement never authorizes a level demotion, even
// when it quotes the RFC.
// PREVENTS: one paragraph doing both jobs, so a demotion recorded as a
// retirement would pass checkLevelRatchet with no correction ever written.
func TestARetirementDoesNotAuthorizeALevelCorrection(t *testing.T) {
	const quote = "A speaker SHOULD send the widget when it can"
	text := "Retired 2026-09-26: `RFC9999-2-1` read §2: \"" + quote + "\".\n"
	if correctionAuthorizes("RFC9999-2-1", parseCorrections(text), quote+".\n") {
		t.Fatal("a Retired paragraph authorized a level correction")
	}
	level := "Correction 2026-09-26: `RFC9999-2-1` read §2: \"" + quote + "\".\n"
	if !correctionAuthorizes("RFC9999-2-1", parseCorrections(level), quote+".\n") {
		t.Fatal("the matching Correction paragraph no longer authorizes the demotion")
	}
}

// VALIDATES: D-2 -- retiredIDs takes an id only from the record of its own
// stem, skips the README, and treats an absent directory as no retirement.
// PREVENTS: a paragraph filed under the wrong RFC retiring a row of another,
// and the README's worked example retiring the fixture id it shows.
func TestRetiredIDsReadsOnlyTheOwnStemRecord(t *testing.T) {
	if got, err := retiredIDs(t.TempDir()); err != nil || len(got) != 0 {
		t.Fatalf("an absent record directory answered %v, %v; want no retirement and no error", got, err)
	}
	tree := t.TempDir()
	dir := filepath.Join(tree, correctionRel)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("the fixture directory: %v", err)
	}
	for name, body := range map[string]string{
		"rfc9999.md": retirementParagraph,
		"rfc9998.md": "Retired 2026-09-26: `RFC9999-2-1` read §2.\n",
		"README.md":  "Retired 2026-09-26: `RFC9999-2-2` read §2.\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("the fixture record %s: %v", name, err)
		}
	}
	got, err := retiredIDs(tree)
	if err != nil {
		t.Fatalf("read the records: %v", err)
	}
	if len(got) != 1 || !got["RFC9999-2-3"] {
		t.Fatalf("retired %v, want only RFC9999-2-3 from rfc9999.md", got)
	}
}
