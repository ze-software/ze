// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet, {feature-declined}
// Related: check_evidence.go -- featureShapeGap, which routes a conditional quote here
// Related: notapplicable_roles.go -- genericRoleWords, roleDeterminers and singularLetters, the role vocabulary a never clause must go beyond
package rfc

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// conditionalLeads are the lower-case words that open a clause guarding an
// obligation on a condition ("If the client uses a delegated prefix ...").
var conditionalLeads = []string{"if", "when", "where", "once", "unless"}

// relativeSubjectRE reads a subject held by a restrictive relative clause, in
// lower case: an optional article, then up to four words, the last before
// `that` ("a client that", "the client that holds a lease", "receivers
// that"). The article is group 1, the words between it and the last group 2,
// and the word before `that` group 3.
var relativeSubjectRE = regexp.MustCompile(`^(?:(a|an|the)\s+)?((?:[\w-]+\s+){0,3})([\w-]+)\s+that\s`)

// contentClauseHeads are the words that, before `that`, open a content clause
// rather than a relative one: a copula ("the rule is that") or a verb of
// saying or requiring ("this means that", "the document requires that"). A
// clause whose word before `that` is one of them holds no subject, with an
// article or without one.
var contentClauseHeads = []string{
	"are", "assumes", "be", "clarifies", "defines", "ensures", "expects", "follows", "guarantees",
	"implies", "indicates", "is", "mandates", "means", "notes", "recommends", "requires", "says",
	"shows", "specifies", "states", "suggests", "was", "were",
}

// subjectPrepositions are the prepositions that, between a subject's head and
// `that`, show the relative clause qualifies the preposition's object rather
// than the subject: in "a receiver of the frame that arrived" the frame
// arrived, and the receiver is held by no condition.
var subjectPrepositions = []string{"for", "from", "in", "of", "on", "to", "with"}

// conditionalGuard answers the guard clause of quote's obligation, and whether
// one guards it: before its first mandatory BCP 14 keyword, at the sentence's
// start or after a comma ("As a counterexample, a client that ..."; "In
// particular, if ..."), a clause opens with a conditionalLeads word or is a
// subject held by a relative clause (conditionalClause). The first mandatory
// keyword, not the first keyword, bounds it, so in "A sender MAY pad a frame,
// and, if it does, it MUST mark it" the MUST is read as guarded. The loop is
// bounded by the commas of the text before the keyword: each pass moves start
// past one.
func conditionalGuard(quote string) (string, bool) {
	at := firstMandatoryKeyword(quote)
	if at < 0 {
		return "", false
	}
	prefix := strings.ToLower(quote[:at])
	for start := 0; ; {
		if guard, ok := conditionalClause(strings.TrimLeft(prefix[start:], " ")); ok {
			return guard, true
		}
		comma := strings.IndexByte(prefix[start:], ',')
		if comma < 0 {
			return "", false
		}
		start += comma + 1
	}
}

// firstMandatoryKeyword answers the byte offset of the first mandatory BCP 14
// keyword in quote as a whole word, -1 when it holds none.
func firstMandatoryKeyword(quote string) int {
	at := -1
	for _, level := range gatedLevelNames() {
		found := wordIndex(quote, level)
		if found < 0 {
			continue
		}
		if at < 0 || found < at {
			at = found
		}
	}
	return at
}

// conditionalClause answers the guard of one lower-case clause, and whether it
// opens a condition. A conditionalLeads word opens one, and its guard is the
// text after the word. A relative-clause subject (relativeSubjectRE) opens one
// when its word before `that` is no contentClauseHeads word and, with no
// article, is plural, so "receivers that" counts and "notify the receiver
// that" does not; when no subjectPrepositions word sits between the subject
// and `that`; and when the relative clause holds words before the keyword, so
// "a 1 octet field that MUST be set" is a description, not a condition. Its
// guard is the text after `that`.
func conditionalClause(clause string) (string, bool) {
	first, rest, _ := strings.Cut(clause, " ")
	if slices.Contains(conditionalLeads, strings.TrimRight(first, ",")) {
		return trimGuard(rest), true
	}
	parts := relativeSubjectRE.FindStringSubmatch(clause)
	if parts == nil {
		return "", false
	}
	head := parts[3]
	if slices.Contains(contentClauseHeads, head) {
		return "", false
	}
	if parts[1] == "" && !strings.HasSuffix(head, "s") {
		return "", false
	}
	for _, word := range append(strings.Fields(parts[2]), head) {
		if slices.Contains(subjectPrepositions, word) {
			return "", false
		}
	}
	guard := trimGuard(clause[len(parts[0]):])
	if guard == "" {
		return "", false
	}
	return guard, true
}

// trimGuard cuts the spaces and commas around a guard clause.
func trimGuard(guard string) string {
	return strings.Trim(guard, " ,")
}

// neverClauseRE reads the never clause of a conditional's reason, `ze never
// <condition>: <producer> <how>`, opening the reason or after a `;`. The
// condition is group 1, the producer group 2 (producerRE's whole match), and
// how Ze avoids the condition the last group.
var neverClauseRE = regexp.MustCompile(`(?:^|;)\s*ze never ([^:;]*[^:;\s])\s*:\s*(` + producerRE.String() + `)\s+([^;\s][^;]*)`)

// guardStopwords are the words that carry no condition of their own, so a
// never clause sharing only these with the guard clause names no condition
// the quote states. Closed: a word it does not hold is a content word, unless
// it is a role word (genericRoleWords) or a determiner (roleDeterminers).
const guardStopwords = "and are as at be been being by can do does for from had has have if in into is it its may no nor not of on or so such than then them they to was were which who will with would"

// neverScope is what a never clause answers for: the shape of evidence that
// owes it, opening the refusal of a reason with none ("its quote '...' is a
// conditional obligation"); the words the never clause's condition must share
// a content word with (the quote's guard clause, or a heading form's feature
// words); and what those words are, for the refusal that names them.
type neverScope struct {
	opening, evidence, against string
}

// neverClauseGap judges the reason of a {feature-declined} conditional or
// heading form: the text after its quote must carry a never clause
// (neverClauseRE), whose condition shares a content word with scope.evidence
// (sharedContentWord), so the condition Ze never meets is the one the quote
// guards its obligation on, or the feature the heading names; and the
// producer it names must be a function this tree declares outside a test
// (resolveProducer, the resolution every producer check shares), so the claim
// rests on code a reader can open.
func neverClauseGap(req *Requirement, code *sourceReader, scope neverScope) (string, bool) {
	// The reason is the whole payload, and its first quoted segment is the
	// quote or the feature words, which hold no quotation mark
	// (featureDeclinedRE): the never clause is read after that segment's
	// closing mark, so a never clause inside the quotation marks proves
	// nothing, however the parser trimmed the words.
	why := req.Annotation.Reason
	if open := strings.IndexByte(why, '"'); open >= 0 {
		if closing := strings.IndexByte(why[open+1:], '"'); closing >= 0 {
			why = why[open+1+closing+1:]
		}
	}
	var tb textbuf.Buffer
	parts := neverClauseRE.FindStringSubmatch(why)
	if parts == nil {
		return tb.Str(scope.opening).
			Str(", and its reason carries no never clause, so nothing shows that Ze never meets the condition. Write ").
			Str(neverClauseForm).Str(" in the reason, naming the function that shows it").String(), true
	}
	condition, producer := parts[1], parts[2]
	if !sharedContentWord(condition, scope.evidence) {
		return tb.Str("its never clause's condition ").Str(pyRepr(condition)).
			Str(" shares no word with ").Str(scope.against).Str(" ").Str(pyRepr(scope.evidence)).
			Str(", so it names another condition than the one the obligation rests on. ").
			Str("Write the condition in those words: ").Str(neverClauseForm).String(), true
	}
	if strings.HasSuffix(producerFile(producer), "_test.go") {
		return tb.Str("its never clause names the test ").Str(producer).
			Str(". A test decides nothing about what Ze does; name the code that shows Ze never ").Str(condition).String(), true
	}
	state, unresolved := resolveProducer(code, producer)
	switch state {
	case producerFound:
		return "", false
	case producerUnnamed:
		// Unreachable: neverClauseRE embeds producerRE. Refused rather than
		// passed, because a guard that skips a state it cannot read is no guard.
		return tb.Str("its never clause names ").Str(pyRepr(producer)).
			Str(", which is not <path>.go::<Symbol>. Write ").Str(neverClauseForm).String(), true
	case producerFileAbsent:
		return tb.Str("its never clause names the producer ").Str(producer).
			Str(", whose file this checkout does not carry. Name the function that shows Ze never ").Str(condition).String(), true
	case producerUnresolved:
		return tb.Str("its never clause names the producer ").Str(producer).Str(", but ").
			Str(unresolved).Str(". Name the one function that shows Ze never ").Str(condition).
			Str(" today").String(), true
	default:
		panic("BUG: unexpected producer resolution state")
	}
}

// sharedContentWord answers whether condition and guard hold one content word
// in common, compared in lower case and singular (singularLetters), so
// "configures an address" meets "configure addresses". A content word is one
// guardStopwords, roleDeterminers and genericRoleWords do not hold.
func sharedContentWord(condition, guard string) bool {
	in := contentWords(guard)
	for _, word := range contentWords(condition) {
		if slices.Contains(in, word) {
			return true
		}
	}
	return false
}

// contentWords answers the content words of text (sharedContentWord), each
// lower case and singular.
func contentWords(text string) []string {
	skip := strings.Fields(guardStopwords + " " + roleDeterminers + " " + genericRoleWords)
	var words []string
	for _, word := range strings.FieldsFunc(strings.ToLower(text), isNotWordRune) {
		if slices.Contains(skip, word) {
			continue
		}
		singular := singularLetters(word)
		if slices.Contains(skip, singular) {
			continue
		}
		words = append(words, singular)
	}
	return words
}

// isNotWordRune answers whether r separates words: anything but a letter, a
// digit or a hyphen.
func isNotWordRune(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
}
