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
	"slices"

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

// Figure carries proof and attainable counts over the same rows owed.
type Figure struct {
	Proven     int            `json:"proven"`
	Total      int            `json:"total"`
	Attainable int            `json:"attainable"`
	Excluded   map[string]int `json:"excluded"`
}

// add sums counts without sharing the source's exclusion map.
func (f *Figure) add(other Figure) {
	f.Proven += other.Proven
	f.Total += other.Total
	f.Attainable += other.Attainable
	for kind, count := range other.Excluded {
		if f.Excluded == nil {
			f.Excluded = make(map[string]int)
		}
		f.Excluded[kind] += count
	}
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

// CeilingString renders attainable rows without changing the proof figure.
// Zero rows measure nothing, so they carry no percentage.
func (f Figure) CeilingString() string {
	var tb textbuf.Buffer
	if f.Total == 0 {
		return "no rows"
	}
	return tb.Int(int64(f.Attainable)).Str(" of ").Int(int64(f.Total)).Str(" (").
		Float(100*float64(f.Attainable)/float64(f.Total), 1).Str("%)").String()
}

// StemFigures is one RFC's two figures.
//
// Exactly one of two states holds, and the cells say which: not Walked (a
// summary with no valid extraction sign-off, whose requirement list nothing
// has bounded), or walked with both figures counted. Every RFC a profile
// lists has a summary (owner: "all RFC should have a valid summary"), so
// there is no third, summary-less state.
type StemFigures struct {
	Stem string
	// Walked is whether the summary carries a valid extraction sign-off
	// (evaluateExtractions). An unwalked summary's list is a claim nobody
	// bounded, so its counts are kept and never published as a figure.
	Walked    bool
	Mandatory Figure
	Optional  Figure
	// MandatoryByKind counts the proven mandatory rows by the strongest
	// evidence kind among the tags covering each one (evidenceStrengthOrder,
	// carriers.go). Every proven mandatory row lands in exactly one kind, so
	// the counts sum to Mandatory.Proven; a proven row whose tags reach no
	// carrier lands under kindUnknown, the label evidenceLabel gives such a
	// path, rather than leaving the sum short in silence.
	MandatoryByKind map[string]int
	// Unmet is the sorted ids of the owed rows the summary or the audit says
	// are unmet, at every level: an annotation whose declared traits say Unmet
	// (`{gap}`, `{partial}`, `{linux-limit}`), or a recorded audit verdict
	// auditVerdictUnmet names. It is listed whether or not the summary is
	// walked, because it is the summary's own disclosure, not a measurement.
	Unmet []string
}

// notWalked is the cell an unwalked RFC renders in place of a figure.
const notWalked = "not walked"

// mandatoryCell renders the mandatory figure.
func (s StemFigures) mandatoryCell() string { return s.cell(s.Mandatory) }

// optionalCell renders the optional figure.
func (s StemFigures) optionalCell() string { return s.cell(s.Optional) }

// cell renders one figure in the state the RFC is in.
func (s StemFigures) cell(figure Figure) string {
	if !s.Walked {
		return notWalked
	}
	return figure.String()
}

// StemFiguresOf answers both figures, the evidence kinds and the unmet ids of
// every summary in one rendered corpus. It is the ONE derivation of per-RFC
// proof: `./le rfc tree`, the published proof share and the pages each read
// it, so no surface counts "proven" its own way.
//
// The population is every summary file and every declared Meta, so a Meta
// whose stem lists no file still answers rather than going missing.
func StemFiguresOf(in *RenderInput) (map[string]StemFigures, error) {
	signed, _, err := evaluateExtractions(in.Deriver, in.Requirements)
	if err != nil {
		return nil, err
	}
	walked := make(map[string]bool, len(signed))
	for stem := range signed {
		walked[stem] = true
	}
	stems := make(map[string]bool, len(in.Stems)+len(in.Metas))
	for stem := range in.Stems {
		stems[stem] = true
	}
	for stem := range in.Metas {
		stems[stem] = true
	}
	evidence := figureEvidence{tags: tagsByRID(in.Tags), carriers: in.Carriers, audits: in.Audits}
	return figuresByStem(in.Requirements, stems, walked, provenRequirements(in.Discrimination), evidence), nil
}

// figureEvidence is what the figures read beside the proof set: the tags that
// say which kind of test covers a row, the carriers that classify those tags,
// and the audit verdicts that can name a row unmet.
type figureEvidence struct {
	tags     map[string][]Tag
	carriers []Carrier
	audits   map[string]Audit
}

// strongestKind answers the strongest evidence kind among the tags covering
// one requirement, and kindUnknown when none reaches a carrier.
//
// A gap tag is skipped: it demonstrates the obligation is unmet, so it is no
// evidence of proof.
func (e figureEvidence) strongestKind(rid string) string {
	best, bestAt := kindUnknown, len(evidenceStrengthOrder)
	for _, tag := range e.tags[rid] {
		if tag.Gap {
			continue
		}
		carrier, held := CarrierFor(tag.File, e.carriers)
		if !held {
			continue
		}
		at := indexOf(evidenceStrengthOrder, carrier.Kind)
		if at < 0 {
			continue
		}
		if at < bestAt {
			best, bestAt = carrier.Kind, at
		}
	}
	return best
}

// unmet answers whether the summary or the audit says this row is not met:
// its annotation's declared traits say Unmet, or its recorded audit verdict is
// one auditVerdictUnmet names.
func (e figureEvidence) unmet(requirement *Requirement) bool {
	if requirement.Annotation != nil {
		traits, known := AnnotationTraitsOf(requirement.Annotation.Kind)
		if known && traits.Unmet {
			return true
		}
	}
	verdict, held := e.audits[requirement.RFC].Verdict(requirement.RID)
	return held && auditVerdictUnmet(verdictValue(verdict))
}

// figuresByStem answers both figures for every stem.
//
// stems is every summary; a stem with no requirement row still answers, with
// zero totals. walked is the stems carrying a valid extraction sign-off.
// proven is provenRequirements over the tree's verdicts. evidence classifies
// each proven mandatory row by kind and names the unmet rows.
//
// Which rows count, by annotation, is the same row partition as ProvenShareOf
// (provenshare.go, owner ruling 2026-09-02), proven only by a verified record:
//   - `{rollup}` is out of both sides: it carries no obligation of its own.
//   - `{single-polarity}` is the one annotation that can be proven: its
//     annotation records why the other side has no input, and the row is
//     proven only when a verified record proves its one side, here and in
//     ProvenShareOf, which sums these figures.
//   - `{not-applicable}`, `{lower-layer}`, `{feature-declined}` and
//     `{linux-limit}` stay in the denominator and are never proven by Ze,
//     whatever records their tags carry: annotating a row may not raise its figure.
//   - `{gap}` and `{partial}` stay in the denominator and are NEVER proven,
//     whatever records their tags carry: the summary itself says the
//     obligation is unmet. A test demonstrating a gap, or the half of a
//     partial row that is met, is not the requirement met. This is what
//     holds a requirement only unreached code meets at 0% (owner, 2026-10-25:
//     the IKE virtual-IP pool, `RFC3948-5.1-1`).
func figuresByStem(requirements []Requirement, stems, walked, proven map[string]bool,
	evidence figureEvidence,
) map[string]StemFigures {
	out := make(map[string]StemFigures, len(stems))
	for stem := range stems {
		out[stem] = StemFigures{Stem: stem, Walked: walked[stem]}
	}
	listed := map[string]bool{}
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
		if evidence.unmet(requirement) && !listed[requirement.RID] {
			listed[requirement.RID] = true
			figures.Unmet = append(figures.Unmet, requirement.RID)
		}
		if requirement.Gated() {
			figures.Mandatory.count(requirement, met)
			if met {
				figures.countKind(evidence.strongestKind(requirement.RID))
			}
		} else {
			figures.Optional.count(requirement, met)
		}
		out[requirement.RFC] = figures
	}
	for stem := range out {
		slices.Sort(out[stem].Unmet)
	}
	return out
}

// countKind adds one proven mandatory row under its evidence kind.
func (s *StemFigures) countKind(kind string) {
	if s.MandatoryByKind == nil {
		s.MandatoryByKind = make(map[string]int)
	}
	s.MandatoryByKind[kind]++
}

// count adds one owed row. Only declared ceiling exclusions remove attainable
// work; gaps and partials remain future work, and unknown kinds get no exemption.
func (f *Figure) count(requirement *Requirement, proven bool) {
	f.Total++
	if proven {
		f.Proven++
	}
	if requirement.Annotation != nil {
		traits, known := AnnotationTraitsOf(requirement.Annotation.Kind)
		if known && traits.CeilingExcluded {
			if f.Excluded == nil {
				f.Excluded = make(map[string]int)
			}
			f.Excluded[requirement.Annotation.Kind]++
			return
		}
	}
	f.Attainable++
}

// rowCounts answers whether a row is owed (in the denominator) and whether it
// is proven (in the numerator).
func rowCounts(requirement *Requirement, proven map[string]bool) (owed, met bool) {
	if requirement.Annotation == nil {
		return true, proven[requirement.RID]
	}
	traits, known := AnnotationTraitsOf(requirement.Annotation.Kind)
	if !known {
		return true, false
	}
	if !traits.Counts {
		return false, false
	}
	return true, traits.Proves && proven[requirement.RID]
}

// Rollup is the figures of a set of RFCs: one tier of a use-case profile's
// declared members, or an aggregate's members.
type Rollup struct {
	// RFCs counts every member.
	RFCs int `json:"rfcs"`
	// NotWalked counts the members that contribute no rows. It is stated
	// beside the figures, because a set whose figure reads 100% while members
	// are unwalked is not complete.
	NotWalked int    `json:"not-walked"`
	Mandatory Figure `json:"mandatory"`
	Optional  Figure `json:"optional"`
}

// rollupOf sums the figures of members. Only walked members contribute rows:
// an unwalked list is unbounded, so its rows would make the figure a
// measurement of the list.
func rollupOf(figures map[string]StemFigures, members []string) Rollup {
	out := Rollup{}
	for _, stem := range members {
		out.RFCs++
		one := figures[stem]
		if !one.Walked {
			out.NotWalked++
			continue
		}
		out.Mandatory.add(one.Mandatory)
		out.Optional.add(one.Optional)
	}
	return out
}

// mandatoryComplete answers whether the set has a member, every mandatory row
// of it is proven, and no member is unwalked. A set with no member is never
// complete: 0 of 0 measures nothing (AC-5).
func (r Rollup) mandatoryComplete() bool {
	if r.RFCs == 0 {
		return false
	}
	if r.NotWalked != 0 {
		return false
	}
	return r.Mandatory.Proven == r.Mandatory.Total
}

// String renders the rollup with its count of members that add no rows.
func (r Rollup) String() string {
	var tb textbuf.Buffer
	tb.Str("mandatory ").Str(r.Mandatory.String()).Str(", optional ").Str(r.Optional.String())
	tb.Str("; ceiling mandatory ").Str(r.Mandatory.CeilingString()).
		Str(", optional ").Str(r.Optional.CeilingString())
	writeFigureExclusions(&tb, "mandatory", r.Mandatory)
	writeFigureExclusions(&tb, "optional", r.Optional)
	tb.Str("; ").Int(int64(r.RFCs)).Str(" RFCs, ").Int(int64(r.NotWalked)).Str(" not walked")
	return tb.String()
}

// StemCeiling names a measured RFC ceiling; zero-total RFCs have no such minimum.
type StemCeiling struct {
	Stem       string `json:"stem"`
	Attainable int    `json:"attainable"`
	Total      int    `json:"total"`
}

// String renders this ceiling with the same arithmetic as its source figure.
func (s StemCeiling) String() string {
	return (Figure{Attainable: s.Attainable, Total: s.Total}).CeilingString()
}

// CeilingSummary carries a view's walked-only sums and lowest measured ceilings.
type CeilingSummary struct {
	Rollup
	LowestMandatory *StemCeiling `json:"lowest-mandatory"`
	LowestOptional  *StemCeiling `json:"lowest-optional"`
}

// CeilingOf sums a view's distinct RFC rows. Callers MUST supply each RFC once.
// Unwalked and summary-less rows contribute no measurements. Minima exclude
// empty figures, and ties choose RFC-number order rather than input order.
func CeilingOf(rows []*TreeRow) CeilingSummary {
	var out CeilingSummary
	for _, row := range rows {
		out.RFCs++
		if !row.Summary {
			out.NotWalked++
			continue
		}
		if !row.Walked {
			out.NotWalked++
			continue
		}
		figures := row.figures()
		out.Mandatory.add(figures.Mandatory)
		out.Optional.add(figures.Optional)
		out.LowestMandatory = lowerCeiling(out.LowestMandatory, row.Stem, figures.Mandatory)
		out.LowestOptional = lowerCeiling(out.LowestOptional, row.Stem, figures.Optional)
	}
	return out
}

// lowerCeiling keeps the lowest nonempty fraction without percentage rounding.
func lowerCeiling(current *StemCeiling, stem string, figure Figure) *StemCeiling {
	if figure.Total == 0 {
		return current
	}
	if current != nil {
		left := int64(figure.Attainable) * int64(current.Total)
		right := int64(current.Attainable) * int64(figure.Total)
		if left > right {
			return current
		}
		if left == right && statusRowCompare(stem, current.Stem) >= 0 {
			return current
		}
		current.Stem, current.Attainable, current.Total = stem, figure.Attainable, figure.Total
		return current
	}
	return &StemCeiling{Stem: stem, Attainable: figure.Attainable, Total: figure.Total}
}

// writeFigureExclusions exposes each declared classification separately.
func writeFigureExclusions(tb *textbuf.Buffer, label string, figure Figure) {
	if len(figure.Excluded) == 0 {
		return
	}
	tb.Str("; ").Str(label).Str(" excluded: ")
	separator := ""
	for _, kind := range AnnotationKinds() {
		count := figure.Excluded[kind]
		if count == 0 {
			continue
		}
		tb.Str(separator).Str(kind).Byte(' ').Int(int64(count))
		separator = ", "
	}
}
