// Design: docs/architecture/core-design.md -- the nine detectors, and one review
// Overview: ste.go -- the checker these detectors serve
//
// Each detector reads one prose unit and appends its findings. checkSynonyms is
// the exception because synonym rotation is a document property.
//
// Precision takes priority over recall. Each list contains only patterns that
// are wrong in all repository prose contexts. Thus, some STE violations pass.
// When review finds a missed habit, widen the list and add its test case.
package ste

import (
	"slices"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// excerptRunes bounds the excerpt a finding carries. It is CHARACTERS, so an em
// dash is never cut in half.
const excerptRunes = 120

// add appends one finding, taking the excerpt from the unit when the caller
// names none.
func add(found *[]Finding, u unit, path, surface, habit, detail, fix string, excerpt ...string) {
	text := u.Text
	if len(excerpt) > 0 {
		text = excerpt[0]
	}
	*found = append(*found, Finding{
		File:    path,
		Line:    u.Line,
		Surface: surface,
		Habit:   habit,
		Number:  habitNumber(habit),
		Detail:  detail,
		Fix:     fix,
		Excerpt: runeHead(text, excerptRunes),
	})
}

// quoted renders the word a finding names, in the script's spelling.
func quoted(word string) string {
	var tb textbuf.Buffer
	return tb.Byte('"').Str(word).Byte('"').String()
}

// checkHedging finds habit 2.
//
// The match is case-insensitive except for ALL-CAPS forms. MUST, SHOULD, and MAY
// are RFC 2119 keywords. Sentence-initial hedges also match.
func checkHedging(u unit, path, surface string, found *[]Finding) {
	for _, hit := range hedgeWordList.findAll(u.Text) {
		word := hit.Entry
		if strings.ToUpper(word) == word && len(word) > 1 {
			continue // RFC 2119 keyword
		}
		lower := strings.ToLower(word)
		add(found, u, path, surface, "hedging", quoted(lower), fixOf(hedges, lower))
	}
	for _, hit := range hedgePhraseList.findAll(u.Text) {
		phrase := strings.ToLower(hit.Entry)
		add(found, u, path, surface, "hedging", quoted(phrase), fixOf(hedgePhrases, phrase))
	}
}

// checkPlainWords finds the formal word where a plain one exists, under habit 1.
func checkPlainWords(u unit, path, surface string, found *[]Finding) {
	for _, hit := range plainList.findAll(u.Text) {
		raw := hit.Entry
		if isProtocolIdentifier(raw) && !sentenceBoundaryBefore(u.Text[:hit.Start]) {
			continue // a protocol identifier, not the verb
		}
		word := lowerFields(raw)
		var tb textbuf.Buffer
		fix := tb.Str(`use the plain word: "`).Str(fixOf(plainWords, word)).Byte('"').String()
		add(found, u, path, surface, "synonym-rotation", quoted(word), fix)
	}
}

// isProtocolIdentifier reports whether the matched text is a plain word that is
// also a protocol identifier, where the capital letter is the tell.
func isProtocolIdentifier(raw string) bool {
	return slices.Contains(plainWordIdentifiers, raw)
}

// sentenceBoundaryBefore reports whether the text before a match is the end of
// the preceding sentence, or the start of the unit. A sentence-initial capital
// is not evidence of a proper noun.
func sentenceBoundaryBefore(prefix string) bool {
	trimmed := pyRStrip(prefix)
	if trimmed == "" {
		return true
	}
	last := trimmed[len(trimmed)-1]
	return last == '.' || last == '!' || last == '?' || last == ':'
}

// checkLatin finds a Latin abbreviation, reported under hedging.
func checkLatin(u unit, path, surface string, found *[]Finding) {
	lowered := strings.ToLower(u.Text)
	for _, entry := range latinAbbreviations {
		if !strings.Contains(lowered, entry.Word) {
			continue
		}
		var tb textbuf.Buffer
		fix := tb.Str(`write "`).Str(entry.Fix).Byte('"').String()
		add(found, u, path, surface, "hedging", quoted(entry.Word), fix)
	}
}

// restrictedMeaning is an approved word used outside its approved meaning.
// `above` and `below` are physical positions, never limits.
type restrictedMeaning struct {
	Detail string
	Fix    string
	Match  func(string) bool
}

var restrictedMeanings = []restrictedMeaning{
	{`"above" for a limit`, `write "more than"`, func(text string) bool {
		return hasBounded(aboveLimitRe, text, edgeNone)
	}},
	{`"below" for a limit`, `write "less than"`, func(text string) bool {
		return hasBounded(belowLimitRe, text, edgeNone)
	}},
}

var (
	aboveLimitRe = mustPattern(`(?i)above{SP}+{D}`)
	belowLimitRe = mustPattern(`(?i)below{SP}+{D}`)
)

// checkRestrictedMeanings reports an approved word used for something it does
// not mean, under habit 1.
func checkRestrictedMeanings(u unit, path, surface string, found *[]Finding) {
	for _, meaning := range restrictedMeanings {
		if meaning.Match(u.Text) {
			add(found, u, path, surface, "synonym-rotation", meaning.Detail, meaning.Fix)
		}
	}
}

// articleBeforeIDRe is a definite article before a noun that carries an
// alphanumeric identifier. The identifier makes the noun proper, so the article
// is incorrect. Kept to this repository's own nouns, because a general
// `the \w+ \d` pattern would flag "the first 3 bytes".
var articleBeforeIDRe = mustPattern(`(?i)the{SP}+(?:RFC|AS|ASN|table|port|peer|prefix|section|chapter|figure|step|rule){SP}+{D}`)

// checkArticles reports the article before a proper noun, under habit 1.
func checkArticles(u unit, path, surface string, found *[]Finding) {
	for _, loc := range findBounded(articleBeforeIDRe, u.Text, edgeNone) {
		detail := quoted(pyJoinFields(u.Text[loc[0]:loc[1]]))
		add(found, u, path, surface, "synonym-rotation", detail,
			`an alphanumeric identifier makes the noun proper: drop "the"`)
	}
}

// gerundClauseRe matches a gerund clause. An `-ing` form is permitted only as a
// technical noun (`routing table`) or its modifier (`switching relay`). A
// preceding preposition identifies the banned form. It reports frozen-verbs.
var gerundClauseRe = mustPattern(`(?i)(before|after|while|without|when){SP}+([a-z]+ing)`)

// checkFrozenVerbs finds habit 3.
func checkFrozenVerbs(u unit, path, surface string, found *[]Finding) {
	for _, loc := range findBounded(gerundClauseRe, u.Text, edgeWord) {
		verb := strings.ToLower(u.Text[loc[4]:loc[5]])
		if isNotGerund(verb) {
			continue
		}
		preposition := strings.ToLower(u.Text[loc[2]:loc[3]])
		var detail textbuf.Buffer
		var fix textbuf.Buffer
		add(found, u, path, surface, "frozen-verbs",
			detail.Byte('"').Str(preposition).Byte(' ').Str(verb).Byte('"').String(),
			fix.Str(`name the actor: "`).Str(preposition).Str(` you <verb>". The -ing form`).
				Str(" is permitted only as a technical noun or its modifier").String())
	}

	for _, loc := range findBounded(lightVerbRe, u.Text, edgeWord) {
		verb := nominalizedVerb[strings.ToLower(u.Text[loc[4]:loc[5]])]
		var fix textbuf.Buffer
		add(found, u, path, surface, "frozen-verbs",
			quoted(pyJoinFields(u.Text[loc[0]:loc[1]])),
			fix.Str(`use the verb "`).Str(verb).Byte('"').String())
	}
	for _, loc := range findBounded(frozenOfRe, u.Text, edgeWord) {
		verb := nominalizedVerb[strings.ToLower(u.Text[loc[4]:loc[5]])]
		var fix textbuf.Buffer
		add(found, u, path, surface, "frozen-verbs",
			quoted(pyJoinFields(u.Text[loc[0]:loc[1]])),
			fix.Str(`use the verb: "`).Str(u.Text[loc[2]:loc[3]]).Str(" you ").Str(verb).
				Str(` ..."`).String())
	}
}

// isNotGerund reports whether a word ending in `-ing` is one of the words that
// is not a gerund.
func isNotGerund(word string) bool {
	return slices.Contains(notGerund, word)
}

// checkMarketing finds habit 4.
func checkMarketing(u unit, path, surface string, found *[]Finding) {
	for _, hit := range marketingList.findAll(u.Text) {
		word := strings.ToLower(hit.Entry)
		add(found, u, path, surface, "marketing-adjectives", quoted(word),
			"give the number, the limit, or the mechanism")
	}
}

// checkPhrasal finds habit 6.
func checkPhrasal(u unit, path, surface string, found *[]Finding) {
	for _, hit := range phrasalList.findAll(u.Text) {
		phrase := lowerFields(hit.Entry)
		var fix textbuf.Buffer
		add(found, u, path, surface, "phrasal-verbs", quoted(phrase),
			fix.Str(`use one verb: "`).Str(fixOf(phrasalVerbs, phrase)).Byte('"').String())
	}
}

// checkRunOns finds habit 5: a sentence past its word bound, a semicolon, and a
// paragraph past its sentence bound.
func checkRunOns(u unit, path, surface string, found *[]Finding) {
	limit := MaxDescriptiveWords
	rule := "6.3"
	if u.Procedural {
		limit = maxProcedural
		rule = "5.1"
	}

	parts := Sentences(u.Text)
	for _, sentence := range parts {
		// No cheap prefilter here. STE counting usually LOWERS the total
		// (Rules 8.5 through 8.7), but "Stop()" becomes two tokens once the
		// parenthesis is collapsed, so a whitespace count can undercount.
		// Measuring 42 fewer run-ons is worse than measuring them 3 seconds
		// slower.
		count := WordCount(sentence)
		if count > limit {
			var detail textbuf.Buffer
			add(found, u, path, surface, "run-ons",
				detail.Int(int64(count)).Str(" words (STE Rule ").Str(rule).Str(" allows ").
					Int(int64(limit)).Byte(')').String(),
				"one topic per sentence, or a vertical list (Rule 4.3)", sentence)
		}
		if strings.Contains(sentence, ";") {
			add(found, u, path, surface, "run-ons", "semicolon (STE Rule 8.1)",
				"write two sentences, or a vertical list", sentence)
		}
	}

	if u.Paragraph && len(parts) > maxSentencesPerParagraph {
		var detail textbuf.Buffer
		add(found, u, path, surface, "run-ons",
			detail.Int(int64(len(parts))).
				Str(" sentences in one paragraph (STE Rule 6.6 allows 6)").String(),
			"split the paragraph", parts[0])
	}
}

// checkSynonyms reports one finding for each concept a document rotates names
// for.
func checkSynonyms(all []unit, path, surface string, found *[]Finding) {
	texts := make([]string, len(all))
	for index := range all {
		texts[index] = all[index].Text
	}
	lowered := strings.ToLower(strings.Join(texts, " "))

	for _, set := range termSets {
		var seen []string
		for _, rotation := range set.Rotations {
			if rotationList[rotation].has(lowered) {
				seen = append(seen, rotation)
			}
		}
		if len(seen) == 0 {
			continue
		}
		usesCanonical := canonicalList[set.Canonical].has(lowered)
		if !usesCanonical && len(seen) <= 1 {
			continue
		}

		line := 1
		for index := range all {
			unitText := strings.ToLower(all[index].Text)
			hit := false
			for _, rotation := range seen {
				if rotationList[rotation].has(unitText) {
					hit = true
					break
				}
			}
			if hit {
				line = all[index].Line
				break
			}
		}

		var detail textbuf.Buffer
		detail.Join(seen, ", ")
		if usesCanonical {
			detail.Str(" beside ").Str(set.Canonical)
		}
		var fix textbuf.Buffer
		*found = append(*found, Finding{
			File: path, Line: line, Surface: surface, Habit: "synonym-rotation",
			Number: habitNumber("synonym-rotation"), Detail: detail.String(),
			Fix: fix.Str(`use "`).Str(set.Canonical).Str(`" every time`).String(),
		})
	}
}

// extract answers the reviewable units of one document.
func extract(text, surface string) []unit {
	switch surface {
	case SurfaceYANG:
		return unitsYANG(text)
	case SurfaceGo:
		return unitsGo(pySplitLines(text))
	default:
		return unitsMarkdown(pySplitLines(blankHTMLComments(text)))
	}
}

// hasReason reports whether an opt-out states one.
//
// The marker must have a reason. Otherwise, silent exemptions accumulate. The
// script fails to enforce this. Its `(?P<reason>.+?)\s*(?:-->|$)` pattern treats
// the terminator in `<!-- ste: ignore-file -->` as the reason. The `$` branch
// then closes the match, and the empty exemption covers the document.
//
// Terminator-only text is not a reason. The journal class is
// escape-hatch-scoped-wider-than-its-justification.
func hasReason(reason string) bool {
	return strings.ContainsFunc(reason, func(r rune) bool {
		return r != '-' && r != '>' && !pySpace(r)
	})
}

// Review answers every finding in one document, and the reason it was skipped
// when it was skipped.
func Review(path, text, surface string) (findings []Finding, skipReason string) {
	if loc := ignoreFileRe.FindStringSubmatchIndex(text); loc != nil {
		group := ignoreFileRe.SubexpIndex("reason")
		if reason := text[loc[2*group]:loc[2*group+1]]; hasReason(reason) {
			return nil, reason
		}
	}
	head := pySplitLines(text)
	if len(head) > 8 {
		head = head[:8]
	}
	joined := strings.Join(head, "\n")
	for _, marker := range generatedMarkers {
		if strings.Contains(joined, marker) {
			return nil, "generated file"
		}
	}

	var found []Finding
	all := extract(text, surface)
	for _, u := range all {
		checkHedging(u, path, surface, &found)
		checkPlainWords(u, path, surface, &found)
		checkLatin(u, path, surface, &found)
		checkRestrictedMeanings(u, path, surface, &found)
		checkArticles(u, path, surface, &found)
		checkFrozenVerbs(u, path, surface, &found)
		checkMarketing(u, path, surface, &found)
		checkPhrasal(u, path, surface, &found)
		checkRunOns(u, path, surface, &found)
	}
	if surface != SurfaceGo { // per-file term consistency is a document property
		checkSynonyms(all, path, surface, &found)
	}

	// By line and then by habit name, which is the script's
	// `sort(key=lambda f: (f.line, f.habit))`. The sort is STABLE, so two
	// findings agreeing on both keys keep the order the detectors produced.
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Line != found[j].Line {
			return found[i].Line < found[j].Line
		}
		return found[i].Habit < found[j].Habit
	})
	return found, ""
}
