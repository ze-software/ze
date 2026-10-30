// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet, {feature-declined}, the heading form
// Related: check_conditional.go -- neverClauseGap and contentWords, the never clause and the words it shares
// Related: inventory.go -- quoteSource.headingTitles and documentTitle, the titles the feature words are read in
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// genericHeadingWords are the words, singular and lower case as contentWords
// answers them, that title a section of nearly any RFC ("Introduction",
// "Message Format", "Security Considerations"), so a heading naming only these
// and stopwords names no feature: declined by such a heading, every obligation
// of a document's procedures would leave the count. Closed: a word it does not
// hold is a feature word, unless contentWords drops it as a stopword, a
// determiner or a role word.
const genericHeadingWords = "abstract acknowledgement acknowledgment appendix background behavior " +
	"consideration contents convention definition description detail document error example extension " +
	"field format general handling header iana information introduction message method mode model " +
	"notation note operation option overview packet parameter procedure processing protocol rationale " +
	"reference requirement rule scope section security specification state structure summary " +
	"terminology type usage use value"

// headingWordsGeneric answers whether words hold no content word
// (contentWords) that genericHeadingWords does not hold.
func headingWordsGeneric(words string) bool {
	generic := strings.Fields(genericHeadingWords)
	for _, word := range contentWords(words) {
		if !slices.Contains(generic, word) {
			return false
		}
	}
	return true
}

// featureHeadingGap judges a {feature-declined} heading form: the feature
// words must name the feature in a title of the row's own section or of one of
// its ancestors (quoteSource.headingTitles), so the heading scopes the row's
// obligation to that feature, the way RFC 8907's "5.4.2.3.  CHAP Login" scopes
// its START packet to CHAP. Words the RFC's own title holds are refused even
// where a heading repeats them, because they name the document, and the whole
// document is not one feature. The reason then owes the never clause, whose
// condition shares a content word with the feature words and whose producer
// the tree declares outside a test (neverClauseGap).
func featureHeadingGap(req *Requirement, source *quoteSource, code *sourceReader) (string, bool) {
	words := req.Annotation.Heading
	var tb textbuf.Buffer
	if source == nil {
		return tb.Str("the RFC's own text is not in this repository, so its heading ").Str(pyRepr(words)).
			Str(" can be checked against nothing; fetch it to ").Str(fullRel).Byte('/').
			Str(req.RFC).Str(".txt").String(), true
	}
	own, cited := source.resolve(req.Section)
	if !cited {
		return tb.Str("the section it cites, ").Str(req.Section).
			Str(", names no heading of the RFC, so no heading can name the feature ").Str(pyRepr(words)).String(), true
	}
	if phraseIn(source.documentTitle, words) {
		return tb.Str("its heading ").Str(pyRepr(words)).Str(" is in the RFC's own title ").
			Str(pyRepr(source.documentTitle)).
			Str(", which names the whole document, and the whole document is not one feature. ").
			Str("Name the feature a section heading names, or the obligation is owed").String(), true
	}
	titles := source.headingTitles(own)
	for _, title := range titles {
		if phraseIn(title, words) {
			return neverClauseGap(req, code, neverScope{
				opening:  "its heading form declines the feature " + pyRepr(words),
				evidence: words, against: "the heading's feature words",
			})
		}
	}
	tb.Str("its heading ").Str(pyRepr(words)).Str(" is in no title of section ").Str(own).
		Str(" or of a section above it (")
	for at, title := range titles {
		if at > 0 {
			tb.Str(", ")
		}
		tb.Str(pyRepr(title))
	}
	return tb.Str("), so no heading scopes the row to that feature. ").
		Str("Write the feature words as the heading spells them, whole words, or quote the RFC's sentence instead. ").
		Str(featureDeclinedFormat).String(), true
}

// phraseIn answers whether the words of phrase appear in text as one run of
// whole words, each compared lower case and singular (singularLetters), the
// way a role is compared, so "CHAP" is in "CHAP Login", "DUID-LLT" is in
// "... Plus Time (DUID-LLT)", and "CHAP" is not in "MS-CHAP v1 Login". False
// for a phrase with no word.
func phraseIn(text, phrase string) bool {
	need := headingTokens(phrase)
	if len(need) == 0 {
		return false
	}
	have := headingTokens(text)
	for start := 0; start+len(need) <= len(have); start++ {
		if slices.Equal(have[start:start+len(need)], need) {
			return true
		}
	}
	return false
}

// headingTokens answers the words of text, lower case and singular, split on
// every rune but a letter, a digit or a hyphen (isNotWordRune).
func headingTokens(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), isNotWordRune)
	for at, word := range words {
		words[at] = singularLetters(word)
	}
	return words
}
