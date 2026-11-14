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
	// carriers.go), and the OS verified mandatory rows under kindOS. Every
	// row MustProven counts lands in exactly one kind, so the counts sum to
	// MustProven; a Ze-proven row whose tags reach no carrier lands under
	// kindUnknown, the label evidenceLabel gives such a path, rather than
	// leaving the sum short in silence.
	MandatoryByKind map[string]int
	// Unmet is the sorted ids of the owed rows the summary or the audit says
	// are unmet, at every level: an annotation whose declared traits say Unmet
	// (`{gap}`, `{partial}`, `{linux-limit}`), or a recorded audit verdict
	// auditVerdictUnmet names. It is listed whether or not the summary is
	// walked, because it is the summary's own disclosure, not a measurement.
	Unmet []string
	// OSVerified and OSGap split the gated rows a layer under Ze performs
	// (traits OSMet, `{lower-layer}`): OS verified when osVerified holds, an
	// OS testing gap otherwise. They sum to Mandatory.Excluded[lower-layer].
	// Ze's tests prove neither, so Mandatory.Proven holds neither; an OS
	// verified row is done all the same, so the published proven count
	// (MustProven) counts it (owner, 2026-11-13). One shipped kernel is the
	// only OS a record speaks for, so neither has a per-version split.
	OSVerified int
	OSGap      int
	// OptionalOSVerified and OptionalOSGap split the optional rows a layer
	// under Ze performs the same way, with the same osVerified predicate and
	// producer match: a kernel-met row is OS verified or an OS testing gap at
	// every level, never "not owed" (owner, 2026-10-31). They sum to
	// Optional.Excluded[lower-layer] and change no other count.
	OptionalOSVerified int
	OptionalOSGap      int
	// ProofPending, ExclusionUnchecked, NotAssessed and NotBuilt split the MUST
	// rows that are owed, not proven and not excluded (Mandatory.Attainable
	// less Mandatory.Proven), and the Optional ones the open optional rows the
	// same way: each row lands in the one section openSectionOf names
	// (countOpen). All four are counted row by row, never as a remainder, so
	// SectionsOf can refuse a split whose sections do not add back to the open
	// rows.
	ProofPending               int
	ExclusionUnchecked         int
	NotAssessed                int
	NotBuilt                   int
	OptionalProofPending       int
	OptionalExclusionUnchecked int
	OptionalNotAssessed        int
	OptionalNotBuilt           int
	// NotYet, NotEver and NotDecided split the same open optional rows by
	// their {intent} marker: not yet, not ever, or no marker at all. They
	// change no other count. Proven is never declared, so a proven row lands
	// in none of them whatever marker it carries. They are private planning
	// facts: no section and no published shape carries them, and only the
	// text of `./le rfc tree` and `./le rfc next` prints them (intentCounts).
	NotYet     int
	NotEver    int
	NotDecided int
}

// notWalked is the cell an unwalked RFC renders in place of a figure.
const notWalked = "not walked"

// kindOS is the evidence kind MandatoryByKind counts an OS verified MUST row
// under. It is not a carrier kind (carriers.go): a carrier names the layer a
// Ze test exercises, while this names who performs the row.
const kindOS = "os"

// MustProven answers the MUST rows this RFC publishes as proven: the rows a
// verified record proves Ze meets, and the rows a layer under Ze performs
// whose record proves it (OS verified). The status page Proof cell, the tree
// row's published `mandatory-proven` and every reader of that count take it
// from here, so they answer one number.
func (s *StemFigures) MustProven() int { return mustProven(s.Mandatory.Proven, s.OSVerified) }

// mustProven is the ONE definition of a published MUST proven count: Ze proven
// plus OS verified. A requirement the operating system meets counts as done
// in every published figure once its test proves it (owner, 2026-11-13), while
// the MUST bar keeps the two as separate segments, because who performs a row
// is what the bar shows. A row on the no-break or kernel-producer escape, or a
// lower-layer row with no proving record, is in neither operand: the escape is
// published as escaped, never as proven.
func mustProven(zeProven, osVerified int) int { return zeProven + osVerified }

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

// tested answers whether a non-gap `RFC requirement:` tag covers the row,
// whatever carrier it reaches and whether or not a record proves it. It is
// what tells proof pending from not assessed (openSectionOf): a row no such
// tag covers has no test whose break could prove it, so nothing yet says
// whether Ze meets it (owner, 2026-11-13: "Keep the number real.").
//
// A gap tag is skipped, as strongestKind skips it: it demonstrates the
// obligation is unmet, so it is no test of the requirement met.
func (e figureEvidence) tested(rid string) bool {
	for _, tag := range e.tags[rid] {
		if !tag.Gap {
			return true
		}
	}
	return false
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
// shelters, or whose evidence fails, stays owed at its level, as exclusion
// unchecked, or as not built for a {linux-limit} row, whose traits say unmet
// (openSectionOf), so annotating a row never raises a figure on unchecked
// evidence (owner ruling, 2026-10-29).
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
//     either level (countOS), a count apart from Ze's proven rows that the
//     published proven count adds back (MustProven), under kindOS.
//   - `{gap}` and `{partial}` stay in the denominator and are NEVER proven,
//     whatever records their tags carry: the summary itself says the
//     obligation is unmet. A test demonstrating a gap, or the half of a
//     partial row that is met, is not the requirement met. This is what
//     holds a requirement only unreached code meets at 0% (owner, 2026-10-25:
//     the IKE virtual-IP pool, `RFC3948-5.1-1`).
//
// Every owed row that is neither proven nor excluded lands, at its level, in
// exactly one of proof pending, exclusion unchecked, not assessed and not
// built, by the one rule both levels share (countOpen, openSectionOf).
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
		unmet := evidence.unmet(requirement)
		if unmet && !listed[requirement.RID] {
			listed[requirement.RID] = true
			figures.Unmet = append(figures.Unmet, requirement.RID)
		}
		figures.countOS(requirement, evidence.osProofs)
		excluded := evidence.excluded(requirement)
		figures.countOpen(requirement, met, excluded, unmet, evidence.tested(requirement.RID))
		if requirement.Gated() {
			figures.Mandatory.count(requirement, met, excluded)
			// The kinds sum to MustProven: a Ze-proven row under its
			// strongest test kind, an OS verified row under kindOS. No row is
			// both, because a lower-layer row's traits never prove.
			if met {
				figures.countKind(evidence.strongestKind(requirement.RID))
			} else if osVerified(requirement, evidence.osProofs) {
				figures.countKind(kindOS)
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

// openSection is the section of a bar an owed row that is neither proven nor
// excluded lands in. Zero is no section, so a row openSectionOf never placed
// cannot read as one.
type openSection uint8

const (
	openUnspecified openSection = iota
	// openProofPending: a tagged test covers the row, and no recorded break
	// proves it yet.
	openProofPending
	// openExclusionUnchecked: a not-owed claim whose evidence fails its check.
	openExclusionUnchecked
	// openNotAssessed: no tagged test covers the row, and nothing says it is
	// unmet, so nothing yet says whether Ze meets it.
	openNotAssessed
	// openNotBuilt: the summary or the audit says the obligation is unmet, or
	// the owner declined the optional row.
	openNotBuilt
)

// openSectionOf is the ONE rule that places an open row (owed, not proven,
// not excluded) in a section, at both levels:
//   - not built when unmet holds (figureEvidence.unmet: a `{gap}` or
//     `{partial}` row, a `{linux-limit}` row whose register entry fails its
//     check, or an audit verdict of wrong, unimplemented or partial), or when
//     the owner marked the row `{intent: not-ever}`: its proof is not pending;
//   - exclusion unchecked for a `{not-applicable}` or `{feature-declined}`
//     row whose evidence fails its check (ceilingExcluded, yet not excluded):
//     its kind bars tests, so no proof can be recorded while it keeps the
//     annotation (owner, 2026-11-13);
//   - proof pending for an open row a non-gap tag covers (tested): a tag with
//     no record, a stale record or a no-break escape;
//   - not assessed for every other open row: no tagged test covers it, so
//     calling its proof pending would claim a test that does not exist
//     (owner, 2026-11-13: "Keep the number real.").
//
// A `{lower-layer}` row is always excluded (figureEvidence.excluded), so it
// never reaches here: it is OS verified or an OS testing gap (countOS).
// unmet is figureEvidence.unmet over the row and tested is
// figureEvidence.tested over its id, which the caller already asked.
func openSectionOf(requirement *Requirement, unmet, tested bool) openSection {
	if unmet {
		return openNotBuilt
	}
	if declinedForever(requirement) {
		return openNotBuilt
	}
	if ceilingExcluded(requirement) {
		return openExclusionUnchecked
	}
	if tested {
		return openProofPending
	}
	return openNotAssessed
}

// declinedForever answers whether the owner marked the row `{intent:
// not-ever}`: Ze will not support it. The marker is legal only on an optional
// row (intentOnGatedRow), so a gated row never carries it.
func declinedForever(requirement *Requirement) bool {
	if requirement.Intent == nil {
		return false
	}
	return requirement.Intent.State == IntentNotEver
}

// countOpen adds one owed row that is neither proven nor excluded to the
// section openSectionOf names, at its level, gated (ProofPending,
// ExclusionUnchecked, NotAssessed, NotBuilt) or not (the Optional ones), and
// leaves every other row alone. Both levels call it, so one rule splits both
// bars.
func (s *StemFigures) countOpen(requirement *Requirement, met, excluded, unmet, tested bool) {
	if met {
		return
	}
	if excluded {
		return
	}
	pending, unchecked := &s.OptionalProofPending, &s.OptionalExclusionUnchecked
	unassessed, notBuilt := &s.OptionalNotAssessed, &s.OptionalNotBuilt
	if requirement.Gated() {
		pending, unchecked = &s.ProofPending, &s.ExclusionUnchecked
		unassessed, notBuilt = &s.NotAssessed, &s.NotBuilt
	}
	switch openSectionOf(requirement, unmet, tested) {
	case openProofPending:
		*pending++
	case openExclusionUnchecked:
		*unchecked++
	case openNotAssessed:
		*unassessed++
	case openNotBuilt:
		*notBuilt++
	case openUnspecified:
		panic("BUG: openSectionOf placed an open row in no section: " + requirement.RID)
	}
}

// intentCounts is the private planning split of the open optional rows by
// their {intent} marker (countIntent). No published shape carries it: the
// text of `./le rfc tree` and `./le rfc next` prints it after the sections, a
// row from the figures it was computed from, a rollup from its members' sum.
// A row or rollup read back from published JSON holds none, and prints no
// intent clause rather than a false zero.
type intentCounts struct {
	notEver, notYet, notDecided int
}

// intents answers the figures' intent counts.
func (s *StemFigures) intents() intentCounts {
	return intentCounts{notEver: s.NotEver, notYet: s.NotYet, notDecided: s.NotDecided}
}

// add sums other into c.
func (c *intentCounts) add(other intentCounts) {
	c.notEver += other.notEver
	c.notYet += other.notYet
	c.notDecided += other.notDecided
}

// writeIntents renders the intent clause, `; intent: not ever N, not yet N,
// not decided N`, a shape no section clause takes (Sections.String). A nil
// count writes nothing: it is the state of a row or rollup decoded from JSON,
// which holds no intent.
func writeIntents(tb *textbuf.Buffer, intent *intentCounts) {
	if intent == nil {
		return
	}
	tb.Str("; intent: not ever ").Int(int64(intent.notEver)).Str(", not yet ").Int(int64(intent.notYet)).
		Str(", not decided ").Int(int64(intent.notDecided))
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

// countKind adds one mandatory row MustProven counts under its evidence kind.
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
// declared members, or an aggregate's members. It is published through
// MarshalJSON (rollup_published.go) under the flat keys a tree row publishes,
// never by its own field names.
type Rollup struct {
	// RFCs counts every member.
	RFCs int
	// NotWalked counts the members that contribute no rows. It is stated
	// beside the figures, because a set whose figure reads 100% while members
	// are unwalked is not complete.
	NotWalked int
	// Mandatory and Optional are the walked members' figures summed.
	// Mandatory.Proven is Ze proven alone; the published `mandatory-proven`
	// is Sections.Must.Proven, Ze proven plus OS verified. A rollup decoded
	// from JSON holds only their proven and total counts.
	Mandatory Figure
	Optional  Figure
	// Sections is the walked members' section counts summed (sectionsOf),
	// the two bars `./le rfc tree` and `./le rfc next` print.
	Sections Sections
	// intent is the walked members' intent counts summed, private and never
	// marshaled: String prints it after the sections. nil for a rollup
	// decoded from JSON, which holds no intent.
	intent *intentCounts
}

// rollupOf sums the figures of members. Only walked members contribute rows:
// an unwalked list is unbounded, so its rows would make the figure a
// measurement of the list. Each member's sections come from sectionsOf, which
// StemFiguresOf has already checked for every walked RFC.
func rollupOf(figures map[string]StemFigures, members []string) Rollup {
	out := Rollup{}
	intent := intentCounts{}
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
		intent.add(one.intents())
	}
	out.intent = &intent
	return out
}

// mandatoryComplete answers whether the set has a member, no member is
// unwalked, and its MUST bar holds no proof pending, exclusion unchecked, not
// assessed, not built or OS testing gap row: every MUST row the stack owes is
// Ze proven or OS verified, and a row not owed blocks nothing (AC-9). A set
// with no member is never complete: 0 of 0 measures nothing (AC-5).
func (r Rollup) mandatoryComplete() bool {
	if r.RFCs == 0 {
		return false
	}
	if r.NotWalked != 0 {
		return false
	}
	must := &r.Sections.Must
	if must.ProofPending != 0 {
		return false
	}
	if must.ExclusionUnchecked != 0 {
		return false
	}
	if must.NotAssessed != 0 {
		return false
	}
	if must.NotBuilt != 0 {
		return false
	}
	return must.OSGap == 0
}

// String renders the rollup's sections, then its intent counts when it holds
// them, with its count of members that add no rows. A set no walked member
// adds a row to renders `no rows`: 0 of 0 is neither complete nor empty, and
// a bar of zeros would read as a measurement.
func (r Rollup) String() string {
	var tb textbuf.Buffer
	if r.Sections.Must.Total+r.Sections.Should.Total == 0 {
		tb.Str(noRowsCell)
	} else {
		tb.Str(r.Sections.String())
		writeIntents(&tb, r.intent)
	}
	tb.Str("; ").Int(int64(r.RFCs)).Str(" RFCs, ").Int(int64(r.NotWalked)).Str(" not walked")
	return tb.String()
}
