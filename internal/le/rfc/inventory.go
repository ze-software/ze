// Design: docs/architecture/core-design.md -- bounding what a summary MISSED
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// inventory.go walks an RFC's OWN text and answers every normative sentence in
// it, located as `<section>:<n>`.
//
// It exists because every other check in this gate judges the requirements a
// summary LISTS, and none of them can see an obligation nobody wrote down. A
// green gate is bounded by what was extracted, so this half bounds the MISS.
//
// Only DISPOSITIONS are ever authored. Sites, sections, quotes, the register
// and every published count are derived here at check time, so an unclassified
// site cannot be hidden and a hand-typed "seen" count cannot exist.
package rfc

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The three registers a source can be written in, strongest first. A sign-off
// may declare the DERIVED register or a WEAKER one; a stronger claim than the
// source supports is refused.
const (
	registerRFC2119    = "rfc2119"
	registerProse      = "prose"
	registerManualWalk = "manual-walk"
)

// Registers answers them strongest first, which is the order the envelope's
// split is published in.
func Registers() []string {
	return []string{registerRFC2119, registerProse, registerManualWalk}
}

// registerStrength ranks a claim against what the source supports.
var registerStrength = map[string]int{
	registerRFC2119: 3, registerProse: 2, registerManualWalk: 1,
}

// frontSection is the section a site is attributed to when it precedes the
// first numbered heading. A site must never be DROPPED for living in the
// preamble: that would be a silent hole in the very bound this exists to give.
const frontSection = "front"

// The site scans. The capitalised set matches the gated levels, and the prose
// set is the same words case-insensitively -- a strict superset, which is why
// an RFC with capitalised keywords can never derive an EMPTY prose inventory.
var (
	siteKeywordRE = regexp.MustCompile(`\b(?:MUST NOT|MUST|SHALL NOT|SHALL|REQUIRED)\b`)
	siteProseRE   = regexp.MustCompile(`(?i)\b(?:must not|must|shall not|shall|required)\b`)
)

// boilerplateRE matches the RFC 2119 / RFC 8174 key-words paragraph, which is
// not an obligation on a speaker: it is the document saying how to read its
// other sentences.
//
// It excludes MORE than that paragraph, and the name says less than the pattern
// does. Both alternatives also match a REFERENCE-LIST entry citing either RFC
// by title, because the entry carries "Key words ..." within 600 characters of
// "BCP 14". Not one of those entries carries a MUST-level keyword, so nothing
// is lost, and a scope documented narrower than the code is how the next reader
// mis-reasons about it.
var boilerplateRE = regexp.MustCompile(reBoilerplate())

func reBoilerplate() string {
	var tb textbuf.Buffer
	return tb.Str(`(?is)key\s+words.{0,600}?(?:interpreted|RFC\s*2119|BCP\s*14)`).
		Str(`|interpreted\s+as\s+described\s+in\s+\[?(?:RFC\s*2119|BCP\s*14)`).String()
}

// sectionHeadingRE matches a heading at column 0. The numeric form tolerates a
// missing dot; the alpha form REQUIRES it, because "A speaker MUST ..." at
// column 0 would otherwise read as appendix A. The alpha form takes a number
// after the letter with no dot between ("A2.  IPv6 Extension Headers", RFC
// 4302), and under the word Appendix, in either case, the letter may end in a
// colon ("Appendix D:  Configuration Parameters", RFC 3101). Only under the
// word, because a bare "S: 250 OK" is a line of a transcript. An annex
// subsection may also omit the trailing dot when a dotted number follows the
// letter ("B.1 Level 1 Complete Sequence Numbers PDU", RFC 1195): the ".1"
// already separates it from a sentence opening with "A".
//
// Groups: 1 the appendix letter written under the word, 2 the number, 3 the
// dot after the number, 4 the letter with its trailing dot, 5 the letter with
// a dotted number and no trailing dot, 6 the title.
//
// It OVER-MATCHES, deliberately and unavoidably: RFCs put column-0 attribute
// tables, packet diagrams and tables of contents in the same text stream, and
// no pattern can separate "3.1  Route Selection" from a table row numbered 3.1
// by shape alone. The derivation is built to SURVIVE a false match rather than
// to prevent one -- see sectionBodies. The one false match it refuses is the
// undotted number out of sequence, see columnZeroHeadings.
var sectionHeadingRE = regexp.MustCompile(
	`^(?:(?:Appendix|APPENDIX)[ \t]+([A-Z]\d*(?:\.\d+)*)[.:]|(?:(?:Appendix|APPENDIX)[ \t]+)?(\d+(?:\.\d+)*)(\.?)|([A-Z]\d*(?:\.\d+)*)\.|([A-Z](?:\.\d+)+))[ \t]+(\S.*)$`)

// indentedHeadingRE matches a heading once the body margin is cut off its line,
// in a text where sectionHeadingRE finds nothing (owner decision D-5,
// 2026-09-26). RFC 905 is the case: its clauses are numbered at the same
// five-space margin as its prose. Because a heading there shares the margin
// with every other line, the shape is narrower than the column-0 one: a number
// with no trailing dot, because "1.  When ..." is how such a text numbers the
// notes inside a clause; two blanks, because a justified line wrapped at a
// number has one; and a title that starts with a letter, which a diagram ruler
// does not. An annex is "ANNEX B - TITLE", and its clauses are "B.1  TITLE".
// A clause with no title ("1.3" alone on its line) is dotted, so a bare page
// number is not read as one.
var indentedHeadingRE = regexp.MustCompile(
	`^(?:(?:ANNEX|Annex|Appendix)[ \t]+([A-Z])\b[ \t]*(.*)|(\d+(?:\.\d+)*|[A-Z](?:\.\d+)+)[ \t]{2,}([A-Za-z].*)|(\d+(?:\.\d+)+)[ \t]*)$`)

// tocLeaderRE matches a table-of-contents line: dot leaders then a page number,
// arabic or roman. indentedHeadings reads no heading in a paragraph holding one.
var tocLeaderRE = regexp.MustCompile(`\.{3,}[ \t]*\S+[ \t]*$`)

// pageFooterRE matches the "[Page N]" furniture that would otherwise land
// inside any quote whose sentence crosses a page boundary.
var pageFooterRE = regexp.MustCompile(`\[Page\s+\d+\]\s*$`)

// Site is one normative sentence of an RFC's own text.
type Site struct {
	ID      string `json:"id"` // "<section>:<n>"
	Quote   string `json:"quote"`
	Section string `json:"section"`
}

// SectionEntry is one section of the source and how many sites it holds.
type SectionEntry struct {
	ID    string `json:"id"`
	Sites int    `json:"sites"`
}

// Inventory is the whole derived walk of one RFC's text.
type Inventory struct {
	Stem       string         `json:"stem"`
	Register   string         `json:"register"`
	SourcePath string         `json:"source-path"`
	SourceSHA  string         `json:"source-sha"`
	Sections   []SectionEntry `json:"sections"`
	Sites      []Site         `json:"sites"`
	// KeywordSites is what the capitalised scan alone would have found.
	KeywordSites int `json:"keyword-sites"`
}

// normalize strips every line and drops the blank ones, which is what makes a
// fingerprint survive a reflow of the source.
func normalize(src string) string {
	lines := strings.Split(src, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "\n")
}

// RequirementSHA answers the fingerprint this gate records for a text: the
// first shaHexLen hex characters of the normalized text's SHA-256.
func RequirementSHA(text string) string {
	sum := sha256.Sum256([]byte(normalize(text)))
	return hex.EncodeToString(sum[:])[:shaHexLen]
}

// SourcePath answers the tree-relative path of an RFC's own text, and false
// when this repository holds none.
//
// The SAME two locations, in the same order, that every other reader of the
// source searches. One lookup, so a source one reader can see and another
// cannot is impossible by construction.
func SourcePath(tree, stem string) (string, bool) {
	for _, sub := range []string{fullRel, draftsRel} {
		var tb textbuf.Buffer
		rel := tb.Str(sub).Byte('/').Str(stem).Str(".txt").String()
		if _, err := os.Stat(treePath(tree, rel)); err == nil {
			return rel, true
		}
	}
	return "", false
}

// SourceText answers an RFC's own text, and false when it is absent or cannot
// be read. Invalid UTF-8 is replaced rather than refused, which is what the
// corpus needs: several RFCs carry stray bytes in their diagrams.
func SourceText(tree, stem string) (string, bool) {
	rel, ok := SourcePath(tree, stem)
	if !ok {
		return "", false
	}
	raw, err := os.ReadFile(treePath(tree, rel))
	if err != nil {
		return "", false
	}
	return replaceInvalidUTF8(string(raw)), true
}

// replaceInvalidUTF8 is Python's errors="replace" on a decode: every byte that
// is not valid UTF-8 becomes U+FFFD. Go hands back the raw bytes instead, and
// the difference would reach the derived quote and therefore the source
// fingerprint.
//
// Per BYTE, not per run, because that is what CPython's decoder does for an
// isolated invalid byte. The two differ on a TRUNCATED multi-byte sequence,
// where CPython emits one replacement and this emits one per byte. Nothing in
// rfc/full or rfc/drafts is invalid UTF-8 on 2026-08-26 (measured over all 178
// files), so the difference is unreachable today and the cheaper reading is
// the honest one to state.
func replaceInvalidUTF8(src string) string {
	if utf8.ValidString(src) {
		return src
	}
	var tb textbuf.Buffer
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRuneInString(src[i:])
		if r == utf8.RuneError && size == 1 {
			tb.Str("\uFFFD")
			i++
			continue
		}
		tb.Str(src[i : i+size])
		i += size
	}
	return tb.String()
}

// stripPageFurniture removes the whole page break: the blank run before the
// "[Page N]" footer, the footer, the form feed, the running header, and the
// blank run after it. The running header is every line up to the first blank
// one, because some RFCs write it on two lines (RFC 792: "September 1981",
// then "RFC 792").
//
// Removing only the three furniture LINES is not enough. RFCs break pages
// mid-sentence, and the blank lines bracketing the break would still read as a
// paragraph boundary, truncating the quote at "A speaker MUST do the first" and
// losing the rest of the obligation.
//
// The cost, stated rather than hidden: when a page happens to break BETWEEN
// paragraphs, those two paragraphs are joined. The sentence splitter still
// separates them at the punctuation between them; only a paragraph ending
// without terminal punctuation merges with its successor.
func stripPageFurniture(text string) string {
	var out []string
	// "" is ordinary text; "header" is inside the break, still owed the running
	// header; "inHeader" is reading the header's lines; "blanks" is header
	// consumed, still swallowing the blank run.
	state := ""
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.ReplaceAll(raw, "\f", "")
		if pageFooterRE.MatchString(line) {
			for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
				out = out[:len(out)-1]
			}
			state = "header"
			continue
		}
		if state != "" {
			if strings.TrimSpace(line) == "" {
				if state == "inHeader" {
					state = "blanks"
				}
				continue // the form-feed line, and the blanks around the header
			}
			if state != "blanks" {
				state = "inHeader"
				continue
			}
			state = "" // first real line of the new page: the text resumes here
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// sectionBody is one section of the source, in first-appearance order.
type sectionBody struct {
	id   string
	body string
}

// sectionBodies answers every section, with a leading front entry. It is the
// one heading derivation: the site inventory (sitesFor) and the row quote check
// (newQuoteSource) both cut the text here.
//
// The reading is chosen once for the whole text. A text with any column-0
// heading is read at column 0 only, so its sections and site ids are the ones
// it always had. A text with none is read at its body margin (indentedHeadings,
// owner decision D-5).
//
// Every id appears EXACTLY ONCE and every input line lands in exactly one body.
// Both are load-bearing, because the heading pattern over-matches:
//
// The heading's own TITLE stays in its section's body. Dropping the matched
// line drops whatever it said, and for a false match that is a live obligation
// deleted from the inventory without a word.
//
// A repeated id EXTENDS the section it already opened rather than starting a
// second one. Two entries sharing an id emit a duplicate row the artifact
// parser refuses, and each body would restart the per-section site counter at
// 1, so both would produce a site "7:1" and one would silently disappear.
func sectionBodies(text string) []sectionBody {
	lines := strings.Split(text, "\n")
	heading := columnZeroHeadings()
	if !hasColumnZeroHeading(lines) {
		heading = indentedHeadings(lines)
	}
	order := []string{frontSection}
	bodies := map[string][]string{frontSection: {}}
	current := frontSection
	for at, line := range lines {
		id, title, found := heading(lines, at)
		if !found {
			bodies[current] = append(bodies[current], line)
			continue
		}
		current = id
		if _, seen := bodies[current]; !seen {
			order = append(order, current)
			bodies[current] = []string{}
		} else if len(bodies[current]) > 0 {
			// A blank line, so the resumed run is a fresh paragraph rather
			// than a continuation of a sentence written elsewhere.
			bodies[current] = append(bodies[current], "")
		}
		bodies[current] = append(bodies[current], title, "")
	}
	out := make([]sectionBody, 0, len(order))
	for _, id := range order {
		out = append(out, sectionBody{id: id, body: strings.Join(bodies[id], "\n")})
	}
	return out
}

// headingReader answers whether lines[at] opens a section, and its id and
// title when it does.
type headingReader func(lines []string, at int) (string, string, bool)

// hasColumnZeroHeading answers whether any line is a column-0 heading. One
// such line decides the reading of the whole text, so a text that has always
// been cut at column 0 keeps every site id it had.
func hasColumnZeroHeading(lines []string) bool {
	heading := columnZeroHeadings()
	for at := range lines {
		if _, _, found := heading(lines, at); found {
			return true
		}
	}
	return false
}

// columnZeroHeadings answers the reader for a text cut at column 0: a line
// sectionHeadingRE matches, unless it is an undotted number that does not
// number the next section. The reader is stateful and reads the lines in
// order, once; not safe for concurrent use.
//
// The exception is what separates a heading from a table row or a byte dump
// that opens with a number at column 0. The RFC 3579 and RFC 2869 attribute
// tables open each row "0" or "1" ("1        1       1       1           80
// Message-Authenticator"), and the RFC 2759 hash example opens each dump with
// a byte ("55 73 65 72") and one label with a count ("24 octet
// NT-Response:"). Read as headings, they end the section they sit in and file
// its notes under a section "0", or back under section "1", so a verbatim
// quote from them is refused as outside the section the RFC puts it in.
//
// A heading written as a bare number opens the section after the highest one
// opened so far, and such a row repeats a number, goes back, or skips ahead,
// so an undotted number is a heading only when it is that next number. Zero is
// never one. A dotted number keeps the old reading: every such row and dump
// in the corpus opens with an undotted number (measured 2026-09-27), and a
// dotted heading can skip a number ("10.  Full Copyright Statement" after
// section 7, RFC 2548).
func columnZeroHeadings() headingReader {
	highest := 0
	return func(lines []string, at int) (string, string, bool) {
		found := sectionHeadingRE.FindStringSubmatch(lines[at])
		if found == nil {
			return "", "", false
		}
		for _, letter := range []string{found[1], found[4], found[5]} {
			if letter != "" {
				return letter, found[6], true
			}
		}
		id := found[2]
		top, _, dotted := strings.Cut(id, ".")
		number, err := strconv.Atoi(top)
		if err != nil {
			return "", "", false // A run of digits no int holds is a value, not a section number.
		}
		undotted := !dotted && found[3] == ""
		if undotted && number != highest+1 {
			return "", "", false
		}
		highest = max(highest, number)
		return id, found[6], true
	}
}

// indentedHeadings answers the reader for a text with no column-0 heading. A
// heading there is a line at the body margin that indentedHeadingRE matches,
// with two more conditions, because at the margin a heading has the shape of
// other lines:
//
//   - It begins a paragraph. A justified line inside a paragraph can start
//     with a clause number and two blanks ("12.2.3.8.4  are  optional ...",
//     RFC 905), and read as a heading it would cut the paragraph in two.
//   - Its paragraph holds no table-of-contents line, one ending in dot leaders
//     and a page number. Read as headings, the contents would open every
//     section in contents order and hand the introduction that follows them
//     to the last entry. The paragraph, not the line, because a long entry
//     wraps and only its last line carries the leaders.
func indentedHeadings(lines []string) headingReader {
	margin := bodyMargin(lines)
	contents := make([]bool, len(lines))
	for first := 0; first < len(lines); {
		last := first
		for last < len(lines) && strings.TrimSpace(lines[last]) != "" {
			last++
		}
		listed := false
		for at := first; at < last; at++ {
			if tocLeaderRE.MatchString(lines[at]) {
				listed = true
			}
		}
		for at := first; at < last; at++ {
			contents[at] = listed
		}
		first = last + 1
	}
	return func(lines []string, at int) (string, string, bool) {
		if contents[at] {
			return "", "", false
		}
		if at > 0 && strings.TrimSpace(lines[at-1]) != "" {
			return "", "", false
		}
		return marginHeading(lines[at], margin)
	}
}

// marginHeading reads line as a heading written at the body margin, answering
// its id and title. A line indented more or less than the margin is body text,
// which keeps a label in a diagram or a table out of the section list.
func marginHeading(line string, margin int) (string, string, bool) {
	if len(line) <= margin {
		return "", "", false
	}
	if strings.TrimLeft(line[:margin], " ") != "" {
		return "", "", false
	}
	if line[margin] == ' ' {
		return "", "", false
	}
	found := indentedHeadingRE.FindStringSubmatch(line[margin:])
	if found == nil {
		return "", "", false
	}
	if found[1] != "" {
		return found[1], found[2], true
	}
	if found[3] != "" {
		return found[3], found[4], true
	}
	return found[5], "", true
}

// bodyMargin answers the indentation, in leading spaces, that more non-blank
// lines share than any other, the smaller on a tie. In RFC 905 that is the
// five spaces its prose and its headings are both written at.
func bodyMargin(lines []string) int {
	counts := map[int]int{}
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		counts[len(line)-len(trimmed)]++
	}
	margin, most := 0, 0
	for indent, count := range counts {
		if count > most || (count == most && indent < margin) {
			margin, most = indent, count
		}
	}
	return margin
}

// quoteHaystack is the ONE text a quote is matched against: the source with its
// page furniture stripped, its whitespace collapsed and its wrapped hyphens
// joined (joinWrappedHyphens). Every quote is held against it in the form
// quoteNeedle gives, so the two compare in one form. Three readers hold a
// quote against an RFC, a requirement row (checkRowQuotes), a {feature-declined}
// annotation (featureDeclinedQuote) and a level correction
// (correctionAuthorizes), and all three call this, so a sentence one of them
// finds the others find too.
//
// The strip is load-bearing: 163 of 6243 keyword sentences in the corpus cross
// a page break (measured 2026-09-26), and a raw collapse leaves the footer and
// the running header inside each of them.
func quoteHaystack(source string) string {
	return joinWrappedHyphens(squashWhitespace(stripPageFurniture(source)))
}

// quoteNeedle answers the form a quote is searched for in a quoteHaystack: its
// whitespace collapsed and its wrapped hyphens joined, the two steps the
// haystack takes after its page furniture goes.
func quoteNeedle(quote string) string {
	return joinWrappedHyphens(squashWhitespace(quote))
}

// joinWrappedHyphens drops the blank after a hyphen that ends a word. An RFC
// wraps a hyphenated word at its hyphen ("close-" at the end of one line,
// "notify" on the next), and the whitespace collapse reads that as
// "close- notify". A row copying the word writes "close-notify", and a row
// copying the collapsed text writes "close- notify". Joined on both sides,
// either spelling matches, and a quote that matched before still does.
//
// A hyphen with a blank on both sides (" - ") is a dash, not a wrapped word,
// and stays as it is.
func joinWrappedHyphens(text string) string {
	if !strings.Contains(text, "- ") {
		return text
	}
	var joined strings.Builder
	joined.Grow(len(text))
	for at := 0; at < len(text); at++ {
		joined.WriteByte(text[at])
		if endsWrappedWord(text, at) {
			at++ // The blank after the hyphen.
		}
	}
	return joined.String()
}

// endsWrappedWord answers whether text[at] is a hyphen that ends a word and is
// followed by a blank: the shape a wrapped hyphenated word collapses to.
func endsWrappedWord(text string, at int) bool {
	if text[at] != '-' {
		return false
	}
	if at == 0 {
		return false
	}
	if at+1 == len(text) {
		return false
	}
	return text[at-1] != ' ' && text[at+1] == ' '
}

// quoteSource is one RFC's text cut into sections, each body already a quote
// haystack, for the row check that scopes a quote to the section it cites.
// Safe for concurrent use once built: nothing writes to it after newQuoteSource
// and withErrata.
type quoteSource struct {
	sections []sectionBody
	// readErratum answers the stored text of one of this RFC's errata by
	// number, false when the store holds none (errata.go). Nil when no store
	// was given, and a row citing an erratum is then refused.
	readErratum func(number string) (string, bool)
}

// wholeText answers whether the RFC has no heading under either reading
// sectionBodies makes, so its front matter is its whole text. Owner decision D-1 (2026-09-26): such a
// text is one citable section, because a row has no numbered section to cite.
// RFC 792, RFC 1997 and the TFTP option RFCs are written this way.
func (q *quoteSource) wholeText() bool {
	return len(q.sections) == 1
}

// newQuoteSource cuts the RAW source into sections and builds each body's
// haystack by quoteHaystack. Cutting first is safe because no page footer or
// running header is read as a heading, and a page break landing between two
// sections leaves its furniture in the earlier body, where quoteHaystack strips
// it.
func newQuoteSource(source string) *quoteSource {
	bodies := sectionBodies(source)
	for i := range bodies {
		bodies[i].body = quoteHaystack(bodies[i].body)
	}
	return &quoteSource{sections: bodies}
}

// resolve answers the heading a cited section anchors to: the cited id when
// the RFC has a heading of that id, else its nearest heading ancestor ("3.b"
// resolves to "3"). The second result is false when no heading answers, and the
// caller MUST refuse the anchor: a whole-document fallback would let a quote
// pass under a section that does not exist.
//
// A text with no heading at all is the one exception, and it is not a
// fallback: every citation resolves to its single section, the front matter,
// because there is no section for the citation to be wrong about. A text with
// one heading or more never reaches this branch, so its front matter stays
// uncitable.
func (q *quoteSource) resolve(cited string) (string, bool) {
	if q.wholeText() {
		return frontSection, true
	}
	if cited == noSection {
		return "", false
	}
	for id := cited; ; {
		if q.has(id) {
			return id, true
		}
		cut := strings.LastIndexByte(id, '.')
		if cut < 0 {
			return "", false
		}
		id = id[:cut]
	}
}

// has answers whether a section of this id exists. The front matter of a text
// with headings is not a section a row can cite, so it never answers; resolve
// handles the text that has no heading before asking.
func (q *quoteSource) has(id string) bool {
	if id == frontSection {
		return false
	}
	for _, section := range q.sections {
		if section.id == id {
			return true
		}
	}
	return false
}

// inSection answers whether quote is one contiguous span of the section id or
// of one of its subsections, compared in quoteNeedle's form. Each body is
// searched on its own, because a span joining the end of one section to the
// start of the next is no sentence of the RFC.
func (q *quoteSource) inSection(id, quote string) bool {
	quote = quoteNeedle(quote)
	prefix := id + "."
	for _, section := range q.sections {
		if section.id != id && !strings.HasPrefix(section.id, prefix) {
			continue
		}
		if strings.Contains(section.body, quote) {
			return true
		}
	}
	return false
}

// sectionOf answers the first section whose body carries quote, so a refusal
// can name where the sentence really is. False when no section carries it.
func (q *quoteSource) sectionOf(quote string) (string, bool) {
	quote = quoteNeedle(quote)
	for _, section := range q.sections {
		if strings.Contains(section.body, quote) {
			return section.id, true
		}
	}
	return "", false
}

// boilerplateEnd answers the offset one past the first terminator at or after
// start: end punctuation with whitespace after it.
//
// The lookahead is what tells "RFC 2119. 6PE routers ..." from the dots inside
// "DOI 10.17487/RFC2119" and "www.rfc-editor.org", which every RFC 2119
// reference entry carries. Cutting on a bare terminator would shear them in two.
func boilerplateEnd(text string, start int) (int, bool) {
	for i := start; i < len(text); i++ {
		if text[i] != '.' && text[i] != '!' && text[i] != '?' {
			continue
		}
		if i+1 < len(text) && isASCIISpace(text[i+1]) {
			return i + 1, true
		}
	}
	return 0, false
}

func isASCIISpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

// splitOffBoilerplate cuts the key-words paragraph away from whatever the
// sentence splitter fused onto it.
//
// Both readers of a sentence drop it whole when the boilerplate pattern
// matches, so the splitter decides how much the exclusion takes. The splitter
// cannot cut before a digit or a lowercase letter, which leaves "... as
// described in RFC 2119. 6PE routers MUST support X." as ONE sentence.
// Excluding the key-words paragraph then takes the obligation with it, and it
// leaves no trace: the gate reads an RFC that asks for nothing.
//
// A chunk with NO terminator is cut at the end of the boilerplate match when
// its tail still carries a MUST-level keyword. Leaving it whole was an
// under-count, and an under-count is the one direction that cannot be seen
// downstream: the caller drops a boilerplate-matching sentence entire, so the
// obligation left inside it never becomes a site, and the RFC reads as asking
// for nothing. Over-counting is visible to a reviewer, who deletes a row;
// under-counting is silent, and the gate cannot ask for evidence it never knew
// was owed.
//
// The tail is required to carry a keyword before the cut is taken, so a chunk
// that is only boilerplate is still dropped whole and no listing is promoted to
// an obligation.
//
// WHAT IT DOES NOT REACH. An obligation BEFORE the key-words paragraph is never
// cut, because the paragraph's own keyword listing sits to the left of an
// "interpreted as described in" match and a left-hand cut would promote that
// listing to an obligation.
func splitOffBoilerplate(sentence string) []string {
	var out []string
	rest := sentence
	for {
		loc := boilerplateRE.FindStringIndex(rest)
		if loc == nil {
			break
		}
		end, ok := boilerplateEnd(rest, loc[1])
		if !ok {
			// No terminator, so the sentence splitter fused the key-words paragraph
			// to whatever follows. Cut at the end of the boilerplate match itself
			// when the tail still states an obligation, so the obligation survives
			// the exclusion the caller is about to apply.
			if tail := strings.TrimSpace(rest[loc[1]:]); siteKeywordRE.MatchString(tail) {
				if head := strings.TrimSpace(rest[:loc[1]]); head != "" {
					out = append(out, head)
				}
				rest = tail
				continue
			}
			break
		}
		// The cut can land inside a citation, and the quote a reviewer reads is
		// what suffers. The keyword survives, so the direction is an over-count
		// and never a missed obligation.
		head := strings.TrimSpace(rest[:end])
		tail := strings.TrimSpace(rest[end:])
		if head == "" || tail == "" {
			break
		}
		out = append(out, head)
		rest = tail
	}
	out = append(out, strings.TrimSpace(rest))
	kept := out[:0]
	for _, one := range out {
		if one != "" {
			kept = append(kept, one)
		}
	}
	return kept
}

// splitSentences cuts a whitespace-collapsed paragraph at every boundary the
// Python pattern names: end punctuation, whitespace, then something that starts
// a sentence.
//
// Demanding the follower rules out "e.g. the" and "Fig. 3" without an
// abbreviation list. What it costs is a sentence opening on a digit or a
// lowercase letter, which stays fused to the one before it -- which is what
// splitOffBoilerplate exists to cover.
func splitSentences(flat string) []string {
	var out []string
	start := 0
	for i := 1; i < len(flat); i++ {
		if !isASCIISpace(flat[i]) {
			continue
		}
		prev := flat[i-1]
		if prev != '.' && prev != '!' && prev != '?' {
			continue
		}
		end := i
		for end < len(flat) && isASCIISpace(flat[end]) {
			end++
		}
		if end >= len(flat) || !startsASentence(flat[end]) {
			continue
		}
		out = append(out, flat[start:i])
		start = end
		i = end
	}
	return append(out, flat[start:])
}

func startsASentence(c byte) bool {
	return (c >= 'A' && c <= 'Z') || c == '"' || c == '(' || c == '['
}

// sentences answers every sentence of a section body, paragraph by paragraph,
// whitespace collapsed.
//
// Paragraph-at-a-time so a sentence never runs across a blank line, and
// whitespace collapsed so the derived quote is stable no matter how the source
// wrapped it.
func sentences(body string) []string {
	var out []string
	for _, para := range paragraphRE.Split(body, -1) {
		flat := strings.Join(strings.Fields(para), " ")
		if flat == "" {
			continue
		}
		for _, chunk := range splitSentences(flat) {
			out = append(out, splitOffBoilerplate(chunk)...)
		}
	}
	return out
}

var paragraphRE = regexp.MustCompile(`\n\s*\n`)

// sitesFor answers every normative sentence, located as <section>:<n> in
// document order.
func sitesFor(text string, pattern *regexp.Regexp) []Site {
	var out []Site
	for _, section := range sectionBodies(text) {
		n := 0
		for _, sentence := range sentences(section.body) {
			if !pattern.MatchString(sentence) {
				continue
			}
			if boilerplateRE.MatchString(sentence) {
				continue
			}
			n++
			var tb textbuf.Buffer
			out = append(out, Site{
				ID:      tb.Str(section.id).Byte(':').Int(int64(n)).String(),
				Quote:   sentence,
				Section: section.id,
			})
		}
	}
	return out
}

// DeriveRegister answers which keyword register the SOURCE is written in, and
// therefore what a sign-off can be graded against.
//
// sourced is the gated requirements a keyword site must back, less the ids the
// extraction sanctions as unsourced (sourcedGatedCounts). An id the walk admits
// no capitalised sentence states never consumes the room a sourced one needs.
//
// Derived from the text, never authored: the RFCs that would most benefit from
// claiming the strong grade are exactly the ones whose source cannot support it.
func DeriveRegister(keywordSites, proseSites, sourced int) string {
	if keywordSites > 0 && keywordSites >= sourced {
		return registerRFC2119
	}
	if proseSites > 0 {
		return registerProse
	}
	return registerManualWalk
}

// inventoryKey is every input the derivation reads.
//
// The RAW bytes, never the normalized fingerprint. Normalizing strips each line
// and drops the blank ones, and the derivation depends on exactly those two
// things: the heading pattern anchors at the line start, so leading whitespace
// decides whether a line is a heading at all, and paragraphs split on blank
// lines. Two bodies sharing one normalized digest derive different section
// sets.
//
// The path is in the key because it is not a function of the bytes: the same
// text found under rfc/full and under rfc/drafts is two different sources.
type inventoryKey struct {
	stem    string
	sourced int
	signed  string
	raw     string
	path    string
}

// Deriver answers inventories for one checkout, remembering what it has already
// walked.
//
// The memo is here rather than in a package variable because a run derives the
// inventory of every signed stem several times -- the shared signed set, the
// violations, and the ledger render -- at about 8.5ms mean and 90ms worst per
// RFC. A fully drained corpus would otherwise add seconds to every run, and a
// gate that doubles verify time is one people learn to skip.
type Deriver struct {
	tree string
	memo map[inventoryKey]*Inventory
}

// NewDeriver answers a deriver for one checkout.
func NewDeriver(tree string) *Deriver {
	return &Deriver{tree: tree, memo: map[inventoryKey]*Inventory{}}
}

// Tree answers the checkout this deriver reads.
func (d *Deriver) Tree() string { return d.tree }

// Inventory answers the full derived inventory for one stem, and nil when this
// repository holds no source text for it.
//
// nil is NOT an empty inventory. An empty inventory says "the source states no
// obligations"; nil says "I could not look", and the two must never render
// alike.
//
// sourced is the stem's gated requirements less its sanctioned unsourced ids,
// as sourcedGatedCounts answers it: the count the register is derived from.
func (d *Deriver) Inventory(stem string, sourced int) (*Inventory, error) {
	return d.InventoryUnder(stem, sourced, registerRFC2119)
}

// InventoryUnder answers Inventory with its sites derived under signed, the
// register an artifact signs, when signed is prose and the source supports
// rfc2119. Register still answers what the source supports, which is
// the ceiling a sign-off is refused above.
//
// A weaker sign-off is legal, so it is judged against the sites its own
// register reads. Deriving the stronger register's sites for it instead reds a
// prose walk the moment its stem's sourced count reaches the keyword sites.
func (d *Deriver) InventoryUnder(stem string, sourced int, signed string) (*Inventory, error) {
	raw, ok := SourceText(d.tree, stem)
	if !ok {
		// "This repository holds no source text" is a state each caller
		// REPORTS as its own violation, naming the artifact and the two paths
		// it looked in. A sentinel error would make every one of them unwrap
		// it to say the same thing.
		return nil, nil //nolint:nilnil // nil means "I could not look", stated in the doc comment
	}
	rel, _ := SourcePath(d.tree, stem)
	key := inventoryKey{stem: stem, sourced: sourced, signed: signed, raw: raw, path: rel}
	if found, seen := d.memo[key]; seen {
		return found, nil
	}

	stripped := stripPageFurniture(raw)
	keyword := sitesFor(stripped, siteKeywordRE)
	register := DeriveRegister(len(keyword), 0, sourced)
	// The prose scan runs only where a site set or the register reads it.
	var prose []Site
	if register != registerRFC2119 {
		prose = sitesFor(stripped, siteProseRE)
		register = DeriveRegister(len(keyword), len(prose), sourced)
	}
	// Only prose lowers the set. A manual-walk sign-off rests on a declared
	// section walk rather than on an inventory, so it keeps the sites the
	// source derives, as it always has.
	under := register
	if signed == registerProse && register == registerRFC2119 {
		under = registerProse
	}
	var sites []Site
	if under == registerRFC2119 {
		sites = keyword
	}
	if under == registerProse {
		if register == registerRFC2119 {
			prose = sitesFor(stripped, siteProseRE)
		}
		sites = prose
	}

	counts := map[string]int{}
	for _, site := range sites {
		counts[site.Section]++
	}
	bodies := sectionBodies(stripped)
	sections := make([]SectionEntry, 0, len(bodies))
	for _, section := range bodies {
		sections = append(sections, SectionEntry{ID: section.id, Sites: counts[section.id]})
	}

	// Asserted at the PRODUCER, not left for a downstream map to swallow. A
	// locator is the only handle a reviewer's decision has on a sentence, so
	// two sentences sharing one is an obligation nobody judges. sectionBodies
	// makes both impossible by construction; this says so if it ever stops.
	siteIDs := make([]string, 0, len(sites))
	for _, site := range sites {
		siteIDs = append(siteIDs, site.ID)
	}
	sectionIDs := make([]string, 0, len(sections))
	for _, section := range sections {
		sectionIDs = append(sectionIDs, section.ID)
	}
	if err := refuseDuplicates(rel, "site locator", siteIDs); err != nil {
		return nil, err
	}
	if err := refuseDuplicates(rel, "section id", sectionIDs); err != nil {
		return nil, err
	}

	inv := &Inventory{
		Stem: stem, Register: register, SourcePath: rel,
		SourceSHA: RequirementSHA(raw), Sections: sections, Sites: sites,
		KeywordSites: len(keyword),
	}
	// Memoised only on the way OUT, so a derivation that raised the guard above
	// is never cached as an answer.
	d.memo[key] = inv
	return inv, nil
}

func refuseDuplicates(rel, label string, ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, one := range ids {
		if seen[one] {
			var tb textbuf.Buffer
			return parseErr(tb.Str(rel).Str(": the derivation produced duplicate ").
				Str(label).Byte(' ').Str(pyRepr(one)).Str(". Every derived ").Str(label).
				Str(" is unique or the sign-off cannot address the sentence it names ").
				Str("-- see sectionBodies"))
		}
		seen[one] = true
	}
	return nil
}
