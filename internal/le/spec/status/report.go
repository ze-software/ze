// Design: docs/architecture/core-design.md -- the spec inventory's payload
//
// The answer is the ROWS: one record per spec. A slice rather than a struct
// wrapping one, so `| json` answers the array the script's --json answered and
// `| count` counts specs (internal/le/inventory taught this: a payload whose answer
// IS the rows declares itself as the rows).
//
// Everything the page prints above the rows -- the total, the per-status
// breakdown, the three category counts -- is DERIVED from the same slice at
// render time. A breakdown printed beside a total is a claim that the two
// agree, so nothing here is carried separately from what was counted
// (plan/journal/gate-excludes-part-of-its-population.md, 2026-08-22).

package specstatus

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Spec is one row of the inventory.
type Spec struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Depends     string `json:"depends"`
	Phase       string `json:"phase"`
	Set         string `json:"set"`
	Updated     string `json:"updated"`
	GitModified string `json:"git-modified"`
	// Bucket is the RELEASE bucket, from the directory the spec sits in:
	// specpath.After, specpath.Immediate or specpath.PreRelease.
	Bucket string `json:"bucket"`
	// Category is the committed-backlog / idea-capture / other split, from the
	// spec's status. It answers a different question from Bucket: what state
	// the work is in, rather than which release owes it.
	Category string `json:"category"`
	// Stale is true for a skeleton idea past the TTL (flagged for triage).
	Stale bool `json:"stale"`
}

// Inventory is every spec the population held, in reporting order.
type Inventory []Spec

// reportingOrder is a REPORTING ORDER, not the status vocabulary. A name in it
// buys a position and nothing else, which is why "verification" is here
// (committed work in flight, so it belongs beside in-progress) and "done" is not
// (terminal, so the sorted tail below prints it last). The vocabulary itself
// lives in ai/rules/planning.md and in the oneOf call that validates a spec's
// Status row (internal/le/hookruntime.validateSpecText); a third copy here would
// drift from both.
var reportingOrder = []string{
	statusUnparsed, statusInProgress, statusVerification, statusReady,
	statusDesign, statusSkeleton, statusBlocked, statusDeferred, statusUnknown,
}

// summaryOrder answers the statuses the summary line names, in print order: the
// reporting order first, then every status it never heard of, sorted.
//
// A status the order does not name is printed after the named ones so the counts
// always sum to the total. Dropping it made the one line a reader trusts
// under-report: on 2026-08-22 the summary claimed 242 specs over six counts
// summing to 240, because two specs carry "done".
func summaryOrder(counts map[string]int) []string {
	named := make(map[string]bool, len(reportingOrder))
	for _, st := range reportingOrder {
		named[st] = true
	}
	var rest []string
	for st := range counts {
		if !named[st] {
			rest = append(rest, st)
		}
	}
	sort.Strings(rest)
	order := make([]string, 0, len(reportingOrder)+len(rest))
	order = append(order, reportingOrder...)
	return append(order, rest...)
}

// statusPhrases answers one "<count> <status>" phrase per status the counts
// hold, in summaryOrder's print order. A status counted zero times is omitted,
// and every status present is named, so the phrases always sum to the total.
//
// It is separate from Text because a second surface prints the same breakdown:
// the session-start hook, through StatusPhrases.
func statusPhrases(counts map[string]int) []string {
	phrases := make([]string, 0, len(counts))
	for _, st := range summaryOrder(counts) {
		if counts[st] == 0 {
			continue
		}
		var tb textbuf.Buffer
		phrases = append(phrases, tb.Int(int64(counts[st])).Byte(' ').Str(st).String())
	}
	return phrases
}

// categories splits the inventory three ways and counts the flagged skeletons.
// Specs arrive sorted by status order, so each category stays ordered.
func (in Inventory) categories() (backlog, ideas, other Inventory, stale int) {
	for _, s := range in {
		switch s.Category {
		case Backlog:
			backlog = append(backlog, s)
		case Idea:
			ideas = append(ideas, s)
			if s.Stale {
				stale++
			}
		default:
			other = append(other, s)
		}
	}
	return backlog, ideas, other, stale
}

// Column widths of the inventory table. The last column takes no width: it ends
// the line, so padding it would write trailing spaces.
const (
	colFlag    = 5
	colBucket  = 11
	colStatus  = 12
	colUpdated = 10
	colSpec    = 34
	colPhase   = 5
	colSet     = 10
	colDepends = 10
)

// Text renders the inventory as the page `./le spec status` prints: the summary
// line, the category line, then one section per category.
//
// It is the DEFAULT rendering (leroot.Prose). Every pipe operator still goes to
// the engine and reads the rows above.
func (in Inventory) Text() string {
	counts := map[string]int{}
	for _, s := range in {
		counts[s.Status]++
	}

	var tb textbuf.Buffer
	tb.Str("Specs: ").Int(int64(len(in))).Str(" total (")
	for i, phrase := range statusPhrases(counts) {
		if i != 0 {
			tb.Str(", ")
		}
		tb.Str(phrase)
	}
	tb.Str(")\n")

	backlog, ideas, other, stale := in.categories()
	tb.Str("Categories: committed backlog ").Int(int64(len(backlog))).
		Str(" (design/ready/in-progress/verification) | idea capture ").Int(int64(len(ideas))).
		Str(" skeletons (").Int(int64(stale)).Str(" past the ").Int(int64(SkeletonTTLWeeks)).
		Str("-week TTL) | other ").Int(int64(len(other))).Str("\n\n")

	categorySection(&tb, "Committed backlog: design / ready / in-progress / verification", backlog)
	categorySection(&tb, "Idea capture: skeleton stubs (STALE = past TTL, triage or drop)", ideas)
	categorySection(&tb, "Other: blocked / deferred / done / unknown / unparsed", other)

	return tb.String()
}

// categorySection writes one category's heading and rows.
func categorySection(tb *textbuf.Buffer, title string, rows Inventory) {
	tb.Str("── ").Str(title).Str(" (").Int(int64(len(rows))).Str(") ──\n")
	if len(rows) == 0 {
		tb.Str("  (none)\n\n")
		return
	}
	specRow(tb, "Flag", "Bucket", "Status", "Updated", "Spec", "Phase", "Set", "Depends")
	specRow(tb,
		dashes(colFlag), dashes(colBucket), dashes(colStatus), dashes(colUpdated),
		dashes(colSpec), dashes(colPhase), dashes(colSet), dashes(colDepends),
	)
	for _, s := range rows {
		flag := ""
		if s.Stale {
			flag = "STALE"
		}
		specRow(tb, flag, s.Bucket, s.Status, s.Updated, s.Name, s.Phase, s.Set, s.Depends)
	}
	tb.Byte('\n')
}

// specRow writes one padded row. The widths are in RUNES, which is what the
// fixed-width verb the script used pads by, so a box-drawing separator lands in
// the same column as an ASCII value.
func specRow(tb *textbuf.Buffer, flag, bucket, status, updated, spec, phase, set, depends string) {
	tb.PadRight(flag, colFlag).Str("  ").
		PadRight(bucket, colBucket).Str("  ").
		PadRight(status, colStatus).Str("  ").
		PadRight(updated, colUpdated).Str("  ").
		PadRight(spec, colSpec).Str("  ").
		PadRight(phase, colPhase).Str("  ").
		PadRight(set, colSet).Str("  ").
		Str(depends).Byte('\n')
}

// dashes answers the rule cell under a column that many runes wide. The unit is
// runes rather than bytes because that is what the padding counts, and the glyph
// is three bytes.
func dashes(runes int) string {
	var tb textbuf.Buffer
	return tb.Repeat("─", runes).String()
}
