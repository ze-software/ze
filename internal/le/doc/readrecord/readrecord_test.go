// VALIDATES: a section read record is written only for a section a Read
// returned whole, and stays current until that section's text changes
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-4, AC-5,
// the record format both the Read hook and the judge read).
// PREVENTS: credit for a Read that stopped inside the section, and credit that
// survives a decision row added after the read.
package readrecord

import (
	"path/filepath"
	"strings"
	"testing"
)

// recordPage is a page with prose, a two-row section, a subsection inside it,
// and a later section.
const recordPage = "# Widget\n\nHow it answers.\n\n## Design decisions\n\n| Decision | It forbids | Source | Date |\n" +
	"|---|---|---|---|\n| One | Two | plan/records/a.md | 2026-08-21 |\n\n### Detail\n\nInside.\n\n" +
	"```\n## Not a heading\n```\n\n## Wire format\n\nJSON.\n"

// TestSectionRecordFollowsTheSection is the record's contract. Method: the
// section spans its heading to its last non-blank line, subsection and fence
// included; a Read cut before the next heading records nothing; a Read of the
// whole page records it; a row added to the section stales the record, and an
// edit of another section does not.
func TestSectionRecordFollowsTheSection(t *testing.T) {
	section, found := Find(Sections(recordPage), "Design decisions")
	if !found {
		t.Fatal("the section was not found")
	}
	if section.Start != 5 || section.End != 17 {
		t.Fatalf("the section spans lines %d to %d, want 5 to 17", section.Start, section.End)
	}
	if _, found := Find(Sections(recordPage), "Not a heading"); found {
		t.Fatal("a line inside a fence was read as a heading")
	}

	marker := filepath.Join(t.TempDir(), ".section-read-page-sess")
	lines := strings.Split(strings.TrimSuffix(recordPage, "\n"), "\n")
	cut := strings.Join(lines[:section.End], "\n")
	if err := RecordRead(marker, cut, 1, len(lines)); err != nil {
		t.Fatal(err)
	}
	if read, err := SectionRead(marker, section); err != nil || read {
		t.Fatalf("a Read that stopped before the next heading recorded the section (read %v, err %v)", read, err)
	}

	if err := RecordRead(marker, recordPage, 1, len(lines)); err != nil {
		t.Fatal(err)
	}
	if read, err := SectionRead(marker, section); err != nil || !read {
		t.Fatalf("a whole Read did not record the section (read %v, err %v)", read, err)
	}

	edited := strings.Replace(recordPage, "JSON.", "JSON, always.", 1)
	if same, _ := Find(Sections(edited), "Design decisions"); same.Digest() != section.Digest() {
		t.Fatal("an edit of another section changed this section's digest")
	}
	grown := strings.Replace(recordPage, "2026-08-21 |\n", "2026-08-21 |\n| Three | Four | plan/records/b.md | 2026-11-20 |\n", 1)
	added, _ := Find(Sections(grown), "Design decisions")
	if read, err := SectionRead(marker, added); err != nil || read {
		t.Fatalf("a record of the old section counts for the grown one (read %v, err %v)", read, err)
	}
}
