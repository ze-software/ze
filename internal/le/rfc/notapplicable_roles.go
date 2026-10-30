// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet
// Overview: summary.go -- parseNotApplicable, which reads the quote and the bound role these functions judge
// Related: meta.go -- zeRolesFrom, the `Ze roles` row every bound role is held against
// Related: check_evidence.go -- containsWords and wordIndex, the whole-word matchers
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// genericRoleWords are the words that name a party to any protocol rather
// than a role one RFC defines. A bound role made of these words alone is
// refused: "an implementation" or "a router" binds Ze nearly everywhere, so a
// row that excludes Ze through one claims nothing a reader can check. The RFC
// that scopes an obligation names the specific role (a route reflector, a
// DHCP relay agent), and the row must name it too. Compared after roleKey, so
// a plural and an article are the same word.
const genericRoleWords = "implementation implementations node host system entity device party peer speaker router endpoint sender receiver"

// roleDeterminers are the determiners and quantifiers that pick out some
// members of a role without naming another: roleKey drops them from the front
// of a role, so "each remote speaker" is the role "speaker", and they may stand
// between the subject of a quoted obligation and its keyword ("Speakers all
// MUST"). "local" and "remote" say which end of a session, never which role.
const roleDeterminers = "a an the this that these those each every any all other some local remote"

// rolePronouns are the words that stand for a party named elsewhere. A quoted
// obligation whose subject is one of them names no role, so the sentence that
// names the party is the one to quote.
const rolePronouns = "it they this that these those he she we you one which who them"

// bcp14Keywords are the BCP 14 keywords as an RFC writes them, upper case. A
// quote holding one is an obligation, and the party it binds is its subject:
// the words directly before the first keyword.
var bcp14Keywords = []string{
	"MUST NOT", "MUST", "REQUIRED", "SHALL NOT", "SHALL", "SHOULD NOT", "SHOULD",
	"NOT RECOMMENDED", "RECOMMENDED", "MAY", "OPTIONAL",
}

// firstKeyword answers the byte offset and spelling of the first BCP 14
// keyword in quote as a whole word, -1 when it holds none. Where two keywords
// start at one offset the longer wins, so "MUST NOT" is not read as "MUST".
func firstKeyword(quote string) (int, string) {
	at, keyword := -1, ""
	for _, candidate := range bcp14Keywords {
		found := wordIndex(quote, candidate)
		if found < 0 {
			continue
		}
		if at < 0 || found < at || (found == at && len(candidate) > len(keyword)) {
			at, keyword = found, candidate
		}
	}
	return at, keyword
}

// boundRoleRefusal judges the role a quoted {not-applicable} row binds against
// its quote, answering the refusal sentence, which the caller ends with the
// format. When the quote holds a BCP 14 keyword, the bound role must be its
// subject: the words directly before the first keyword, with only determiners
// and quantifiers (roleDeterminers) between, and a subject that is a pronoun
// is refused because it names no role. A quote with no keyword is a scoping
// sentence ("This section applies only to relay agents."), and the bound role
// need only be in it as whole words. Either way a role of generic words alone
// is refused by the caller, and a role ze fills by checkNotApplicableRoles.
func boundRoleRefusal(quote, bound string) (string, bool) {
	var tb textbuf.Buffer
	at, keyword := firstKeyword(quote)
	if at < 0 {
		if containsWords(roleKey(quote), roleKey(bound)) {
			return "", false
		}
		return tb.Str(": {not-applicable} binds ").Str(pyRepr(bound)).
			Str(", which is not in the quoted sentence; name the role the sentence itself names, or quote the sentence that names this one. ").String(), true
	}
	subject := roleWords(quote[:at])
	if len(subject) > 0 && slices.Contains(strings.Fields(rolePronouns), subject[len(subject)-1]) {
		return tb.Str(": {not-applicable} quotes an obligation whose subject is the pronoun ").Str(pyRepr(subject[len(subject)-1])).
			Str(" before ").Str(keyword).Str(", which names no role. ").Str(subjectAdvice).String(), true
	}
	for len(subject) > 0 && slices.Contains(strings.Fields(roleDeterminers), subject[len(subject)-1]) {
		subject = subject[:len(subject)-1]
	}
	if len(subject) > 0 && slices.Contains(strings.Fields(rolePronouns), subject[len(subject)-1]) {
		return tb.Str(": {not-applicable} quotes an obligation whose subject is the pronoun ").Str(pyRepr(subject[len(subject)-1])).
			Str(" before ").Str(keyword).Str(", which names no role. ").Str(subjectAdvice).String(), true
	}
	role := strings.Fields(roleKey(bound))
	if len(subject) >= len(role) && slices.Equal(subject[len(subject)-len(role):], role) {
		return "", false
	}
	tb.Str(": {not-applicable} binds ").Str(pyRepr(bound)).Str(", which is not the subject of the quoted obligation: ")
	if len(subject) == 0 {
		tb.Str("no word stands before its first keyword ").Str(keyword)
	} else {
		tb.Str("the words before its first keyword ").Str(keyword).Str(" are ").Str(pyRepr(strings.Join(subject, " ")))
	}
	return tb.Str(", and an obligation binds the party it names as its subject. ").Str(subjectAdvice).String(), true
}

// subjectAdvice is what a refusal of a quoted obligation's subject tells the
// author to quote instead.
const subjectAdvice = "Quote the sentence of the cited section that names the bound role as the subject of its keyword, or one that scopes the section to that role (\"This section applies only to relay agents.\" binds relay agents). "

// roleWords is text as a subject is compared: lower case, each word with the
// punctuation around it cut and singular (singularLetters), empty words
// dropped. "routers," and "Routers" are the one word "router".
func roleWords(text string) []string {
	var words []string
	for word := range strings.FieldsSeq(strings.ToLower(text)) {
		word = strings.TrimFunc(word, func(r rune) bool { return !isWordRune(r) })
		if word == "" {
			continue
		}
		words = append(words, singularLetters(word))
	}
	return words
}

// genericRole answers whether every word of role is one of genericRoleWords,
// so the role names no party a reader could tell from ze.
func genericRole(role string) bool {
	words := strings.Fields(roleKey(role))
	if len(words) == 0 {
		return false
	}
	generic := strings.Fields(roleKey(genericRoleWords))
	for _, word := range words {
		if !slices.Contains(generic, word) {
			return false
		}
	}
	return true
}

// roleKey is a role, or the text a role is looked for in, as two roles
// compare: lower case, single-spaced, with no leading determiner or
// quantifier (roleDeterminers) and each word singular, so "a speaker", "The
// Speaker", "every speaker" and "speakers" name one role. A word ending in
// "ies" ends in "y" ("proxies", "entities"); otherwise a word loses a trailing
// "es" after s, x, z, ch or sh ("classes", "boxes"), or a trailing "s" that
// does not follow another ("routers", but not "class"). Both sides of every
// comparison pass through it, so a word it cuts wrongly ("status") is cut the
// same way on both.
func roleKey(role string) string {
	words := strings.Fields(strings.ToLower(role))
	determiners := strings.Fields(roleDeterminers)
	for len(words) > 1 && slices.Contains(determiners, words[0]) {
		words = words[1:]
	}
	for i, word := range words {
		words[i] = singularWord(word)
	}
	return strings.Join(words, " ")
}

// singularWord cuts one trailing plural from a lower-case word, as roleKey
// describes. Punctuation after the word ("frames." closing a quoted
// sentence) is kept and does not hide the plural.
func singularWord(word string) string {
	trimmed := strings.TrimRightFunc(word, func(r rune) bool { return !isWordRune(r) })
	return singularLetters(trimmed) + word[len(trimmed):]
}

// singularLetters is singularWord over a word with no trailing punctuation.
func singularLetters(word string) string {
	if stem, plural := strings.CutSuffix(word, "ies"); plural && len(stem) > 1 {
		return stem + "y"
	}
	if stem, plural := strings.CutSuffix(word, "es"); plural && len(stem) > 1 {
		for _, sibilant := range []string{"s", "x", "z", "ch", "sh"} {
			if strings.HasSuffix(stem, sibilant) {
				return stem
			}
		}
	}
	if stem, plural := strings.CutSuffix(word, "s"); plural && len(stem) > 1 && !strings.HasSuffix(stem, "s") {
		return stem
	}
	return word
}

// qualifiedGenericRole answers the declared Ze role a bound role is a generic
// head noun qualified only by that role's words: "BGP implementation" against
// "BGP speaker". The head ("implementation") names any party and the
// qualifier ("BGP") names nothing ze does not also do, so the bound role may
// be ze itself. Empty when the role has no qualifier, a head of its own, or a
// qualifier word no declared role holds.
func qualifiedGenericRole(bound string, roles []string) string {
	words := strings.Fields(roleKey(bound))
	if len(words) < 2 || !genericRole(words[len(words)-1]) {
		return ""
	}
	for _, role := range roles {
		held := strings.Fields(roleKey(role))
		qualified := true
		for _, qualifier := range words[:len(words)-1] {
			if !slices.Contains(held, qualifier) {
				qualified = false
				break
			}
		}
		if qualified {
			return role
		}
	}
	return ""
}

// checkNotApplicableRoles holds the role each quoted {not-applicable} row
// binds against the roles the summary's Meta table declares Ze fills
// (Meta.ZeRoles), so a row cannot exclude Ze from an obligation on a role Ze
// has. A summary with such a row and no `Ze roles` row is refused, naming the
// row to add: the roles are declared once per summary, never per row, so two
// rows cannot give Ze two different roles. A bound role is refused when it
// equals a declared role, holds one, or is held by one, as whole words after
// roleKey ("BGP speakers" binds ze when ze is a "BGP speaker", and so does
// "speaker"), and when it is a generic head noun qualified only by a declared
// role's words (qualifiedGenericRole). The Meta table is read only when a
// quoted row exists, so a summary with none is not held to it here.
func checkNotApplicableRoles(text, stem, source string, requirements []Requirement) error {
	where := source
	if where == "" {
		where = stem
	}
	var roles []string
	read := false
	for i := range requirements {
		req := &requirements[i]
		if req.Annotation == nil || req.Annotation.Kind != AnnotationNotApplicable || req.Annotation.BoundRole == "" {
			continue
		}
		if !read {
			read = true
			meta, err := ParseMeta(text, stem, where)
			if err != nil {
				return err
			}
			roles = meta.ZeRoles
		}
		var tb textbuf.Buffer
		tb.Str(where).Byte(':').Int(int64(req.Line)).Str(": ").Str(req.RID).Str(" {not-applicable} binds ").
			Str(pyRepr(req.Annotation.BoundRole))
		if len(roles) == 0 {
			return parseErr(tb.Str(", and ").Str(stem).Str(" declares no role ze fills, so nothing shows the role is not ze's. Add the row `| ").
				Str(metaZeRoles).Str(" | <role>, <role> |` to the `## Meta` table of ").Str(where).
				Str(", naming every role ze fills for this RFC. ").Str(notApplicableFormat))
		}
		bound := roleKey(req.Annotation.BoundRole)
		for _, role := range roles {
			key := roleKey(role)
			if !containsWords(key, bound) && !containsWords(bound, key) {
				continue
			}
			return parseErr(tb.Str(", and ").Str(stem).Str(" declares ze is ").Str(pyRepr(role)).
				Str(" in its Meta `").Str(metaZeRoles).
				Str("` row: one role holds the other, so the quoted sentence may bind ze and the row is owed, not excluded. Annotate it {gap: ...} or test it. ").
				Str(subjectAdvice).Str(notApplicableFormat))
		}
		if role := qualifiedGenericRole(req.Annotation.BoundRole, roles); role != "" {
			return parseErr(tb.Str(", a word for any party qualified only by words of the role ").Str(pyRepr(role)).
				Str(" that ").Str(stem).Str(" declares ze fills in its Meta `").Str(metaZeRoles).
				Str("` row, so the bound role may be ze itself. Annotate it {gap: ...} or test it. ").
				Str(subjectAdvice).Str(notApplicableFormat))
		}
	}
	return nil
}
