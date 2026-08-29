package site

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestMirrorConvertsBackWhenTheSourceHoldsBlockHTML checks AC-5: a Markdown
// source that carries block HTML gets a mirror made from the RENDERED body, not
// a copy of the source.
//
// The method is to render one source that holds a table written as HTML, then
// ask what the mirror says. A mirror copied from the source would show the
// reader "<table>" and "<td>"; a mirror converted back shows a Markdown table.
func TestMirrorConvertsBackWhenTheSourceHoldsBlockHTML(t *testing.T) {
	source := strings.Join([]string{
		"# Interface state",
		"",
		"<table>",
		"  <tr><th>Field</th><th>Meaning</th></tr>",
		"  <tr><td>oper</td><td>link state</td></tr>",
		"</table>",
		"",
		"Read it with `ze show interfaces`.",
		"",
	}, "\n")

	if !containsBlockHTML(source) {
		t.Fatalf("the source holds <table> on its own line, so it must read as block HTML")
	}
	body, _, err := renderMarkdown([]byte(source))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	mirror, err := htmlToMarkdown(body, "https://ze-software.net/features/interfaces/")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	for _, unwanted := range []string{"<table>", "<td>", "<tr>"} {
		if strings.Contains(mirror, unwanted) {
			t.Errorf("the mirror must not show the reader %s; it was copied from the source, not converted:\n%s", unwanted, mirror)
		}
	}
	for _, want := range []string{
		"# Interface state",
		"| Field | Meaning |",
		"| --- | --- |",
		"| oper | link state |",
		"`ze show interfaces`",
	} {
		if !strings.Contains(mirror, want) {
			t.Errorf("the mirror must carry %q:\n%s", want, mirror)
		}
	}
}

// TestMirrorMatchesThePublishedConversion holds the converter to the mirror the
// retired renderer published.
//
// The fixture is one published pair taken from gh-pages at its last Python-era
// commit: the page and the index.md beside it, which the retired build made by
// converting that page's own <main>. Every component case the converter carries
// -- the eyebrow, the chips, the link list, the fenced block and the bullet
// list -- is exercised by this one page, so a case that stops working shows up
// here rather than in a producer written five phases later.
func TestMirrorMatchesThePublishedConversion(t *testing.T) {
	page := readTestdata(t, "published-labs-vlan-qos.html")
	want := readTestdata(t, "published-labs-vlan-qos.md")

	content, err := extractMain(page)
	if err != nil {
		t.Fatalf("extract main: %v", err)
	}
	got, err := htmlToMarkdown(content, "https://ze-software.net/labs/vlan-qos/")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if got != want {
		t.Errorf("the mirror differs from the published one:\n%s", firstDifference(want, got))
	}
}

// readTestdata answers one fixture file.
func readTestdata(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(content)
}

// firstDifference names the first line two texts disagree on, with the lines
// around it, so a failure reads as one change rather than as two whole files.
func firstDifference(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for index := 0; index < len(wantLines) || index < len(gotLines); index++ {
		wantLine, gotLine := "", ""
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if wantLine == gotLine {
			continue
		}
		return "line " + strconv.Itoa(index+1) + "\n  want: " + wantLine + "\n   got: " + gotLine
	}
	return "no line differs"
}
