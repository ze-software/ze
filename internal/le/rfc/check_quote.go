// Design: docs/contributing/rfc-conformance-gates.md -- the row quote rule and the unquoted ratchet
// Related: inventory.go -- quoteHaystack and quoteSource, the one matcher every quote path shares
// Related: summary.go -- Requirement.Quote, the span of a row that states the RFC's sentence
package rfc

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// minRowQuote is the shortest row text the quote rule accepts. The same floor
// as a correction's quote: a shorter span ("MUST be set to 0.") is found in
// many places of one RFC and so proves nothing about which sentence the row
// states.
const minRowQuote = minCorrectionQuote

// quoteRevisions is what the quote rules read out of the commit under test and
// its parent. readQuoteRevisions builds it once, and the row check, the ratchet
// and the report read it.
//
// Both sides are COMMITTED (owner decision, 2026-09-26), the model
// checkDiscriminationRatchet uses: `./le verify worktree` checks the commit out
// detached, where the tree equals HEAD, so a scope of "the tree against HEAD"
// would judge nothing at the one gate that runs. A row still uncommitted is
// judged once it is committed.
type quoteRevisions struct {
	// scope holds the requirement ids whose row the commit under test added,
	// or whose quote or cited section it changed.
	scope map[string]bool
	// head and prior count the unquoted rows at HEAD and at HEAD^ of each stem
	// whose summary or RFC text the commit changed. A stem the commit did not
	// touch has the same count on both sides, so it is not read.
	head  map[string]int
	prior map[string]int
	// unjudged names each touched stem the rules could not judge, with the
	// reason: a summary git holds that does not parse, or rows with no RFC
	// text on one side. Never counted as zero (ai/rules/principles.md).
	unjudged []string
	// known is false when HEAD^ does not resolve or git could not name the
	// paths the commit changed. The rules then judged nothing, and the report
	// says so rather than print a clean result.
	known bool
}

// readQuoteRevisions reads the rows and the RFC text of every stem the commit
// under test touched, at HEAD and at HEAD^.
func readQuoteRevisions(tree string) quoteRevisions {
	if !revisionExists(tree, priorRevision) {
		return quoteRevisions{}
	}
	stems, ok := quoteChangedStems(tree)
	if !ok {
		return quoteRevisions{}
	}
	paths := make([]string, 0, 3*len(stems))
	for _, stem := range stems {
		paths = append(paths, stemPath(summaryRel, stem, ".md"), stemPath(fullRel, stem, ".txt"),
			stemPath(draftsRel, stem, ".txt"))
	}
	headBlobs, headOK := gitCatBlobs(tree, headRevision, paths)
	priorBlobs, priorOK := gitCatBlobs(tree, priorRevision, paths)
	if !headOK {
		return quoteRevisions{}
	}
	if !priorOK {
		return quoteRevisions{}
	}
	out := quoteRevisions{scope: map[string]bool{}, head: map[string]int{}, prior: map[string]int{}, known: true}
	for _, stem := range stems {
		headRows, headRead := quoteRowsAt(headBlobs, stem)
		priorRows, priorRead := quoteRowsAt(priorBlobs, stem)
		if !headRead {
			out.unjudged = append(out.unjudged, stem+" (a summary git holds does not parse)")
			continue
		}
		if !priorRead {
			out.unjudged = append(out.unjudged, stem+" (a summary git holds does not parse)")
			continue
		}
		scopeChangedRows(out.scope, headRows, priorRows)
		headCount, headJudged := unquotedCount(headRows, quoteSourceAt(headBlobs, stem))
		priorCount, priorJudged := unquotedCount(priorRows, quoteSourceAt(priorBlobs, stem))
		if !headJudged {
			out.unjudged = append(out.unjudged, stem+" (no RFC text at HEAD or HEAD^)")
			continue
		}
		if !priorJudged {
			out.unjudged = append(out.unjudged, stem+" (no RFC text at HEAD or HEAD^)")
			continue
		}
		out.head[stem] = headCount
		out.prior[stem] = priorCount
	}
	return out
}

// quoteChangedStems answers the stems whose summary or RFC text the commit
// under test changed, sorted, and false when git could not answer.
func quoteChangedStems(tree string) ([]string, bool) {
	raw, ok := gitOutput(tree, "diff", "--name-only", "--no-renames", "-z", priorRevision, headRevision,
		"--", summaryRel, fullRel, draftsRel)
	if !ok {
		return nil, false
	}
	stems := map[string]bool{}
	for rel := range strings.SplitSeq(string(raw), "\x00") {
		dir, name := path.Split(rel)
		switch {
		case dir == summaryRel+"/" && strings.HasSuffix(name, ".md"):
			stems[strings.TrimSuffix(name, ".md")] = true
		case (dir == fullRel+"/" || dir == draftsRel+"/") && strings.HasSuffix(name, ".txt"):
			stems[strings.TrimSuffix(name, ".txt")] = true
		}
	}
	return sortedSet(stems), true
}

// stemPath answers dir/stem+suffix.
func stemPath(dir, stem, suffix string) string {
	var tb textbuf.Buffer
	return tb.Str(dir).Byte('/').Str(stem).Str(suffix).String()
}

// quoteRowsAt parses one stem's summary out of one revision's blobs. A summary
// the revision does not hold has no rows, which is a real answer; one that does
// not parse answers false.
func quoteRowsAt(blobs map[string]string, stem string) ([]Requirement, bool) {
	rel := stemPath(summaryRel, stem, ".md")
	text, held := blobs[rel]
	if !held {
		return nil, true
	}
	rows, err := parseSummaryText(text, stem, rel)
	if err != nil {
		return nil, false
	}
	return rows, true
}

// quoteSourceAt answers one stem's RFC text out of one revision's blobs, from
// the two locations SourcePath searches in the same order, and nil when the
// revision holds neither.
func quoteSourceAt(blobs map[string]string, stem string) *quoteSource {
	for _, dir := range []string{fullRel, draftsRel} {
		if text, held := blobs[stemPath(dir, stem, ".txt")]; held {
			return newQuoteSource(text)
		}
	}
	return nil
}

// scopeChangedRows adds to scope each HEAD row that HEAD^ does not hold, or
// holds with another quote or another cited section. A row whose cited section
// moved is an edit too: the same sentence can be verbatim in one section and
// absent from the next.
func scopeChangedRows(scope map[string]bool, headRows, priorRows []Requirement) {
	prior := make(map[string]*Requirement, len(priorRows))
	for i := range priorRows {
		if _, held := prior[priorRows[i].RID]; !held {
			prior[priorRows[i].RID] = &priorRows[i]
		}
	}
	for i := range headRows {
		row := &headRows[i]
		was, held := prior[row.RID]
		if !held {
			scope[row.RID] = true
			continue
		}
		if was.Quote() != row.Quote() || was.Section != row.Section {
			scope[row.RID] = true
		}
	}
}

// unquotedCount counts the rows the row check refuses. A stem with rows and no
// RFC text answers false: every row would be refused for the missing text, and
// that count measures the repository rather than the rows.
func unquotedCount(rows []Requirement, source *quoteSource) (int, bool) {
	if len(rows) == 0 {
		return 0, true
	}
	if source == nil {
		return 0, false
	}
	count := 0
	for i := range rows {
		if _, refused := rowQuoteRefusal(&rows[i], source); refused {
			count++
		}
	}
	return count, true
}

// unquotedFigures counts the unquoted rows of each stem in the tree, the backlog
// `./le rfc check` prints. It also names each stem with rows and no RFC text,
// which it counts in no figure.
func unquotedFigures(tree string, requirements []Requirement) (map[string]int, []string) {
	byStem := map[string][]Requirement{}
	for i := range requirements {
		byStem[requirements[i].RFC] = append(byStem[requirements[i].RFC], requirements[i])
	}
	figures := map[string]int{}
	var unjudged []string
	for _, stem := range slices.Sorted(maps.Keys(byStem)) {
		var source *quoteSource
		if text, found := SourceText(tree, stem); found {
			source = newQuoteSource(text)
		}
		count, judged := unquotedCount(byStem[stem], source)
		if !judged {
			unjudged = append(unjudged, stem+" (no RFC text in the tree)")
			continue
		}
		if count > 0 {
			figures[stem] = count
		}
	}
	return figures, unjudged
}

// checkRowQuotes refuses each in-scope requirement row whose text is not a
// verbatim span of the section it cites (ai/rules/rfc-compliance.md: a
// requirement list a model produced is a claim, never evidence; the RFC's own
// sentence is). known false means git could not name the scope, and the rule
// then judges nothing rather than the whole corpus.
func checkRowQuotes(tree string, requirements []Requirement, scope map[string]bool, known bool) []string {
	if !known {
		return nil
	}
	sources := map[string]*quoteSource{}
	var errs []string
	for i := range requirements {
		req := &requirements[i]
		if !scope[req.RID] {
			continue
		}
		source, loaded := sources[req.RFC]
		if !loaded {
			if text, found := SourceText(tree, req.RFC); found {
				source = newQuoteSource(text)
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
// the section it cites or one of that section's subsections. A nil source
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

// checkUnquotedRatchet refuses each stem whose count of unquoted rows the
// commit under test raised over HEAD^. The row check judges the rows the commit
// added or edited; this also catches a row the commit left alone that stopped
// being verbatim, because its RFC text changed under it.
func checkUnquotedRatchet(revisions quoteRevisions) []string {
	var errs []string
	for _, stem := range slices.Sorted(maps.Keys(revisions.head)) {
		was, now := revisions.prior[stem], revisions.head[stem]
		if now <= was {
			continue
		}
		var tb textbuf.Buffer
		errs = append(errs, tb.Str(stemPath(summaryRel, stem, ".md")).Str(": ").Str(stem).
			Str(" unquoted rows ").Int(int64(was)).Str(" -> ").Int(int64(now)).Str(" over ").Str(priorRevision).
			Str(": the commit under test raised the count of rows whose text is not a verbatim span of the section they cite. The count MUST NOT rise. Copy the RFC's own sentence into each row the commit added or edited").String())
	}
	return errs
}
