// Design: docs/contributing/rfc-conformance-gates.md -- coverage figures per RFC, profile and aggregate
// Related: profile.go -- the profiles and aggregates these figures roll up over
// Related: provenshare.go -- the denominator rule these figures share (owner ruling, 2026-09-02)
//
// profilecoverage.go answers the two coverage figures the owner rule of
// 2026-10-25 orders RFC work by: a MANDATORY figure over the gated levels and
// an OPTIONAL figure over every other level, each one rows proven over rows
// owed, per RFC and summed over a set of RFCs.
//
// A row is PROVEN only when a recorded break showed its test goes red: a tagged
// unit covering it carries a discrimination record that still verifies against
// this tree and that is a proof rather than the no-break escape. A tag alone, an
// annotation alone, or a stale record proves nothing here.
package rfc

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Proven answers whether this verdict proves its cover: a record that still
// verifies against the tree, observed by a route that broke the producer.
//
// The ONE per-row proof predicate. The site's ledger publishes the same two
// facts apart (Verified and Proves) so a reader can tell a stale record from an
// escape; anything that COUNTS a proof asks this.
func (v *DiscriminationVerdict) Proven() bool { return v.Verified() && v.Record.Proves() }

// provenRequirements answers the requirement ids at least one proving verdict
// covers.
func provenRequirements(verdicts []DiscriminationVerdict) map[string]bool {
	out := map[string]bool{}
	for index := range verdicts {
		if verdicts[index].Proven() {
			out[verdicts[index].Record.RID] = true
		}
	}
	return out
}

// Figure is rows proven over rows owed.
type Figure struct {
	Proven int `json:"proven"`
	Total  int `json:"total"`
}

// add sums two figures.
func (f Figure) add(other Figure) Figure {
	return Figure{Proven: f.Proven + other.Proven, Total: f.Total + other.Total}
}

// String renders the figure. A zero Total renders no percentage at all: 0 of 0
// is neither 0% nor 100%, and either number would be read as a measurement.
func (f Figure) String() string {
	var tb textbuf.Buffer
	if f.Total == 0 {
		return "no rows"
	}
	return tb.Int(int64(f.Proven)).Str(" of ").Int(int64(f.Total)).Str(" (").
		Float(100*float64(f.Proven)/float64(f.Total), 1).Str("%)").String()
}

// StemFigures is one RFC's two figures.
//
// Exactly one of three states holds, and the cells say which: Absent (a
// profile needs the RFC and the corpus holds no summary of it), not Walked (a
// summary with no valid extraction sign-off, whose requirement list nothing
// has bounded), or walked with both figures counted.
type StemFigures struct {
	Stem string
	// Absent is an RFC a profile lists under Absent. It renders 0% on both
	// figures, never "not walked" and never blank (owner, 2026-10-25): the
	// work it stands for is owed, and a blank reads as nothing owed.
	Absent bool
	// Walked is whether the summary carries a valid extraction sign-off
	// (evaluateExtractions). An unwalked summary's list is a claim nobody
	// bounded, so its counts are kept and never published as a figure.
	Walked    bool
	Mandatory Figure
	Optional  Figure
}

// notWalked is the cell an unwalked RFC renders in place of a figure.
const notWalked = "not walked"

// absentCell is the cell an absent RFC renders on both figures.
const absentCell = "0% (absent from the corpus)"

// mandatoryCell renders the mandatory figure.
func (s StemFigures) mandatoryCell() string { return s.cell(s.Mandatory) }

// optionalCell renders the optional figure.
func (s StemFigures) optionalCell() string { return s.cell(s.Optional) }

// cell renders one figure in the state the RFC is in.
func (s StemFigures) cell(figure Figure) string {
	if s.Absent {
		return absentCell
	}
	if !s.Walked {
		return notWalked
	}
	return figure.String()
}

// absentFigures answers the figures of an RFC a profile needs and the corpus
// does not hold.
func absentFigures(number int) StemFigures {
	var tb textbuf.Buffer
	return StemFigures{Stem: tb.Str("rfc").Int(int64(number)).String(), Absent: true}
}

// figuresByStem answers both figures for every stem.
//
// stems is every summary; a stem with no requirement row still answers, with
// zero totals. walked is the stems carrying a valid extraction sign-off.
// proven is provenRequirements over the tree's verdicts.
//
// Which rows count, by annotation:
//   - `{rollup}` is out of both sides: it carries no obligation of its own
//     (provenshare.go, owner ruling 2026-09-02).
//   - `{not-applicable}` and `{lower-layer}` stay in the denominator and are
//     never proven by Ze: annotating a row may not raise its figure.
//   - `{gap}` and `{partial}` stay in the denominator and are NEVER proven,
//     whatever records their tags carry: the summary itself says the
//     obligation is unmet. A test demonstrating a gap, or the half of a
//     partial row that is met, is not the requirement met. This is what
//     holds a requirement only unreached code meets at 0% (owner, 2026-10-25:
//     the IKE virtual-IP pool, `RFC3948-5.1-1`).
func figuresByStem(requirements []Requirement, stems, walked, proven map[string]bool) map[string]StemFigures {
	out := make(map[string]StemFigures, len(stems))
	for stem := range stems {
		out[stem] = StemFigures{Stem: stem, Walked: walked[stem]}
	}
	for index := range requirements {
		requirement := &requirements[index]
		figures, held := out[requirement.RFC]
		if !held {
			continue
		}
		owed, met := rowCounts(requirement, proven)
		if !owed {
			continue
		}
		row := Figure{Total: 1}
		if met {
			row.Proven = 1
		}
		if requirement.Gated() {
			figures.Mandatory = figures.Mandatory.add(row)
		} else {
			figures.Optional = figures.Optional.add(row)
		}
		out[requirement.RFC] = figures
	}
	return out
}

// rowCounts answers whether a row is owed (in the denominator) and whether it
// is proven (in the numerator).
func rowCounts(requirement *Requirement, proven map[string]bool) (owed, met bool) {
	if requirement.Annotation == nil {
		return true, proven[requirement.RID]
	}
	switch requirement.Annotation.Kind {
	case AnnotationRollup:
		return false, false
	case AnnotationGap, AnnotationPartial:
		return true, false
	}
	return true, proven[requirement.RID]
}

// Rollup is the figures of a set of RFCs: a profile's closure, or an
// aggregate's members.
type Rollup struct {
	// RFCs counts every member, absent ones included.
	RFCs int `json:"rfcs"`
	// NotWalked and Absent count the members that contribute no rows. Both
	// are stated beside the figures, because a set whose figure reads 100%
	// while members are unwalked or absent is not complete. A member with a
	// text and no summary (a node `./le rfc tree` reaches through the graph)
	// has no walked list either, so it counts not walked.
	NotWalked int    `json:"not-walked"`
	Absent    int    `json:"absent"`
	Mandatory Figure `json:"mandatory"`
	Optional  Figure `json:"optional"`
}

// rollupOf sums the figures of members, adding absent RFCs a profile lists.
// Only walked members contribute rows: an unwalked list is unbounded, so its
// rows would make the figure a measurement of the list.
func rollupOf(figures map[string]StemFigures, members []string, absent []int) Rollup {
	out := Rollup{RFCs: len(members) + len(absent), Absent: len(absent)}
	for _, stem := range members {
		one := figures[stem]
		if !one.Walked {
			out.NotWalked++
			continue
		}
		out.Mandatory = out.Mandatory.add(one.Mandatory)
		out.Optional = out.Optional.add(one.Optional)
	}
	return out
}

// mandatoryComplete answers whether every mandatory row of the set is proven
// and no member is unwalked or absent. A set with absent RFCs is never
// complete, whatever its walked members read (owner, 2026-10-25).
func (r Rollup) mandatoryComplete() bool {
	if r.Absent != 0 {
		return false
	}
	if r.NotWalked != 0 {
		return false
	}
	return r.Mandatory.Proven == r.Mandatory.Total
}

// String renders the rollup with its two counts of members that add no rows.
func (r Rollup) String() string {
	var tb textbuf.Buffer
	tb.Str("mandatory ").Str(r.Mandatory.String()).Str(", optional ").Str(r.Optional.String())
	tb.Str("; ").Int(int64(r.RFCs)).Str(" RFCs, ").Int(int64(r.NotWalked)).Str(" not walked, ").
		Int(int64(r.Absent)).Str(" absent at 0%")
	return tb.String()
}
