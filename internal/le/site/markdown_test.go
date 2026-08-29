package site

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// TestMarkdownReadsTheSameAsThePublishedPage is the evidence for assumption A-3
// and for AC-4: goldmark reads a real page the way python-markdown read it.
//
// The fixture pair is docs/guide/anomaly.md and the page the retired renderer
// published from it, taken from gh-pages at its last Python-era commit. The
// page was chosen by measuring rather than by taste: it carries 13 list items
// and 31 table rows, which are the two constructs the two parsers disagree
// about, and none of the post-rendering passes that a later phase owns, so a
// difference here is the parser's.
//
// Three transforms of the published page belong to a later phase and are
// removed before the comparison rather than reproduced: the table of contents
// this file inserts, the page sidebar the shell places, and the hero eyebrow
// the docs producer adds. Each is named below, so nothing is normalized away
// without saying which producer owns it.
//
// Link TARGETS are compared by their stem. The published page carries
// "../ddos-mitigation/", which the docs producer rewrote from the source's
// "ddos-mitigation.md" through the manifest; both name one page, and the
// rewriting is phase 3's, not goldmark's.
func TestMarkdownReadsTheSameAsThePublishedPage(t *testing.T) {
	source := readTestdata(t, "docs-guide-anomaly.md")
	published := readTestdata(t, "published-guides-anomaly.html")

	body, headings, err := renderMarkdown([]byte(source))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	content, err := extractMain(published)
	if err != nil {
		t.Fatalf("extract main: %v", err)
	}
	reference := publishedBodyOnly(content)

	wantWords, wantLinks := readablePage(t, reference)
	gotWords, gotLinks := readablePage(t, body)

	if strings.Join(wantWords, " ") != strings.Join(gotWords, " ") {
		t.Errorf("the page reads differently:\n%s", firstDifference(
			strings.Join(wantWords, "\n"), strings.Join(gotWords, "\n")))
	}
	if len(wantLinks) != len(gotLinks) {
		t.Fatalf("the page must carry %d links, not %d: want %v got %v",
			len(wantLinks), len(gotLinks), wantLinks, gotLinks)
	}
	for index, want := range wantLinks {
		got := gotLinks[index]
		if want.label != got.label {
			t.Errorf("link %d must read %q, not %q", index, want.label, got.label)
		}
		if linkStem(want.href) != linkStem(got.href) {
			t.Errorf("link %d must reach %q, not %q", index, want.href, got.href)
		}
	}

	// The contents list is built from the same headings the body carries, so a
	// heading the body shows must be one a reader can jump to.
	contents := renderDocTOC(headings)
	for _, heading := range headings {
		if heading.Level < 2 {
			continue
		}
		if !strings.Contains(contents, ">"+heading.Label+"</a>") {
			t.Errorf("the contents list must name the heading %q:\n%s", heading.Label, contents)
		}
	}
}

// The three published-page passes a later phase owns, removed before the
// comparison above: the contents list markdown.go splices in, the sidebar
// shell.go places, and the eyebrow the docs producer puts above the title.
var (
	publishedContents = regexp.MustCompile(`(?s)<nav class="doc-toc".*?</nav>`)
	publishedSidebar  = regexp.MustCompile(`(?s)<aside class="page-sidebar".*?</aside>`)
	publishedEyebrow  = regexp.MustCompile(`(?s)<span class="journey-eyebrow">.*?</span>`)
)

func publishedBodyOnly(content string) string {
	content = publishedContents.ReplaceAllString(content, "")
	content = publishedSidebar.ReplaceAllString(content, "")
	return publishedEyebrow.ReplaceAllString(content, "")
}

// readableLink is one link as a reader meets it: the words it shows and where
// it goes.
type readableLink struct {
	label string
	href  string
}

// readablePage answers the words one page fragment shows and the links it
// carries, in reading order.
func readablePage(t *testing.T, fragment string) ([]string, []readableLink) {
	t.Helper()
	var words []string
	var links []readableLink
	var anchor *readableLink
	tokenizer := xhtml.NewTokenizer(strings.NewReader(fragment))
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			if !errors.Is(tokenizer.Err(), io.EOF) {
				t.Fatalf("read fragment: %v", tokenizer.Err())
			}
			return words, links
		case xhtml.TextToken:
			text := strings.Fields(string(tokenizer.Text()))
			words = append(words, text...)
			if anchor != nil {
				anchor.label = strings.TrimSpace(anchor.label + " " + strings.Join(text, " "))
			}
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttributes := tokenizer.TagName()
			if string(name) != "a" {
				continue
			}
			for hasAttributes {
				var key, value []byte
				key, value, hasAttributes = tokenizer.TagAttr()
				if string(key) == "href" {
					links = append(links, readableLink{href: string(value)})
					anchor = &links[len(links)-1]
				}
			}
		case xhtml.EndTagToken:
			if name, _ := tokenizer.TagName(); string(name) == "a" {
				anchor = nil
			}
		case xhtml.CommentToken, xhtml.DoctypeToken:
			// Neither shows a reader anything.
		}
	}
}

// linkStem answers the page a link names, with the spelling the two renderers
// disagree about removed: "../ddos-mitigation/" and "ddos-mitigation.md" both
// answer "ddos-mitigation". A link that leaves the site answers itself.
func linkStem(href string) string {
	if strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "#") {
		return href
	}
	trimmed := strings.TrimSuffix(href, "/")
	return strings.TrimSuffix(path.Base(trimmed), ".md")
}

// TestFrontMatterSplitsMetadataFromTheBody covers the page metadata a source
// may open with, and the three malformed forms a build must refuse by name
// rather than pass over.
func TestFrontMatterSplitsMetadataFromTheBody(t *testing.T) {
	metadata, body, err := parseFrontMatter([]byte("---\ntitle: \"Ze BGP\"\n# a note\ncategory: routing\n---\n# Heading\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if metadata["title"] != "Ze BGP" || metadata["category"] != "routing" || len(metadata) != 2 {
		t.Errorf("metadata must read the two keys and skip the comment: %v", metadata)
	}
	if string(body) != "# Heading\n" {
		t.Errorf("the body must start after the closing fence: %q", body)
	}

	plain, same, err := parseFrontMatter([]byte("# Heading\n"))
	if err != nil || len(plain) != 0 || string(same) != "# Heading\n" {
		t.Errorf("a source with no metadata block must answer an empty map and its own text: %v %q %v", plain, same, err)
	}

	refusals := []struct {
		name   string
		source string
	}{
		{"a block that never closes", "---\ntitle: Ze\n"},
		{"a line with no colon", "---\ntitle\n---\n"},
		{"a repeated key", "---\ntitle: One\ntitle: Two\n---\n"},
	}
	for _, refusal := range refusals {
		t.Run(refusal.name, func(t *testing.T) {
			if _, _, err := parseFrontMatter([]byte(refusal.source)); err == nil {
				t.Errorf("front matter with %s must be refused, not passed over", refusal.name)
			}
		})
	}
}

// TestDocTOCGoesAfterTheFirstClosingDiv pins the splice rule. It is literal
// rather than structural: the retired renderer searched for the first "</div>"
// in the body, which is where the hero block closes, and every published page
// carries the contents list there. A body with no such tag takes it at the top.
func TestDocTOCGoesAfterTheFirstClosingDiv(t *testing.T) {
	const contents = `<nav class="doc-toc">list</nav>`
	const hero = `<div class="hero"><h1>T</h1></div>`
	if got := insertDocTOC(hero+"\n<p>after</p>", contents); !strings.HasPrefix(got, hero+"\n"+contents) {
		t.Errorf("the contents list must follow the first </div>: %s", got)
	}
	if got := insertDocTOC("<h1>T</h1>\n<p>after</p>", contents); !strings.HasPrefix(got, contents+"\n<h1>") {
		t.Errorf("a body with no </div> must take the contents list at the top: %s", got)
	}
	if got := insertDocTOC("<h1>T</h1>", ""); got != "<h1>T</h1>" {
		t.Errorf("a page with no headings must take no contents list: %s", got)
	}
}

// TestMirrorIsWrittenBesideThePage checks the mirror contract: index.md sits in
// the page's own directory, so a reader reaches it at the page's URL with
// index.md in place of the trailing slash.
func TestMirrorIsWrittenBesideThePage(t *testing.T) {
	page := filepath.Join(t.TempDir(), "guides", "anomaly", "index.html")
	if err := writeMarkdownMirror(page, "# Anomaly\n"); err != nil {
		t.Fatalf("write mirror: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(page), "index.md"))
	if err != nil {
		t.Fatalf("the mirror must sit beside the page: %v", err)
	}
	if string(content) != "# Anomaly\n" {
		t.Errorf("the mirror must carry what it was given: %q", content)
	}
}
