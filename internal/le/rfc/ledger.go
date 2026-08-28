// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// ledger.go reads the three AUTHORED pages the generated ledger quotes back:
// the public status table (docs/features/rfc-status.md), the declared remainder
// (rfc/not-enrolled.txt), and the forward Meta row of each summary.
//
// None of the three is derived, so each one is parsed STRICTLY. A malformed
// disposition row would silently un-declare a summary, and a Meta field naming
// obsolescence in a spelling nothing reads would treat an obsoleted document as
// a current one. Both were real: three summaries naming a real successor were
// read as current for as long as the label was matched one way and the corpus
// wrote it four.
package rfc

import (
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The three kinds a summary can be declared under in rfc/not-enrolled.txt.
// Only dispositionNonNormative is a claim about the DOCUMENT; the other two are
// debt, and the rendered backlog says so.
const (
	dispositionNonNormative = "non-normative"
	dispositionBacklog      = "backlog"
	dispositionBlocked      = "blocked"
)

var dispositionKinds = map[string]bool{
	dispositionNonNormative: true,
	dispositionBacklog:      true,
	dispositionBlocked:      true,
}

// dispositionKindNames answers the closed set in sorted order.
//
// Sorted because the refusals below name it, and a message built from a map
// walk would name the same set in a different order on every run.
func dispositionKindNames() []string { return sortedSet(dispositionKinds) }

// Disposition is one rfc/not-enrolled.txt row: why this summary is not enrolled.
type Disposition struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// notEnrolledRel is the declared remainder, repo-relative.
const notEnrolledRel = "rfc/not-enrolled.txt"

// parseDispositions reads rfc/not-enrolled.txt into {stem: Disposition}.
//
// Same comment and blank-line tolerance as parseEnrolled, and the same
// first-token stem, so one reader serves both files. Everything after that is
// REFUSED rather than skipped: a malformed line would silently un-declare a
// summary, and an un-declared summary is exactly the absence this file exists
// to abolish. A typo must cost a red gate, not a quiet hole.
func parseDispositions(text string) (map[string]Disposition, error) {
	out := map[string]Disposition{}
	for n, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := splitFieldsN(line, 3)
		stem := fields[0]
		var where textbuf.Buffer
		at := where.Str(notEnrolledRel).Byte(':').Int(int64(n + 1)).Str(": ").String()
		if _, seen := out[stem]; seen {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(at).Str("duplicate stem ").Str(pyRepr(stem)).
				Str(". One row per summary: two rows can carry two different kinds and ").
				Str("nothing decides between them"))
		}
		if len(fields) < 2 {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(at).Str(pyRepr(line)).
				Str(" has no kind. Each row is '<stem> <kind> <reason>' with kind one of ").
				Str(pyRepr(dispositionKindNames())))
		}
		kind := fields[1]
		if !dispositionKinds[kind] {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(at).Str("kind ").Str(pyRepr(kind)).Str(" for ").
				Str(stem).Str(" is not one of ").Str(pyRepr(dispositionKindNames())).
				Str(". Use 'non-normative' when the DOCUMENT imposes nothing, 'backlog' ").
				Str("when the extraction is owed, 'blocked' when something outside the ").
				Str("summary prevents enrolment"))
		}
		reason := ""
		if len(fields) > 2 {
			reason = strings.TrimSpace(fields[2])
		}
		if reason == "" {
			var tb textbuf.Buffer
			return nil, parseErr(tb.Str(at).Str(stem).Str(" is declared ").Str(kind).
				Str(" with no reason. A bare kind is an absence with a label on it: say ").
				Str("what makes it true"))
		}
		out[stem] = Disposition{Kind: kind, Reason: reason}
	}
	return out, nil
}

// splitFieldsN is Python's str.split(None, n-1): at most n fields, the last one
// holding the unsplit remainder with its own leading whitespace consumed.
//
// strings.Fields cannot answer it, because it splits every run and would break
// a reason into words. strings.SplitN cannot either, because it splits on one
// literal and a row separated by two spaces or a tab would gain an empty field.
func splitFieldsN(s string, n int) []string {
	out := make([]string, 0, n)
	rest := s
	for len(out) < n-1 {
		rest = strings.TrimLeft(rest, " \t\n\v\f\r")
		if rest == "" {
			return out
		}
		cut := strings.IndexAny(rest, " \t\n\v\f\r")
		if cut < 0 {
			return append(out, rest)
		}
		out = append(out, rest[:cut])
		rest = rest[cut:]
	}
	rest = strings.TrimLeft(rest, " \t\n\v\f\r")
	if rest == "" {
		return out
	}
	return append(out, rest)
}

// loadDispositions answers the declared remainder, or empty when the file does
// not exist yet.
//
// An absent file is NOT an error: the disposition check reports every summary
// that is neither enrolled nor declared, so an absent file surfaces as one
// violation per un-enrolled summary -- which names the actual problem -- rather
// than as one message about a missing path.
func loadDispositions(tree string) (map[string]Disposition, error) {
	path := treePath(tree, notEnrolledRel)
	raw, err := os.ReadFile(path) // #nosec G304 -- rfc/not-enrolled.txt under the checkout
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Disposition{}, nil
		}
		var tb textbuf.Buffer
		return nil, parseErr(tb.Str(relTo(tree, path)).Str(": cannot read: ").Err(err))
	}
	return parseDispositions(string(raw))
}

// statusRel is the public claim, repo-relative.
const statusRel = "docs/features/rfc-status.md"

// LedgerRow is one docs/features/rfc-status.md row, by the three cells the
// checks and the render read.
type LedgerRow struct {
	Status    string `json:"status"`
	Coverage  string `json:"coverage"`
	Remaining string `json:"remaining"`
}

var (
	// The three shapes a first cell can key a row by. An RFC number, an
	// Internet-Draft stem, and a non-RFC non-draft stem (sflow-v5), each
	// keyed by exactly what Requirement.RFC holds for that summary: without
	// the last two a {gap} on such a summary could never find its disclosure
	// row and would be accused of hiding.
	statusRFCRE   = regexp.MustCompile(`\ARFC\s*(\d+)\z`)
	statusDraftRE = regexp.MustCompile(`\Adraft-[\p{L}\p{N}_.-]+\z`)
	statusStemRE  = regexp.MustCompile(`\A[a-z][a-z0-9]*(-[a-z0-9.]+)+\z`)
)

// parseStatusLedger reads docs/features/rfc-status.md rows into {stem: row}.
//
// A line that is not a table row, and a row whose first cell keys nothing, are
// both skipped rather than refused. The page is prose with a table in it, so a
// paragraph is not a defect; a row this cannot key is a row about something
// other than an RFC, and check_status_completeness is what notices an enrolled
// RFC with no row of its own.
func parseStatusLedger(text string) map[string]LedgerRow {
	rows := map[string]LedgerRow{}
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) < 3 {
			continue
		}
		var key string
		switch {
		case statusRFCRE.MatchString(cells[0]):
			var tb textbuf.Buffer
			key = tb.Str("rfc").Str(statusRFCRE.FindStringSubmatch(cells[0])[1]).String()
		case statusDraftRE.MatchString(cells[0]), statusStemRE.MatchString(cells[0]):
			key = cells[0]
		default:
			continue
		}
		row := LedgerRow{Status: cells[2]}
		if len(cells) > 3 {
			row.Coverage = cells[3]
		}
		if len(cells) > 4 {
			row.Remaining = cells[4]
		}
		rows[key] = row
	}
	return rows
}

// splitTableRow cuts one markdown row into its stripped cells.
//
// The strip of the outer pipes happens on the WHOLE line first, so a row with
// no trailing pipe keeps its last cell rather than gaining an empty one.
func splitTableRow(line string) []string {
	body := strings.Trim(strings.TrimSpace(line), "|")
	cells := strings.Split(body, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// loadStatusLedger reads the public page.
//
// The read is the caller's to share: one gate run parses this page once and
// hands the result to every consumer, because six consumers reading it
// themselves would be six parses of the same rows.
func loadStatusLedger(tree string) (map[string]LedgerRow, error) {
	text, err := readFile(treePath(tree, statusRel), statusRel)
	if err != nil {
		return nil, err
	}
	return parseStatusLedger(text), nil
}

var (
	// The forward Meta row. The label is matched loosely on purpose, because
	// the corpus writes it four ways and a reader that knew one of them
	// FAILED OPEN: rfc5575, rfc6810 and rfc1334 each named a real successor
	// the gate never asked about. The trailing [^|]* absorbs a qualifier,
	// which rfc1334 writes as `| Obsoleted-by (partial) |`.
	obsoletedRowRE = regexp.MustCompile(`(?mi)^\|\s*Obsoleted[ -]by[^|]*\|([^|]*)\|`)
	// Every Meta-table field NAME, so a fifth spelling of OBSOLESCENCE is
	// REFUSED rather than skipped. Widening the label above fixes the four
	// spellings that exist today; this refuses the one somebody writes
	// tomorrow.
	metaFieldRE       = regexp.MustCompile(`(?m)^\|([^|\n]*)\|`)
	obsolescenceRE    = regexp.MustCompile(`(?i)obsolet`)
	knownObsolescence = regexp.MustCompile(`(?i)^\s*Obsoletes\b|^\s*Obsoleted[ -]by\b`)
	rfcRefRE          = regexp.MustCompile(`(?i)\bRFC\s*(\d{3,5})\b`)
)

// noSuccessorValue answers Python's _NO_SUCCESSOR_RE.match over a STRIPPED
// value: the row exists and says nothing obsoletes this document.
//
// It is a hand scanner because the pattern ends in a negative lookahead and
// RE2 has none. The match is anchored, the value arrives stripped, so the
// leading \s* consumes nothing and only the alternation has to be walked.
//
// The lookahead is what keeps rfc2661 honest: its row opens with "-" and then
// explains in prose that RFC 3931 is a distinct protocol rather than a
// successor. Accepting a bare dash there and stopping is the whole point; a
// parser that scanned the rest of the value would demand 18 forward pointers
// into a document which obsoletes nothing.
func noSuccessorValue(value string) bool {
	if value == "" {
		return true
	}
	for _, open := range openings(value) {
		rest := value[open:]
		for _, width := range noSuccessorWords(rest) {
			after := rest[width:]
			if closesAndEnds(after) {
				return true
			}
		}
	}
	return false
}

// openings answers the two ways the optional `\(?` can be taken.
func openings(value string) []int {
	if strings.HasPrefix(value, "(") {
		return []int{1, 0}
	}
	return []int{0}
}

// noSuccessorWords answers every width the `(?:none|n/?a|-+)` branch can
// consume at the start of rest, longest first.
//
// Every width is offered rather than the greedy one alone, because the
// lookahead after it can reject a longer match and accept a shorter: `\)?`
// and the lookahead are what the engine backtracks into.
func noSuccessorWords(rest string) []int {
	lower := strings.ToLower(rest)
	var out []int
	if strings.HasPrefix(lower, "none") {
		out = append(out, 4)
	}
	if strings.HasPrefix(lower, "n/a") {
		out = append(out, 3)
	}
	if strings.HasPrefix(lower, "na") {
		out = append(out, 2)
	}
	dashes := 0
	for dashes < len(rest) && rest[dashes] == '-' {
		dashes++
	}
	for w := dashes; w >= 1; w-- {
		out = append(out, w)
	}
	return out
}

// closesAndEnds answers whether `\)?(?![\w/])` succeeds at after.
func closesAndEnds(after string) bool {
	if strings.HasPrefix(after, ")") && notWordOrSlash(after[1:]) {
		return true
	}
	return notWordOrSlash(after)
}

// notWordOrSlash is the negative lookahead itself.
func notWordOrSlash(s string) bool {
	if s == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s)
	return !isPyWordRune(r) && r != '/'
}

// isPyWordRune is Python's \w for a str pattern, which is unicode-aware: the
// ASCII word bytes plus anything str.isalnum() accepts. Go's own \w is
// ASCII-only, so a value opening `None` followed by a non-ASCII letter would
// otherwise be read as "nothing obsoletes this" by one half and as a live
// successor by the other.
func isPyWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// parseSuccessorStem answers the stem of the document that obsoletes this one,
// read off the forward Meta row.
//
// Empty when the summary carries no such row, or the row says nothing does. It
// REFUSES when a Meta field names obsolescence in a spelling this reader does
// not know: a spelling nobody parses is the failure this check exists to close.
//
// The row is written as a CHAIN, oldest first, because that is how the corpus
// already writes it: rfc3768's row reads "RFC 5798, which was in turn obsoleted
// by RFC 9568". The LAST reference is therefore the document that states these
// obligations today, and ai/rules/rfc-compliance.md is explicit that the
// lineage which matters runs forward.
func parseSuccessorStem(text, stem, source string) (string, error) {
	where := source
	if where == "" {
		where = stem
	}
	// Refuse an unknown obsolescence field BEFORE looking for the row this
	// reader does know. A reader that silently skips what it does not
	// recognize cannot be trusted to have found anything.
	for _, field := range metaFieldRE.FindAllStringSubmatch(text, -1) {
		if !obsolescenceRE.MatchString(field[1]) || knownObsolescence.MatchString(field[1]) {
			continue
		}
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(": Meta field `").
			Str(strings.TrimSpace(field[1])).
			Str("` names obsolescence in a spelling nothing reads, so the row would be ").
			Str("skipped in silence. Write the forward row as `| Obsoleted by |` or ").
			Str("`| Obsoleted-by |`, and the backward row as `| Obsoletes |`. A ").
			Str("qualifier after either label is kept"))
	}
	m := obsoletedRowRE.FindStringSubmatch(text)
	if m == nil {
		return "", nil
	}
	value := strings.TrimSpace(m[1])
	if noSuccessorValue(value) {
		return "", nil
	}
	refs := rfcRefRE.FindAllStringSubmatch(value, -1)
	if len(refs) == 0 {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(": the forward Meta row says ").
			Str(pyRepr(value)).
			Str(", which names no RFC. Write the chain of obsoleting documents oldest ").
			Str("first, or write `None`"))
	}
	var name textbuf.Buffer
	successor := name.Str("rfc").Str(refs[len(refs)-1][1]).String()
	if successor == stem {
		var tb textbuf.Buffer
		return "", parseErr(tb.Str(where).Str(": the forward Meta row ends at ").
			Str(successor).Str(", which is this document. A summary cannot obsolete itself"))
	}
	return successor, nil
}

// summarySuccessors answers {stem: the stem that obsoletes it} over every
// summary that declares one.
//
// Derived from the summaries on every run rather than kept in a list: a
// hand-kept list of superseded RFCs rots the day the IETF publishes the next
// one, and nothing notices.
func summarySuccessors(tree string, stems map[string]bool) (map[string]string, error) {
	if stems == nil {
		var err error
		if stems, err = summaryStems(tree); err != nil {
			return nil, err
		}
	}
	out := map[string]string{}
	for _, stem := range sortedSet(stems) {
		var name textbuf.Buffer
		rel := name.Str(summaryRel).Byte('/').Str(stem).Str(".md").String()
		path := treePath(tree, rel)
		// A stem with no file is SKIPPED, never refused. The caller can pass
		// a stem set from anywhere, and a summary this run cannot open owes
		// no forward pointer.
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}
		text, err := readFile(path, rel)
		if err != nil {
			return nil, err
		}
		successor, err := parseSuccessorStem(text, stem, rel)
		if err != nil {
			return nil, err
		}
		if successor != "" {
			out[stem] = successor
		}
	}
	return out, nil
}
