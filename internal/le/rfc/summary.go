// Design: docs/architecture/core-design.md -- the registry of RFC obligations
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// summary.go reads rfc/short/*.md. Each Compliance Checklist line carries a
// permanent id anchored to the section it cites, and that anchor is the whole
// design: RFCs are immutable, so a section number is the most stable name
// available, and an id claiming §5.3 on a line citing §7.1 is a contradiction
// the parser refuses rather than a drift nobody notices.
package rfc

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The section-citation readers. Three styles are in the wild: "(§5.3)",
// "(Section 6)" and "(S4.1)".
//
// Python writes the third as `\bS(?=\d)` and RE2 has no lookahead, so each
// branch captures its own section here and the first non-empty group wins. The
// bare-S branch demands the digit the lookahead demanded, which is what stops
// SHOULD and SHALL matching and what keeps the S of "AS4" out.
var (
	sectionRE     = regexp.MustCompile(reSection())
	crossRFCSecRE = regexp.MustCompile(reCrossRFCSec())
)

const (
	secBody  = `[0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*`
	secDigit = `\d[0-9A-Za-z]*(?:\.[0-9A-Za-z]+)*`
)

func reSection() string {
	var tb textbuf.Buffer
	return tb.Str(`(?:§\s*(`).Str(secBody).Str(`)|\bSection\s+(`).Str(secBody).
		Str(`)|\bS(`).Str(secDigit).Str(`))`).String()
}

func reCrossRFCSec() string {
	var tb textbuf.Buffer
	return tb.Str(`RFC\s*\d+\s*(?:§\s*[0-9A-Za-z.]+|\bSection\s+[0-9A-Za-z.]+|\bS\d[0-9A-Za-z.]*)`).String()
}

// firstSection answers the first section cited in text, or "" when none is.
func firstSection(text string) string {
	found := sectionRE.FindStringSubmatch(text)
	if found == nil {
		return ""
	}
	for _, group := range found[1:] {
		if group != "" {
			return strings.TrimSpace(group)
		}
	}
	return ""
}

// extractSection answers the section an id anchors to: the FIRST section of
// THIS RFC that the line cites, or noSection when the line cites none of its
// own.
//
// The citation is the TRAILING parenthetical by convention, and that
// distinction is load-bearing: a requirement whose prose mentions another
// section first must anchor to the section it is FROM, not the one it refers
// TO. A cite naming another document is scrubbed first, because anchoring an
// RFC 1071 requirement to RFC 2328's §A.3.1 would name the wrong document.
func extractSection(text string) string {
	scrubbed := crossRFCSecRE.ReplaceAllString(text, "")

	if tail := trailingParenRE.FindStringSubmatch(scrubbed); tail != nil {
		if sec := firstSection(tail[1]); sec != "" {
			return sec
		}
	}
	if sec := firstSection(scrubbed); sec != "" {
		return sec
	}
	return noSection
}

// parseAnnotation reads one `{kind: reason}` body.
func parseAnnotation(body, where string) (*Annotation, error) {
	var tb textbuf.Buffer
	if !strings.Contains(body, ":") {
		return nil, parseErr(tb.Str(where).Str(": annotation {").Str(body).
			Str("} has no reason. Every annotation must justify itself: {kind: why}"))
	}
	kind, rest, _ := strings.Cut(body, ":")
	kind = strings.TrimSpace(kind)
	rest = strings.TrimSpace(rest)
	if !annotationKinds[kind] {
		known := append(AnnotationKinds(), SupersededKind)
		sort.Strings(known)
		return nil, parseErr(tb.Str(where).Str(": unknown annotation kind ").Str(pyRepr(kind)).
			Str("; expected one of ").Str(pyRepr(known)))
	}
	if rest == "" {
		return nil, parseErr(tb.Str(where).Str(": annotation {").Str(kind).
			Str("} has an empty reason. A bare annotation is an escape hatch; say why."))
	}
	if kind == AnnotationSinglePolarity {
		polarity, why, _ := strings.Cut(rest, ";")
		polarity = strings.TrimSpace(polarity)
		why = strings.TrimSpace(why)
		if !polarities[polarity] {
			return nil, parseErr(tb.Str(where).Str(": single-polarity needs a polarity from ").
				Str(pyRepr(Polarities())).Str(", got ").Str(pyRepr(polarity)).
				Str(". Format: {single-polarity: negative; why}"))
		}
		if why == "" {
			return nil, parseErr(tb.Str(where).
				Str(": single-polarity needs a reason explaining why the other ").
				Str("polarity cannot be tested. Format: {single-polarity: negative; why}"))
		}
		return &Annotation{Kind: kind, Polarity: polarity, Reason: why}, nil
	}
	return &Annotation{Kind: kind, Reason: rest}, nil
}

// parseSuccessor reads `{superseded: restated RFC9568-5.2.3-2; why}` and its
// three sibling forms. The reason is mandatory on all four.
func parseSuccessor(body, where string) (*Successor, error) {
	var tb textbuf.Buffer
	_, rest, _ := strings.Cut(body, ":")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, parseErr(tb.Str(where).Str(": {").Str(SupersededKind).
			Str("} has an empty body. Say where the obligation went: {").
			Str(SupersededKind).Str(": ").Str(successorRestated).Str(" <ID>; why}, {").
			Str(SupersededKind).Str(": ").Str(successorDropped).Str("; why}, {").
			Str(SupersededKind).Str(": ").Str(successorUnextracted).Str(" <§section>; why} or {").
			Str(SupersededKind).Str(": ").Str(successorUnresolved).Str("; why}"))
	}
	head, reason, found := strings.Cut(rest, ";")
	reason = strings.TrimSpace(reason)
	if !found || reason == "" {
		return nil, parseErr(tb.Str(where).Str(": {").Str(SupersededKind).Str(": ").
			Str(strings.TrimSpace(head)).Str("} has no reason. A forward pointer nobody ").
			Str("explained is a pointer nobody checked; say what the successor does with ").
			Str("this obligation. Format: {").Str(SupersededKind).
			Str(": <disposition> [<ID>]; why}"))
	}
	parts := strings.Fields(head)
	disposition := ""
	if len(parts) > 0 {
		disposition = parts[0]
	}
	if !successorDispositions[disposition] {
		return nil, parseErr(tb.Str(where).Str(": unknown {").Str(SupersededKind).
			Str("} disposition ").Str(pyRepr(disposition)).Str("; expected one of ").
			Str(pyRepr(successorDispositionNames())))
	}
	if successorTargeted[disposition] {
		names := "successor section"
		// textbuf rather than `+`: c_string_concat refuses a `+` beside a string
		// literal in any compiled Go file.
		var tb textbuf.Buffer
		example := tb.Str(successorUnextracted).Str(" §8.2.3").String()
		if disposition == successorRestated {
			names = "successor requirement id"
			var rb textbuf.Buffer
			example = rb.Str(successorRestated).Str(" RFC9568-5.2.3-2").String()
		}
		if len(parts) != 2 {
			return nil, parseErr(tb.Str(where).Str(": {").Str(SupersededKind).Str(": ").
				Str(disposition).Str("} needs exactly one ").Str(names).Str(", got ").
				Str(pyRepr(strings.TrimSpace(head))).Str(". Format: {").Str(SupersededKind).
				Str(": ").Str(example).Str("; why}"))
		}
		return &Successor{Disposition: disposition, Target: parts[1], Reason: reason}, nil
	}
	if len(parts) != 1 {
		return nil, parseErr(tb.Str(where).Str(": {").Str(SupersededKind).Str(": ").
			Str(disposition).Str("} names nothing, got ").
			Str(pyRepr(strings.TrimSpace(head))).Str(". Only ").Str(successorRestated).
			Str(" and ").Str(successorUnextracted).Str(" name a target"))
	}
	return &Successor{Disposition: disposition, Reason: reason}, nil
}

// stripMarkers peels every trailing `{...}` group off a requirement line.
//
// A line carries at most one coverage annotation AND at most one {superseded}
// marker, in either order, and the two COMPOSE. The loop is what makes that
// possible: the pattern anchors at end of line and matches ONE group, so a
// single search would leave the second group inside the requirement TEXT,
// unparsed and dragged into extractSection as if it were prose.
func stripMarkers(rest, where string) (*Annotation, *Successor, string, error) {
	var annotation *Annotation
	var successor *Successor
	for {
		loc := annotationRE.FindStringSubmatchIndex(rest)
		if loc == nil {
			return annotation, successor, rest, nil
		}
		body := strings.TrimSpace(rest[loc[2]:loc[3]])
		kind := strings.TrimSpace(strings.SplitN(body, ":", 2)[0])
		var tb textbuf.Buffer
		if kind == SupersededKind {
			if successor != nil {
				return nil, nil, "", parseErr(tb.Str(where).Str(": two {").Str(SupersededKind).
					Str("} markers on one line. An obligation has one destination"))
			}
			parsed, err := parseSuccessor(body, where)
			if err != nil {
				return nil, nil, "", err
			}
			successor = parsed
		} else {
			if annotation != nil {
				return nil, nil, "", parseErr(tb.Str(where).
					Str(": two coverage annotations on one line ({").Str(annotation.Kind).
					Str("} and {").Str(kind).
					Str("}). A requirement has one coverage disposition"))
			}
			parsed, err := parseAnnotation(body, where)
			if err != nil {
				return nil, nil, "", err
			}
			annotation = parsed
		}
		rest = strings.TrimSpace(rest[:loc[0]])
	}
}

// validateID refuses an id that is not <PREFIX>-<section>-<ordinal>, and one
// whose section disagrees with the section the line cites. That cross-check is
// the payoff of anchoring to sections: a sequential counter can drift away from
// the text it names and nothing notices.
func validateID(rid, stem, section, where string) error {
	var tb textbuf.Buffer
	found := idRE.FindStringSubmatch(rid)
	if found == nil {
		return parseErr(tb.Str(where).Str(": malformed requirement id ").Str(pyRepr(rid)).
			Str("; expected ").Str(Prefix(stem)).Str("-<section>-<n>, e.g. ").
			Str(Prefix(stem)).Str("-5.3-4"))
	}
	anchor := section
	if anchor == "" {
		anchor = noSection
	}
	var want textbuf.Buffer
	wantHead := want.Str(Prefix(stem)).Byte('-').Str(anchor).String()
	if found[1] != wantHead {
		cited := "no section"
		if section != "" {
			var cite textbuf.Buffer
			cited = cite.Str("§").Str(section).String()
		}
		return parseErr(tb.Str(where).Str(": id ").Str(pyRepr(rid)).
			Str(" disagrees with its section (").Str(cited).Str("); expected ").
			Str(wantHead).Str("-<n>. The id is anchored to the section it cites, so the ").
			Str("two can never drift apart"))
	}
	ordinal, err := strconv.Atoi(found[2])
	if err != nil || ordinal < 1 {
		return parseErr(tb.Str(where).Str(": id ").Str(pyRepr(rid)).Str(" ordinal starts at 1"))
	}
	return nil
}

// parseChecklistLine parses one Compliance Checklist line. It answers nil for a
// line that is not a checklist entry.
//
// It raises for a line that is TRYING to be a requirement and is malformed,
// including a legacy line with no id. It never answers nil for those: skipping
// a MUST is how a gate goes green while an obligation is unenforced.
func parseChecklistLine(line, stem, source string, lineno int) (*Requirement, error) {
	where := "line"
	if source != "" {
		var tb textbuf.Buffer
		where = tb.Str(source).Byte(':').Int(int64(lineno)).String()
	}

	found := checklistRE.FindStringSubmatch(line)
	if found == nil {
		if firstTagRE.MatchString(line) && levelBracketRE.MatchString(line) {
			// A line CARRYING an RFC 2119 keyword bracket is a compliance
			// line, full stop. Deciding this from the FIRST bracket alone was
			// a fail-open: the retired counter form has an unrecognized first
			// bracket, so it was dismissed as an ad-hoc category line and
			// silently dropped, taking a live MUST out of the ledger with it.
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).
				Str(": checklist line carries an RFC 2119 keyword but does not parse: ").
				Str(pyRepr(strings.TrimSpace(line))).Str(". Expected: - [ ] [").
				Str(Prefix(stem)).Str("-<section>-<n>] [MUST] text (§N). (The old ").
				Str(Prefix(stem)).Str("-RNNN counter form is retired -- ids are anchored ").
				Str("to the section they cite.)"))
		}
		// An ad-hoc category tag ([FORMAT], [IPSEC]) with no 2119 keyword is
		// an implementation-task checklist: not a requirement, not an error.
		// The caller skips it, which is why no sentinel is owed: prose is the
		// ordinary content of a summary rather than a condition to report.
		return nil, nil //nolint:nilnil // a line that is not a requirement
	}

	ticked := strings.TrimSpace(found[1]) != ""
	rid := found[2]
	if rid == "" {
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(where).Str(": checklist line has no requirement id: ").
			Str(pyRepr(strings.TrimSpace(line))).
			Str(". Every line needs one so tests can reference it: - [ ] [").
			Str(Prefix(stem)).Str("-<section>-<n>] [").Str(found[3]).Str("] ..."))
	}

	rest := strings.TrimSpace(found[4])
	annotation, successor, rest, err := stripMarkers(rest, where)
	if err != nil {
		return nil, err
	}
	section := extractSection(rest)
	if err := validateID(rid, stem, section, where); err != nil {
		return nil, err
	}
	return &Requirement{
		RFC: stem, RID: rid, Level: found[3], Text: rest, Section: section,
		Annotation: annotation, Source: source, Line: lineno,
		Ticked: ticked, Superseded: successor,
	}, nil
}

// parseSummaryText parses every checklist line in a summary. It raises on a
// duplicate id: ids are permanent and unique.
func parseSummaryText(text, stem, source string) ([]Requirement, error) {
	var out []Requirement
	seen := map[string]int{}
	for i, line := range strings.Split(text, "\n") {
		lineno := i + 1
		req, err := parseChecklistLine(line, stem, source, lineno)
		if err != nil {
			return nil, err
		}
		if req == nil {
			continue
		}
		if first, ok := seen[req.RID]; ok {
			where := source
			if where == "" {
				where = stem
			}
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(where).Str(": duplicate requirement id ").Str(req.RID).
				Str(" (lines ").Int(int64(first)).Str(" and ").Int(int64(lineno)).
				Str("). IDs are permanent and unique."))
		}
		seen[req.RID] = lineno
		out = append(out, *req)
	}
	return out, nil
}

// parseSummaryFile parses one rfc/short/<stem>.md.
func parseSummaryFile(tree, path string) ([]Requirement, error) {
	stem := strings.TrimSuffix(filepath.Base(path), ".md")
	rel := relTo(tree, path)
	text, err := readFile(path, rel)
	if err != nil {
		return nil, err
	}
	return parseSummaryText(text, stem, rel)
}

// summaryStems answers every stem under rfc/short/.
func summaryStems(tree string) (map[string]bool, error) {
	dir := treePath(tree, summaryRel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(relTo(tree, dir)).Str(": cannot read: ").Err(err))
	}
	out := map[string]bool{}
	for _, entry := range entries {
		if name := entry.Name(); strings.HasSuffix(name, ".md") {
			out[strings.TrimSuffix(name, ".md")] = true
		}
	}
	return out, nil
}

// parseEnrolled reads rfc/enrolled.txt: one stem per line, comments and blank
// lines skipped, the first word of a row keying it and the rest of the row
// answering why that RFC is enrolled.
//
// The reason is KEPT rather than discarded. Each row is `<stem>\t<why>`, the
// file's own header says so, and an author writes that sentence once. A reader
// publishing the enrolment therefore has the reason to publish with it, and
// nothing has to re-derive a justification the file already carries.
func parseEnrolled(text string) (map[string]bool, map[string]string) {
	enrolled := map[string]bool{}
	reasons := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := splitFieldsN(line, 2)
		enrolled[fields[0]] = true
		if len(fields) > 1 {
			reasons[fields[0]] = strings.TrimSpace(fields[1])
		}
	}
	return enrolled, reasons
}

// loadEnrolled answers the enrolled set and the reason each row states, or two
// empty maps when the file is absent. Absent is a legal state; unreadable is
// not, and reaches the caller.
func loadEnrolled(tree string) (map[string]bool, map[string]string, error) {
	path := treePath(tree, enrolledRel)
	raw, err := os.ReadFile(path) // #nosec G304 -- rfc/enrolled.txt under the checkout
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, map[string]string{}, nil
		}
		var tb textbuf.Buffer
		return nil, nil, parseErr(tb.Str(relTo(tree, path)).Str(": cannot read: ").Err(err))
	}
	enrolled, reasons := parseEnrolled(string(raw))
	return enrolled, reasons, nil
}

// titleRowRE reads the forward Meta `| Title |` row of a summary.
//
// Anchored at the start of a line and matched case-insensitively, in the shape
// obsoletedRowRE already reads the obsolescence row with. One labelled source
// and nothing else: the H1 separator is an em dash in one summary, a double
// hyphen in another and a colon in a third, and one H1 carries a `(short)`
// suffix, so a fallback parser would have to guess which half is the title.
var titleRowRE = regexp.MustCompile(`(?mi)^\|\s*Title\s*\|([^|]*)\|`)

// summaryTitle answers the RFC's own title, read off the summary's Meta table,
// and the empty string for a summary carrying no such row.
//
// Empty rather than a guess. A wrong title on a published page states a fact
// about a standards document that the document does not state, and the caller
// shows the display name alone instead (OD-4).
func summaryTitle(text string) string {
	found := titleRowRE.FindStringSubmatch(text)
	if found == nil {
		return ""
	}
	return strings.TrimSpace(found[1])
}

// summaryTitles answers the title every summary of one checkout declares.
//
// A stem whose summary declares none is ABSENT from the map rather than present
// with an empty value, so a caller can tell "this summary states no title" from
// "this stem has no summary" (ai/rules/principles.md).
func summaryTitles(tree string, stems map[string]bool) (map[string]string, error) {
	out := make(map[string]string, len(stems))
	for _, stem := range sortedSet(stems) {
		var name textbuf.Buffer
		rel := name.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
		text, err := readFile(treePath(tree, rel), rel)
		if err != nil {
			return nil, err
		}
		if title := summaryTitle(text); title != "" {
			out[stem] = title
		}
	}
	return out, nil
}

// gatedCounts answers the number of MUST-level requirements each summary
// declares.
func gatedCounts(requirements []Requirement) map[string]int {
	out := map[string]int{}
	for _, req := range requirements {
		if req.Gated() {
			out[req.RFC]++
		}
	}
	return out
}

// sortedSet answers a set's members sorted, for a message or an envelope.
func sortedSet(set map[string]bool) []string { return sortedKeys(set) }
