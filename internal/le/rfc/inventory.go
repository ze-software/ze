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
// column 0 would otherwise read as appendix A.
//
// It OVER-MATCHES, deliberately and unavoidably: RFCs put column-0 attribute
// tables, packet diagrams and tables of contents in the same text stream, and
// no pattern can separate "3.1  Route Selection" from a table row numbered 3.1
// by shape alone. The derivation is built to SURVIVE a false match rather than
// to prevent one -- see sectionBodies.
var sectionHeadingRE = regexp.MustCompile(
	`^(?:Appendix\s+)?(?:(\d+(?:\.\d+)*)\.?|([A-Z](?:\.\d+)*)\.)[ \t]+(\S.*)$`)

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
// blank run after it.
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
	// header; "blanks" is header consumed, still swallowing the blank run.
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
				continue // the form-feed line, and the blanks around the header
			}
			if state == "header" {
				state = "blanks"
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

// sectionBodies answers every section, with a leading front entry.
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
	order := []string{frontSection}
	bodies := map[string][]string{frontSection: {}}
	current := frontSection
	for line := range strings.SplitSeq(text, "\n") {
		found := sectionHeadingRE.FindStringSubmatch(line)
		if found == nil {
			bodies[current] = append(bodies[current], line)
			continue
		}
		current = found[1]
		if current == "" {
			current = found[2]
		}
		if _, seen := bodies[current]; !seen {
			order = append(order, current)
			bodies[current] = []string{}
		} else if len(bodies[current]) > 0 {
			// A blank line, so the resumed run is a fresh paragraph rather
			// than a continuation of a sentence written elsewhere.
			bodies[current] = append(bodies[current], "")
		}
		bodies[current] = append(bodies[current], found[3], "")
	}
	out := make([]sectionBody, 0, len(order))
	for _, id := range order {
		out = append(out, sectionBody{id: id, body: strings.Join(bodies[id], "\n")})
	}
	return out
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
// Derived from the text, never authored: the RFCs that would most benefit from
// claiming the strong grade are exactly the ones whose source cannot support it.
func DeriveRegister(keywordSites, proseSites, gated int) string {
	if keywordSites > 0 && keywordSites >= gated {
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
	stem  string
	gated int
	raw   string
	path  string
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
func (d *Deriver) Inventory(stem string, gated int) (*Inventory, error) {
	raw, ok := SourceText(d.tree, stem)
	if !ok {
		// "This repository holds no source text" is a state each caller
		// REPORTS as its own violation, naming the artifact and the two paths
		// it looked in. A sentinel error would make every one of them unwrap
		// it to say the same thing.
		return nil, nil //nolint:nilnil // nil means "I could not look", stated in the doc comment
	}
	rel, _ := SourcePath(d.tree, stem)
	key := inventoryKey{stem: stem, gated: gated, raw: raw, path: rel}
	if found, seen := d.memo[key]; seen {
		return found, nil
	}

	stripped := stripPageFurniture(raw)
	keyword := sitesFor(stripped, siteKeywordRE)
	register := DeriveRegister(len(keyword), 0, gated)
	sites := keyword
	if register != registerRFC2119 {
		prose := sitesFor(stripped, siteProseRE)
		register = DeriveRegister(len(keyword), len(prose), gated)
		sites = nil
		if register == registerProse {
			sites = prose
		}
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
