// Design: website/AI.md -- one Markdown pipeline renders every Markdown-sourced page
// Detail: shell.go wraps the body this file produces; mirror.go writes the Markdown sibling.
package site

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// markdownEngine converts a page source into the body HTML a page shell wraps.
//
// Every option is written out rather than left to a library default, so this
// call states the pipeline it is.
//
//   - Table is the one extension. The retired renderer ran python-markdown with
//     tables, fenced_code, sane_lists and toc; goldmark parses fenced code and
//     CommonMark lists itself, and this file builds the table of contents, so
//     the table syntax is all that is left to add. Strikethrough, linkify and
//     task lists are NOT added: python-markdown parsed none of them, so adding
//     them would change what a published page says.
//   - WithAutoHeadingID gives every heading the id its table of contents links
//     to. The slugs are goldmark's own and differ from python-markdown's; the
//     owner accepted that on 2026-08-29, and the page's own contents list is
//     built from the same ids, so it stays self-consistent.
//   - WithUnsafe passes raw HTML through, which python-markdown also did. Every
//     source is a file in this repository, so the input is trusted; a page
//     rendered from a source outside this repository would need this reviewed.
var markdownEngine = goldmark.New(
	goldmark.WithExtensions(extension.Table),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(
		goldmarkhtml.WithUnsafe(),
		goldmarkhtml.WithWriter(textWriter{Writer: goldmarkhtml.NewWriter()}),
		renderer.WithNodeRenderers(util.Prioritized(textNodes{writer: goldmarkhtml.NewWriter()}, 100)),
	),
)

// textWriter writes HTML text content, escaping only what text content owes.
//
// goldmark's own writer escapes a quotation mark to &quot; wherever text is
// written, a code span and a fenced code block included. A browser renders the
// reference as a quotation mark, so the page LOOKS right, but the published
// HTML then carries &quot; where the page source carried ", and every reader of
// the file rather than of the rendering meets the reference instead of the
// character. The retired python-markdown renderer left the character alone, so
// the move to goldmark is what introduced this.
//
// RawWrite is the only method overridden, and the choice is load-bearing.
// goldmark writes a link title through Write (renderLink), and that lands
// inside a double-quoted title attribute, where the reference IS owed. The four
// RawWrite call sites are a code block, a code span, a text node and a code
// string node, none of which is inside an attribute.
type textWriter struct {
	goldmarkhtml.Writer
}

// textNodes renders the two nodes that carry prose, so a quotation mark in
// prose is published as itself.
//
// The reason this is a node renderer and not another Writer method: goldmark
// routes prose and an attribute value through ONE method, Writer.Write. It
// writes a link title and a code fence language through it too, and both land
// inside a double-quoted attribute where the escape is owed. A Write that
// unescaped for prose broke those attributes, which is what
// TestCodeKeepsAQuotationMarkAndStillEscapesMarkup measured
// (title="a "quoted" title"). Overriding the two prose NODES instead leaves
// Write untouched for every attribute, so the two cases stop sharing an answer.
//
// Priority 100 puts these ahead of goldmark's own renderer, registered at 1000.
type textNodes struct {
	writer goldmarkhtml.Writer
}

// RegisterFuncs claims the text and string nodes. Every other node keeps
// goldmark's own rendering.
func (t textNodes) RegisterFuncs(registerer renderer.NodeRendererFuncRegisterer) {
	registerer.Register(ast.KindText, t.renderText)
	registerer.Register(ast.KindString, t.renderString)
}

// renderText writes one text node.
//
// The line-break arms mirror goldmark's own, minus the options this engine
// does not set: HardWraps, XHTML and EastAsianLineBreaks are each left at their
// default, so a soft break is one newline.
func (t textNodes) renderText(writer util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	text, isText := node.(*ast.Text)
	if !isText {
		return ast.WalkContinue, nil
	}
	value := text.Segment.Value(source)
	if text.IsRaw() {
		textWriter{Writer: t.writer}.RawWrite(writer, value)
		return ast.WalkContinue, nil
	}
	t.writeProse(writer, value)
	if text.HardLineBreak() {
		_, _ = writer.WriteString("<br>\n")
		return ast.WalkContinue, nil
	}
	if text.SoftLineBreak() {
		_ = writer.WriteByte('\n')
	}
	return ast.WalkContinue, nil
}

// renderString writes one string node, which carries a value the parser built
// rather than a span of the source.
func (t textNodes) renderString(writer util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	value, isString := node.(*ast.String)
	if !isString {
		return ast.WalkContinue, nil
	}
	if value.IsCode() {
		_, _ = writer.Write(value.Value)
		return ast.WalkContinue, nil
	}
	if value.IsRaw() {
		textWriter{Writer: t.writer}.RawWrite(writer, value.Value)
		return ast.WalkContinue, nil
	}
	t.writeProse(writer, value.Value)
	return ast.WalkContinue, nil
}

// writeProse writes prose, resolving what goldmark resolves and then undoing
// the one escape prose does not owe.
//
// The embedded writer does the resolving, because Write is where a character
// reference and a backslash escape are read, and forking that is how a renderer
// drifts from the parser beside it. Its answer is then read back and &quot; is
// returned to the character it stands for. Prose is never an attribute value,
// so nothing here can break one.
func (t textNodes) writeProse(writer util.BufWriter, value []byte) {
	var resolved bytes.Buffer
	buffered := bufio.NewWriter(&resolved)
	t.writer.Write(buffered, value)
	_ = buffered.Flush()
	_, _ = writer.Write(bytes.ReplaceAll(resolved.Bytes(), escapedQuote, plainQuote))
}

// The reference goldmark spells a quotation mark as, and the character it
// stands for.
var (
	escapedQuote = []byte("&quot;")
	plainQuote   = []byte(`"`)
)

// RawWrite writes source as HTML text content.
//
// The loop is bounded by the length of source, which the caller has already
// read from the page.
func (textWriter) RawWrite(writer util.BufWriter, source []byte) {
	written := 0
	for index := range len(source) {
		replacement := textContentEscape(source[index])
		if replacement == nil {
			continue
		}
		_, _ = writer.Write(source[written:index])
		_, _ = writer.Write(replacement)
		written = index + 1
	}
	_, _ = writer.Write(source[written:])
}

// Text content owes an escape for three characters: an ampersand opens a
// character reference, and the two angle brackets open a tag. A NUL byte is not
// an escape but a replacement, which is what the HTML syntax requires.
var (
	escapedAmpersand     = []byte("&amp;")
	escapedLess          = []byte("&lt;")
	escapedGreater       = []byte("&gt;")
	replacementCharacter = []byte("\ufffd")
)

// textContentEscape answers the bytes one byte is written as inside HTML text
// content, or nil when the byte stands for itself.
//
// A quotation mark stands for itself here. It is escaped only inside an
// attribute value, which this method never writes.
func textContentEscape(character byte) []byte {
	switch character {
	case 0x00:
		return replacementCharacter
	case '&':
		return escapedAmpersand
	case '<':
		return escapedLess
	case '>':
		return escapedGreater
	}
	return nil
}

// headingSlugs answers the id of one heading, and remembers what it has given
// so a page cannot carry the same id twice.
//
// goldmark's own generator drops a character it does not recognize but still
// writes a separator for the space beside it, and it neither collapses a run of
// separators nor trims one from an end. A heading that opens with punctuation
// therefore gets an id that opens with a hyphen, and "`| display` and `| fill`"
// became "-display-and--fill". The retired python-markdown generator collapsed
// and trimmed, so every such anchor changed spelling in the move to goldmark.
// Collapsing and trimming here restores the published spelling as well as
// reading better.
//
// NOT safe for concurrent use, and it does not need to be: one instance serves
// one page, built beside the parse that uses it.
type headingSlugs struct {
	taken map[string]bool
}

// Generate answers the id for one heading.
//
// The loop is bounded by the length of value, which is the heading text this
// repository's own Markdown carries.
func (h *headingSlugs) Generate(value []byte, kind ast.NodeKind) []byte {
	slug := make([]byte, 0, len(value))
	for _, character := range string(value) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			slug = append(slug, byte(character))
		case character >= 'A' && character <= 'Z':
			slug = append(slug, byte(character)+('a'-'A'))
		case character == ' ' || character == '\t' || character == '\n' || character == '-' || character == '_':
			slug = appendSeparator(slug)
		}
	}
	slug = bytes.TrimRight(slug, "-")
	if len(slug) == 0 {
		slug = []byte("id")
		if kind == ast.KindHeading {
			slug = []byte("heading")
		}
	}
	return h.claim(slug)
}

// appendSeparator writes one hyphen, unless the slug is empty or already ends
// in one. A leading separator is what put a hyphen at the front of an id, and a
// repeated one is what doubled it in the middle.
func appendSeparator(slug []byte) []byte {
	if len(slug) == 0 {
		return slug
	}
	if slug[len(slug)-1] == '-' {
		return slug
	}
	return append(slug, '-')
}

// claim answers slug when this page has not used it, and otherwise the first
// numbered variant that is free.
//
// The separator before the number is an underscore, which is what the retired
// generator wrote and what the published site carries. goldmark's own default
// is a hyphen, and taking it would have moved 37 anchors across 9 pages for no
// reader-visible gain.
//
// The counter is bounded by the number of headings on the page, because each
// turn of the loop is answered by a heading that already took a spelling.
func (h *headingSlugs) claim(slug []byte) []byte {
	if !h.taken[string(slug)] {
		h.taken[string(slug)] = true
		return slug
	}
	for suffix := 1; ; suffix++ {
		numbered := fmt.Sprintf("%s_%d", slug, suffix)
		if h.taken[numbered] {
			continue
		}
		h.taken[numbered] = true
		return []byte(numbered)
	}
}

// Put records an id goldmark generated elsewhere, so Generate cannot hand out
// the same spelling later.
func (h *headingSlugs) Put(value []byte) {
	h.taken[string(value)] = true
}

// docHeading is one entry of a page's table of contents.
type docHeading struct {
	Level int
	ID    string
	Label string
}

// renderMarkdown converts one page source into body HTML and the headings its
// table of contents is built from.
//
// The source is parsed once and rendered from the same tree, so the heading ids
// in the answer are the ids the body carries.
func renderMarkdown(source []byte) (string, []docHeading, error) {
	context := parser.NewContext(parser.WithIDs(&headingSlugs{taken: map[string]bool{}}))
	document := markdownEngine.Parser().Parse(text.NewReader(source), parser.WithContext(context))
	headings := documentHeadings(document, source)
	var body bytes.Buffer
	if err := markdownEngine.Renderer().Render(&body, source, document); err != nil {
		return "", nil, fmt.Errorf("render markdown: %w", err)
	}
	return body.String(), headings, nil
}

// documentHeadings answers every heading of one parsed page, in reading order.
//
// The walk is over a tree this repository's own Markdown produced, so its depth
// is bounded by the nesting of that source rather than by anything a peer
// chooses.
func documentHeadings(document ast.Node, source []byte) []docHeading {
	var headings []docHeading
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, isHeading := node.(*ast.Heading)
		if !isHeading {
			return ast.WalkContinue, nil
		}
		id, hasID := heading.AttributeString("id")
		if !hasID {
			return ast.WalkSkipChildren, nil
		}
		identifier, isBytes := id.([]byte)
		if !isBytes {
			return ast.WalkSkipChildren, nil
		}
		headings = append(headings, docHeading{
			Level: heading.Level,
			ID:    string(identifier),
			Label: inlineText(heading, source),
		})
		return ast.WalkSkipChildren, nil
	})
	return headings
}

// inlineText answers the text a reader sees inside one inline container, with
// every mark dropped. A heading that reads "`ze bgp` state" gives back
// "ze bgp state", which is what the contents list shows.
func inlineText(node ast.Node, source []byte) string {
	var label strings.Builder
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch typed := child.(type) {
		case *ast.Text:
			label.Write(typed.Segment.Value(source))
		case *ast.String:
			label.Write(typed.Value)
		case *ast.RawHTML, *ast.AutoLink:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(label.String())
}

// frontMatterFence opens and closes the metadata block a page source may carry
// before its Markdown body.
const frontMatterFence = "---"

// parseFrontMatter splits one page source into its scalar metadata and its
// Markdown body. A source with no metadata block answers an empty map and the
// source unchanged.
//
// A malformed block is an operating error rather than a programmer error: the
// source is a file an author edits, so a missing colon or a repeated key is a
// mistake a build must name rather than pass over. Every value is a string;
// nothing in this site's front matter is a list or a nested map.
func parseFrontMatter(source []byte) (map[string]string, []byte, error) {
	metadata := map[string]string{}
	text := string(source)
	rest, opened := strings.CutPrefix(text, frontMatterFence+"\n")
	if !opened {
		return metadata, source, nil
	}
	block, body, closed := strings.Cut(rest, "\n"+frontMatterFence+"\n")
	if !closed {
		return nil, nil, fmt.Errorf("front matter opens with %s and never closes", frontMatterFence)
	}
	for offset, line := range strings.Split(block, "\n") {
		number := offset + 2
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, separated := strings.Cut(trimmed, ":")
		if !separated {
			return nil, nil, fmt.Errorf("front matter line %d must be `key: value`", number)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return nil, nil, fmt.Errorf("front matter line %d has an empty key", number)
		}
		if _, repeated := metadata[key]; repeated {
			return nil, nil, fmt.Errorf("duplicate front matter key: %s", key)
		}
		metadata[key] = unquoteFrontMatter(strings.TrimSpace(value))
	}
	return metadata, []byte(body), nil
}

// unquoteFrontMatter drops one matched pair of surrounding quotes, so a value
// that needs a leading space or a colon can be written quoted.
func unquoteFrontMatter(value string) string {
	if len(value) < 2 {
		return value
	}
	quote := value[0]
	if quote != '\'' && quote != '"' {
		return value
	}
	if value[len(value)-1] != quote {
		return value
	}
	return value[1 : len(value)-1]
}

// renderDocTOC builds the "On this page" navigation from a page's headings.
//
// Only level 2 and deeper appear: the level 1 heading is the page title and
// linking a page to its own top adds nothing. A heading with no id or no text
// is dropped, and a page left with no heading gets no navigation rather than an
// empty one.
func renderDocTOC(headings []docHeading) string {
	var listed []docHeading
	for _, heading := range headings {
		if heading.Level >= 2 && heading.ID != "" && heading.Label != "" {
			listed = append(listed, heading)
		}
	}
	if len(listed) == 0 {
		return ""
	}
	items, _ := tocItems(listed, 0, shallowestLevel(listed))
	if items == "" {
		return ""
	}
	return `<nav class="doc-toc" aria-labelledby="doc-toc-title">` +
		`<h2 id="doc-toc-title">On this page</h2>` +
		"<ol>\n" + items + "\n</ol></nav>"
}

// shallowestLevel answers the smallest heading level a page carries, which is
// the level its contents list starts at.
//
// It is the SMALLEST rather than the FIRST heading's level, and the difference
// is a page that opens with a level 3 heading and later carries a level 2 one:
// starting at 3 makes the level 2 heading shallower than the walk, so the walk
// stops there and the page's last section disappears from its own contents.
// docs/features/configuration.md is such a page, with nine level 3 headings
// followed by one level 2.
func shallowestLevel(headings []docHeading) int {
	shallowest := headings[0].Level
	for _, heading := range headings[1:] {
		if heading.Level < shallowest {
			shallowest = heading.Level
		}
	}
	return shallowest
}

// tocItems renders the headings at one level and answers the index it stopped
// at, so the caller can continue from the first heading it did not consume.
//
// The recursion is over heading levels, which run from 2 to 6, so it is five
// frames deep at most and the source that feeds it is a file in this
// repository rather than input a peer chooses.
func tocItems(headings []docHeading, from, level int) (string, int) {
	var items []string
	index := from
	for index < len(headings) {
		heading := headings[index]
		if heading.Level < level {
			break
		}
		if heading.Level > level {
			nested, next := tocItems(headings, index, heading.Level)
			index = next
			if nested == "" {
				continue
			}
			if len(items) == 0 {
				items = append(items, nested)
				continue
			}
			items[len(items)-1] = strings.TrimSuffix(items[len(items)-1], "</li>") +
				"\n<ol>\n" + nested + "\n</ol></li>"
			continue
		}
		index++
		items = append(items, fmt.Sprintf(`<li><a href="#%s">%s</a></li>`,
			html.EscapeString(heading.ID), html.EscapeString(heading.Label)))
	}
	return strings.Join(items, "\n"), index
}

// insertDocTOC splices the navigation after the FIRST literal "</div>" of the
// body, which is where the page's hero block closes.
//
// The rule is literal on purpose. The retired renderer searched for that exact
// string, so a body whose first "</div>" belongs to something else got its
// navigation there, and copying the rule keeps every published page where it
// was. A body with no "</div>" takes the navigation at the top.
func insertDocTOC(body, toc string) string {
	if toc == "" {
		return body
	}
	const marker = "</div>"
	position := strings.Index(body, marker)
	if position < 0 {
		return toc + "\n" + body
	}
	end := position + len(marker)
	return body[:end] + "\n" + toc + body[end:]
}
