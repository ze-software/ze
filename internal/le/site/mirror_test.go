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

// TestMirrorPreservesLiteralTextThroughMarkdown renders each converted mirror
// again so decoded tags, entities and ampersands cannot change its meaning.
func TestMirrorPreservesLiteralTextThroughMarkdown(t *testing.T) {
	for _, scenario := range []struct {
		name, fragment, text string
	}{
		{
			"prose",
			`<p>Literal &lt;img src=x onerror="alert(1)"&gt; &amp;lt;entity&amp;gt; &amp; ordinary text.</p>`,
			`Literal <img src=x onerror="alert(1)"> &lt;entity&gt; & ordinary text.`,
		},
		{
			"literal blockquote marker",
			`<p>&gt; literal marker</p>`,
			`> literal marker`,
		},
		{
			"inline code",
			`<p>Run <code>show &lt;peer&gt; &amp; &amp;lt;literal&amp;gt;</code>.</p>`,
			`Run show <peer> & &lt;literal&gt;.`,
		},
		{
			"inline code with line break",
			`<code>show<br>peer</code>`,
			`show peer`,
		},
		{
			"chip",
			`<span class="chip">A &lt; B &amp; &amp;lt;literal&amp;gt;</span>`,
			`A < B & &lt;literal&gt;`,
		},
		{
			"empty chip",
			`<span class="chip"> </span>`,
			``,
		},
		{
			"chip with line break",
			`<span class="chip">A<br>B</span>`,
			`A B`,
		},
		{
			"tag with nested code",
			`<span class="tag"><code>A &amp;lt; B &amp; C</code></span>`,
			`A &lt; B & C`,
		},
		{
			"card label",
			`<span class="card-label">A &lt; B &amp; C</span>`,
			`A < B & C`,
		},
		{
			"tag class on prose formatting",
			`<strong class="tag">A &lt;img src=x onerror="alert(1)"&gt; &amp; C</strong>`,
			`A <img src=x onerror="alert(1)"> & C`,
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			mirror, err := htmlToMarkdown(scenario.fragment, "https://ze-software.net/project/")
			if err != nil {
				t.Fatal(err)
			}
			rendered, _, err := renderMarkdown([]byte(mirror))
			if err != nil {
				t.Fatal(err)
			}
			if got := visibleText(rendered); got != scenario.text {
				t.Errorf("mirror changed visible text: got %q, want %q; Markdown: %s", got, scenario.text, mirror)
			}
			if strings.Contains(rendered, `<img src=x onerror=`) {
				t.Error("literal markup became an image element")
			}
		})
	}
}

// TestMirrorKeepsPreformattedLiteralText checks that prose escaping does not
// alter the indentation, markup characters or entity text of a fenced block.
func TestMirrorKeepsPreformattedLiteralText(t *testing.T) {
	fragment := "<pre><code>  show &lt;peer&gt; &amp; &amp;lt;literal&amp;gt;\n    next</code></pre>"
	mirror, err := htmlToMarkdown(fragment, "https://ze-software.net/project/")
	if err != nil {
		t.Fatal(err)
	}
	if want := "```\n  show <peer> & &lt;literal&gt;\n    next\n```"; !strings.Contains(mirror, want) {
		t.Errorf("fenced code changed: want %q in %q", want, mirror)
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
