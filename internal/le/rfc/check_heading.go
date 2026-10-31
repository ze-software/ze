// Design: docs/contributing/rfc-conformance-gates.md -- the exclusion evidence ratchet, {feature-declined}, the heading form
// Related: check_conditional.go -- neverClauseGap and contentWords, the never clause and the words it shares
// Related: inventory.go -- quoteSource.headingTitles and documentTitle, the titles the feature words are read in
// Related: meta.go -- declinedFeaturesFrom, the `Declined features` row every heading form is held against
package rfc

import (
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// genericHeadingWords are the words, singular and lower case as contentWords
// answers them, that title a section of nearly any RFC ("Introduction",
// "Message Format", "Security Considerations", "Session Timers"), so a heading
// naming only these and stopwords names no feature: declined by such a
// heading, every obligation of a document's procedures would leave the count.
// Closed: a word it does not hold is a feature word, unless contentWords drops
// it as a stopword, a determiner or a role word.
const genericHeadingWords = "abstract acknowledgement acknowledgment address addressing algorithm appendix attribute author " +
	"background behavior " +
	"behaviour " + //nolint:misspell // RFCs written in UK English title sections "Behaviour".
	"capability change code compatibility configuration connection consideration contents convention " +
	"data definition description detail document encoding error example extension " +
	"field flag format general handling header iana information introduction length management message " +
	"method mode model negotiation notation note notification operation option overview packet parameter " +
	"procedure processing protocol rationale reference registration reply request requirement response rule " +
	"scope section security session specification state structure summary " +
	"terminology timer transport type usage use value"

// headingWordsGeneric answers whether words hold no content word
// (contentWords) that genericHeadingWords does not hold.
func headingWordsGeneric(words string) bool {
	return len(featureWords(words)) == 0
}

// featureWords answers the content words of words (contentWords) that
// genericHeadingWords does not hold, in order: the words that can name a
// feature.
func featureWords(words string) []string {
	generic := strings.Fields(genericHeadingWords)
	var out []string
	for _, word := range contentWords(words) {
		if !slices.Contains(generic, word) {
			out = append(out, word)
		}
	}
	return out
}

// hyphensAsBlanks answers text with each hyphen a blank, so "BGP-4" reads as
// the words "BGP" and "4". The document-title comparison reads words this way,
// because a title spells its protocol's name inside a hyphenated word
// ("(BGP-4)") that a heading names alone ("BGP Error Handling").
func hyphensAsBlanks(text string) string {
	return strings.ReplaceAll(text, "-", " ")
}

// featureHeadingGap judges a {feature-declined} heading form: the feature
// words must name the feature in a title of the row's own section or of one of
// its ancestors (quoteSource.headingTitles), so the heading scopes the row's
// obligation to that feature, the way RFC 8907's "5.4.2.3.  CHAP Login" scopes
// its START packet to CHAP. Words the RFC's own title holds are refused even
// where a heading repeats them, because they name the document, and the whole
// document is not one feature; so are words whose every content word is the
// title's or generic (genericHeadingWords), such as "TACACS+ Packet" in RFC
// 8907. Both read the title and the words with hyphens as blanks
// (hyphensAsBlanks), and an RFC whose title cannot be read is refused, since
// the words cannot then be told from the document's name. The reason then owes
// the never clause, whose condition shares a content word with the feature
// words and whose producer the tree declares outside a test (neverClauseGap).
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
	if source.documentTitle == "" {
		return tb.Str("its heading ").Str(pyRepr(words)).
			Str(" cannot be told from the document's name, because the RFC's own title cannot be read from its front matter (documentTitleOf). ").
			Str("Quote the RFC's sentence that makes the feature optional or conditional instead. ").
			Str(featureDeclinedFormat).String(), true
	}
	title := hyphensAsBlanks(source.documentTitle)
	if phraseIn(title, hyphensAsBlanks(words)) {
		return tb.Str("its heading ").Str(pyRepr(words)).Str(" is in the RFC's own title ").
			Str(pyRepr(source.documentTitle)).
			Str(", which names the whole document, and the whole document is not one feature. ").
			Str("Name the feature a section heading names, or the obligation is owed").String(), true
	}
	if !holdsWordOutside(hyphensAsBlanks(words), contentWords(title)) {
		return tb.Str("its heading ").Str(pyRepr(words)).
			Str(" holds no word that names a feature: each of its words is in the RFC's own title ").
			Str(pyRepr(source.documentTitle)).
			Str(" or titles a section of nearly any RFC (genericHeadingWords). Name the feature a section heading names, or the obligation is owed").
			String(), true
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

// holdsWordOutside answers whether words hold a feature word (featureWords)
// that taken does not hold.
func holdsWordOutside(words string, taken []string) bool {
	for _, word := range featureWords(words) {
		if !slices.Contains(taken, word) {
			return true
		}
	}
	return false
}

// checkDeclinedFeatureHeadings holds every {feature-declined} heading form of
// a summary against the features its Meta table declares the owner declined
// (Meta.DeclinedFeatures), so a heading cannot decline a feature nobody
// decided to decline. The heading's words must equal, or hold as one run of
// whole words, an entry's words (phraseIn), and a summary with a heading form
// and no `Declined features` row, or no entry the heading names, is refused
// naming the row to write. A feature word under minHeadingWords characters
// passes only as the whole words of a matched entry, so the owner may name
// "PE" and "ID" names nothing by accident. The Meta table is read only when a
// heading form exists, so a summary with none is not held to it here.
func checkDeclinedFeatureHeadings(text, stem, source string, requirements []Requirement) error {
	where := source
	if where == "" {
		where = stem
	}
	var declined []DeclinedFeature
	read := false
	for i := range requirements {
		req := &requirements[i]
		if req.Annotation == nil || req.Annotation.Kind != AnnotationFeatureDeclined || req.Annotation.Heading == "" {
			continue
		}
		if !read {
			read = true
			meta, err := ParseMeta(text, stem, where)
			if err != nil {
				return err
			}
			declined = meta.DeclinedFeatures
		}
		if err := declinedHeadingRefusal(req, declined, stem, where); err != nil {
			return err
		}
	}
	return nil
}

// declinedHeadingRefusal answers the refusal of one heading form held against
// declined (checkDeclinedFeatureHeadings), or nil when an entry it names
// passes.
func declinedHeadingRefusal(req *Requirement, declined []DeclinedFeature, stem, where string) error {
	words := req.Annotation.Heading
	var tb textbuf.Buffer
	tb.Str(where).Byte(':').Int(int64(req.Line)).Str(": ").Str(req.RID).Str(" {feature-declined} heading ").
		Str(pyRepr(words))
	if len(declined) == 0 {
		return parseErr(tb.Str(", and ").Str(stem).
			Str(" declares no declined feature, so nothing ties the heading to an owner decision. Add the row `| ").
			Str(metaDeclinedFeatures).Str(" | ").Str(words).Str(" (<owner-decision source>) |` to the `## Meta` table of ").
			Str(where).Str(", naming where the owner declined the feature. ").Str(featureDeclinedFormat))
	}
	matched := false
	for _, entry := range declined {
		if !phraseIn(words, entry.Words) {
			continue
		}
		matched = true
		if headingWordLongEnough(words, entry) {
			return nil
		}
	}
	if !matched {
		tb.Str(", and the Meta `").Str(metaDeclinedFeatures).Str("` row of ").Str(stem).
			Str(" holds no entry the heading ").Str(pyRepr(words)).
			Str(" names as whole words, so nothing ties the heading to an owner decision. If the owner declined it, write the row as `| ").
			Str(metaDeclinedFeatures).Str(" | ")
		for _, entry := range declined {
			tb.Str(entry.Words).Str(" (").Str(entry.Source).Str("), ")
		}
		return parseErr(tb.Str(words).Str(" (<owner-decision source>) |`; otherwise the obligation is owed. ").
			Str(featureDeclinedFormat))
	}
	return parseErr(tb.Str(" names its feature only by words under ").Int(int64(minHeadingWords)).
		Str(" characters, which name a feature only as the whole words of a `").Str(metaDeclinedFeatures).
		Str("` entry, such as `PE (owner ...)` for the heading \"PE Routers\". Name the entry by that word alone, or the feature as the heading spells it. ").
		Str(featureDeclinedFormat))
}

// headingWordLongEnough answers whether the heading words hold a feature word
// (featureWords) of at least minHeadingWords characters, or one exactly one
// character shorter that is the whole of entry's words, compared lower case
// and singular (headingTokens). A word shorter still never passes.
func headingWordLongEnough(words string, entry DeclinedFeature) bool {
	whole := strings.Join(headingTokens(entry.Words), " ")
	for _, word := range featureWords(words) {
		if len(word) >= minHeadingWords {
			return true
		}
		if len(word) == minHeadingWords-1 && word == whole {
			return true
		}
	}
	return false
}

// checkDeclinedFeaturesInText refuses an entry of a summary's `Declined
// features` row whose words its RFC text never names as one run of whole
// words (phraseIn over each section), read through the loaded quote source
// (loadQuoteSource). An entry the RFC never names is held by no heading the
// RFC has, so it declines nothing a reader can find. Every summary declaring
// the row is judged, once, and one whose RFC text is absent is refused,
// because the words cannot be found in a text nobody has.
func checkDeclinedFeaturesInText(tree string, metas map[string]Meta, requirements []Requirement) []string {
	sources := map[string]*quoteSource{}
	judged := map[string]bool{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		declined := metas[req.RFC].DeclinedFeatures
		if len(declined) == 0 {
			continue
		}
		if judged[req.RFC] {
			continue
		}
		judged[req.RFC] = true
		source := loadQuoteSource(sources, tree, req.RFC)
		var tb textbuf.Buffer
		if source == nil {
			errs = append(errs, tb.Str(req.Source).Str(": the Meta `").Str(metaDeclinedFeatures).
				Str("` row cannot be checked, because rfc/full/").Str(req.RFC).
				Str(".txt is absent; fetch it with `curl -o rfc/full/").Str(req.RFC).
				Str(".txt https://www.rfc-editor.org/rfc/").Str(req.RFC).Str(".txt`").String())
			continue
		}
		for _, entry := range declined {
			if source.holdsPhrase(entry.Words) {
				continue
			}
			var line textbuf.Buffer
			errs = append(errs, line.Str(req.Source).Str(": the Meta `").Str(metaDeclinedFeatures).
				Str("` row declares ").Str(pyRepr(entry.Words)).Str(", which rfc/full/").Str(req.RFC).
				Str(".txt never names; write each declined feature in the RFC's own words, so a heading can name it").String())
		}
	}
	return errs
}

// holdsPhrase answers whether any section of the RFC holds phrase as one run
// of whole words (phraseIn).
func (q *quoteSource) holdsPhrase(phrase string) bool {
	for i := range q.sections {
		if phraseIn(q.sections[i].body, phrase) {
			return true
		}
	}
	return false
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
