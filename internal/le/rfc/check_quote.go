// Design: docs/contributing/rfc-conformance-gates.md -- the row quote rule, judged over every row
// Related: inventory.go -- quoteHaystack and quoteSource, the one matcher every quote path shares
// Related: summary.go -- Requirement.Quote, the span of a row that states the RFC's sentence
package rfc

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// minRowQuote is the shortest row text the quote rule accepts. The same floor
// as a correction's quote: a shorter span ("MUST be set to 0.") is found in
// many places of one RFC and so proves nothing about which sentence the row
// states.
const minRowQuote = minCorrectionQuote

// stemPath answers dir/stem+suffix.
func stemPath(dir, stem, suffix string) string {
	var tb textbuf.Buffer
	return tb.Str(dir).Byte('/').Str(stem).Str(suffix).String()
}

// checkRowQuotes refuses each requirement row in the corpus whose text is not a
// verbatim span of the section it cites (ai/rules/rfc-compliance.md: a
// requirement list a model produced is a claim, never evidence; the RFC's own
// sentence is). Every row is judged, whether or not the commit under test
// touched it, so a row that stops being verbatim because its RFC text or a
// cited erratum changed under it is refused as well. A stem with no RFC text
// is refused row by row by rowQuoteRefusal. Each stem's RFC text is read once.
func checkRowQuotes(tree string, requirements []Requirement) []string {
	sources := map[string]*quoteSource{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		source, loaded := sources[req.RFC]
		if !loaded {
			if text, found := SourceText(tree, req.RFC); found {
				source = newQuoteSource(text).withErrata(treeErrata(tree, req.RFC))
			}
			sources[req.RFC] = source
		}
		if message, refused := rowQuoteRefusal(req, source); refused {
			errs = append(errs, message)
		}
	}
	return errs
}

// rowQuoteRefusal judges one row's quote against its RFC. It answers the
// refusal and true, or "" and false when the row's text is a verbatim span of
// the section it cites or one of that section's subsections. A row that cites
// an erratum is judged by erratumRowRefusal against the RFC as the erratum
// corrects it. A nil source
// means the RFC's text is not in the repository, which is refused: a quote
// nobody can check is the claim this rule exists to replace.
func rowQuoteRefusal(req *Requirement, source *quoteSource) (string, bool) {
	quote := squashWhitespace(req.Quote())
	var tb textbuf.Buffer
	tb.Str(requirementWhere(*req)).Str(": ").Str(req.RFC).Byte(' ').Str(req.RID).
		Str(" (section ").Str(req.Section).Str(") ").Str(pyRepr(truncateRunes(quote, 70))).Str(": ")
	if source == nil {
		return tb.Str("the RFC's own text is not in this repository, so the row's quote can be checked against nothing. Fetch it to ").
			Str(fullRel).Byte('/').Str(req.RFC).Str(".txt or ").Str(draftsRel).Byte('/').
			Str(req.RFC).Str(".txt").String(), true
	}
	if len(quote) < minRowQuote {
		return tb.Str("the row's text is ").Int(int64(len(quote))).Str(" characters. A row states the RFC's own sentence, at least ").
			Int(int64(minRowQuote)).Str(" characters of it, so the span identifies one sentence").String(), true
	}
	resolved, ok := source.resolve(req.Section)
	if !ok {
		return tb.Str("unresolved anchor: the cited section names no heading of the RFC, and no heading ancestor of it does either. Cite the section the sentence is in").String(), true
	}
	if cited := citedErrata(req.Text); len(cited) > 0 {
		return erratumRowRefusal(&tb, req, source, resolved, quote, cited)
	}
	if source.inSection(resolved, quote) {
		return "", false
	}
	if found, elsewhere := source.sectionOf(quote); elsewhere {
		return tb.Str("the sentence is in section ").Str(found).Str(", not in section ").Str(resolved).
			Str(" or its subsections. Cite the section it is in").String(), true
	}
	return tb.Str("the row's text is not a verbatim span of section ").Str(resolved).
		Str(" or its subsections. A row states the RFC's own sentence, copied verbatim (whitespace and page breaks are ignored), never a paraphrase").String(), true
}
