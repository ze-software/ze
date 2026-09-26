// Design: docs/contributing/rfc-conformance-gates.md -- the row quote rule and its backfill
// Related: check_quote.go -- the rule every rewritten row must then pass
package rfc

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// keyApply is the switch that lets quote-backfill write. Without it the action
// only reports, so an operator reads the review list before anything moves.
const keyApply = "apply"

// The kinds a judged row carries. A quoted row carries backfillQuote; every
// other kind names why the row stays as it is, and the Text renderer and the
// per-kind counts read these names.
const (
	backfillQuote = "quote"

	// Review: one site maps the row, but the machine cannot trust the pair.
	backfillSeveralSites   = "several-sites"
	backfillLeadIn         = "lead-in"
	backfillQualified      = "qualified"
	backfillPartial        = "partial"
	backfillNoAnchor       = "unresolved-anchor"
	backfillOutsideSection = "outside-section"
	backfillShortSentence  = "short-sentence"
	backfillLevelDiffers   = "level-differs"
	backfillNumberAbsent   = "number-absent"
	backfillPolarity       = "polarity-differs"
	backfillLowOverlap     = "low-overlap"
	backfillRewriteRefused = "rewrite-refused"

	// Human: no site gives the machine a sentence to copy.
	backfillUnsourced    = "unsourced"
	backfillUnmapped     = "unmapped"
	backfillNoExtraction = "no-extraction"
	backfillNoRFCText    = "no-rfc-text"
)

// backfillOverlapMin is the share of the row's content words the sentence MUST
// carry for the rewrite to apply. A paraphrase that shares less than half its
// words with the sentence is more often a wrong mapping than a loose wording
// (ten known wrongly mapped rows, measured 2026-09-26, all land on the review
// list; docs/contributing/rfc-conformance-gates.md, "The row quote"), and a
// quote written over a wrong mapping makes the mapping look verified.
const backfillOverlapMin = 0.5

// quoteBackfillRow is one requirement row the backfill judged.
type quoteBackfillRow struct {
	RID  string `json:"rid"`
	Kind string `json:"kind"`
	// Text is the row's text as the summary holds it now.
	Text string `json:"text"`
	// Quote is the site sentence: the new text for a quoted row, the candidate
	// a human reads for a reviewed one, and empty when no site maps the row.
	Quote string `json:"quote,omitempty"`
	// Reason says why a row goes to the review or human list rather than
	// being rewritten.
	Reason string `json:"reason,omitempty"`
	// line is the row's summary line, rewritten when apply is set.
	line    int
	newLine string
}

// quoteBackfillReport is what quote-backfill answers for one stem.
type quoteBackfillReport struct {
	Stem    string `json:"stem"`
	Applied bool   `json:"applied"`
	// Verbatim counts the rows whose text already passes the row check. They
	// are left alone.
	Verbatim int                `json:"verbatim"`
	Quoted   []quoteBackfillRow `json:"quoted"`
	Review   []quoteBackfillRow `json:"review"`
	Human    []quoteBackfillRow `json:"human"`
}

// quoteBackfill judges every row of one stem and, when apply is set, rewrites
// the rows it can quote mechanically. It never writes a row the row check
// would refuse, and it never touches the citation, the level, the id or a
// marker.
func quoteBackfill(tree, stem string, apply bool) (*quoteBackfillReport, error) {
	// The stem names a file this action rewrites, so a stem that is not a stem
	// ("../x") is refused before it becomes a path outside rfc/short.
	if !stemRE.MatchString(stem) {
		return nil, errors.New("rfc quote-backfill: " + pyRepr(stem) + " is not a summary stem")
	}
	summaryPath := treePath(tree, summaryRel, stem+".md")
	body, err := os.ReadFile(summaryPath) //nolint:gosec // a path under the checkout's rfc/short
	if err != nil {
		return nil, err
	}
	summaryName := stemPath(summaryRel, stem, ".md")
	requirements, err := parseSummaryText(string(body), stem, summaryName)
	if err != nil {
		return nil, err
	}
	report := &quoteBackfillReport{Stem: stem, Quoted: []quoteBackfillRow{},
		Review: []quoteBackfillRow{}, Human: []quoteBackfillRow{}}

	text, found := SourceText(tree, stem)
	if !found {
		for i := range requirements {
			report.Human = append(report.Human, backfillHuman(&requirements[i], backfillNoRFCText,
				"the RFC's own text is not in this repository"))
		}
		return report, nil
	}
	source := newQuoteSource(text)

	sites, unsourced, extracted, err := backfillSites(tree, stem)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(body), "\n")
	for i := range requirements {
		req := &requirements[i]
		if _, refused := rowQuoteRefusal(req, source); !refused {
			report.Verbatim++
			continue
		}
		row := judgeBackfillRow(req, lines[req.Line-1], sites[req.RID], unsourced[req.RID], extracted, source)
		switch {
		case row.Kind == backfillQuote:
			report.Quoted = append(report.Quoted, row)
		case backfillNeedsHuman(row.Kind):
			report.Human = append(report.Human, row)
		default:
			report.Review = append(report.Review, row)
		}
	}
	if !apply || len(report.Quoted) == 0 {
		return report, nil
	}

	for _, row := range report.Quoted {
		lines[row.line-1] = row.newLine
	}
	info, err := os.Stat(summaryPath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(summaryPath, []byte(strings.Join(lines, "\n")), info.Mode().Perm()); err != nil {
		return nil, err
	}
	report.Applied = true
	return report, nil
}

// backfillSites reads the stem's extraction artifact: the sites mapped to each
// row, and the rows a section declares unsourced. The third result is false
// when the stem has no artifact, which is a real answer: every row then needs
// a human.
func backfillSites(tree, stem string) (map[string][]ExtractionSite, map[string]bool, bool, error) {
	path := treePath(tree, extractionRel, stem+".json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	artifact, err := parseExtractionArtifact(tree, path)
	if err != nil {
		return nil, nil, false, err
	}
	sites := map[string][]ExtractionSite{}
	for _, site := range artifact.Sites {
		if site.Disposition == DispositionMapped && site.MappedTo != "" {
			sites[site.MappedTo] = append(sites[site.MappedTo], site)
		}
	}
	unsourced := map[string]bool{}
	for _, section := range artifact.Sections {
		for _, rid := range section.UnsourcedIDs {
			unsourced[rid] = true
		}
	}
	return sites, unsourced, true, nil
}

// backfillNeedsHuman answers whether a kind means no site offers a sentence.
func backfillNeedsHuman(kind string) bool {
	switch kind {
	case backfillUnsourced, backfillUnmapped, backfillNoExtraction, backfillNoRFCText:
		return true
	}
	return false
}

func backfillHuman(req *Requirement, kind, reason string) quoteBackfillRow {
	return quoteBackfillRow{RID: req.RID, Kind: kind, Text: req.Text, Reason: reason}
}

// judgeBackfillRow decides one row that is not verbatim yet. Every test that
// can send the row to review runs before the rewrite is built, and the rewrite
// itself is then parsed back and held against the row check, so a quote the
// check would refuse is never written.
func judgeBackfillRow(req *Requirement, line string, sites []ExtractionSite, unsourced, extracted bool,
	source *quoteSource,
) quoteBackfillRow {
	if !extracted {
		return backfillHuman(req, backfillNoExtraction, "the RFC has no extraction artifact, so no site maps the row")
	}
	if len(sites) == 0 {
		if unsourced {
			return backfillHuman(req, backfillUnsourced, "the extraction declares the row unsourced: no keyword sentence states it")
		}
		return backfillHuman(req, backfillUnmapped, "no site of the extraction maps the row")
	}
	row := quoteBackfillRow{RID: req.RID, Text: req.Text, line: req.Line}
	if len(sites) > 1 {
		var tb textbuf.Buffer
		row.Kind, row.Reason = backfillSeveralSites, tb.Int(int64(len(sites))).
			Str(" sites map the row; a human chooses the sentence").String()
		return row
	}
	row.Quote = squashWhitespace(sites[0].Quote)
	kind, reason := backfillPairRefusal(req, row.Quote, source)
	if kind != "" {
		row.Kind, row.Reason = kind, reason
		return row
	}
	newLine, refusal := backfillRewrite(req, line, row.Quote, source)
	if refusal != "" {
		row.Kind, row.Reason = backfillRewriteRefused, refusal
		return row
	}
	row.Kind, row.newLine = backfillQuote, newLine
	return row
}

// backfillPairRefusal holds the row against its one site sentence. It answers
// the review kind and its reason, or two empty strings when the rewrite may be
// built.
func backfillPairRefusal(req *Requirement, sentence string, source *quoteSource) (string, string) {
	var tb textbuf.Buffer
	if strings.HasSuffix(sentence, ":") {
		return backfillLeadIn, "the site is a lead-in ending in \":\"; the obligation is in the list after it"
	}
	if len(sentence) < minRowQuote {
		return backfillShortSentence, tb.Str("the sentence is ").Int(int64(len(sentence))).
			Str(" characters, under the row check's ").Int(int64(minRowQuote)).String()
	}
	if backfillQualifies(req.Quote()) {
		return backfillQualified, "the row quotes the RFC and adds its own words, which narrow what the row claims; the whole sentence would widen it"
	}
	resolved, ok := source.resolve(req.Section)
	if !ok {
		return backfillNoAnchor, "the cited section names no heading of the RFC"
	}
	if !source.inSection(resolved, sentence) {
		return backfillOutsideSection, tb.Str("the sentence is not in section ").Str(resolved).
			Str(" or its subsections; the backfill never rewrites the citation").String()
	}
	if forces := backfillForces(sentence); !forces[levelForce[req.Level]] {
		return backfillLevelDiffers, tb.Str("the sentence does not state the row's level [").
			Str(req.Level).Str("]").String()
	}
	quote := req.Quote()
	if stated, claimed := backfillKeywordCount(sentence), max(backfillKeywordCount(quote), 1); stated > claimed {
		return backfillPartial, tb.Str("the sentence states ").Int(int64(stated)).Str(" RFC 2119 keywords and the row ").
			Int(int64(claimed)).Str("; the whole sentence would widen what the row claims").String()
	}
	if number, absent := backfillMissingNumber(quote, sentence); absent {
		return backfillNumberAbsent, tb.Str("the row's number ").Str(number).Str(" is not in the sentence").String()
	}
	if backfillNegated(quote) != backfillNegated(sentence) {
		return backfillPolarity, "the row and the sentence disagree on negation"
	}
	shared, total := backfillOverlap(quote, sentence)
	if total == 0 || float64(shared)/float64(total) < backfillOverlapMin {
		return backfillLowOverlap, tb.Str("the row shares ").Int(int64(shared)).Str(" of its ").
			Int(int64(total)).Str(" content words with the sentence, under half").String()
	}
	return "", ""
}

// backfillRewrite builds the new line: the sentence in place of the text before
// the trailing section parenthetical, everything from that parenthetical to the
// end of the line kept byte for byte. It parses the line back and refuses it
// unless the id, level, section and markers are unchanged and the row check
// accepts the quote. The third result is the refusal, empty on success.
func backfillRewrite(req *Requirement, line, sentence string, source *quoteSource) (string, string) {
	start := strings.Index(line, req.Text)
	loc := trailingParenRE.FindStringIndex(req.Text)
	if start < 0 || loc == nil {
		return "", "the row's text carries no trailing section parenthetical to keep"
	}
	newLine := line[:start] + sentence + " " + req.Text[loc[0]:] + line[start+len(req.Text):]
	parsed, err := parseChecklistLine(newLine, req.RFC, req.Source, req.Line)
	if err != nil {
		return "", "the rewritten line does not parse: " + err.Error()
	}
	if parsed == nil || parsed.RID != req.RID || parsed.Level != req.Level || parsed.Section != req.Section {
		return "", "the rewritten line changes the id, the level or the section"
	}
	if squashWhitespace(parsed.Quote()) != sentence {
		return "", "the rewritten line does not read back as the sentence"
	}
	if message, refused := rowQuoteRefusal(parsed, source); refused {
		return "", "the row check refuses the rewrite: " + message
	}
	return newLine, ""
}

// The obligation force of each RFC 2119 keyword. Synonyms share a force, so a
// [SHALL] row matches a MUST sentence, and a negated keyword is its own force,
// so a [MUST] row never matches a sentence that only says MUST NOT.
const (
	forceMust = iota + 1
	forceMustNot
	forceShould
	forceShouldNot
	forceMay
)

var levelForce = map[string]int{
	levelMust: forceMust, "SHALL": forceMust, "REQUIRED": forceMust,
	levelMustNot: forceMustNot, "SHALL NOT": forceMustNot,
	levelShould: forceShould, "RECOMMENDED": forceShould,
	"SHOULD NOT": forceShouldNot, "NOT RECOMMENDED": forceShouldNot,
	"MAY": forceMay, "OPTIONAL": forceMay,
}

var (
	// backfillKeywordRE finds an RFC 2119 keyword as a whole word. The
	// alternation is longest first, so "MUST NOT" is never read as "MUST".
	backfillKeywordRE = regexp.MustCompile(`\b(?:` + levelAlternation + `)\b`)
	// backfillProseKeywordRE is the same set in any case, for an RFC written
	// before RFC 2119 whose sentences say "must" in lower case.
	backfillProseKeywordRE = regexp.MustCompile(`(?i)\b(?:` + levelAlternation + `)\b`)
	backfillNumberRE       = regexp.MustCompile(`\d+(?:\.\d+)*`)
	backfillWordRE         = regexp.MustCompile(`[a-z0-9]+(?:'[a-z]+)?`)
)

// backfillForces answers the forces the sentence states. It reads upper-case
// keywords first, and lower-case ones only when the sentence carries none.
func backfillForces(sentence string) map[int]bool {
	found := backfillKeywordRE.FindAllString(sentence, -1)
	if len(found) == 0 {
		found = backfillProseKeywordRE.FindAllString(sentence, -1)
	}
	forces := map[int]bool{}
	for _, keyword := range found {
		forces[levelForce[strings.ToUpper(squashWhitespace(keyword))]] = true
	}
	return forces
}

// backfillQualifies answers whether a row that already carries a double-quoted span
// stating an RFC 2119 keyword also carries words of its own outside it, as in "\"...MUST do A and MUST do B\" -- the
// A half". Those words say which part of the sentence the row covers, and its tagged tests
// prove that part only, so the whole sentence would widen the obligation (R-7).
func backfillQualifies(quote string) bool {
	parts := strings.Split(quote, "\"")
	if len(parts) < 3 {
		return false
	}
	// A quoted term ("Forwarding State" bit) is a name, not a sentence the row narrows.
	quotesASentence := false
	for i := 1; i < len(parts); i += 2 {
		if backfillProseKeywordRE.MatchString(parts[i]) {
			quotesASentence = true
		}
	}
	if !quotesASentence {
		return false
	}
	var outside textbuf.Buffer
	for i := 0; i < len(parts); i += 2 {
		outside.Str(parts[i]).Byte(' ')
	}
	return len(backfillContentWords(outside.String())) > 0
}

// backfillKeywordCount answers how many RFC 2119 keywords the text states, upper case
// first and any case only when it carries none, the reading backfillForces takes.
func backfillKeywordCount(text string) int {
	if found := backfillKeywordRE.FindAllString(text, -1); len(found) > 0 {
		return len(found)
	}
	return len(backfillProseKeywordRE.FindAllString(text, -1))
}

// backfillMissingNumber answers the first number of the row that the sentence
// does not carry, and true. RFC7432-10-1 is the shape: the row allowed a length
// of 0 where the sentence says 32 or 128.
func backfillMissingNumber(row, sentence string) (string, bool) {
	carried := map[string]bool{}
	for _, number := range backfillNumberRE.FindAllString(sentence, -1) {
		carried[number] = true
	}
	for _, number := range backfillNumberRE.FindAllString(row, -1) {
		if !carried[number] {
			return number, true
		}
	}
	return "", false
}

// backfillNegated answers whether the text states a negation: "not", "never",
// or a contraction ending in "n't", in any case.
func backfillNegated(text string) bool {
	for _, word := range backfillWordRE.FindAllString(strings.ToLower(text), -1) {
		if word == "not" || word == "never" || word == "cannot" || strings.HasSuffix(word, "n't") {
			return true
		}
	}
	return false
}

// backfillStopWords carries no meaning a paraphrase could get wrong. The RFC
// 2119 keywords and negations are here too, because the level and polarity
// tests judge those.
var backfillStopWords = backfillWordSet("the and for that this with from are was were been being " +
	"has have had its into than then when which who such any all each not never will can also " +
	"only other these those there their they them but per via must shall should may required " +
	"recommended optional cannot")

// backfillWordSet answers the space-separated words as a set.
func backfillWordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

// backfillOverlap answers how many of the row's distinct content words the
// sentence carries, and how many the row has. Both sides pass through the
// same stem, so "sends" meets "send" and "received" meets "receive".
func backfillOverlap(row, sentence string) (int, int) {
	carried := backfillContentWords(sentence)
	shared, total := 0, 0
	for word := range backfillContentWords(row) {
		total++
		if carried[word] {
			shared++
		}
	}
	return shared, total
}

func backfillContentWords(text string) map[string]bool {
	words := map[string]bool{}
	for _, word := range backfillWordRE.FindAllString(strings.ToLower(text), -1) {
		if len(word) < 3 || backfillStopWords[word] || backfillNumberRE.MatchString(word) {
			continue
		}
		words[backfillStem(word)] = true
	}
	return words
}

// backfillStem cuts a plural, then an "-ing" or "-ed", then a final "e", each
// only while three letters remain. It is crude on purpose: it runs on both
// sides of the comparison, so it needs to be consistent, not correct.
func backfillStem(word string) string {
	if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && len(word) > 3 {
		word = word[:len(word)-1]
	}
	for _, suffix := range []string{"ing", "ed"} {
		if strings.HasSuffix(word, suffix) && len(word)-len(suffix) >= 3 {
			word = word[:len(word)-len(suffix)]
			break
		}
	}
	if strings.HasSuffix(word, "e") && len(word) > 3 {
		word = word[:len(word)-1]
	}
	return word
}

// Text renders the report for a terminal: the counts, then every quoted row,
// then every reviewed row with its reason, then the human rows by kind.
func (r *quoteBackfillReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(r.Stem).Str(": ").Int(int64(len(r.Quoted)))
	if r.Applied {
		tb.Str(" quoted")
	} else {
		tb.Str(" to quote (dry run, nothing written)")
	}
	tb.Str(", ").Int(int64(len(r.Review))).Str(" to review, ").Int(int64(len(r.Human))).
		Str(" need a human, ").Int(int64(r.Verbatim)).Str(" already verbatim\n")
	for _, row := range r.Quoted {
		tb.Str("  quote  ").Str(row.RID).Str(": ").Str(row.Quote).Byte('\n')
	}
	for _, row := range r.Review {
		tb.Str("  review ").Str(row.RID).Str(" [").Str(row.Kind).Str("]: ").Str(row.Reason).Byte('\n')
	}
	for _, row := range r.Human {
		tb.Str("  human  ").Str(row.RID).Str(" [").Str(row.Kind).Str("]\n")
	}
	return tb.String()
}

// quoteBackfillAnswer is `./le rfc quote-backfill stem <stem> [apply]`.
func quoteBackfillAnswer(args leaction.Arguments) (any, int) {
	if !args.Has(keyStem) {
		leaction.ReportError(errors.New("rfc quote-backfill requires stem <stem>"))
		return nil, 2
	}
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	report, err := quoteBackfill(tree, args.One(keyStem), args.Has(keyApply))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, 0
}
