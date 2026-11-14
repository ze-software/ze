// Design: docs/contributing/rfc-conformance-gates.md -- Coverage figures: the one split every surface reads.
// Related: register_le_help_rfc_tree_sections.go -- the registration.
// Related: ui_fixture_le_rfc_answers.go -- leRFCPartialDriver, the HEAD export and the le run this body uses.

package fixture

import (
	"regexp"
	"strings"
)

// leTreeMustSections and leTreeShouldStates match the two level clauses of
// one sections cell, as rfc.Sections.String renders it: `MUST <kept> kept:`
// then the MUST bar's segments, and `SHOULD <total>:` then the SHOULD states,
// each clause ending at the next `;`. The `not owed` clauses carry no count
// before their colon, so neither pattern matches them.
var (
	leTreeMustSections = regexp.MustCompile(`MUST \d+ kept: ([^;\n]*)`)
	leTreeShouldStates = regexp.MustCompile(`SHOULD \d+: ([^;\n]*)`)
	// leTreeSegment is one `<name> <count>` segment of a level clause.
	leTreeSegment = regexp.MustCompile(`^(\D+) (\d+)$`)
	// leTreeSpace is any run of whitespace, so a help page wrapped at its
	// width reads as the one sentence it was declared as.
	leTreeSpace = regexp.MustCompile(`\s+`)
)

// leTreeMustTarget is the MUST bar of spec-rfc-figures-built-and-proven
// ("The two bars"), left to right. It is the design the tree is held to, and
// it is compared only with words read out of the tree's own output.
var leTreeMustTarget = []string{"Ze proven", "OS verified", "Proof pending", "OS testing gap", "Exclusion unchecked",
	"Not assessed", "Not built"}

// leTreeRetiredWord is the merged section the split replaces. Neither the
// tree nor its help may still print it.
const leTreeRetiredWord = "correctness gap"

// leTreeSectionWords is the section vocabulary one `le rfc tree` text answer
// prints: the MUST bar's segment names and the SHOULD state names, in order.
type leTreeSectionWords struct {
	must   []string
	should []string
}

// leHelpRFCTreeSections proves `le help rfc tree` names every section the
// tree prints, in the tree's order, and that the tree prints the MUST bar of
// the design: Ze proven, OS verified, Proof pending, OS testing gap, Exclusion
// unchecked, Not assessed, Not built, and no "correctness gap" on either
// surface.
//
// Method: the shipped le runs over the fixture's private HEAD export. The
// section words are read out of `le rfc tree` text, from every row and rollup
// line that carries sections, and each line must name the same words. The
// help's declared text, whitespace folded, must then hold each of those words
// in order. Only after that is the tree's MUST list compared with the design's
// list, so a red names what the tree prints.
func leHelpRFCTreeSections(t leRFCPartialTree) string {
	tree := t.runLE("rfc", "tree")
	leRFCAnswersRequire(tree.code == 0, "le rfc tree answered %d: %s", tree.code, tree.stderr)
	help := t.runLE(leHelpPagesHelpWord, "rfc", "tree")
	leRFCAnswersRequire(help.code == 0, "le help rfc tree answered %d: %s", help.code, help.stderr)

	words := leTreeSectionWordsOf(tree.stdout)
	page := leTreeSpace.ReplaceAllString(help.stdout, " ")
	leTreeRequireNamedInOrder(page, words)

	leRFCAnswersRequire(len(words.must) == len(leTreeMustTarget),
		"le rfc tree prints the MUST sections %q, the design's MUST bar is %q", words.must, leTreeMustTarget)
	for index, want := range leTreeMustTarget {
		leRFCAnswersRequire(strings.EqualFold(words.must[index], want),
			"le rfc tree prints the MUST sections %q, the design's MUST bar is %q", words.must, leTreeMustTarget)
	}
	leRFCAnswersRequire(!strings.Contains(strings.ToLower(tree.stdout), leTreeRetiredWord),
		"le rfc tree still prints %q", leTreeRetiredWord)
	leRFCAnswersRequire(!strings.Contains(strings.ToLower(page), leTreeRetiredWord),
		"le help rfc tree still names %q: %s", leTreeRetiredWord, page)
	return "OK: le help rfc tree names the sections le rfc tree prints, " + strings.Join(words.must, ", ")
}

// leTreeSectionWordsOf reads the section vocabulary out of `le rfc tree`
// text. Every line that carries a MUST clause must name the same MUST words
// and the same SHOULD words as the first, so a rollup line and a row line
// cannot disagree; a tree with no such line is refused, because an empty
// vocabulary would pass every later check.
func leTreeSectionWordsOf(text string) leTreeSectionWords {
	var first *leTreeSectionWords
	lines := 0
	for line := range strings.SplitSeq(text, "\n") {
		must := leTreeMustSections.FindStringSubmatch(line)
		if must == nil {
			continue
		}
		should := leTreeShouldStates.FindStringSubmatch(line)
		leRFCAnswersRequire(should != nil, "tree line carries a MUST clause and no SHOULD clause: %q", line)
		words := leTreeSectionWords{must: leTreeSegmentNames(line, must[1]), should: leTreeSegmentNames(line, should[1])}
		lines++
		if first == nil {
			first = &words
			continue
		}
		leRFCAnswersRequire(strings.Join(words.must, "|") == strings.Join(first.must, "|"),
			"tree line names the MUST sections %q, an earlier line %q: %q", words.must, first.must, line)
		leRFCAnswersRequire(strings.Join(words.should, "|") == strings.Join(first.should, "|"),
			"tree line names the SHOULD states %q, an earlier line %q: %q", words.should, first.should, line)
	}
	leRFCAnswersRequire(first != nil, "le rfc tree printed no line with sections:\n%s", text)
	leRFCAnswersRequire(lines > 1, "le rfc tree printed one line with sections, so no row was compared with another:\n%s", text)
	return *first
}

// leTreeSegmentNames answers the names of one level clause's `<name> <count>`
// segments, refusing a segment in any other shape.
func leTreeSegmentNames(line, clause string) []string {
	var names []string
	for part := range strings.SplitSeq(clause, ", ") {
		segment := leTreeSegment.FindStringSubmatch(strings.TrimSpace(part))
		leRFCAnswersRequire(segment != nil, "tree segment %q is not `<name> <count>`: %q", part, line)
		names = append(names, segment[1])
	}
	return names
}

// leTreeRequireNamedInOrder requires the help page to name every MUST word in
// the tree's order, then every SHOULD word in the tree's order after the last
// MUST word, ignoring case. A word the help names out of order, or not at all,
// is a help that describes a tree other than the one le prints.
func leTreeRequireNamedInOrder(page string, words leTreeSectionWords) {
	lower := strings.ToLower(page)
	at := 0
	for _, level := range [][]string{words.must, words.should} {
		for _, word := range level {
			found := strings.Index(lower[at:], strings.ToLower(word))
			leRFCAnswersRequire(found >= 0,
				"le help rfc tree does not name %q after offset %d, in the order le rfc tree prints %q then %q: %s",
				word, at, words.must, words.should, page)
			at += found + len(word)
		}
	}
}
