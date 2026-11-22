// VALIDATES: a section read record is written only for a section a Read
// returned whole, and stays current until that section's text changes
// (spec-design-needs-owner-approval, AC-4, AC-5,
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

// lastSectionPage is a page whose last section is Design decisions, ending in
// a newline: nine lines, the section lines 5 to 9.
const lastSectionPage = "# Widget\n\nHow it answers.\n\n## Design decisions\n\n| Decision | It forbids |\n" +
	"|---|---|\n| One | Two |\n"

// TestLastSectionReadToTheEndIsRecorded is the record of a section that runs
// to the page's last line. Method: a Read of the whole page records it, both
// when total counts the empty line after the final newline (the Read tool's
// totalLines) and when it does not; a Read that stopped one line short of the
// end does not make SectionRead true under either count.
// PREVENTS: a Design decisions section at the end of a page that no Read can
// ever record, so the edit hook and `le spec design judge` refuse forever.
func TestLastSectionReadToTheEndIsRecorded(t *testing.T) {
	section, found := Find(Sections(lastSectionPage), "Design decisions")
	if !found {
		t.Fatal("the section was not found")
	}
	if section.Start != 5 || section.End != 9 {
		t.Fatalf("the section spans lines %d to %d, want 5 to 9", section.Start, section.End)
	}
	count := strings.Count(lastSectionPage, "\n")
	lines := strings.Split(strings.TrimSuffix(lastSectionPage, "\n"), "\n")
	cut := strings.Join(lines[:len(lines)-1], "\n")
	for _, total := range []int{count + 1, count} {
		whole := filepath.Join(t.TempDir(), ".section-read-page-sess")
		if err := RecordRead(whole, lastSectionPage, 1, total); err != nil {
			t.Fatal(err)
		}
		if read, err := SectionRead(whole, section); err != nil || !read {
			t.Fatalf("a whole Read with total %d did not record the last section (read %v, err %v)", total, read, err)
		}
		short := filepath.Join(t.TempDir(), ".section-read-page-sess")
		if err := RecordRead(short, cut, 1, total); err != nil {
			t.Fatal(err)
		}
		if read, err := SectionRead(short, section); err != nil || read {
			t.Fatalf("a Read one line short with total %d recorded the last section (read %v, err %v)", total, read, err)
		}
	}
}

// TestSectionThroughNamesTheLineToReach is the line a Read must reach to
// record a section. Method: a section closed by a later heading of its level
// or a higher one reaches that heading's line, a subsection closed by the same
// heading reaches it too, and the last section, like the page's title,
// reaches the page's last line; ReadLimit counts the lines from the heading
// through that line.
// PREVENTS: a refusal whose advised Read stops before the closing heading and
// so records nothing.
func TestSectionThroughNamesTheLineToReach(t *testing.T) {
	sections := Sections(recordPage)
	want := []struct {
		heading string
		through int
		limit   int
	}{
		{"Widget", 21, 21},
		{"Design decisions", 19, 15},
		{"Detail", 19, 9},
		{"Wire format", 21, 3},
	}
	for _, row := range want {
		section, found := Find(sections, row.heading)
		if !found {
			t.Fatalf("the section %q was not found", row.heading)
		}
		if section.Through != row.through {
			t.Errorf("section %q reaches line %d, want %d", row.heading, section.Through, row.through)
		}
		if section.ReadLimit() != row.limit {
			t.Errorf("section %q has read limit %d, want %d", row.heading, section.ReadLimit(), row.limit)
		}
	}
	last, _ := Find(Sections(lastSectionPage), "Design decisions")
	if last.Through != 9 || last.ReadLimit() != 5 {
		t.Errorf("the last section reaches line %d with limit %d, want 9 and 5", last.Through, last.ReadLimit())
	}
}

// TestCompleteSectionsAndWholeDigest verifies the pure in-memory helpers shared
// by RecordRead and the Antigravity transcript reader. Method: CompleteSections
// excludes an unclosed trailing section when reachesEnd is false and includes
// it when reachesEnd is true; WholeDigest treats CRLF, LF and missing final
// newline identically while changing when file content changes.
func TestCompleteSectionsAndWholeDigest(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(recordPage, "\n"), "\n")
	cut := strings.Join(lines[:17], "\n")
	if _, found := Find(CompleteSections(cut, false), "Design decisions"); found {
		t.Error("CompleteSections included an unclosed section when reachesEnd is false")
	}
	if _, found := Find(CompleteSections(recordPage, false), "Design decisions"); !found {
		t.Error("CompleteSections dropped a closed section when reachesEnd is false")
	}
	if _, found := Find(CompleteSections(recordPage, false), "Wire format"); found {
		t.Error("CompleteSections included the unclosed last section when reachesEnd is false")
	}
	if _, found := Find(CompleteSections(recordPage, true), "Wire format"); !found {
		t.Error("CompleteSections dropped the last section when reachesEnd is true")
	}

	base := WholeDigest("# Door\n\nRule.\n")
	if got := WholeDigest("# Door\r\n\r\nRule.\r\n"); got != base {
		t.Errorf("WholeDigest differed on CRLF: got %s, want %s", got, base)
	}
	if got := WholeDigest("# Door\n\nRule."); got != base {
		t.Errorf("WholeDigest differed without trailing newline: got %s, want %s", got, base)
	}
	if got := WholeDigest("# Door\n\nChanged.\n"); got == base {
		t.Error("WholeDigest did not change when content changed")
	}
}
