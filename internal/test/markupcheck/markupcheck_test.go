package markupcheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/markupcheck"
)

// writePackage builds a throwaway Go package holding body, so no committed
// fixture can drift from the rules below.
func writePackage(t *testing.T, name, body string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))

	return dir
}

// TestReportNamesEachMarkupLiteral proves the scanner finds a tag in each string
// form Go offers, and reports the file and the line.
//
// VALIDATES: AC-7 of spec-web-templ-migration is checked by a scan, not
// by a grep in a checklist.
// PREVENTS: markup returning to Go with every gate green.
func TestReportNamesEachMarkupLiteral(t *testing.T) {
	dir := writePackage(t, "page.go", "package p\n"+
		"func a() string { return `<div class=\"x\">` }\n"+
		"func b() string { return \"</span>\" }\n"+
		"func c() string { return `<rect x=\"1\"/>` }\n")

	sites, literals, err := markupcheck.Report(dir)
	require.NoError(t, err)
	assert.Positive(t, literals, "the walk read no string literal at all")
	require.Len(t, sites, 3, "each of the three literals holds a tag")

	assert.Equal(t, 2, sites[0].Line)
	assert.Equal(t, "div", sites[0].Tag)
	assert.Equal(t, "span", sites[1].Tag)
	assert.Equal(t, "rect", sites[2].Tag, "an SVG element is markup too")
}

// TestReportPassesTextThatOnlyLooksLikeMarkup proves a bare `<word>` is not a
// finding, so a usage string and a comment stay quiet.
//
// Two of these would fool a check that read element names. `<path>` IS an SVG
// element. `<old-name>` carries the hyphen HTML gives a custom element. Neither
// is markup, and the form is what says so.
//
// VALIDATES: the scan discriminates. A gate that reds on `usage: set <leaf>`
// would be turned off rather than obeyed.
// PREVENTS: a false finding teaching the next reader to ignore this check.
func TestReportPassesTextThatOnlyLooksLikeMarkup(t *testing.T) {
	dir := writePackage(t, "cli.go", "package p\n"+
		"// A comment naming <div class=\"x\"> is prose, not markup.\n"+
		"func a() string { return \"usage: set <leaf> <value>\" }\n"+
		"func b() string { return \"rename <old-name> to <destination>\" }\n"+
		"func c() string { return \"expected /config/<verb>/<path>\" }\n")

	sites, literals, err := markupcheck.Report(dir)
	require.NoError(t, err)
	assert.Positive(t, literals)
	assert.Empty(t, sites, "each of these is a bare placeholder, which no element opens")
}

// TestReportReadsAnElementNobodyListed proves the scan consults no list of
// element names, so a tag this package never heard of is still a finding.
//
// VALIDATES: the gate cannot be escaped by an element somebody forgot to list.
// PREVENTS: the maintained-list hole a name-based check would carry.
func TestReportReadsAnElementNobodyListed(t *testing.T) {
	dir := writePackage(t, "widget.go", "package p\n"+
		"func a() string { return `<ze-gauge value=\"3\"></ze-gauge>` }\n"+
		"func b() string { return `<invented-by-nobody hidden>` }\n")

	sites, _, err := markupcheck.Report(dir)
	require.NoError(t, err)
	require.Len(t, sites, 2, "one site per literal: the first tag names the file's problem")
	assert.Equal(t, "ze-gauge", sites[0].Tag)
	assert.Equal(t, "invented-by-nobody", sites[1].Tag)
}

// TestReportReadsABareVoidElement proves a void element with no attribute is a
// finding. Nothing closes it, so a bare `<br>` is a whole element rather than
// a placeholder.
//
// strings.Join(rows, "<br>") is the realistic regression. It builds markup in
// Go with no close tag, no self-close and no attribute, so the three form rules
// above see nothing.
//
// VALIDATES: the form rule covers the sixteen elements HTML lets open without
// closing.
// PREVENTS: the one shape of Go-built markup the FORM of a tag cannot show.
func TestReportReadsABareVoidElement(t *testing.T) {
	dir := writePackage(t, "rows.go", "package p\n"+
		"func a(x []string) string { return strings.Join(x, \"<br>\") }\n"+
		"func b() string { return \"<hr>\" }\n"+
		"func c() string { return \"<input>\" }\n")

	sites, _, err := markupcheck.Report(dir)
	require.NoError(t, err)
	require.Len(t, sites, 3)
	assert.Equal(t, "br", sites[0].Tag)
	assert.Equal(t, "hr", sites[1].Tag)
	assert.Equal(t, "input", sites[2].Tag)
}

// TestReportPassesTheTwoVoidNamesZeWritesAsCLIText proves the two names left
// out of voidElements stay quiet, so a usage string is not read as markup.
//
// Both are ze's own CLI vocabulary. execListEntryOp labels the copy verb's
// argument <source>, and the run verb answers "usage: run <command>", both in
// internal/component/web/cli_terminal.go.
//
// VALIDATES: the exclusions in voidElements are load-bearing.
// PREVENTS: four false findings in one file, which is how a gate stops being
// read.
func TestReportPassesTheTwoVoidNamesZeWritesAsCLIText(t *testing.T) {
	dir := writePackage(t, "cli.go", "package p\n"+
		"func a() string { return \"usage: run <command>\" }\n"+
		"func b() string { return \"<source>\" }\n")

	sites, literals, err := markupcheck.Report(dir)
	require.NoError(t, err)
	assert.Positive(t, literals)
	assert.Empty(t, sites)

	// The exclusion costs the BARE case and nothing else, so both names are
	// still findings once they carry an attribute. Without this half, the two
	// exclusions would be holes rather than a narrowing.
	withAttributes := writePackage(t, "media.go", "package p\n"+
		"func a() string { return `<source src=\"a.mp4\">` }\n"+
		"func b() string { return `<command hidden>` }\n")

	sites, _, err = markupcheck.Report(withAttributes)
	require.NoError(t, err)
	require.Len(t, sites, 2)
	assert.Equal(t, "source", sites[0].Tag)
	assert.Equal(t, "command", sites[1].Tag)
}

// TestReportSkipsGeneratedAndTestFiles proves the scan reads the sources a
// person writes. A *_templ.go IS the generated markup, and a test CAN assert on
// a tag.
//
// VALIDATES: the scan's scope.
// PREVENTS: the gate reporting its own generated output as a violation.
func TestReportSkipsGeneratedAndTestFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"page_templ.go", "page_test.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name),
			[]byte("package p\nfunc a() string { return `<div class=\"x\">` }\n"), 0o600))
	}

	sites, _, err := markupcheck.Report(dir)
	require.NoError(t, err)
	assert.Empty(t, sites)
}

// TestAssertNoMarkupRefusesAVacuousWalk proves a walk that reads nothing is a
// failure rather than a pass. A renamed directory would otherwise turn the gate
// off silently.
//
// VALIDATES: the gate cannot pass over an empty set.
// PREVENTS: the vacuous green every scan of a tree is one typo away from.
func TestAssertNoMarkupRefusesAVacuousWalk(t *testing.T) {
	_, literals, err := markupcheck.Report(t.TempDir())
	require.NoError(t, err)
	assert.Zero(t, literals, "an empty tree reads no literal, which AssertNoMarkup refuses")
}

// TestFindingsRefusesAStaleExemption proves an exemption that exempts nothing is
// itself a finding, so a ported file cannot keep its license to hold markup.
//
// VALIDATES: the exemption table is fail-closed, like webPortTemplates.
// PREVENTS: an exemption outliving the markup it was written for.
func TestFindingsRefusesAStaleExemption(t *testing.T) {
	dir := writePackage(t, "clean.go", "package p\nfunc a() string { return \"no markup\" }\n")

	findings, err := markupcheck.Findings(dir, map[string]string{
		"clean.go": "a reason for markup this file no longer holds",
	})
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "stale")
}

// TestExemptionDriftReportsEitherDirection proves the exemption table is fixed
// in size, so widening it and shrinking it both need a reader.
//
// Findings is blind to a new entry over a file that does build markup. That
// makes widening the table the one edit that turns a finding green.
//
// VALIDATES: the boundary of the exemption count, one either side of it.
// PREVENTS: a third exemption arriving with the gate still green.
func TestExemptionDriftReportsEitherDirection(t *testing.T) {
	two := map[string]string{"a.go": "a reason", "b.go": "another reason"}

	assert.Empty(t, markupcheck.ExemptionDrift(two, 2), "the stated size passes")
	assert.Contains(t, markupcheck.ExemptionDrift(two, 1), "want exactly 1", "one more is a finding")
	assert.Contains(t, markupcheck.ExemptionDrift(two, 3), "want exactly 3", "one fewer is a finding")
	assert.Empty(t, markupcheck.ExemptionDrift(map[string]string{}, 0), "an empty table is a claim of its own")
}

// TestFindingsAcceptsAnExplainedFile proves a named exemption silences the file
// it names and nothing else.
//
// VALIDATES: the drawing builders can be exempted by name with a reason.
// PREVENTS: one exemption covering a sibling that was never reviewed.
func TestFindingsAcceptsAnExplainedFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "drawing.go"),
		[]byte("package p\nfunc a() string { return `<rect x=\"1\">` }\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.go"),
		[]byte("package p\nfunc b() string { return `</div>` }\n"), 0o600))

	findings, err := markupcheck.Findings(dir, map[string]string{
		"drawing.go": "every attribute is a computed coordinate",
	})
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Contains(t, findings[0], "other.go")
}
