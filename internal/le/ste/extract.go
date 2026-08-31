// Design: docs/architecture/core-design.md -- what counts as prose on each surface
// Overview: ste.go -- the checker these extractors serve
//
// Each surface has one extractor, and both use one sentence splitter. Extractors
// return only PROSE. Fenced blocks, commented code, link targets, and blockquotes
// are data or external text. The checker measures all returned units. A false
// unit creates a finding that reviewers cannot fix.
package ste

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// The Markdown structures the extractor reads.
var (
	codeSpanRe    = regexp.MustCompile("`[^`]*`")
	linkTargetRe  = regexp.MustCompile(`\]\([^)]*\)`)
	autolinkRe    = mustPattern(`<https?://[^>]*>|https?://{NS}+`)
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlEntityRe  = mustPattern(`&(?:[a-zA-Z]+|#{D}+);`)

	headingRe      = mustPattern(`^{SP}{0,3}#{1,6}{SP}`)
	fenceRe        = mustPattern("^{SP}*(`{3,}|~{3,})")
	tableDividerRe = mustPattern(`^{SP}*\|?[{SPB}:|-]+\|[{SPB}:|-]*$`)
	orderedItemRe  = mustPattern(`^{SP}*{D}+[.)]{SP}+`)
	bulletItemRe   = mustPattern(`^{SP}*[-*+]{SP}+`)
	// A bold field label starts a unit. Otherwise, a rule's three metadata lines
	// join into a 28-word "sentence" without a verb.
	boldFieldRe = mustPattern(`^{SP}*\*\*[^*]+:\*\*`)
	// Two or more spaces inside a line mark column alignment, not wrapped prose.
	// Markdown joins consecutive lines. Aligned lists and tables then become one
	// sentence. One handover's 15-line commit list measured 187 words. Two
	// spaces after punctuation are prose, so punctuation cannot precede the
	// matched run. Each aligned line remains a unit, and all words stay in the
	// population.
	layoutColumnsRe = mustPattern(`[^{SPB}.!?:,;]{SP}{2,}{NS}`)
)

// isLayoutRow reports whether a line is column-aligned layout instead of wrapped
// prose.
//
// The probe removes code spans, links, and comments with no replacement space.
// scrub pads placeholders, which would look like a column. One vendored README
// aligns three numbers inside `(-2105.254  300.680  286.185)`. Its surrounding
// prose remains an ordinary wrapped paragraph.
func isLayoutRow(line string) bool {
	probe := codeSpanRe.ReplaceAllLiteralString(line, "CODE")
	probe = autolinkRe.ReplaceAllLiteralString(probe, "URL")
	probe = htmlCommentRe.ReplaceAllLiteralString(probe, "")
	return layoutColumnsRe.MatchString(pyStrip(probe))
}

// scrub removes data from prose. It first removes a code span, so nested URLs or
// links are already absent when their patterns run.
func scrub(text string) string {
	text = codeSpanRe.ReplaceAllLiteralString(text, " CODE ")
	text = htmlEntityRe.ReplaceAllLiteralString(text, " ") // `&nbsp;` is not a semicolon
	text = autolinkRe.ReplaceAllLiteralString(text, " URL ")
	text = linkTargetRe.ReplaceAllLiteralString(text, "] ") // keep the label, drop the target
	return htmlCommentRe.ReplaceAllLiteralString(text, " ")
}

// blankHTMLComments blanks multi-line HTML comments, preserving line numbers.
//
// An `ste:` directive survives: it IS an HTML comment, and blanking it here
// erased the very marker unitsMarkdown reads.
func blankHTMLComments(text string) string {
	return htmlCommentRe.ReplaceAllStringFunc(text, func(body string) string {
		if strings.Contains(body, "ste:") {
			return body
		}
		return strings.Repeat("\n", strings.Count(body, "\n"))
	})
}

// unitsMarkdown splits Markdown into reviewable units.
//
// Skipped: fenced blocks (data), blockquotes (external quotation, kept verbatim
// per the rule), table dividers, and lines after `<!-- ste: ignore -->`.
func unitsMarkdown(lines []string) []unit {
	var out []unit
	fence := "" // the opening delimiter, empty when not inside a block
	var paragraph []string
	paraLine := 0
	skipNext := false

	flush := func() {
		if len(paragraph) > 0 {
			out = append(out, unit{Text: strings.Join(paragraph, " "), Line: paraLine, Paragraph: true})
		}
		paragraph = nil
		paraLine = 0
	}

	for index, raw := range lines {
		number := index + 1

		if marker := fenceRe.FindStringSubmatch(raw); marker != nil {
			run := marker[1]
			if fence == "" {
				flush()
				fence = run
				continue
			}
			// Only a run of the SAME character, at least as long, closes it. A
			// ~~~ line inside a ``` block used to close it and expose the code.
			if run[0] == fence[0] && len(run) >= len(fence) {
				fence = ""
				continue
			}
		}
		if fence != "" {
			continue
		}
		if ignoreLineRe.MatchString(raw) {
			flush()
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			continue
		}

		line := strings.TrimRight(raw, "\n")
		if pyStrip(line) == "" {
			flush()
			continue
		}
		if strings.HasPrefix(strings.TrimLeftFunc(line, pySpace), ">") { // external quotation
			flush()
			continue
		}
		if tableDividerRe.MatchString(line) {
			flush()
			continue
		}

		text := scrub(line)

		if strings.Contains(line, "|") && strings.HasPrefix(pyStrip(line), "|") {
			flush()
			for cell := range strings.SplitSeq(text, "|") {
				if pyStrip(cell) != "" {
					out = append(out, unit{Text: pyStrip(cell), Line: number})
				}
			}
			continue
		}
		if headingRe.MatchString(line) {
			flush()
			out = append(out, unit{Text: pyStrip(headingRe.ReplaceAllLiteralString(text, "")), Line: number})
			continue
		}
		if boldFieldRe.MatchString(line) {
			flush()
			out = append(out, unit{Text: pyStrip(text), Line: number})
			continue
		}
		if orderedItemRe.MatchString(line) {
			flush()
			out = append(out, unit{
				Text: pyStrip(orderedItemRe.ReplaceAllLiteralString(text, "")),
				Line: number, Procedural: true,
			})
			continue
		}
		if bulletItemRe.MatchString(line) {
			flush()
			out = append(out, unit{Text: pyStrip(bulletItemRe.ReplaceAllLiteralString(text, "")), Line: number})
			continue
		}
		if isLayoutRow(line) {
			flush()
			out = append(out, unit{Text: pyStrip(text), Line: number})
			continue
		}

		if len(paragraph) == 0 {
			paraLine = number
		}
		paragraph = append(paragraph, pyStrip(text))
	}

	flush()
	return out
}

// goMarkers are the structured markers a Go comment carries. They are
// machine-read contracts, not prose: `// Design:`, `// Related:` and their
// siblings are required by `ai/rules/go-standards.md`, and their content is
// paths.
var goMarkers = []string{
	"go:", "nolint", "Design:", "Related:", "Detail:", "Overview:", "RFC:",
	"RFC requirement:", "VALIDATES:", "PREVENTS:", "source:",
	"test-asserts-nothing:", "ste:", "Code generated",
	// Alone on its line, and the ORDER must not change: the Python holds the
	// same list and TestEveryPlainListHoldsThePythonValues compares them by
	// value. c_string_concat reads the comma and space between two literals
	// as a string, sees the `+` opening this one, and refuses a
	// concatenation nobody wrote.
	"+build",
	"TODO",
	"FIXME", "Deprecated:",
}

// goCodeLikeRe recognizes a commented-out line of code, which is code rather
// than prose. The last branch carries the Python `\b`, written as an explicit
// non-word class because Go's own `\b` is ASCII where Python's is Unicode.
var goCodeLikeRe = mustPattern(`:=|==|!=|\{{SP}*$|\}{SP}*$|;{SP}*$|\){SP}*\{|^{SP}*(?:if|for|func|return)(?:{NW}|$)`)

// unitsGo extracts prose comment blocks from Go source.
//
// Consecutive `//` lines join into one unit, so a wrapped sentence is measured
// once rather than for each line.
func unitsGo(lines []string) []unit {
	var out []unit
	var block []string
	blockLine := 0
	inBlockComment := false

	flush := func() {
		if len(block) > 0 {
			text := pyStrip(strings.Join(block, " "))
			if len(pyFields(text)) > 2 {
				out = append(out, unit{Text: text, Line: blockLine, Paragraph: true})
			}
		}
		block = nil
		blockLine = 0
	}

	for index, raw := range lines {
		number := index + 1
		line := pyStrip(raw)

		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false
				line = strings.SplitN(line, "*/", 2)[0]
			}
			body := pyStrip(strings.TrimLeft(line, "*"))
			if body != "" {
				if len(block) == 0 {
					blockLine = number
				}
				block = append(block, scrub(body))
			} else {
				flush()
			}
			continue
		}

		if strings.HasPrefix(line, "/*") {
			body := line[2:]
			if strings.Contains(body, "*/") {
				body = strings.SplitN(body, "*/", 2)[0]
			} else {
				inBlockComment = true
			}
			if pyStrip(body) != "" {
				if blockLine == 0 {
					blockLine = number
				}
				block = append(block, scrub(pyStrip(body)))
			}
			continue
		}

		if strings.HasPrefix(line, "//") {
			body := pyStrip(line[2:])
			if body == "" || hasGoMarker(body) {
				flush()
				continue
			}
			if goCodeLikeRe.MatchString(body) {
				flush()
				continue
			}
			if len(block) == 0 {
				blockLine = number
			}
			block = append(block, scrub(body))
			continue
		}

		flush()
	}

	flush()
	return out
}

// hasGoMarker reports whether a comment body opens with a structured marker.
func hasGoMarker(body string) bool {
	for _, marker := range goMarkers {
		if strings.HasPrefix(body, marker) {
			return true
		}
	}
	return false
}

// yangDescriptionRe finds a `description`, an `error-message` or a `ze:help`
// string. The leading `\b` is applied by findBounded rather than by the pattern.
//
// `ze:help` carries the long explanation of a command node, and the
// `description` beside it carries the one-line summary
// (plan/spec-yang-short-and-long-command-help.md). Both are authored prose, so
// both are reviewed: a pattern naming the description keyword alone would let
// a sentence leave STE scope by moving one statement down. The prefix is
// literal because every module in the tree imports the extensions module as
// `prefix ze`.
var yangDescriptionRe = mustPattern(`(?s)(?:description|error-message|ze:help){SP}+(?:"(?P<body>[^"]*)"|'(?P<body2>[^']*)')`)

// unitsYANG extracts `description`, `error-message` and `ze:help` strings from
// a YANG module.
func unitsYANG(text string) []unit {
	var out []unit
	names := yangDescriptionRe.SubexpNames()
	for _, loc := range findBounded(yangDescriptionRe, text, edgeNone) {
		raw := ""
		for group, name := range names {
			if name != "body" && name != "body2" {
				continue
			}
			if loc[2*group] >= 0 {
				raw = text[loc[2*group]:loc[2*group+1]]
				break
			}
		}
		body := scrub(pyJoinFields(raw))
		if len(pyFields(body)) < 3 {
			continue
		}
		line := strings.Count(text[:loc[0]], "\n") + 1
		out = append(out, unit{Text: body, Line: line, Paragraph: true})
	}
	return out
}

// ─── Sentences and the STE word count ───────────────────────────────────────

// abbreviations hold their dot, so a sentence is not split inside one. They are
// substituted before the inner-dot rule, because their own dots would otherwise
// survive and split the sentence.
var abbreviations = []string{"e.g.", "i.e.", "etc.", "vs.", "approx.", "Mr.", "Ms.", "Dr."}

// held stands in for a dot that does not end a sentence.
const held = "\x00"

// numberedAbbreviationRe is `\b(No|Fig)\.` in front of the number it labels.
//
// `No.` and `Fig.` abbreviate only there, as in "No. 5". Everywhere else the
// dot is a real full stop, and an unconditional hold glues the next sentence
// onto this one. That inflates a word count and reports a run-on nobody can
// fix, because the sentence is already two.
//
// This repository decides the shape of the rule. It writes "answered Yes/No."
// and a table cell of "| No. Answer the person who asked" constantly, and it
// numbers almost nothing: 38 occurrences against 1 when this was split out.
// Filed as F-ste-2 in `plan/learned/HOOK-FRICTION.md`.
var numberedAbbreviationRe = mustPattern(`(No|Fig)\.{SP}*{D}`)

// holdNumberedAbbreviations replaces the dot of `No.` and `Fig.` in front of a
// number, and nowhere else.
//
// The scan resumes just past the DOT rather than past the digit the pattern
// looked ahead at, because Python's lookahead consumes nothing. The bound is
// the input: each iteration advances at least one byte.
func holdNumberedAbbreviations(text string) string {
	var out strings.Builder
	written, search := 0, 0
	for search <= len(text) {
		loc := numberedAbbreviationRe.FindStringSubmatchIndex(text[search:])
		if loc == nil {
			break
		}
		start := search + loc[0]
		dotEnd := search + loc[3] + 1 // one past the dot that follows group 1
		if !wordBoundary(text, start) {
			_, size := utf8.DecodeRuneInString(text[start:])
			search = start + size
			continue
		}
		out.WriteString(text[written:start])
		out.WriteString(text[start : dotEnd-1])
		out.WriteString(held)
		written, search = dotEnd, dotEnd
	}
	out.WriteString(text[written:])
	return out.String()
}

// holdInnerDots replaces every dot that sits between two word characters, which
// keeps 4.5, foo.go and Rule 1.1 whole.
func holdInnerDots(text string) string {
	var out strings.Builder
	for index, r := range text {
		if r == '.' && isWordBefore(text, index) && isWordAfter(text, index+1) {
			out.WriteString(held)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

// sentenceClosers is the punctuation Markdown puts BETWEEN the full stop and
// the space: "**... dictionary.** It cannot ...". Without it, a bolded lead-in
// glues its whole bullet into one 29-word sentence.
const sentenceClosers = "*_`\"'’”)]"

// Sentences splits a unit into sentences, keeping numbers and file names whole.
//
// The command help-shape gate reads it too: a command summary is one sentence,
// and this is the repository's one answer to where a sentence ends
// (internal/le/docvalid/helpshape.go).
func Sentences(text string) []string {
	holdText := text
	for _, abbreviation := range abbreviations {
		holdText = strings.ReplaceAll(holdText, abbreviation,
			strings.ReplaceAll(abbreviation, ".", held))
	}
	holdText = holdNumberedAbbreviations(holdText)
	holdText = holdInnerDots(holdText)

	var parts []string
	for _, part := range splitOnSentenceEnd(holdText) {
		if trimmed := pyStrip(part); trimmed != "" {
			parts = append(parts, strings.ReplaceAll(trimmed, held, "."))
		}
	}
	return parts
}

// splitOnSentenceEnd cuts the text at every `(?<=[.!?])[closers]*\s+`.
//
// No backtracking is needed and none is written: the closer class holds no
// whitespace, so a shorter closer run can never be followed by one. The bound
// is the input: index advances by at least one byte on every iteration.
func splitOnSentenceEnd(text string) []string {
	var parts []string
	start := 0
	for index := 0; index < len(text); {
		r, size := utf8.DecodeRuneInString(text[index:])
		if r != '.' && r != '!' && r != '?' {
			index += size
			continue
		}

		cut := index + size
		closers := cut
		for closers < len(text) {
			next, width := utf8.DecodeRuneInString(text[closers:])
			if !strings.ContainsRune(sentenceClosers, next) {
				break
			}
			closers += width
		}
		spaces := closers
		for spaces < len(text) {
			next, width := utf8.DecodeRuneInString(text[spaces:])
			if !pySpace(next) {
				break
			}
			spaces += width
		}
		if spaces == closers { // no whitespace: the pattern needs one
			index = cut
			continue
		}

		parts = append(parts, text[start:cut])
		start = spaces
		index = spaces
	}
	parts = append(parts, text[start:])
	return parts
}

var (
	parenthesesRe = regexp.MustCompile(`\([^)]*\)`)
	quotedRe      = regexp.MustCompile("\"[^\"]*\"|“[^”]*”")
	// A number with its unit, without the `\b` at either end: replaceMeasures
	// applies those, because the trailing one needs the backtracking RE2 will
	// not do. "3 abc%" is a measurement of "3 abc", and a rejected greedy match
	// would have lost it.
	numberUnitRe = mustPattern(`{D}+(?:\.{D}+)?{SP}+[A-Za-z%]+`)
)

// WordCount counts words the way STE counts them (Rules 8.5 through 8.7).
// Parenthesised text, quoted text, a number with its unit, and a hyphenated
// word each count as one word.
//
// The command help-shape gate reads it too, so the word cap it applies to a
// command summary counts the same words this checker counts
// (internal/le/docvalid/helpshape.go).
func WordCount(sentence string) int {
	text := parenthesesRe.ReplaceAllLiteralString(sentence, " PAREN ")
	text = quotedRe.ReplaceAllLiteralString(text, " QUOTED ")
	text = replaceMeasures(text)

	count := 0
	for _, token := range pyFields(text) {
		if strings.ContainsFunc(token, isWordLike) {
			count++
		}
	}
	return count
}

// isWordLike is the script's `[A-Za-z0-9]` probe: a token with none of those is
// punctuation rather than a word.
func isWordLike(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// replaceMeasures collapses "3 seconds" and "50 %" style measurements to one
// token each.
//
// The bound is the input: each iteration consumes a match or advances one rune.
func replaceMeasures(text string) string {
	var out strings.Builder
	written, search := 0, 0
	for search <= len(text) {
		loc := numberUnitRe.FindStringIndex(text[search:])
		if loc == nil {
			break
		}
		start, end := search+loc[0], search+loc[1]
		if !wordBoundary(text, start) {
			_, size := utf8.DecodeRuneInString(text[start:])
			search = start + size
			continue
		}

		// The pattern greedily takes the `[A-Za-z%]+` unit. Python then returns
		// one character at a time until the final `\b` matches. This code does
		// the same. A `%` suffix has no boundary unless a word character follows.
		unitStart := end
		for unitStart > start && isASCIIUnit(text[unitStart-1]) {
			unitStart--
		}
		for end > unitStart+1 && !wordBoundary(text, end) {
			end--
		}
		if !wordBoundary(text, end) {
			_, size := utf8.DecodeRuneInString(text[start:])
			search = start + size
			continue
		}

		out.WriteString(text[written:start])
		out.WriteString(" MEASURE ")
		written, search = end, end
	}
	out.WriteString(text[written:])
	return out.String()
}

// isASCIIUnit reports whether c belongs to the `[A-Za-z%]` unit class.
func isASCIIUnit(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '%'
}
