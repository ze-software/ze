// Design: docs/contributing/rfc-conformance-gates.md -- the RFC dependency graph
// Related: graph.go -- the edges, cycle components and levels built from these
//
// parse.go reads one RFC or Internet-Draft text into the three facts the graph
// is built from: the RFCs its header says it updates, the ones it obsoletes,
// and the documents its normative references cite. Nothing here knows the
// corpus: a citation of an RFC the repository does not hold is kept, and the
// graph drops it.
package graph

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Text is what one document declares about other documents.
//
// Updates and Obsoletes are RFC stems (`rfc4271`) from the header lines, or
// from the front-matter sentence an old RFC states them in. Cites is every
// stem the normative references name, RFCs and drafts alike, in first-seen
// order. HasReferences says a references section was found at all, so a
// document that cites nothing can be told from one the parser could not read.
type Text struct {
	Updates       []string
	Obsoletes     []string
	Cites         []string
	HasReferences bool
}

// rfcNumberDigitsMax bounds an RFC number. The series has five digits, so a
// longer run of digits after `RFC` is not an RFC number, and it is dropped
// rather than turned into a node nobody can find.
const rfcNumberDigitsMax = 5

// headerLinesMax bounds the header search. The header block of every text in
// the corpus ends well inside it; a line further down that starts `Updates:`
// is body prose.
const headerLinesMax = 40

// frontMatterLinesMax bounds the front matter an old RFC states its lineage
// in, when no table of contents or first section ends it sooner.
const frontMatterLinesMax = 200

var (
	// headerLinkRE is a header line naming a lineage, at column 0. The list is
	// the left column only: the author column follows after a run of spaces.
	headerLinkRE = regexp.MustCompile(`^(Updates|Obsoletes)\s*:?\s+(.*)$`)
	// citationRE is one RFC citation: `RFC 4271`, `RFC4271`, `RFC-4271` and
	// `RFC` wrapped onto the next line all read alike, because the section is
	// matched as one string.
	citationRE = regexp.MustCompile(`\bRFC[\s-]*(\d+)\b`)
	// draftRE is one Internet-Draft name; its version suffix is cut by
	// draftStem.
	draftRE = regexp.MustCompile(`\bdraft-[a-z0-9]+(?:-[a-z0-9]+)+\b`)
	// draftVersionRE is the two-digit version an Internet-Draft name ends in.
	draftVersionRE = regexp.MustCompile(`-\d{2}$`)
	// listNumberRE is one number of a header list, with or without `RFC`.
	listNumberRE = regexp.MustCompile(`^(?:RFCs?\s*-?\s*)?(\d+)$`)
	// anchorRE is a bracketed citation anchor.
	anchorRE = regexp.MustCompile(`\[[^\]]*\]`)
	// parentheticalRE is a header note such as `(if approved)`.
	parentheticalRE = regexp.MustCompile(`\([^()]*\)`)
	// proseLinkRE is the front-matter sentence an old RFC states its lineage
	// in: `This memo obsoletes RFC 1234 and RFC 1235`.
	proseLinkRE = regexp.MustCompile(`(?i)\b(?:this|the present)\s+(?:memo|document|RFC|specification)\s+` +
		`(obsoletes|updates|supersedes)\s+((?:RFCs?[\s-]*\d+(?:\s*\[[^\]]*\])?(?:\s*,\s*|\s*,?\s*and\s+)?)+)`)
	// normativeHeadingRE and combinedHeadingRE are the two reference headings.
	// nonNormativeHeadingRE is the heading the first also matches and that
	// lists what a text does NOT build on: a word boundary sits between
	// `Non-` and `Normative`, and RE2 has no lookbehind to exclude it.
	normativeHeadingRE    = regexp.MustCompile(`(?i)\bnormative\s+references\b`)
	nonNormativeHeadingRE = regexp.MustCompile(`(?i)\bnon[\s-]*normative\s+references\b`)
	combinedHeadingRE     = regexp.MustCompile(`(?i)^(?:(?:appendix\s+)?[0-9a-z]{1,3}(?:\.[0-9]+)*\.?\s+)?` +
		`(?:references|bibliography|references\s+and\s+bibliography)\s*$`)
	// pageFooterRE and pageHeaderRE are the page furniture at column 0 that
	// is not a heading.
	pageFooterRE = regexp.MustCompile(`\[Page \d+\]\s*$`)
	pageHeaderRE = regexp.MustCompile(`^(?:RFC \d+|Internet-Draft)\s{2,}`)
	// tocLeaderRE is a table-of-contents line: a heading with dot leaders.
	tocLeaderRE = regexp.MustCompile(`\.{3,}|\s\d+$`)
)

// Parse reads one document's text. It answers an error only for a text that
// is not a document at all: an empty one.
func Parse(text string) (Text, error) {
	if strings.TrimSpace(text) == "" {
		return Text{}, parseErr("the text is empty, so it states no header and no references")
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	var out Text
	out.Updates, out.Obsoletes = headerLinks(lines)
	proseUpdates, proseObsoletes := proseLinks(lines)
	out.Updates = union(out.Updates, proseUpdates)
	out.Obsoletes = union(out.Obsoletes, proseObsoletes)
	section, found := referenceSection(lines)
	out.HasReferences = found
	out.Cites = citations(section)
	return out, nil
}

// headerLinks reads the `Updates:` and `Obsoletes:` header lines, with their
// continuation lines, from the top of the text.
func headerLinks(lines []string) (updates, obsoletes []string) {
	limit := min(len(lines), headerLinesMax)
	for i := 0; i < limit; i++ {
		found := headerLinkRE.FindStringSubmatch(lines[i])
		if found == nil {
			continue
		}
		list := leftColumn(found[2])
		for strings.HasSuffix(strings.TrimSpace(list), ",") && i+1 < limit {
			next := strings.TrimSpace(lines[i+1])
			if next == "" {
				break
			}
			i++
			list += " " + leftColumn(next)
		}
		stems := listStems(list)
		if found[1] == "Updates" {
			updates = union(updates, stems)
			continue
		}
		obsoletes = union(obsoletes, stems)
	}
	return updates, obsoletes
}

// leftColumn cuts the author column a header line carries after a run of
// three or more spaces.
func leftColumn(line string) string {
	left, _, _ := strings.Cut(line, "   ")
	return left
}

// listStems reads a header list (`1997, 4271`, `RFC 783`, `RFCs 1234, 1235`)
// into stems. A note in parentheses is cut first, and the list stops at `IEN`:
// an IEN number is not an RFC.
func listStems(list string) []string {
	list = parentheticalRE.ReplaceAllString(list, "")
	if at := strings.Index(list, "IEN"); at >= 0 {
		list = list[:at]
	}
	var stems []string
	for item := range strings.SplitSeq(list, ",") {
		item = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(item), "and "))
		found := listNumberRE.FindStringSubmatch(item)
		if found == nil {
			continue
		}
		if stem, ok := rfcStem(found[1]); ok {
			stems = union(stems, []string{stem})
		}
	}
	return stems
}

// proseLinks reads the front-matter sentence an old RFC states its lineage in.
// The front matter ends at the table of contents or the first section.
func proseLinks(lines []string) (updates, obsoletes []string) {
	limit := min(len(lines), frontMatterLinesMax)
	var front textbuf.Buffer
	for _, line := range lines[:limit] {
		if strings.HasPrefix(line, "Table of Contents") {
			break
		}
		if strings.HasPrefix(line, "1.") {
			break
		}
		front.Str(line).Byte(' ')
	}
	for _, found := range proseLinkRE.FindAllStringSubmatch(front.String(), -1) {
		var stems []string
		for _, number := range citationRE.FindAllStringSubmatch(stripBrackets(found[2]), -1) {
			if stem, ok := rfcStem(number[1]); ok {
				stems = union(stems, []string{stem})
			}
		}
		if strings.EqualFold(found[1], "updates") {
			updates = union(updates, stems)
			continue
		}
		obsoletes = union(obsoletes, stems)
	}
	return updates, obsoletes
}

// stripBrackets cuts a citation anchor such as `[RFC-2463]`, so the number in
// the anchor is not read a second time.
func stripBrackets(list string) string {
	return anchorRE.ReplaceAllString(list, "")
}

// referenceSection answers the normative references, or the combined
// References section of a text that has no separate normative one, as one
// string. A heading counts only at column 0: the table of contents repeats
// every heading indented.
func referenceSection(lines []string) (string, bool) {
	var normative, combined textbuf.Buffer
	foundNormative, foundCombined := false, false
	inNormative, inCombined := false, false
	for _, line := range lines {
		if isHeading(line) {
			inNormative = normativeHeadingRE.MatchString(line) && !nonNormativeHeadingRE.MatchString(line)
			// A combined section runs on through its own subsections, which
			// all name references; any other heading ends it.
			inCombined = combinedHeadingRE.MatchString(strings.TrimSpace(line)) ||
				(inCombined && strings.Contains(strings.ToLower(line), "reference"))
			foundNormative = foundNormative || inNormative
			foundCombined = foundCombined || inCombined
			continue
		}
		// The oldest texts (RFC 791, 905, 1071) center or indent the heading,
		// so an indented line that is only a References heading opens a
		// combined section too. It then runs to the next column-0 heading,
		// which over-reaches rather than misses (the spec's accepted error).
		if trimmed := strings.TrimSpace(line); combinedHeadingRE.MatchString(trimmed) && !tocLeaderRE.MatchString(trimmed) {
			inCombined, foundCombined = true, true
			continue
		}
		if inNormative {
			normative.Str(line).Byte('\n')
		}
		if inCombined {
			combined.Str(line).Byte('\n')
		}
	}
	if foundNormative {
		return normative.String(), true
	}
	return combined.String(), foundCombined
}

// isHeading answers whether a line is a heading: text at column 0 that is not
// a reference entry, page furniture or a table-of-contents line.
func isHeading(line string) bool {
	if line == "" {
		return false
	}
	switch line[0] {
	case ' ', '\t', '\f', '[':
		return false
	}
	if pageFooterRE.MatchString(line) {
		return false
	}
	if pageHeaderRE.MatchString(line) {
		return false
	}
	return !tocLeaderRE.MatchString(line)
}

// citations reads every RFC and Internet-Draft a references section names.
func citations(section string) []string {
	var stems []string
	for _, found := range citationRE.FindAllStringSubmatch(section, -1) {
		if stem, ok := rfcStem(found[1]); ok {
			stems = union(stems, []string{stem})
		}
	}
	for _, name := range draftRE.FindAllString(section, -1) {
		stems = union(stems, []string{DraftStem(name)})
	}
	return stems
}

// DraftStem is an Internet-Draft name without its two-digit version, so a
// citation of `-05` and a corpus file at `-16` name one document.
func DraftStem(name string) string {
	return draftVersionRE.ReplaceAllString(name, "")
}

// rfcStem answers the stem of one RFC number, and false for a number that is
// not one: zero, or more than five digits.
func rfcStem(digits string) (string, bool) {
	if len(digits) > rfcNumberDigitsMax {
		return "", false
	}
	number, err := strconv.Atoi(digits)
	if err != nil {
		return "", false
	}
	if number < 1 {
		return "", false
	}
	var tb textbuf.Buffer
	return tb.Str("rfc").Int(int64(number)).String(), true
}

// union appends each stem of more that set does not hold yet, keeping order.
func union(set, more []string) []string {
	for _, stem := range more {
		if !slices.Contains(set, stem) {
			set = append(set, stem)
		}
	}
	return set
}

// ParseError is a text the parser could not read.
type ParseError struct{ msg string }

func (e *ParseError) Error() string { return e.msg }

func parseErr(msg string) error { return &ParseError{msg: msg} }
