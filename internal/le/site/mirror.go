// Design: website/AI.md -- every published page has a Markdown mirror beside it
// Detail: markdown.go renders the HTML this file converts back.
package site

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
)

// blockHTMLLine matches site-layout HTML on a line of its own in a Markdown
// source. A source that carries one cannot be its own mirror: the mirror would
// show a reader the markup instead of the page, so the mirror is converted back
// from the rendered body instead.
var blockHTMLLine = regexp.MustCompile(`(?m)^\s*</?(?:article|details|div|form|section|summary|table|tbody|td|tfoot|th|thead|tr)\b`)

// containsBlockHTML reports whether a Markdown source holds block HTML.
func containsBlockHTML(source string) bool {
	return blockHTMLLine.MatchString(source)
}

// writeMarkdownMirror writes index.md beside a page's index.html.
//
// The site publishes one directory per page, so the mirror is reachable at the
// page's own URL with index.md in place of the trailing slash.
func writeMarkdownMirror(htmlPath, markdown string) error {
	mirror := filepath.Join(filepath.Dir(htmlPath), pageMirrorFile)
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return err
	}
	if err := os.WriteFile(mirror, []byte(markdown), 0o644); err != nil { //nolint:gosec // published web content: a web server, often another account, serves these bytes
		return fmt.Errorf("write %s: %w", mirror, err)
	}
	return nil
}

// mirrorSkipTags name elements a reader of the Markdown never wants: script and
// style are not prose, and the rest are controls or drawing primitives that
// carry no text a mirror should show.
var mirrorSkipTags = map[string]bool{
	"script": true, "style": true, "svg": true, "button": true, "input": true,
	"select": true, "defs": true, "path": true, "textarea": true, "form": true,
	"noscript": true,
}

// mirrorSkipClasses name components whose text is decoration: the dots of a
// terminal frame, the sidebar of related links the mirror's own page already
// links, and the letter index of the FAQ.
var mirrorSkipClasses = map[string]bool{
	"terminal-dots": true, "page-sidebar": true, "faq-index": true,
}

// mirrorVoidTags are the HTML elements that never carry content and never take
// a closing tag.
//
// They are named because skipping is counted by depth: an element that opens
// skip mode and never closes would swallow the rest of the page. A void element
// therefore never opens skip mode and never joins the stack, whether it was
// written "<img ...>" or "<img ... />".
var mirrorVoidTags = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// tagPreformatted is the one element whose text a mirror keeps byte for byte,
// because its indentation is what the block says.
const tagPreformatted = "pre"

// mirrorNode is one open element and the Markdown its children have produced.
type mirrorNode struct {
	tag        string
	attributes map[string]string
	text       strings.Builder
}

// mirrorList is one open list and the number of items it has numbered.
type mirrorList struct {
	ordered bool
	items   int
}

// mirrorConverter turns a rendered page fragment back into Markdown.
//
// It is not a general-purpose converter. It is built against the tags and the
// component classes this site's own producers emit: headings, paragraphs,
// lists, tables, links, code and quotes, plus the status rows, stats, chips and
// cards the hand-authored pages carry. An element it has no case for passes its
// children through, so an unrecognized wrapper costs the mirror nothing.
type mirrorConverter struct {
	base   *url.URL
	stack  []*mirrorNode
	skip   int
	lists  []*mirrorList
	pre    int
	tables [][][]string
	rows   [][]string
}

// htmlToMarkdown converts one rendered page fragment into the Markdown a reader
// gets from the page's index.md. Relative links are resolved against base, so
// the mirror is readable away from the site.
func htmlToMarkdown(fragment, base string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("mirror base URL %q: %w", base, err)
	}
	converter := &mirrorConverter{base: parsed}
	converter.stack = []*mirrorNode{{tag: "root", attributes: map[string]string{}}}
	if err := converter.feed(fragment); err != nil {
		return "", err
	}
	text := converter.stack[0].text.String()
	text = collapseSpacesOutsideCode(text)
	text = mirrorTrailingSpace.ReplaceAllString(text, "\n")
	text = mirrorBlankRun.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text) + "\n", nil
}

// feed reads the fragment one token at a time, the way the retired converter
// did, rather than through a tree parser. A tree parser repairs invalid markup
// by moving elements, and a cell moved out of its table would change what the
// mirror says.
func (converter *mirrorConverter) feed(fragment string) error {
	tokenizer := xhtml.NewTokenizer(strings.NewReader(fragment))
	for {
		switch tokenizer.Next() {
		case xhtml.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return nil
			}
			return fmt.Errorf("read page fragment: %w", tokenizer.Err())
		case xhtml.TextToken:
			converter.data(string(tokenizer.Text()))
		case xhtml.StartTagToken, xhtml.SelfClosingTagToken:
			name, hasAttributes := tokenizer.TagName()
			tag := string(name)
			attributes := map[string]string{}
			for hasAttributes {
				var key, value []byte
				key, value, hasAttributes = tokenizer.TagAttr()
				attributes[string(key)] = string(value)
			}
			converter.start(tag, attributes)
		case xhtml.EndTagToken:
			name, _ := tokenizer.TagName()
			converter.end(string(name))
		case xhtml.CommentToken, xhtml.DoctypeToken:
			// Neither shows a reader anything, so neither reaches the mirror.
		}
	}
}

// mirrorSpaceRun, mirrorTrailingSpace and mirrorBlankRun clean the text a
// pretty-printed page produces: one space for any whitespace run, no space
// before a newline, and one blank line between blocks.
var (
	mirrorSpaceRun      = regexp.MustCompile(`\s+`)
	mirrorTrailingSpace = regexp.MustCompile(`[ \t]+\n`)
	mirrorBlankRun      = regexp.MustCompile(`\n{3,}`)
	mirrorMultiSpace    = regexp.MustCompile(`[ \t]{2,}`)
	mirrorSingleIndent  = regexp.MustCompile("(?m)^ (?:(?:[-*+]|\\d+\\.) |\\*\\*|`)")
	mirrorLabelValue    = regexp.MustCompile(`([^\s:])(\*\*)([ \t]*)`)
)

// collapseSpacesOutsideCode removes the double spaces a pretty-printed page
// leaves at a tag boundary, and never touches a fenced code block, where an
// indented second command must survive as it was written.
func collapseSpacesOutsideCode(text string) string {
	parts := strings.Split(text, "```")
	for index := 0; index < len(parts); index += 2 {
		parts[index] = mirrorMultiSpace.ReplaceAllString(parts[index], " ")
		parts[index] = mirrorSingleIndent.ReplaceAllStringFunc(parts[index], func(match string) string {
			return match[1:]
		})
	}
	return strings.Join(parts, "```")
}

// data records the text of one token. Inside a <pre> it is kept as it was
// written, because the indentation is what the block says.
func (converter *mirrorConverter) data(text string) {
	if converter.skip != 0 {
		return
	}
	if converter.pre != 0 {
		converter.top().text.WriteString(text)
		return
	}
	converter.top().text.WriteString(mirrorSpaceRun.ReplaceAllString(text, " "))
}

// top answers the element currently open.
func (converter *mirrorConverter) top() *mirrorNode {
	return converter.stack[len(converter.stack)-1]
}

// start opens one element.
func (converter *mirrorConverter) start(tag string, attributes map[string]string) {
	void := mirrorVoidTags[tag]
	if converter.skip != 0 {
		// A nested element inside a skipped one is counted rather than named,
		// so an unrelated tag inside a <svg> cannot end the skip early. A void
		// element has no closing tag, so it must not be counted.
		if !void {
			converter.skip++
		}
		return
	}
	if !void && converter.skipsElement(tag, attributes) {
		converter.skip = 1
		return
	}
	switch tag {
	case "br":
		converter.top().text.WriteString("  \n")
		return
	case "hr":
		converter.top().text.WriteString("\n\n---\n\n")
		return
	case "img":
		converter.top().text.WriteString("![" + attributes["alt"] + "](" +
			converter.absolute(attributes["src"]) + ")")
		return
	}
	if void {
		return
	}
	switch tag {
	case "ul":
		converter.lists = append(converter.lists, &mirrorList{})
	case "ol":
		converter.lists = append(converter.lists, &mirrorList{ordered: true})
	case tagPreformatted:
		converter.pre++
	case "table":
		converter.tables = append(converter.tables, nil)
	case "tr":
		converter.rows = append(converter.rows, nil)
	}
	converter.stack = append(converter.stack, &mirrorNode{tag: tag, attributes: attributes})
}

// skipsElement reports whether one element's text is decoration a mirror drops.
func (converter *mirrorConverter) skipsElement(tag string, attributes map[string]string) bool {
	if mirrorSkipTags[tag] {
		return true
	}
	if attributes["aria-hidden"] == "true" {
		return true
	}
	for class := range strings.FieldsSeq(attributes["class"]) {
		if mirrorSkipClasses[class] {
			return true
		}
	}
	return false
}

// end closes one element and appends the Markdown it renders to its parent.
func (converter *mirrorConverter) end(tag string) {
	if converter.skip != 0 {
		converter.skip--
		return
	}
	if mirrorVoidTags[tag] || len(converter.stack) <= 1 {
		return
	}
	node := converter.top()
	converter.stack = converter.stack[:len(converter.stack)-1]
	switch tag {
	case "ul", "ol":
		if len(converter.lists) != 0 {
			converter.lists = converter.lists[:len(converter.lists)-1]
		}
	case tagPreformatted:
		converter.pre--
	}
	switch tag {
	case "td", "th":
		cell := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(node.text.String()), "\n", " "), "|", `\|`)
		if len(converter.rows) != 0 {
			converter.rows[len(converter.rows)-1] = append(converter.rows[len(converter.rows)-1], cell)
		}
		return
	case "tr":
		var row []string
		if len(converter.rows) != 0 {
			row = converter.rows[len(converter.rows)-1]
			converter.rows = converter.rows[:len(converter.rows)-1]
		}
		if len(row) != 0 && len(converter.tables) != 0 {
			converter.tables[len(converter.tables)-1] = append(converter.tables[len(converter.tables)-1], row)
		}
		return
	case "table":
		var rows [][]string
		if len(converter.tables) != 0 {
			rows = converter.tables[len(converter.tables)-1]
			converter.tables = converter.tables[:len(converter.tables)-1]
		}
		converter.top().text.WriteString(renderMarkdownTable(rows))
		return
	case "thead", "tbody", "tfoot":
		return
	}
	converter.top().text.WriteString(converter.renderNode(tag, node))
}

// renderMarkdownTable writes the rows of one table as a Markdown table. The
// first row is the header, and a short row is padded so every line carries the
// same column count.
func renderMarkdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	columns := len(rows[0])
	lines := []string{
		"| " + strings.Join(rows[0], " | ") + " |",
		"| " + strings.Join(repeatString("---", columns), " | ") + " |",
	}
	for _, row := range rows[1:] {
		cells := make([]string, columns)
		copy(cells, row)
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
	}
	return "\n\n" + strings.Join(lines, "\n") + "\n\n"
}

func repeatString(value string, count int) []string {
	out := make([]string, count)
	for index := range out {
		out[index] = value
	}
	return out
}

// absolute answers an href a reader can follow away from the site. A link that
// already names its scheme, and a data URL, are left as they are.
func (converter *mirrorConverter) absolute(href string) string {
	if href == "" || strings.HasPrefix(href, "http://") ||
		strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "data:") {
		return href
	}
	reference, err := url.Parse(href)
	if err != nil {
		return href
	}
	return converter.base.ResolveReference(reference).String()
}

// renderNode answers the Markdown one closed element produces.
//
// The switch is on the tag first and the component class second, which is the
// order the retired converter used and the order a reader checks: a heading is
// a heading whatever class it carries, and a chip is a chip whatever element
// carries it.
func (converter *mirrorConverter) renderNode(tag string, node *mirrorNode) string {
	inner := node.text.String()
	classes := strings.Fields(node.attributes["class"])
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		text := strings.TrimSpace(inner)
		if text == "" {
			return ""
		}
		level, _ := strconv.Atoi(tag[1:])
		return "\n\n" + strings.Repeat("#", level) + " " + text + "\n\n"
	case "p":
		text := strings.TrimSpace(inner)
		if text == "" {
			return ""
		}
		return "\n\n" + text + "\n\n"
	case "a":
		return converter.renderAnchor(node, inner, classes)
	case "strong", "b":
		text := strings.TrimSpace(inner)
		if text == "" {
			return ""
		}
		return "**" + text + "**"
	case "em", "i":
		text := strings.TrimSpace(inner)
		if text == "" {
			return ""
		}
		return "*" + text + "*"
	case "code":
		if converter.pre != 0 {
			return inner
		}
		return "`" + strings.TrimSpace(inner) + "`"
	case tagPreformatted:
		code := strings.Trim(inner, "\n")
		if code == "" {
			return ""
		}
		return "\n\n```\n" + code + "\n```\n\n"
	case "ul", "ol":
		text := strings.Trim(inner, "\n")
		if text == "" {
			return ""
		}
		return "\n\n" + text + "\n\n"
	case "li":
		return converter.renderListItem(inner)
	case "summary":
		text := strings.TrimSpace(inner)
		if text == "" {
			return ""
		}
		return "\n\n**" + text + "**\n\n"
	case "blockquote":
		text := strings.TrimSpace(inner)
		if text == "" {
			return ""
		}
		var quoted []string
		for line := range strings.SplitSeq(text, "\n") {
			quoted = append(quoted, "> "+line)
		}
		return "\n\n" + strings.Join(quoted, "\n") + "\n\n"
	}
	return renderComponent(tag, inner, classes)
}

// renderComponent answers the Markdown for the site's own components, which are
// named by class rather than by tag.
func renderComponent(tag, inner string, classes []string) string {
	text := strings.TrimSpace(inner)
	switch {
	case tag == "div" && hasAnyClass(classes, "status-row", "stat", "quality-meter-row"):
		// A status row writes its label and its value as adjacent elements
		// with no separator, so the two run together as "Label**Value**".
		// One colon after the label separates them.
		if text == "" {
			return ""
		}
		return "- " + mirrorLabelValue.ReplaceAllStringFunc(text, labelValueColon) + "\n"
	case hasAnyClass(classes, "chip", "tag", "card-label", "roadmap-chip"):
		if text == "" {
			return ""
		}
		return "`" + text + "` "
	case hasAnyClass(classes, "quality-stage-number"):
		if text == "" {
			return ""
		}
		return "\n\n**Step " + text + "**\n\n"
	case hasAnyClass(classes, "contribute-label", "contribute-route-kicker", "quality-hero-kicker", "cat"):
		if text == "" {
			return ""
		}
		return "\n\n**" + text + "**\n\n"
	}
	// Every other element is transparent: its children pass through rather
	// than being dropped for want of a case.
	return inner
}

// labelValueColon puts a colon and one space between a label and the bold value
// that follows it.
func labelValueColon(match string) string {
	location := mirrorLabelValue.FindStringSubmatch(match)
	spacing := location[3]
	if spacing == "" {
		spacing = " "
	}
	return location[1] + ":" + location[2] + spacing
}

func hasAnyClass(classes []string, wanted ...string) bool {
	for _, class := range classes {
		if slices.Contains(wanted, class) {
			return true
		}
	}
	return false
}

// renderAnchor answers the Markdown one link produces. A same-page fragment
// keeps its text alone, because a mirror read away from the site cannot follow
// it.
func (converter *mirrorConverter) renderAnchor(node *mirrorNode, inner string, classes []string) string {
	href := node.attributes["href"]
	label := strings.TrimSpace(inner)
	if label == "" {
		label = href
	}
	if href == "" || strings.HasPrefix(href, "#") {
		return label
	}
	target := href
	if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") &&
		!strings.HasPrefix(href, "mailto:") {
		target = converter.absolute(href)
	}
	if hasAnyClass(classes, "card") {
		if heading := cardHeading.FindStringSubmatchIndex(inner); heading != nil {
			marks := inner[heading[2]:heading[3]]
			title := inner[heading[4]:heading[5]]
			return inner[:heading[0]] + marks + " [" + title + "](" + target + ")" + inner[heading[1]:]
		}
		return label + "\n\n[Open page](" + target + ")\n"
	}
	if hasAnyClass(strings.Fields(converter.top().attributes["class"]), "link-list", "contribute-start") {
		return "- [" + label + "](" + target + ")\n"
	}
	return "[" + label + "](" + target + ")"
}

// cardHeading matches the heading inside a card link, which becomes the link
// itself so the card's title is what a reader clicks.
var cardHeading = regexp.MustCompile(`(?m)^(#{1,6}) ([^\n]+)$`)

// renderListItem answers one list item, numbered when its list is ordered. A
// continuation line is indented so it stays inside the item.
func (converter *mirrorConverter) renderListItem(inner string) string {
	marker := "-"
	if len(converter.lists) != 0 {
		list := converter.lists[len(converter.lists)-1]
		if list.ordered {
			list.items++
			marker = strconv.Itoa(list.items) + "."
		}
	}
	text := strings.ReplaceAll(strings.TrimSpace(inner), "\n", "\n  ")
	if text == "" {
		return ""
	}
	return marker + " " + text + "\n"
}

// mainStart matches the opening tag of the one element a page's content lives
// in. A mirror is made from that content alone: the header and the footer are
// on every page and add nothing to a reader who already has the page.
var mainStart = regexp.MustCompile(`<main\b[^>]*\bid="top"[^>]*>`)

// extractMain answers the content of one rendered page, between <main id="top">
// and its closing tag.
func extractMain(page string) (string, error) {
	opening := mainStart.FindStringIndex(page)
	if opening == nil {
		return "", fmt.Errorf(`page carries no <main id="top">`)
	}
	closing := strings.Index(page[opening[1]:], "</main>")
	if closing < 0 {
		return "", fmt.Errorf(`page opens <main id="top"> and never closes it`)
	}
	return page[opening[1] : opening[1]+closing], nil
}
