// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// ledger.go holds the SHAPES the public ledger is expressed in: the closed set
// of dispositions an un-enrolled summary can declare, the row the public page
// renders, and the scanner that reads a forward lineage value.
//
// It parsed three authored files until 2026-09-01. It parses none now. Each of
// those facts is declared once, in the summary's own `## Meta` table
// (meta.go), and the three files are generated from it (render_ledger.go).
// What survives here is what a declaration is CHECKED against, and the reason
// it is checked strictly: a malformed disposition would silently un-declare a
// summary, and a lineage value nothing reads would treat an obsoleted document
// as a current one. The second was real -- three summaries naming a real
// successor were read as current for as long as the label was matched one way
// and the corpus wrote it four.
package rfc

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The four kinds a summary can declare itself under when it is not enrolled.
// Two are claims about the DOCUMENT -- dispositionNonNormative about what it
// obliges, dispositionSourceRestricted about whether its text can be held here
// at all -- and the other two are debt, which the rendered backlog says out
// loud.
const (
	dispositionNonNormative = "non-normative"
	dispositionBacklog      = "backlog"
	dispositionBlocked      = "blocked"
	// dispositionSourceRestricted is the one disposition that is permanent.
	//
	// `blocked` is DEBT: the text is fetchable and somebody will fetch it. This
	// says the standard's own text may not be redistributed, so it can never
	// sit under rfc/full/ and checkEnrolment can never accept an enrolment for
	// it. ISO/IEC 10589 is the case it was written for: Ze implements the IS-IS
	// base protocol, the public page says so truthfully, and no checklist can
	// ever be bounded against a document this repository is not allowed to
	// hold. A gap is an ISSUE and an exclusion is a DECISION
	// (ai/rules/rfc-compliance.md); this is the second.
	dispositionSourceRestricted = "source-restricted"
	// dispositionOutOfScope is a SCOPE decision rather than a claim about the
	// document or a debt against it.
	//
	// The extraction is DONE, which is what separates it from `backlog`: the
	// obligations are written down, in full, under their own requirement ids,
	// and a later decision to build the feature starts from them rather than
	// from nothing. What is absent is the FEATURE, and the owner decided not
	// to offer it for now.
	//
	// It is not a way to look green. A summary declaring it MUST NOT claim
	// public support: checkOutOfScope refuses any Status other than
	// 'Unsupported' or 'Future', so the ledger says out loud that Ze does not
	// do this. ai/rules/rfc-compliance.md draws the same line -- an optional
	// feature Ze declined is an implementation gap a later scope decision can
	// revisit, and never a conformance gap.
	dispositionOutOfScope = "out-of-scope"
)

// Disposition is what an un-enrolled summary declares about itself: the kind,
// and the reason that kind is true.
type Disposition struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// notEnrolledRel is the generated declared remainder, repo-relative.
const notEnrolledRel = "rfc/not-enrolled.txt"

// statusRel is the generated public claim, repo-relative.
const statusRel = "docs/features/rfc-status.md"

// LedgerRow is one public page row, by the three cells the checks read. It is
// derived from a summary's Meta table by rowsFrom, never parsed back off the
// page the render writes.
type LedgerRow struct {
	Status    string `json:"status"`
	Coverage  string `json:"coverage"`
	Remaining string `json:"remaining"`
}

var (
	// rfcRefRE reads an `RFC 1234` reference out of a Meta cell.
	rfcRefRE = regexp.MustCompile(`(?i)\bRFC\s*(\d{3,5})\b`)
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
