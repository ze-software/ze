// Design: website/AI.md -- the passes a rendered page body takes before the shell wraps it
// Detail: docs.go runs these in order; markdown.go produces the body they edit.
package site

import (
	"html"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// hrefAttribute matches one href attribute of the rendered body, written with
// either quote. Markdown renders double quotes, and a source that carries raw
// HTML can write either.
var hrefAttribute = regexp.MustCompile(`href=(?:"([^"]*)"|'([^']*)')`)

// markdownLink matches one inline link of a Markdown source.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)

// rewriteDocLinks resolves the relative links of one rendered documentation
// page.
//
// A link to another page the manifest publishes becomes a site link, so a
// reader stays on the site. Every other relative link becomes a link to the
// source on the code host, opened in a new tab, because the target is a file
// this site does not publish. An absolute link, a mail link and an anchor are
// each left as the author wrote them.
//
// docRel is the source's path relative to docs/, which a relative link
// resolves against. destination is this page's own artifact directory, which
// the site link is computed back from.
func rewriteDocLinks(body, docRel string, manifest map[string]string, destination string) string {
	sourceDirectory := path.Dir(docRel)
	return hrefAttribute.ReplaceAllStringFunc(body, func(attribute string) string {
		match := hrefAttribute.FindStringSubmatch(attribute)
		href := match[1] + match[2]
		if keepsItsOwnHref(href) {
			return attribute
		}
		target, fragment, hasFragment := strings.Cut(href, "#")
		if target == "" {
			return attribute
		}
		resolved := path.Join(sourceDirectory, target)
		if strings.HasSuffix(target, "/") {
			return `href="` + codeHostTree + "docs/" + resolved + `" target="_blank" rel="noopener"`
		}
		if directory, published := manifest[resolved]; published {
			relative := relativeRoute(directory, destination) + "/"
			if hasFragment {
				relative += "#" + fragment
			}
			return `href="` + relative + `"`
		}
		away := repositoryBlobURL("docs/" + resolved)
		if hasFragment {
			away += "#" + fragment
		}
		return `href="` + away + `" target="_blank" rel="noopener"`
	})
}

// rewriteDocLinksMarkdown is the Markdown-flavored twin of rewriteDocLinks,
// applied to the SOURCE rather than to the rendered body, so the published
// index.md mirror points at another page's own mirror.
//
// It resolves fewer links than its twin, and deliberately: a Markdown link
// that names neither a directory nor a .md file is left alone, because the
// mirror is read next to the source it came from.
func rewriteDocLinksMarkdown(source, docRel string, manifest map[string]string, destination string) string {
	sourceDirectory := path.Dir(docRel)
	return markdownLink.ReplaceAllStringFunc(source, func(link string) string {
		match := markdownLink.FindStringSubmatch(link)
		label, href := match[1], match[2]
		if keepsItsOwnHref(href) {
			return link
		}
		target, fragment, hasFragment := strings.Cut(href, "#")
		if target == "" {
			return link
		}
		resolved := path.Join(sourceDirectory, target)
		if strings.HasSuffix(target, "/") {
			return "[" + label + "](" + codeHostTree + "docs/" + resolved + ")"
		}
		if !strings.HasSuffix(target, ".md") {
			return link
		}
		if directory, published := manifest[resolved]; published {
			relative := relativeRoute(directory, destination) + "/" + pageMirrorFile
			if hasFragment {
				relative += "#" + fragment
			}
			return "[" + label + "](" + relative + ")"
		}
		away := repositoryBlobURL("docs/" + resolved)
		if hasFragment {
			away += "#" + fragment
		}
		return "[" + label + "](" + away + ")"
	})
}

// keepsItsOwnHref reports whether a link target is already final: it leaves
// the site, opens a mail client, or points inside this page.
func keepsItsOwnHref(href string) bool {
	return strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
		strings.HasPrefix(href, "mailto:") || strings.HasPrefix(href, "#")
}

// relativeRoute answers the path from one published directory to another,
// which is what a site link between two pages spells.
func relativeRoute(target, from string) string {
	fromParts := strings.Split(strings.Trim(from, "/"), "/")
	targetParts := strings.Split(strings.Trim(target, "/"), "/")
	if from == "" || from == "." {
		fromParts = nil
	}
	if target == "" || target == "." {
		targetParts = nil
	}
	shared := 0
	for shared < len(fromParts) && shared < len(targetParts) && fromParts[shared] == targetParts[shared] {
		shared++
	}
	steps := make([]string, 0, len(fromParts)-shared+len(targetParts)-shared)
	for range fromParts[shared:] {
		steps = append(steps, "..")
	}
	steps = append(steps, targetParts[shared:]...)
	if len(steps) == 0 {
		return "."
	}
	return strings.Join(steps, "/")
}

// tableCell matches one data cell of the rendered body, up to its own closing
// tag. The two table passes below each rewrite the cells they recognize and
// leave the rest as they are.
var tableCell = regexp.MustCompile(`(?s)<td([^>]*)>(.*?)</td>`)

// codeSpan matches one inline code element, which is how a comparison table
// spells a source citation.
var codeSpan = regexp.MustCompile(`(?s)<code>(.*?)</code>`)

// The parts of a citation a comparison table's evidence cell carries.
var (
	repositoryPrefix = regexp.MustCompile(`^(?:ze|freeRtr|vyos-1x)/`)
	lineContinuation = regexp.MustCompile(`^:\d+(?:-\d+)?(?:,:?\d+(?:-\d+)?)*$`)
	lineReference    = regexp.MustCompile(`:\d`)
	bareFileName     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.@-]*\.(?:go|py|java|yang|sh|json|mk|proto|opam|md|txt|conf|tst|ftr|csv|sfdsk|yml|yaml|service|xml|in|i|j2|beg|end|dsk|gns|def|rng)$`)
	citationJoiner   = regexp.MustCompile(`^[\s,;]*(?:[A-Za-z][A-Za-z-]{0,9}\s*){0,2}[\s,;]*$`)
)

// citationKind says what one code span of an evidence cell is.
type citationKind int

const (
	// citationInline is prose the reader reads in place: a command, an
	// identifier, anything that is not a source reference.
	citationInline citationKind = iota
	// citationStart opens a citation line of its own.
	citationStart
	// citationJoin continues the citation line already open: a line range, or
	// a sibling file in the same directory.
	citationJoin
)

// classifyCitation answers what one code span of an evidence cell is, from the
// text a reader sees inside it.
func classifyCitation(text string) citationKind {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.ContainsAny(trimmed, " |<()") {
		return citationInline
	}
	core := repositoryPrefix.ReplaceAllString(trimmed, "")
	if lineContinuation.MatchString(core) {
		return citationJoin
	}
	if !strings.Contains(core, "/") && bareFileName.MatchString(core) {
		return citationJoin
	}
	if lineReference.MatchString(core) {
		return citationStart
	}
	if strings.Contains(core, "/") && strings.Contains(core, ".") {
		return citationStart
	}
	if core == "Makefile" || core == "go.mod" {
		return citationStart
	}
	if strings.HasSuffix(core, "/") && strings.Contains(strings.TrimSuffix(core, "/"), "/") {
		return citationStart
	}
	return citationInline
}

// separatesCitations reports whether the text between two code spans is the
// punctuation joining one citation group, rather than prose of its own.
func separatesCitations(text string) bool {
	return !strings.ContainsAny(text, ".()") && citationJoiner.MatchString(text)
}

// proseCleanups tidy the sentence left behind once the citations are lifted
// out of a cell. Each one removes punctuation that its citation used to
// separate, and they run in this order.
var proseCleanups = []struct {
	pattern *regexp.Regexp
	with    string
}{
	{regexp.MustCompile(`\s*[,:;]\s*([.;,)])`), "$1"},
	{regexp.MustCompile(`\(\s*\)`), ""},
	{regexp.MustCompile(`\(\s*[,;:]\s*`), "("},
	{regexp.MustCompile(`[,;:]\s*\)`), ")"},
	{regexp.MustCompile(`\s+([.,;:)])`), "$1"},
	{regexp.MustCompile(`\(\s+`), "("},
	{regexp.MustCompile(`\s{2,}`), " "},
	{regexp.MustCompile(`(?:\.\s*){2,}`), ". "},
	{regexp.MustCompile(`\s+,`), ","},
	{regexp.MustCompile(`,\s*,`), ","},
}

var (
	proseLeadingPunctuation  = regexp.MustCompile(`^[\s,.;:]+`)
	proseTrailingPunctuation = regexp.MustCompile(`[\s:;,]+$`)
	// maskedReference matches the placeholder cleanProse puts in place of a
	// character reference while the punctuation rules run.
	maskedReference = regexp.MustCompile("\x00\\d+\x00")
)

// characterReference matches one HTML character reference, whose own
// semicolon and digits must survive the punctuation cleanups below.
var characterReference = regexp.MustCompile(`&(?:#\d+|#[xX][0-9a-fA-F]+|[A-Za-z][A-Za-z0-9]*);`)

// cleanProse answers the sentence one evidence cell reads once its citations
// have moved to their own lines, ending in a full stop.
//
// Every character reference is masked before the cleanups run and restored
// after. The rules treat a semicolon as punctuation a lifted citation left
// behind, and the semicolon that closes &quot; is not that: rewriting it turns
// a quotation mark into the literal text &quot, which a reader sees. The
// retired renderer had no such hazard because its serializer left a quotation
// mark alone; goldmark writes the reference, so the mask is owed here.
func cleanProse(prose string) string {
	var references []string
	prose = characterReference.ReplaceAllStringFunc(prose, func(reference string) string {
		references = append(references, reference)
		return "\x00" + strconv.Itoa(len(references)-1) + "\x00"
	})
	for _, cleanup := range proseCleanups {
		prose = cleanup.pattern.ReplaceAllString(prose, cleanup.with)
	}
	prose = maskedReference.ReplaceAllStringFunc(prose, func(mask string) string {
		index, err := strconv.Atoi(strings.Trim(mask, "\x00"))
		if err != nil || index >= len(references) {
			return mask
		}
		return references[index]
	})
	prose = proseLeadingPunctuation.ReplaceAllString(prose, "")
	prose = strings.TrimSpace(proseTrailingPunctuation.ReplaceAllString(prose, ""))
	if prose == "" {
		return ""
	}
	if !strings.ContainsRune(".!?:", rune(prose[len(prose)-1])) {
		prose += "."
	}
	return prose
}

// stripRepositoryPrefix drops the leading repository name from one citation,
// so every reference reads uniformly and the page's own script can resolve it
// against its source map, which is keyed by the bare path.
func stripRepositoryPrefix(span string) string {
	match := codeSpan.FindStringSubmatch(span)
	if match == nil {
		return span
	}
	stripped := repositoryPrefix.ReplaceAllString(match[1], "")
	if stripped == match[1] {
		return span
	}
	return "<code>" + stripped + "</code>"
}

// relayoutEvidenceCells lifts the source citations of a comparison table out
// of the prose and onto their own lines beneath it.
//
// A comparison table states a claim and the files that prove it in one cell,
// which a reader has to disentangle. The citations stay <code> elements, so
// the page's own script still resolves each one to its source.
//
// A cell with no citation is left exactly as it was.
func relayoutEvidenceCells(body string) string {
	return tableCell.ReplaceAllStringFunc(body, func(cell string) string {
		match := tableCell.FindStringSubmatch(cell)
		relaid, changed := relayoutEvidenceCell(match[2])
		if !changed {
			return cell
		}
		return "<td" + match[1] + ">" + relaid + "</td>"
	})
}

// relayoutEvidenceCell answers one cell's new content, and whether it carried
// a citation at all.
func relayoutEvidenceCell(cell string) (string, bool) {
	var prose []string
	var groups [][]string
	open := -1
	for _, segment := range splitAroundCodeSpans(cell) {
		match := codeSpan.FindStringSubmatch(segment)
		if len(match) == 0 || match[0] != segment {
			if open >= 0 && separatesCitations(segment) {
				continue
			}
			open = -1
			prose = append(prose, segment)
			continue
		}
		switch classifyCitation(html.UnescapeString(match[1])) {
		case citationStart:
			groups = append(groups, []string{segment})
			open = len(groups) - 1
		case citationJoin:
			if open < 0 {
				prose = append(prose, segment)
				continue
			}
			groups[open] = append(groups[open], segment)
		case citationInline:
			open = -1
			prose = append(prose, segment)
		}
	}
	if len(groups) == 0 {
		return "", false
	}
	var references textbuf.Buffer
	references.Reset()
	for _, group := range groups {
		stripped := make([]string, 0, len(group))
		for _, span := range group {
			stripped = append(stripped, stripRepositoryPrefix(span))
		}
		references.Str(`<span class="ev-ref">`).Str(strings.Join(stripped, ", ")).Str(`</span>`)
	}
	lead := cleanProse(strings.Join(prose, ""))
	if lead != "" {
		lead += " "
	}
	return lead + `<span class="ev-src">` + references.String() + `</span>`, true
}

// splitAroundCodeSpans breaks one cell into its code spans and the text
// between them, keeping both, so a caller can classify each span in place.
func splitAroundCodeSpans(cell string) []string {
	var parts []string
	last := 0
	for _, span := range codeSpan.FindAllStringIndex(cell, -1) {
		parts = append(parts, cell[last:span[0]], cell[span[0]:span[1]])
		last = span[1]
	}
	return append(parts, cell[last:])
}

// markupTag matches one element of a cell, so the cell's own text can be read
// without it.
var markupTag = regexp.MustCompile(`<[^>]+>`)

// cellVerdicts are the four verdicts a comparison table's cell can state, and
// the class that colors each one.
var cellVerdicts = []struct {
	pattern *regexp.Regexp
	class   string
	symbol  string
}{
	{regexp.MustCompile(`(?i)^yes\b`), "cell-yes", "✓"},
	{regexp.MustCompile(`(?i)^no\b`), "cell-no", "✕"},
	{regexp.MustCompile(`(?i)^partial\b`), "cell-partial", "∿"},
	{regexp.MustCompile(`(?i)^n/a$`), "cell-na", ""},
}

// colorCodeCells tags a Yes, No, Partial or N/A cell with the class that
// colors it, and replaces the first three words with their symbol.
//
// The color and the symbol already carry the verdict, so the word only adds
// width to a table a reader scans. N/A keeps its text because it has no
// symbol. A cell that already carries a class was written by a renderer that
// decided its own colors and is left alone.
func colorCodeCells(body string) string {
	return tableCell.ReplaceAllStringFunc(body, func(cell string) string {
		match := tableCell.FindStringSubmatch(cell)
		attributes, inner := match[1], match[2]
		if strings.Contains(attributes, "class=") {
			return cell
		}
		text := strings.TrimSpace(markupTag.ReplaceAllString(inner, ""))
		for _, verdict := range cellVerdicts {
			if !verdict.pattern.MatchString(text) {
				continue
			}
			content := verdict.symbol
			if content == "" {
				content = inner
			}
			return "<td" + attributes + ` class="` + verdict.class + `">` + content + "</td>"
		}
		return cell
	})
}

// The anchor and its attributes, as the external-link pass reads them.
var (
	anchorOpen   = regexp.MustCompile(`(?i)<a\b[^>]*>`)
	anchorHref   = regexp.MustCompile(`(?i)\bhref=(?:"([^"]*)"|'([^']*)')`)
	anchorTarget = regexp.MustCompile(`(?i)\s+target=(?:"[^"]*"|'[^']*')`)
	anchorRel    = regexp.MustCompile(`(?i)\s+rel=(?:"([^"]*)"|'([^']*)')`)
)

// patchExternalLinkTargets opens every link that leaves this site in a new tab
// and states rel="noopener" on it, so the new page cannot reach back into this
// one through window.opener.
func patchExternalLinkTargets(body string) string {
	return anchorOpen.ReplaceAllStringFunc(body, func(tag string) string {
		match := anchorHref.FindStringSubmatch(tag)
		if match == nil {
			return tag
		}
		if !leavesTheSite(match[1] + match[2]) {
			return tag
		}
		tag = setAnchorAttribute(tag, anchorTarget, ` target="_blank"`)
		return ensureAnchorRel(tag, "noopener")
	})
}

// leavesTheSite reports whether one link target reaches another site.
func leavesTheSite(href string) bool {
	if !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
		return false
	}
	return !strings.HasPrefix(href, siteBase)
}

// setAnchorAttribute replaces one attribute of an opening tag, or writes it
// before the closing bracket when the tag carries none.
func setAnchorAttribute(tag string, attribute *regexp.Regexp, replacement string) string {
	if location := attribute.FindStringIndex(tag); location != nil {
		return tag[:location[0]] + replacement + tag[location[1]:]
	}
	return tag[:len(tag)-1] + replacement + ">"
}

// ensureAnchorRel adds one token to a link's rel attribute, keeping whatever
// the author already stated there.
func ensureAnchorRel(tag, token string) string {
	location := anchorRel.FindStringIndex(tag)
	if location == nil {
		return setAnchorAttribute(tag, anchorRel, ` rel="`+token+`"`)
	}
	match := anchorRel.FindStringSubmatch(tag[location[0]:location[1]])
	values := strings.Fields(match[1] + match[2])
	if slices.Contains(values, token) {
		return tag
	}
	values = append(values, token)
	return tag[:location[0]] + ` rel="` + strings.Join(values, " ") + `"` + tag[location[1]:]
}

// The opening of a page body, as the hero pass reads it: an optional run of
// comments, the title, and an optional lead paragraph.
var (
	heroTitleAndLead = regexp.MustCompile(`(?s)^((?:<!--.*?-->\s*)*)<h1([^>]*)>(.*?)</h1>\n((?:<!--.*?-->\s*)*)<p>(.*?)</p>`)
	heroTitleOnly    = regexp.MustCompile(`(?s)^((?:<!--.*?-->\s*)*)<h1([^>]*)>(.*?)</h1>`)
)

// wrapJourneyHero turns the page's opening title, and the paragraph under it,
// into the shared hero block every published page opens with.
//
// A body whose first element is not a heading takes no hero, which is the
// retired renderer's own rule: the hero is the page title's presentation, and
// a page that opens with something else has no title to present.
func wrapJourneyHero(body, label string) string {
	if match := heroTitleAndLead.FindStringSubmatch(body); match != nil {
		hero := pageHero(match[3], match[5], label, match[2], heroClasses)
		return match[1] + match[4] + hero + body[len(match[0]):]
	}
	if match := heroTitleOnly.FindStringSubmatch(body); match != nil {
		hero := pageHero(match[3], "", label, match[2], heroClasses)
		return match[1] + hero + body[len(match[0]):]
	}
	return body
}

// heroClasses is the class list a documentation hero opens with. A section
// that styles its own hero passes its own list instead, and every list starts
// with this one because the shared layout hangs off it.
const heroClasses = "journey-hero reveal"

// pageHero renders the clay title block a page opens with. The title and the
// lead are the renderer's own markup, already rendered from Markdown, so they
// are spliced rather than escaped; the label is plain text.
//
// classes is written out at every call site rather than defaulted, so a call
// states the hero it renders.
func pageHero(title, lead, label, headingAttributes, classes string) string {
	var hero textbuf.Buffer
	hero.Reset().Str(`<div class="`).Str(html.EscapeString(classes)).Str(`">`)
	if label != "" {
		hero.Str("\n    ").Str(`<span class="journey-eyebrow">`).Str(html.EscapeString(label)).Str(`</span>`)
	}
	hero.Str("\n    <h1").Str(headingAttributes).Byte('>').Str(title).Str("</h1>")
	if lead != "" {
		hero.Str("\n    <p>").Str(lead).Str("</p>")
	}
	hero.Str("\n</div>")
	return hero.String()
}

// The eyebrows a page can take that the recovered registry and the rules below
// both spell, so the two cannot drift apart.
const (
	journeyCommunity = "Community"
	journeyUseCase   = "Use case"
	// labelFAQ names the PAGE rather than a journey, because the header's own
	// navigation writes it too, beside the eyebrow rules that do.
	labelFAQ = "FAQ"
)

// sectionFeatures is the site's feature section, named once because the
// destination table and the header navigation each spell it.
const sectionFeatures = "features/"

// journeyAreaLabels name the eyebrow each family of docs/ sources carries.
var journeyAreaLabels = map[string]string{
	"architecture": "Architecture",
	"features":     "Feature",
	"guide":        "Guide",
	"performance":  "Performance",
	"research":     "Research",
}

// journeyKeyLabels name the eyebrow of a standalone page whose key says
// nothing useful on its own.
var journeyKeyLabels = map[string]string{
	"code-of-conduct": journeyCommunity,
	"faq":             labelFAQ,
	"license":         "License",
	"roadmap":         "Release path",
	"security":        "Security",
}

// journeyLabel answers the eyebrow above one page's title.
//
// The registry's own label wins, because it describes the page's public role.
// The source's front matter comes next, because an author who wrote a label
// there meant it. A page the manifest names then takes the label of its source
// family, which is what a reader is looking at. Every other page takes the
// label of its published section, and a section with no label of its own reads
// as its own name.
func journeyLabel(page sitePage, metadata map[string]string) string {
	if page.Journey != "" {
		return page.Journey
	}
	if stated := metadata["journey"]; stated != "" {
		return stated
	}
	if page.DocRel != "" {
		area, _, _ := strings.Cut(page.DocRel, "/")
		area = strings.TrimSuffix(area, ".md")
		if label, named := journeyAreaLabels[area]; named {
			return label
		}
		return "Documentation"
	}
	key := strings.Trim(normalizePageKey(page.Dest), "/")
	switch {
	case strings.HasPrefix(key, "compare/"):
		return "Compare"
	case strings.HasPrefix(key, "contribute/"):
		return journeyCommunity
	case strings.HasPrefix(key, "quality/"):
		return "Quality"
	}
	if label, named := journeyKeyLabels[key]; named {
		return label
	}
	if key == "" {
		return "Ze"
	}
	return titleWords(strings.ReplaceAll(key, "-", " "))
}

// titleWords capitalizes each word of a label the way the retired renderer's
// title case did, so a key becomes the words a reader sees.
func titleWords(text string) string {
	words := strings.Split(text, " ")
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
	}
	return strings.Join(words, " ")
}
