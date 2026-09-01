// Design: docs/architecture/core-design.md -- the rfc area, as one command
// Overview: rfc.go -- the types, the paths and the closed sets every reader here shares
//
// coverage.go counts. It is the half the ledger renders and the half the
// `--check` summary lines quote, and it holds no markup at all: every table in
// render.go is a formatting of a number produced here.
//
// Two rollups, over two different questions, and they are deliberately not one.
// CoverageRow answers which POLARITIES exist for a requirement. auditCoverage
// answers whether a human read the test and believed it. A requirement can have
// both polarities and a `weak` verdict, so a single "proven" column would have
// to lie about one of them.
package rfc

import (
	"regexp"
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	// The four MUST-level keywords of the RFC's OWN text. Deliberately not
	// siteKeywordRE: that one carries REQUIRED because a normative SITE can
	// be worded that way, and this one is a published measurement whose
	// denominator has never included it.
	sourceKeywordRE = regexp.MustCompile(`\b(?:MUST NOT|MUST|SHALL NOT|SHALL)\b`)
	// The same four words in LOWERCASE only, deliberately not
	// case-insensitive. It is the evidence a pre-RFC-2119 document offers
	// instead of capitalized keywords, and folding the cases together would
	// answer the uppercase question twice.
	sourceProseKeywordRE = regexp.MustCompile(`\b(?:must not|must|shall not|shall)\b`)
)

// Label is the `kind/tier` cell the ledger prints beside every test link.
func (c Carrier) Label() string {
	var tb textbuf.Buffer
	return tb.Str(c.Kind).Byte('/').Str(c.Tier).String()
}

// evidenceLabel answers the `kind/tier` cell for one repo-relative path.
//
// A path with no carrier can only arrive from a synthetic tag -- nothing in the
// tree can produce one -- and it is labeled visibly wrong rather than
// plausibly right: an unrecognized carrier must never render as though
// something proved something.
func evidenceLabel(rel string, carriers []Carrier) string {
	if c, held := CarrierFor(rel, carriers); held {
		return c.Label()
	}
	return "unknown/unrun"
}

// evidenceTier answers the execution tier for one repo-relative path, and
// tierUnrun for a path no carrier claims.
func evidenceTier(rel string, carriers []Carrier) string {
	if c, held := CarrierFor(rel, carriers); held {
		return c.Tier
	}
	return tierUnrun
}

// NightlyOnly answers: this requirement HAS evidence, and none of it runs
// inside ./le verify current mode full.
//
// A nightly-advisory scenario and a merge-gate unit test are both "a tag", and
// flattening them into one proven cell is how a claim nothing blocks on gets
// read as a claim every merge enforces.
func NightlyOnly(found []Tag, carriers []Carrier) bool {
	if len(found) == 0 {
		return false
	}
	for _, tag := range found {
		if evidenceTier(tag.File, carriers) == tierVerify {
			return false
		}
	}
	return true
}

// CoverageRow is one RFC's polarity row. Every field is derived.
type CoverageRow struct {
	RFC       string `json:"rfc"`
	Gated     int    `json:"gated"`
	Both      int    `json:"both"`
	One       int    `json:"one"`
	Annotated int    `json:"annotated"`
	// Missing counts gated requirements with no tag and no annotation.
	Missing int `json:"missing"`
	// NightlyOnly counts gated requirements whose evidence exists but runs in
	// NO ./le verify current mode full stage. Its OWN column, never folded into Both or
	// One: those two are the merge-gate view, and a nightly-only requirement is
	// not merge-gate-proven.
	NightlyOnly int `json:"nightly-only"`
}

// Outstanding is the work still owed before this RFC could be enrolled.
func (c CoverageRow) Outstanding() int { return c.One + c.Missing }

// CoverageRows answers per-RFC polarity coverage. This is the backlog,
// derived rather than maintained.
//
// A hand-kept TODO list of missing tests would rot the moment someone wrote one
// and forgot the list. Counting the tags is the only version that cannot lie.
func CoverageRows(requirements []Requirement, tags []Tag, carriers []Carrier) []CoverageRow {
	byRID := tagsByRID(tags)
	order, byRFC := requirementsByRFC(requirements)

	out := make([]CoverageRow, 0, len(order))
	for _, rfc := range order {
		row := CoverageRow{RFC: rfc}
		gated := 0
		for _, req := range byRFC[rfc] {
			if !req.Gated() {
				continue
			}
			gated++
			found := byRID[req.RID]
			if NightlyOnly(found, carriers) {
				row.NightlyOnly++
			}
			switch {
			case req.Annotation != nil:
				row.Annotated++
			case bothPolarities(found):
				row.Both++
			case len(found) > 0:
				row.One++
			default:
				row.Missing++
			}
		}
		if gated == 0 {
			continue
		}
		row.Gated = gated
		out = append(out, row)
	}
	return out
}

// tagsByRID groups a scan by requirement id, keeping the scan's order.
func tagsByRID(tags []Tag) map[string][]Tag {
	out := map[string][]Tag{}
	for _, tag := range tags {
		out[tag.RID] = append(out[tag.RID], tag)
	}
	return out
}

// requirementsByRFC groups requirements by summary stem, and answers the stems
// in FIRST-SEEN order.
//
// First-seen rather than sorted, because the Python walks an insertion-ordered
// dict here and the rollup's own sort is not total on its own: two RFCs with
// equal outstanding counts are separated by the stem, but the rows this feeds
// reach that sort in this order.
func requirementsByRFC(requirements []Requirement) ([]string, map[string][]Requirement) {
	var order []string
	byRFC := map[string][]Requirement{}
	for _, req := range requirements {
		if _, held := byRFC[req.RFC]; !held {
			order = append(order, req.RFC)
		}
		byRFC[req.RFC] = append(byRFC[req.RFC], req)
	}
	return order, byRFC
}

// bothPolarities answers whether the tags carry a positive AND a negative.
func bothPolarities(found []Tag) bool {
	var positive, negative bool
	for _, tag := range found {
		if tag.Polarity == PolarityPositive {
			positive = true
		}
		if tag.Polarity == PolarityNegative {
			negative = true
		}
	}
	return positive && negative
}

// auditCoverage is one RFC's audit row. Every field derived; nothing here is
// authored anywhere.
//
// TWO partitions, over two different populations, because one denominator
// cannot carry both questions honestly:
//
//   - Auditable = Audited + Unaudited, the REQUIREMENT view. How much of this
//     RFC an audit could be performed on, and how much of that carries a
//     verdict.
//   - Verdicts = Proven + Findings, the RECORD view, total over every verdict
//     recorded for the RFC, and Findings is exactly the number of worklist rows
//     it contributes.
//
// They are not the same population. Conflating them is what this row got wrong:
// counting only the both-polarity subset left a verdict on an ANNOTATED
// requirement in no column at all, and, when it was a fresh enforced, in no
// worklist row either. Five of the tree's 52 verdicts were invisible.
type auditCoverage struct {
	RFC string `json:"rfc"`
	// Auditable is gated, enrolled, and polarity coverage complete.
	Auditable int `json:"auditable"`
	// Audited is that subset carrying a verdict of any value.
	Audited int `json:"audited"`
	// Proven is every recorded verdict that is `enforced` AND fresh.
	Proven int `json:"proven"`
	// Findings is every recorded verdict that is NOT proven.
	Findings int `json:"findings"`
	// Verdicts is every recorded verdict, i.e. Proven + Findings.
	Verdicts int `json:"verdicts"`
}

// unaudited is the auditable remainder carrying no verdict at all.
func (a auditCoverage) unaudited() int { return a.Auditable - a.Audited }

// worklistRow names one requirement whose verdict is anything other than a
// fresh `enforced`.
type worklistRow struct {
	RFC    string `json:"rfc"`
	RID    string `json:"rid"`
	Reason string `json:"reason"`
}

// polarityCovered answers whether a requirement's polarity coverage is
// COMPLETE, by the rule the audit schema uses.
//
// A `{single-polarity}` requirement is exempt from the both-polarity demand:
// the annotation IS the missing polarity's justification, and it is what makes
// an `enforced` verdict legal over one test. Deriving the same fact from tags
// alone, without reading the annotation, made the coverage rollup and the
// schema disagree about what complete coverage is -- and every annotated
// requirement fell outside Auditable while the schema was happy to judge it.
func polarityCovered(req Requirement, found []Tag) bool {
	if bothPolarities(found) {
		return true
	}
	return len(found) > 0 && req.Annotation != nil &&
		req.Annotation.Kind == AnnotationSinglePolarity
}

// auditCoverageInput is what the audit rollup reads.
type auditCoverageInput struct {
	Requirements []Requirement
	Tags         []Tag
	Enrolled     map[string]bool
	Carriers     []Carrier
	Audits       map[string]Audit
	States       map[string]Freshness
}

// auditCoverageRows answers per-RFC audit coverage, and the worklist of every
// requirement that is not proven.
//
// This is deliberately NOT the polarity rollup. That one answers "which
// polarities exist"; subtracting an audit verdict from it would contradict that
// doctrine outright, and would break the partition internal/le/testhealth/actions.go
// asserts. So Proven is a SEPARATE count in a separate section: a requirement
// with both polarities and a `weak` verdict is counted in Both (it has both
// polarities -- true) and NOT in Proven (it is not proven -- also true), and the
// worklist names the verdict as the reason.
//
// Proven requires the verdict to be FRESH as well as `enforced`: a stale verdict
// describes a test that has since changed, so publishing it as proof is the
// stale assurance this whole machinery exists to stop.
func auditCoverageRows(in auditCoverageInput) ([]auditCoverage, []worklistRow) {
	byRID := tagsByRID(in.Tags)
	rows := make([]auditCoverage, 0, len(in.Enrolled))
	worklist := make([]worklistRow, 0)
	for _, rfc := range sortedSet(in.Enrolled) {
		row := auditCoverage{RFC: rfc}
		seen := map[string]bool{}
		recorded := in.Audits[rfc]
		for _, req := range in.Requirements {
			if req.RFC != rfc || seen[req.RID] {
				continue
			}
			seen[req.RID] = true
			verdict, held := recorded.Verdict(req.RID)
			// The requirement view: gated, and polarity coverage complete,
			// reading the same coverage rule the schema reads.
			if req.Gated() && polarityCovered(req, byRID[req.RID]) {
				row.Auditable++
				if held {
					row.Audited++
				}
			}
			if !held {
				continue
			}
			// The record view is TOTAL over recorded verdicts, and
			// deliberately not gated on either flag above: a verdict is
			// schema-legal on any requirement of the RFC, so counting the
			// both-polarity subset left real judgements in no column and,
			// when fresh and enforced, in no worklist row -- the gate then
			// published "everything I hold is proven" over a record that
			// said otherwise. A verdict whose rid matches NO requirement is
			// counted here by neither: the schema check owns that as a
			// violation, which is why this walk can be driven from the
			// requirements and still be total.
			row.Verdicts++
			value := verdictValue(verdict)
			state := FreshState
			if recordedState, known := in.States[req.RID]; known {
				state = recordedState.State
			}
			if value == VerdictEnforced && state == FreshState {
				row.Proven++
				continue
			}
			row.Findings++
			reason := value
			if state != FreshState {
				var tb textbuf.Buffer
				reason = tb.Str(value).Str(" (").Str(state).Byte(')').String()
			}
			worklist = append(worklist, worklistRow{RFC: rfc, RID: req.RID, Reason: reason})
		}
		rows = append(rows, row)
	}
	// Sorted, not append-ordered: the ledger's byte content is compared by the
	// freshness gate, and the requirements arrive in whatever order the
	// summaries were walked in, so an append-ordered worklist would report a
	// fresh ledger as stale on another machine.
	sort.Slice(worklist, func(i, j int) bool {
		a, b := worklist[i], worklist[j]
		if a.RFC != b.RFC {
			return a.RFC < b.RFC
		}
		if a.RID != b.RID {
			return a.RID < b.RID
		}
		return a.Reason < b.Reason
	})
	return rows, worklist
}

// verdictValue reads the `verdict` field of one recorded judgement.
//
// The record has already passed the schema when it reaches here, so the field
// is a string. A document that reached this without it answers the empty
// string, which matches no verdict in the vocabulary and therefore counts as a
// finding rather than as proof.
func verdictValue(verdict map[string]any) string {
	value, _ := verdict["verdict"].(string)
	return value
}

// sourceKeywordCount counts MUST-level keywords in the RFC's own text, or
// answers held=false when this checkout does not have it.
//
// This is the ground truth the summary is supposed to capture. Comparing it
// against the captured count is what exposes a summary that quietly captured
// nothing. Never 0 for an absent source: "I could not look" must not render as
// "nothing was there".
func sourceKeywordCount(tree, stem string) (int, bool) {
	text, held := SourceText(tree, stem)
	if !held {
		return 0, false
	}
	return len(sourceKeywordRE.FindAllString(text, -1)), true
}

// sourceProseKeywordCount counts LOWERCASE must/shall in the RFC's own text.
//
// Read alongside sourceKeywordCount, not instead of it. A zero uppercase count
// with a large lowercase count is the pre-2119 signature: RFC 1035 (1987) has 0
// uppercase MUST and 23 lowercase `must`, and reading the uppercase count alone
// declares the DNS wire format free of obligations. The pair is what tells that
// apart from a genuinely non-normative document, which shows 0 for both.
func sourceProseKeywordCount(tree, stem string) (int, bool) {
	text, held := SourceText(tree, stem)
	if !held {
		return 0, false
	}
	return len(sourceProseKeywordRE.FindAllString(text, -1)), true
}

// unconvertedSummary is one summary that declares no MUST-level requirement,
// with both source keyword counts. Held is false for a stem with no source
// text: the verdict cell says "cannot judge" rather than showing a zero.
type unconvertedSummary struct {
	Stem      string `json:"stem"`
	Upper     int    `json:"upper"`
	UpperHeld bool   `json:"upper-held"`
	Prose     int    `json:"prose"`
	ProseHeld bool   `json:"prose-held"`
}

// unconvertedSummaries answers every summary that captured no GATED
// requirement, with the source keyword counts beside it.
//
// A summary listing zero obligations is either a genuinely non-normative
// reference or a capture failure. The difference is visible only against the
// source text. Reporting these is the point: an absent summary is
// indistinguishable from a compliant one, which is how a standards claim rots.
//
// `captured` must mean "captured a GATED requirement". Passing every parsed
// requirement at any level bought a summary immunity from this table for ONE
// advisory row: a summary with four SHOULDs and zero MUSTs counted as captured
// and never appeared, which is exactly the shape this table exists to expose.
func unconvertedSummaries(tree string, stems map[string]bool,
	captured map[string]bool) []unconvertedSummary {
	out := make([]unconvertedSummary, 0)
	for _, stem := range sortedSet(stems) {
		if captured[stem] {
			continue
		}
		row := unconvertedSummary{Stem: stem}
		row.Upper, row.UpperHeld = sourceKeywordCount(tree, stem)
		row.Prose, row.ProseHeld = sourceProseKeywordCount(tree, stem)
		out = append(out, row)
	}
	return out
}

// capturedGated answers the stems that declare at least one MUST-level row.
func capturedGated(requirements []Requirement) map[string]bool {
	out := map[string]bool{}
	for _, req := range requirements {
		if req.Gated() {
			out[req.RFC] = true
		}
	}
	return out
}

// joinBackticked writes each entry wrapped in backticks, comma-separated.
func joinBackticked(items []string) string {
	var tb textbuf.Buffer
	for i, one := range items {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Byte('`').Str(one).Byte('`')
	}
	return tb.String()
}
