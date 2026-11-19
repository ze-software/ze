// Design: website/AI.md -- Markdown preserves command text in its rendering context.
package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// TestEquivalentDetailMirrorPreservesPipeText renders the detail mirror through
// the site's Markdown engine. Pipes in code and prose must reach the reader
// unchanged, including aliases also published inside the CLI reference table.
func TestEquivalentDetailMirrorPreservesPipeText(t *testing.T) {
	command := &catalogCommand{
		Path: "show test", Mode: "operational", Usage: "show test <ipv4|ipv6>",
		Backend: []string{"linux|vpp"}, Subcommands: []string{"one|two"},
		Pipes:   []catalogPipe{{Name: "view|detail", Description: "Choose <one|two>."}},
		Aliases: []catalogAlias{{Name: "brief", Expansion: "select name | table", Description: "Use <a|b>."}},
	}
	row := &equivalentRow{Command: command}
	mirror := equivalentDetailMirror(&equivalentMapping{}, row, nil)
	page, _, err := renderMarkdown([]byte(mirror))
	if err != nil {
		t.Fatal(err)
	}
	text := visibleText(page)
	for _, want := range []string{command.Usage, "linux|vpp", "one|two", "view|detail", "select name | table", "Choose <one|two>.", "Use <a|b>."} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered detail loses literal %q: %s", want, text)
		}
	}
	if strings.Contains(text, `\|`) {
		t.Errorf("table-cell escaping leaked into detail text: %s", text)
	}
}

// TestCommandMirrorPipeCellsKeepTheirColumns renders the shared pipe/alias
// producers in an actual GFM table and checks both cell count and visible text.
func TestCommandMirrorPipeCellsKeepTheirColumns(t *testing.T) {
	command := &catalogCommand{
		Pipes:   []catalogPipe{{Name: "view|detail", Description: "Choose <one|two>."}},
		Aliases: []catalogAlias{{Name: "brief", Expansion: "select name | table"}},
	}
	page, _, err := renderMarkdown([]byte("| Pipes | End |\n| --- | --- |\n| " + commandMirrorPipes(command) + " | sentinel |\n"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := xhtml.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	var cells []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode {
			if node.Data == "td" {
				cells = append(cells, node)
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if len(cells) != 2 {
		t.Fatalf("pipe row has %d cells, want 2: %s", len(cells), page)
	}
	for _, want := range []string{"view|detail", "Choose <one|two>.", "select name | table", "sentinel"} {
		if !strings.Contains(visibleText(page), want) {
			t.Errorf("table loses %q: %s", want, page)
		}
	}
}

// TestStylesheetBuildRefusesEarlyCommentClose exercises the asset publisher,
// including imported CSS, so a broken comment cannot replace a working asset.
func TestStylesheetBuildRefusesEarlyCommentClose(t *testing.T) {
	source, output := t.TempDir(), t.TempDir()
	directory := filepath.Join(source, "assets", "css")
	writeFixtureFile(t, filepath.Join(directory, "site.css"), "@import url(\"base.css\");\n.last { color: blue; }\n")
	writeFixtureFile(t, filepath.Join(directory, "base.css"), "/* A comment containing */ closes too early. */\n.first { color: red; }\n")
	if err := renderCSS(source, output); err == nil {
		t.Fatal("a stylesheet with an early comment close was published")
	}
	if _, err := os.Stat(filepath.Join(output, "assets", "site.css")); !os.IsNotExist(err) {
		t.Fatalf("a refused stylesheet left a published asset: %v", err)
	}
}

// TestStylesheetBuildKeepsCommentMarkersInsideStrings prevents validation and
// minification from interpreting a quoted comment marker as CSS syntax.
func TestStylesheetBuildKeepsCommentMarkersInsideStrings(t *testing.T) {
	source, output := t.TempDir(), t.TempDir()
	writeFixtureFile(t, filepath.Join(source, "assets", "css", "site.css"), `.first::before { content: "/* literal */"; } /* real comment */ .last { color: blue; }`)
	if err := renderCSS(source, output); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, output, "assets/site.css")
	for _, want := range []string{`"/* literal */"`, ".last{color:blue}"} {
		if !strings.Contains(page, want) {
			t.Errorf("stylesheet loses %q: %s", want, page)
		}
	}
}
