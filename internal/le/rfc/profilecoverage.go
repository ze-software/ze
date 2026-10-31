// Design: docs/contributing/rfc-conformance-gates.md -- coverage figures per RFC, profile and aggregate
// Related: profile.go -- the profiles and aggregates these figures roll up over
// Related: sectioncounts.go -- the split of these figures into the two bars' sections
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
		return noRowsCell
	}
	return tb.Int(int64(f.Proven)).Str(" of ").Int(int64(f.Total)).Str(" (").
		Float(100*float64(f.Proven)/float64(f.Total), 1).Str("%)").String()
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
	// OSVerified and OSGap split the gated rows a layer under Ze performs
	// (traits OSMet, `{lower-layer}`): OS verified when osVerified holds, an
	// OS testing gap otherwise. They sum to Mandatory.Excluded[lower-layer]
	// and change no other count, since Ze proves neither. One shipped kernel
	// is the only OS a record speaks for, so neither has a per-version split.
	OSVerified int
	OSGap      int
	// OptionalOSVerified and OptionalOSGap split the optional rows a layer
	// under Ze performs the same way, with the same osVerified predicate and
	// producer match: a kernel-met row is OS verified or an OS testing gap at
	// every level, never "not owed" (owner, 2026-10-31). They sum to
	// Optional.Excluded[lower-layer] and change no other count.
	OptionalOSVerified int
	OptionalOSGap      int
	// NotYet, NotEver and NotDecided split the optional rows that are owed,
	// not proven and not excluded (Optional.Attainable less Optional.Proven)
	// by their {intent} marker: not yet, not ever, or no marker at all. They
	// change no other count. Proven is never declared, so a proven row lands
	// in none of them whatever marker it carries.
	NotYet     int
	NotEver    int
	NotDecided int
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
// proof: `./le rfc tree`, the published sections and the pages each read
// it, so no surface counts "proven" its own way.
//
// The population is every summary file and every declared Meta, so a Meta
// whose stem lists no file still answers rather than going missing.
func StemFiguresOf(in *RenderInput) (map[string]StemFigures, error) {
	return stemFiguresWith(in, figureEvidenceOf(in))
}

// figureEvidenceOf answers what the figures of a rendered tree read beside its
// proof set, with the tree's own evidence reader as the exclusion judge. A
// caller that reads rows beside the figures (priorityCorpusOf) takes this one
// value, so a row and the figure it sits in are judged by one rule.
func figureEvidenceOf(in *RenderInput) figureEvidence {
	return figureEvidence{tags: tagsByRID(in.Tags), carriers: in.Carriers, audits: in.Audits,
		osProofs: osProofsOf(in.Discrimination), exclusions: newEvidenceReader(in.Tree)}
}

// stemFiguresWith answers StemFiguresOf over the evidence a caller holds.
func stemFiguresWith(in *RenderInput, evidence figureEvidence) (map[string]StemFigures, error) {
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
	figures := figuresByStem(in.Requirements, stems, walked, provenRequirements(in.Discrimination), evidence)
	// Every walked RFC's figures must split into the two bars' sections, so
	// a rollup or a row can sum and print them (sectionsOf) without a check
	// of its own.
	for stem := range figures {
		one := figures[stem]
		if !one.Walked {
			continue
		}
		if _, err := SectionsOf(stem, &one); err != nil {
			return nil, err
		}
	}
	return figures, nil
}

// exclusionJudge answers what a row an annotation says is not owed lacks of
// the evidence its kind owes, and whether it lacks any. Every figure the
// tooling publishes is judged by (*evidenceReader).notOwedGap, the check a new
// row must pass; a unit test of the arithmetic alone may stand in a judge that
// accepts every row.
type exclusionJudge interface {
	notOwedGap(req *Requirement) (string, bool)
}

// figureEvidence is what the figures read beside the proof set: the tags that
// say which kind of test covers a row, the carriers that classify those tags,
// and the audit verdicts that can name a row unmet.
type figureEvidence struct {
	tags     map[string][]Tag
	carriers []Carrier
	audits   map[string]Audit
	// osProofs is osProofsOf over the tree's verdicts: the producers each
	// proving record broke, by requirement id, which osVerified holds a
	// {lower-layer} row's named producer against.
	osProofs map[string][]string
	// exclusions judges the evidence of a row an annotation says is not owed
	// (excluded). It MUST be set whenever a row carries such an annotation:
	// without it the figures cannot tell an evidenced row from a bare one.
	exclusions exclusionJudge
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

// excluded answers whether a row leaves the rows Ze's own code owes
// (Attainable). A row whose annotation's traits do not leave stays. A row a
// layer under Ze performs (osMet) leaves for the OS sections whatever its
// evidence: those stay in the MUST bar, so the move can only lower the
// Ze-proven share, never raise it. A row an annotation says is not owed leaves
// only when its evidence passes the check a new row must pass
// ((*evidenceReader).notOwedGap); one the exclusion evidence backlog
// shelters, or whose evidence fails, stays owed as a Ze correctness gap (an
// open row at SHOULD level), so annotating a row never raises a figure on
// unchecked evidence (owner ruling, 2026-10-29).
func (e figureEvidence) excluded(requirement *Requirement) bool {
	if !ceilingExcluded(requirement) {
		return false
	}
	if osMet(requirement) {
		return true
	}
	if e.exclusions == nil {
		panic("BUG: figureEvidence holds no evidence reader, so a not-owed row cannot be judged: " + requirement.RID)
	}
	_, lacks := e.exclusions.notOwedGap(requirement)
	return !lacks
}

// figuresByStem answers both figures for every stem.
//
// stems is every summary; a stem with no requirement row still answers, with
// zero totals. walked is the stems carrying a valid extraction sign-off.
// proven is provenRequirements over the tree's verdicts. evidence classifies
// each proven mandatory row by kind and names the unmet rows.
//
// Which rows count, by annotation (owner ruling 2026-09-02), proven only by a
// verified record:
//   - `{rollup}` is out of both sides: it carries no obligation of its own.
//   - `{single-polarity}` is the one annotation that can be proven: its
//     annotation records why the other side has no input, and the row is
//     proven only when a verified record proves its one side.
//   - `{not-applicable}`, `{lower-layer}`, `{feature-declined}` and
//     `{linux-limit}` are never proven by Ze, whatever records their tags
//     carry. Each leaves the attainable rows only through excluded, so a
//     not-owed row whose evidence fails its check stays owed.
//     A `{lower-layer}` row's proving record makes it OS verified instead, at
//     either level (countOS), a count apart from Ze's proven rows.
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
		figures.countOS(requirement, evidence.osProofs)
		excluded := evidence.excluded(requirement)
		if requirement.Gated() {
			figures.Mandatory.count(requirement, met, excluded)
			if met {
				figures.countKind(evidence.strongestKind(requirement.RID))
			}
		} else {
			figures.countIntent(requirement, met, excluded)
			figures.Optional.count(requirement, met, excluded)
		}
		out[requirement.RFC] = figures
	}
	for stem := range out {
		slices.Sort(out[stem].Unmet)
	}
	return out
}

// countOS adds one row a layer under Ze performs to the OS verified or OS gap
// count of its level, gated (OSVerified, OSGap) or not (OptionalOSVerified,
// OptionalOSGap), and leaves every other row alone. One predicate serves both
// levels, so a kernel-met row is split the same way at every level (owner,
// 2026-10-31).
func (s *StemFigures) countOS(requirement *Requirement, proofs map[string][]string) {
	if !osMet(requirement) {
		return
	}
	verified, gap := &s.OptionalOSVerified, &s.OptionalOSGap
	if requirement.Gated() {
		verified, gap = &s.OSVerified, &s.OSGap
	}
	if osVerified(requirement, proofs) {
		*verified++
		return
	}
	*gap++
}

// countIntent adds one owed optional row that is not proven and not excluded
// to NotYet, NotEver or NotDecided by its {intent} marker, and leaves every
// other row alone.
func (s *StemFigures) countIntent(requirement *Requirement, met, excluded bool) {
	if met {
		return
	}
	if excluded {
		return
	}
	if requirement.Intent == nil {
		s.NotDecided++
		return
	}
	switch requirement.Intent.State {
	case IntentNotYet:
		s.NotYet++
	case IntentNotEver:
		s.NotEver++
	case IntentUnspecified:
		panic("BUG: an intent marker parsed to no state")
	default:
		panic("BUG: unexpected intent state")
	}
}

// osMet answers whether a layer under Ze performs the row: its annotation is
// a declared kind whose traits say OSMet. An unknown kind is not OS met.
func osMet(requirement *Requirement) bool {
	if requirement.Annotation == nil {
		return false
	}
	traits, known := AnnotationTraitsOf(requirement.Annotation.Kind)
	return known && traits.OSMet
}

// osVerified answers whether a row a layer under Ze performs is OS VERIFIED:
// a tagged unit covering it carries a record that still verifies and proves,
// and that record broke the producer the row's annotation names.
//
// proofs is osProofsOf over the tree's verdicts, so the predicate is
// DiscriminationVerdict.Proven, the one that proves a Ze row (spec R-4): a
// tag alone, a stale record or the no-break escape leaves an OS testing gap.
// The producer match is what makes the proof about the state Ze installs: the
// annotation names the function installing it, and a red observed by breaking
// any other function, in the same file or not, proves a different fact.
func osVerified(requirement *Requirement, proofs map[string][]string) bool {
	if !osMet(requirement) {
		return false
	}
	return slices.Contains(proofs[requirement.RID], requirement.Annotation.Producer)
}

// osProofsOf answers, by requirement id, every producer a proving verdict
// broke: the record's own and each paired break's, since the red was observed
// with all of them applied together.
func osProofsOf(verdicts []DiscriminationVerdict) map[string][]string {
	out := map[string][]string{}
	for index := range verdicts {
		verdict := &verdicts[index]
		if !verdict.Proven() {
			continue
		}
		rid := verdict.Record.RID
		out[rid] = append(out[rid], verdict.Record.Producer)
		for _, paired := range verdict.Record.Paired {
			out[rid] = append(out[rid], paired.Producer)
		}
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

// count adds one owed row. Only a declared ceiling exclusion the figures accept
// (figureEvidence.excluded) removes attainable work; gaps and partials remain
// future work, and unknown kinds get no exemption.
func (f *Figure) count(requirement *Requirement, proven, excluded bool) {
	f.Total++
	if proven {
		f.Proven++
	}
	if excluded {
		if f.Excluded == nil {
			f.Excluded = make(map[string]int)
		}
		f.Excluded[requirement.Annotation.Kind]++
		return
	}
	f.Attainable++
}

// ceilingExcluded answers whether a row's annotation is a declared kind whose
// traits leave it out of the attainable rows. An unknown kind is not excluded.
func ceilingExcluded(requirement *Requirement) bool {
	if requirement.Annotation == nil {
		return false
	}
	traits, known := AnnotationTraitsOf(requirement.Annotation.Kind)
	return known && traits.CeilingExcluded
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
	// Sections is the walked members' section counts summed (sectionsOf),
	// the two bars `./le rfc tree` and `./le rfc next` print.
	Sections Sections `json:"sections"`
}

// rollupOf sums the figures of members. Only walked members contribute rows:
// an unwalked list is unbounded, so its rows would make the figure a
// measurement of the list. Each member's sections come from sectionsOf, which
// StemFiguresOf has already checked for every walked RFC.
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
		sections := sectionsOf(&one)
		out.Sections.add(&sections)
	}
	return out
}

// mandatoryComplete answers whether the set has a member, no member is
// unwalked, and its MUST bar holds neither a Ze correctness gap nor an OS
// testing gap: every MUST row the stack owes is Ze proven or OS verified, and a
// row not owed blocks nothing (AC-9). A set with no member is never complete:
// 0 of 0 measures nothing (AC-5).
func (r Rollup) mandatoryComplete() bool {
	if r.RFCs == 0 {
		return false
	}
	if r.NotWalked != 0 {
		return false
	}
	if r.Sections.Must.ZeGap != 0 {
		return false
	}
	return r.Sections.Must.OSGap == 0
}

// String renders the rollup's sections with its count of members that add no
// rows. A set no walked member adds a row to renders `no rows`: 0 of 0 is
// neither complete nor empty, and a bar of zeros would read as a measurement.
func (r Rollup) String() string {
	var tb textbuf.Buffer
	if r.Sections.Must.Total+r.Sections.Should.Total == 0 {
		tb.Str(noRowsCell)
	} else {
		tb.Str(r.Sections.String())
	}
	tb.Str("; ").Int(int64(r.RFCs)).Str(" RFCs, ").Int(int64(r.NotWalked)).Str(" not walked")
	return tb.String()
}
