// Design: docs/contributing/rfc-conformance-gates.md -- the {not-applicable} shapes, and the pre-RFC 2119 keyword register
// Related: notapplicable_roles.go -- boundRoleRefusal, the parse-time reader of a quote's first keyword
// Related: check_evidence.go -- wordIndex, the whole-word matcher, and the evidence-time readers
// Related: check_conditional.go -- conditionalGuard, which reads the first mandatory keyword
package rfc

import "strings"

// keywordRegister is how an RFC's text spells the BCP 14 keywords its
// obligations are stated in, so a quote is read for a keyword the way its own
// document writes one. Zero is no register: a caller that has not asked the
// stem cannot say which spellings count, and every method refuses it.
type keywordRegister uint8

const (
	registerUnspecified keywordRegister = iota
	// registerUpperCase is a document published once RFC 2119 defined the
	// keywords, and every draft: only the upper-case spelling is a keyword.
	registerUpperCase
	// registerPre2119 is an RFC published before RFC 2119, which states its
	// obligations in lowercase. Its "must" is read as MUST and its "must not"
	// as MUST NOT, everywhere a keyword is looked for (ai/META-DECISIONS.md),
	// beside the upper-case spellings a few such RFCs already use.
	registerPre2119
)

// bcp14RFCNumber is the number of RFC 2119, the document that defined the
// BCP 14 keywords.
const bcp14RFCNumber = 2119

// pre2119Spellings are the lowercase words a pre-RFC 2119 document states an
// obligation in, keyed by the level each is read as. The owner rule names
// these two and no other, so a lowercase "should" or "may" stays prose and is
// never read as SHOULD or MAY.
var pre2119Spellings = map[string]string{levelMust: "must", levelMustNot: "must not"}

// keywordRegisterOf answers the register of the document stem names. The one
// fact it reads is the RFC number the stem carries: the RFC Editor numbers
// RFCs in the order it publishes them, so a number below 2119 is a document
// published before RFC 2119 defined the keywords. A stem that is not rfc and
// digits alone is a draft or another named document, and every one of those
// postdates RFC 2119, so its register is upper case.
func keywordRegisterOf(stem string) keywordRegister {
	digits, numbered := strings.CutPrefix(stem, "rfc")
	if !numbered || digits == "" {
		return registerUpperCase
	}
	number := 0
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return registerUpperCase
		}
		// Past 2119 the answer is decided, and stopping there keeps a stem
		// of more digits than an int holds from overflowing.
		if number > bcp14RFCNumber {
			return registerUpperCase
		}
		number = number*10 + int(digit-'0')
	}
	if number < bcp14RFCNumber {
		return registerPre2119
	}
	return registerUpperCase
}

// find answers the byte offset of the first whole-word spelling of level in
// text that this register reads as that level, -1 when none is. Every
// spelling of one level has the same length, so a caller that steps past a
// keyword by len(level) steps past the spelling it found.
func (r keywordRegister) find(text, level string) int {
	at := wordIndex(text, level)
	switch r {
	case registerUpperCase:
		return at
	case registerPre2119:
		lower, read := pre2119Spellings[level]
		if !read {
			return at
		}
		found := wordIndex(text, lower)
		if found < 0 {
			return at
		}
		if at < 0 {
			return found
		}
		return min(at, found)
	case registerUnspecified:
		panic("BUG: keywordRegister.find on an unspecified register: ask keywordRegisterOf for the stem")
	}
	panic("BUG: keywordRegister.find on an unknown register")
}

// firstKeyword answers the byte offset of the first BCP 14 keyword in quote
// as a whole word, and the level it is read as, -1 when it holds none. Where
// two keywords start at one offset the longer wins, so "MUST NOT" is not read
// as "MUST", nor "must not" as MUST.
func (r keywordRegister) firstKeyword(quote string) (int, string) {
	at, keyword := -1, ""
	for _, candidate := range bcp14Keywords {
		found := r.find(quote, candidate)
		if found < 0 {
			continue
		}
		if at < 0 || found < at || (found == at && len(candidate) > len(keyword)) {
			at, keyword = found, candidate
		}
	}
	return at, keyword
}

// firstMandatoryKeyword answers the byte offset of the first mandatory BCP 14
// keyword in quote as a whole word, -1 when it holds none.
func (r keywordRegister) firstMandatoryKeyword(quote string) int {
	at := -1
	for _, level := range gatedLevelNames() {
		found := r.find(quote, level)
		if found < 0 {
			continue
		}
		if at < 0 || found < at {
			at = found
		}
	}
	return at
}

// levelsIn answers the advisory and the mandatory BCP 14 levels quote holds
// as whole words, each in rfc.go's sorted order.
func (r keywordRegister) levelsIn(quote string) ([]string, []string) {
	var advisory, mandatory []string
	for _, level := range advisoryLevelNames() {
		if r.find(quote, level) >= 0 {
			advisory = append(advisory, level)
		}
	}
	for _, level := range gatedLevelNames() {
		if r.find(quote, level) >= 0 {
			mandatory = append(mandatory, level)
		}
	}
	return advisory, mandatory
}
