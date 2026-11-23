package commit

import (
	"slices"
	"strings"
	"testing"
)

// The header and rule plan/records/weekly-fragment-measure.md sets for every
// update's table, copied from that file so the section is judged against the
// record it fills.
const (
	weeklyMeasureRecordHeader = "| Item | Before | Corrects | Exposure | Upstream | Asked by | Fragment |"
	weeklyMeasureRecordRule   = "|------|--------|----------|----------|----------|----------|----------|"
)

// weeklyMeasureAllFacts answers the six headings of a fragment that carries
// every fact: text under each, none of it only "none".
func weeklyMeasureAllFacts() []newsSection {
	return []newsSection{
		{name: "Before", body: "A revoked certificate was accepted."},
		{name: "After", body: "A revoked certificate is refused."},
		{name: "Corrects", body: "The 7 September post said revocation was checked."},
		{name: "Exposure", body: "Gateways that trust a CA which revokes certificates."},
		{name: "Upstream", body: "The library's check was reported upstream."},
		{name: "Asked by", body: "An operator, on a GitHub issue."},
	}
}

// TestReportMeasuresFragmentsAsTheRecordTable asserts the fragment-measure
// section answers the table the measure record takes for the range.
//
// VALIDATES: the block opens with the range's full commit IDs as the record's
// heading, then the record's header and rule; each fragment the range adds is
// one row, in path order, with its topic, fragment for a fact it writes, none
// for a fact it writes only none, and its file stem; a fragment added before
// the range and the README are not rows; the note names what the writer still
// marks; a fragment missing a heading, and a malformed one, each fail the
// section naming the path and the reason; an empty range answers a zero count
// and the empty table.
// PREVENTS: a measure table that drops a fragment, reads a missing heading as
// none, or no longer matches the record's columns.
func TestReportMeasuresFragmentsAsTheRecordTable(t *testing.T) {
	root := newCommitRepository(t)
	weeklyCommit(t, root, "vocabulary and an old fragment", "", map[string]string{
		newsTopicsPath: newsVocabulary,
		newsFragmentDirectory + "old.md": newsFragmentText(
			"journal: bgp-old 2026-10-01\ntopic: BGP\n", newsValidSections()),
	})
	from := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	someNone := newsValidSections()
	someNone[2].body = "The 12 October post said the rewrite stayed."
	weeklyCommit(t, root, "two fragments", "", map[string]string{
		newsFragmentDirectory + "all-facts.md": newsFragmentText(
			"spec: all-facts\ntopic: BGP\nfound-by: review\n", weeklyMeasureAllFacts()),
		newsFragmentDirectory + "some-none.md": newsFragmentText("journal: bgp-reset 2026-11-09\ntopic: BGP\n", someNone),
		newsFragmentReadme:                     "# News fragments\n",
	})

	events := weeklyRange(t, root, from)
	body, err := answerWeeklyFragmentMeasure(t.Context(), events)
	if err != nil {
		t.Fatalf("the fragment-measure section failed: %v", err)
	}
	want := []string{
		"## " + events.FromCommit + ".." + events.ToCommit,
		"",
		weeklyMeasureRecordHeader,
		weeklyMeasureRecordRule,
		"| BGP | fragment | fragment | fragment | fragment | fragment | all-facts |",
		"| BGP | fragment | fragment | none | none | none | some-none |",
	}
	if !slices.Equal(body.Block, want) {
		t.Errorf("block:\n%s\nwant:\n%s", strings.Join(body.Block, "\n"), strings.Join(want, "\n"))
	}
	if fact := weeklyFact(t, body, "fragments measured"); fact.Value != "2" {
		t.Errorf("fragments measured = %q, want 2", fact.Value)
	}
	for _, mark := range []string{"failed", "research", "n/a", "no fragment", "missing a heading"} {
		if !strings.Contains(body.Note, mark) {
			t.Errorf("the note does not say %q: %q", mark, body.Note)
		}
	}

	head := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	empty, err := answerWeeklyFragmentMeasure(t.Context(), weeklyRange(t, root, head))
	if err != nil {
		t.Fatalf("the fragment-measure section failed over an empty range: %v", err)
	}
	if fact := weeklyFact(t, empty, "fragments measured"); fact.Value != "0" {
		t.Errorf("an empty range measured %q fragments, want 0", fact.Value)
	}
	if len(empty.Block) != 4 {
		t.Errorf("an empty range answered block %q, want the heading, a blank line, the header and the rule", empty.Block)
	}

	missing := newsFragmentDirectory + "missing-heading.md"
	weeklyCommit(t, root, "a fragment missing a heading", "", map[string]string{
		missing: newsFragmentText("spec: missing-heading\ntopic: BGP\n", slices.Delete(newsValidSections(), 4, 5)),
	})
	_, err = answerWeeklyFragmentMeasure(t.Context(), weeklyRange(t, root, head))
	if err == nil {
		t.Fatal("a range adding a fragment missing a heading answered the section")
	}
	if !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "## Upstream is missing") {
		t.Errorf("the failure does not name %s and its missing Upstream heading: %v", missing, err)
	}

	afterMissing := strings.TrimSpace(runCommitGitOutput(t, root, "rev-parse", "HEAD"))
	broken := newsFragmentDirectory + "broken.md"
	weeklyCommit(t, root, "a malformed fragment", "", map[string]string{broken: "no front matter at all\n"})
	_, err = answerWeeklyFragmentMeasure(t.Context(), weeklyRange(t, root, afterMissing))
	if err == nil {
		t.Fatal("a range adding a fragment that does not parse answered the section")
	}
	if !strings.Contains(err.Error(), broken) || !strings.Contains(err.Error(), "no front matter") {
		t.Errorf("the failure does not name %s and why it does not parse: %v", broken, err)
	}
}

// TestWeeklyMeasureColumnsReadEveryHeadingOnce asserts the measure's fact
// columns read each heading newsHeadings declares exactly once, and nothing
// else.
//
// VALIDATES: a heading added to the fragment format, or renamed, makes this
// test red until a column reads it.
// PREVENTS: a fact the fragments carry that the measure table never reports.
func TestWeeklyMeasureColumnsReadEveryHeadingOnce(t *testing.T) {
	read := map[string]int{}
	for _, column := range weeklyMeasureColumns {
		for _, heading := range column.headings {
			read[heading]++
		}
	}
	for _, heading := range newsHeadings {
		if read[heading.Name] != 1 {
			t.Errorf("heading %s is read by %d measure columns, want 1", heading.Name, read[heading.Name])
		}
		delete(read, heading.Name)
	}
	for heading := range read {
		t.Errorf("a measure column reads %s, which newsHeadings does not declare", heading)
	}
}
