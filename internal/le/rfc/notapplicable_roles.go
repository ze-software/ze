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
// than a role one RFC defines. A bound role made of these words alone, once
// nonRoleAdjectives are set aside, is refused: "an implementation" or "a
// compliant router" binds Ze nearly everywhere, so a row that excludes Ze
// through one claims nothing a reader can check. The RFC that scopes an
// obligation names the specific role (a route reflector, a DHCP relay agent),
// and the row must name it too. Compared after roleKey, so a plural and an
// article are the same word.
const genericRoleWords = "implementation implementations node host system entity device party peer speaker router endpoint sender receiver"

// nonRoleAdjectives are the adjectives that say how well, or in which
// direction, a party behaves, never which role it fills. genericRole sets them
// aside before it judges a role, so "a compliant implementation" and "a
// receiving speaker" are as generic as "an implementation". The list is
// closed: an adjective it does not hold is read as part of the role.
const nonRoleAdjectives = "compliant conforming conformant robust receiving sending transmitting originating new existing standard"

// roleDeterminers are the determiners and quantifiers that pick out some
// members of a role without naming another: roleKey drops them from the front
// of a role, so "each remote speaker" is the role "speaker", and they may
// stand before the subject of a quoted obligation or between it and its
// keyword ("Speakers all MUST"). "local" and "remote" say which end of a
// session, never which role.
const roleDeterminers = "a an the this that these those each every any all other some local remote"

// rolePronouns are the words that stand for a party named elsewhere. A quoted
// obligation whose subject is one of them names no role, so the sentence that
// names the party is the one to quote.
const rolePronouns = "it they this that these those he she we you one which who them"

// subjectBreakers are the words that make the subject of a quoted obligation
// more than one role: a conjunction joins two parties ("Clients and servers
// MUST"), a relative pronoun opens a clause that narrows or widens the party
// ("A router that receives ... MUST"), a preposition qualifies it by another
// party ("A speaker of a confederation MUST"), and "not" negates it. A
// subject holding one, or a comma, is refused rather than parsed: no reader
// can tell from the words alone which party the keyword binds.
const subjectBreakers = "and or nor that which who of from to by with not"

// scopingMarkers are the words, each matched as whole words without case, that
// make a sentence with no BCP 14 keyword scope its section to a role: "This
// section applies only to relay agents." A sentence with no keyword and none
// of them scopes nothing, so it is not role-condition evidence.
var scopingMarkers = []string{"apply", "applies", "applicable", "only", "solely", "limited to", "restricted to"}

// bcp14Keywords are the BCP 14 keywords as an RFC writes them, upper case:
// every level rfc.go declares, gated and advisory, read from those two sets
// so the vocabulary is declared once. A quote holding one is an obligation,
// and the party it binds is its subject: the words before the first keyword.
var bcp14Keywords = append(gatedLevelNames(), advisoryLevelNames()...)

// notApplicableShapes names the two quotes a {not-applicable} row may carry,
// for every refusal of a quote of neither shape.
const notApplicableShapes = "A {not-applicable} quote is one of two shapes: the row's own obligation sentence, whose subject is the bound role, or a sentence of the cited section with no BCP 14 keyword that scopes it to the role with apply, applies, applicable, only, solely, limited to or restricted to. "

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
// format. A quote holding a BCP 14 keyword is shape (a), the row's own
// obligation sentence, and its subject must be the bound role
// (subjectRefusal); the check that it is the row's own sentence needs the row
// and runs in (*evidenceReader).gap. A quote with no keyword is shape (b), a
// scoping sentence (scopingRefusal). Either way a role of generic words alone
// is refused by the caller, and a role ze fills by checkNotApplicableRoles.
func boundRoleRefusal(quote, bound string) (string, bool) {
	at, keyword := firstKeyword(quote)
	if at < 0 {
		return scopingRefusal(quote, bound)
	}
	return subjectRefusal(quote[:at], keyword, bound)
}

// scopingRefusal judges a quote with no BCP 14 keyword: it must carry a
// scoping marker (scopingMarkers), and the bound role must be in it as whole
// words after roleKey.
func scopingRefusal(quote, bound string) (string, bool) {
	var tb textbuf.Buffer
	lower := strings.ToLower(quote)
	if !slices.ContainsFunc(scopingMarkers, func(marker string) bool { return containsWords(lower, marker) }) {
		return tb.Str(": {not-applicable} quotes ").Str(pyRepr(quote)).
			Str(", which holds no BCP 14 keyword and scopes nothing to a role. ").Str(notApplicableShapes).
			Str("Otherwise annotate the row {gap: ...}. ").String(), true
	}
	if containsWords(roleKey(quote), roleKey(bound)) {
		return "", false
	}
	return tb.Str(": {not-applicable} binds ").Str(pyRepr(bound)).
		Str(", which is not in the quoted sentence; name the role the sentence itself names, or quote the sentence that names this one. ").String(), true
}

// subjectRefusal judges the subject of a quoted obligation, the words before
// its first keyword, against the bound role. Determiners and quantifiers
// (roleDeterminers) may stand before the role and between it and the keyword;
// nothing else may. A subject holding a comma or a subjectBreakers word is
// refused, since it names more than one party or qualifies the party by a
// clause, and so is a subject that is a pronoun, which names no role.
func subjectRefusal(subject, keyword, bound string) (string, bool) {
	var tb textbuf.Buffer
	words := roleTokens(subject)
	breakers := strings.Fields(subjectBreakers)
	breaker := ""
	if strings.ContainsRune(subject, ',') {
		breaker = ","
	}
	for _, word := range words {
		if breaker == "" && slices.Contains(breakers, word) {
			breaker = word
		}
	}
	if breaker != "" {
		return tb.Str(": {not-applicable} quotes an obligation whose subject ").Str(pyRepr(strings.TrimSpace(subject))).
			Str(" holds ").Str(pyRepr(breaker)).Str(" before ").Str(keyword).
			Str(", so it names more than one party or qualifies the party by a clause, and no reader can tell which one the keyword binds. Quote a sentence of the cited section with no keyword that scopes it to the role (\"This section applies only to relay agents.\"), or annotate the row {gap: ...}. ").String(), true
	}
	determiners := strings.Fields(roleDeterminers)
	for len(words) > 1 && slices.Contains(determiners, words[len(words)-1]) {
		words = words[:len(words)-1]
	}
	for len(words) > 1 && slices.Contains(determiners, words[0]) {
		words = words[1:]
	}
	if len(words) == 1 && slices.Contains(strings.Fields(rolePronouns), words[0]) {
		return tb.Str(": {not-applicable} quotes an obligation whose subject is the pronoun ").Str(pyRepr(words[0])).
			Str(" before ").Str(keyword).Str(", which names no role. ").Str(subjectAdvice).String(), true
	}
	for i, word := range words {
		words[i] = singularLetters(word)
	}
	if strings.Join(words, " ") == roleKey(bound) {
		return "", false
	}
	tb.Str(": {not-applicable} binds ").Str(pyRepr(bound)).Str(", which is not the subject of the quoted obligation: ")
	if len(words) == 0 {
		tb.Str("no word stands before its first keyword ").Str(keyword)
	} else {
		tb.Str("the words before its first keyword ").Str(keyword).Str(" are ").Str(pyRepr(strings.Join(words, " ")))
	}
	return tb.Str(", and an obligation binds the party it names as its subject, with only determiners or quantifiers beside it. ").Str(subjectAdvice).String(), true
}

// subjectAdvice is what a refusal of a quoted obligation's subject tells the
// author to quote instead, or to record.
const subjectAdvice = "Quote the sentence of the cited section that names the bound role as the subject of its keyword, or one with no keyword that scopes the section to that role (\"This section applies only to relay agents.\" binds relay agents), or annotate the row {gap: ...}. "

// roleTokens is text as its words are compared: lower case, each word with
// the punctuation around it cut, empty words dropped, nothing singular yet,
// so a determiner, a pronoun and a breaker are still the words their lists
// spell ("this" stays "this"). "routers," and "Routers" are the one word
// "routers".
func roleTokens(text string) []string {
	var words []string
	for word := range strings.FieldsSeq(strings.ToLower(text)) {
		word = strings.TrimFunc(word, func(r rune) bool { return !isWordRune(r) })
		if word == "" {
			continue
		}
		words = append(words, word)
	}
	return words
}

// genericRole answers whether every word of role, once nonRoleAdjectives are
// set aside, is one of genericRoleWords, so the role names no party a reader
// could tell from ze. A role of adjectives alone ("a compliant") names no
// party either, and is generic too.
func genericRole(role string) bool {
	words := roleCore(role)
	if len(words) == 0 {
		return strings.TrimSpace(role) != ""
	}
	generic := strings.Fields(roleKey(genericRoleWords))
	for _, word := range words {
		if !slices.Contains(generic, word) {
			return false
		}
	}
	return true
}

// roleCore is the words of roleKey(role) with nonRoleAdjectives set aside.
func roleCore(role string) []string {
	adjectives := strings.Fields(nonRoleAdjectives)
	var core []string
	for word := range strings.FieldsSeq(roleKey(role)) {
		if !slices.Contains(adjectives, word) {
			core = append(core, word)
		}
	}
	return core
}

// roleKey is a role, or the text a role is looked for in, as two roles
// compare: roleTokens, with no leading determiner or quantifier
// (roleDeterminers) and each word singular, single-spaced, so "a speaker",
// "The Speaker", "every speaker" and "speakers" name one role, and "a route
// reflector (RR)" is "route reflector rr". A word ending in "ies" ends in "y"
// ("proxies", "entities"); otherwise a word loses a trailing "es" after s, x,
// z, ch or sh ("classes", "boxes"), or a trailing "s" that does not follow
// another ("routers", but not "class"). Both sides of every comparison pass
// through it, so a word it cuts wrongly ("status") is cut the same way on
// both, and subjectRefusal cuts a subject's words the same way.
func roleKey(role string) string {
	words := roleTokens(role)
	determiners := strings.Fields(roleDeterminers)
	for len(words) > 1 && slices.Contains(determiners, words[0]) {
		words = words[1:]
	}
	for i, word := range words {
		words[i] = singularLetters(word)
	}
	return strings.Join(words, " ")
}

// singularLetters cuts one trailing plural from a lower-case word with no
// punctuation, as roleKey describes.
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
// be ze itself. nonRoleAdjectives are set aside first, so "a compliant BGP
// implementation" is "BGP implementation". Empty when the role has no
// qualifier, a head of its own, or a qualifier word no declared role holds.
func qualifiedGenericRole(bound string, roles []string) string {
	words := roleCore(bound)
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
