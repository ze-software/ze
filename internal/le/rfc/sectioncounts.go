// Design: docs/contributing/rfc-conformance-gates.md -- coverage figures split into the two bars' sections
// Related: profilecoverage.go -- StemFigures, the per-RFC figures every section is a field or difference of
// Related: provenshare.go -- the proof share the home page and test health still publish
//
// sectioncounts.go is the ONE split of an RFC's figures into the sections the
// two bars draw (spec-rfc-figures-sectioned-presentation, AC-12). `./le rfc
// tree`, `./le rfc next`, the per-RFC files under data/rfc/, the home page and
// /quality/health/ each read it, so no surface partitions the rows a second way.
package rfc

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// MustSections splits every MUST row Ze or the stack under it owes.
//
// Total is every counted MUST row. Kept is the MUST bar: the rows the stack
// must keep for Ze to be RFC compliant, ZeProven + OSVerified + OSGap + ZeGap.
// NotOwed is the rest (not applicable, declined, OS blocked), split by
// annotation kind, so Kept + NotOwed is Total.
type MustSections struct {
	Total         int            `json:"total"`
	Kept          int            `json:"kept"`
	ZeProven      int            `json:"ze-proven"`
	OSVerified    int            `json:"os-verified"`
	OSGap         int            `json:"os-gap"`
	ZeGap         int            `json:"ze-gap"`
	NotOwed       int            `json:"not-owed"`
	NotOwedByKind map[string]int `json:"not-owed-by-kind"`
}

// ShouldSections splits every SHOULD / MAY row: proven, performed by a layer
// under Ze (OS verified or OS gap, as a MUST row is), left out by an annotation
// that says it does not bind Ze (NotOwed, by kind), or open and declared not
// ever, not yet, or not at all (not decided). The seven add up to Total.
type ShouldSections struct {
	Total         int            `json:"total"`
	Proven        int            `json:"proven"`
	OSVerified    int            `json:"os-verified"`
	OSGap         int            `json:"os-gap"`
	NotOwed       int            `json:"not-owed"`
	NotEver       int            `json:"not-ever"`
	NotYet        int            `json:"not-yet"`
	NotDecided    int            `json:"not-decided"`
	NotOwedByKind map[string]int `json:"not-owed-by-kind"`
}

// Sections is one RFC's, or one set of RFCs', two bars.
type Sections struct {
	Must   MustSections   `json:"must"`
	Should ShouldSections `json:"should"`
}

// SectionsOf splits one walked RFC's figures into the sections, and refuses
// figures that do not add back to their own totals: such figures are a defect
// in StemFiguresOf, and a bar drawn from them would disagree with its total.
// StemFiguresOf calls it for every walked RFC, so every figure it answers
// splits cleanly.
func SectionsOf(stem string, figures *StemFigures) (Sections, error) {
	if err := checkExcludedKinds(stem, figures.Mandatory.Excluded); err != nil {
		return Sections{}, err
	}
	if err := checkExcludedKinds(stem, figures.Optional.Excluded); err != nil {
		return Sections{}, err
	}
	out := sectionsOf(figures)
	if err := checkMustSections(stem, figures, &out.Must); err != nil {
		return Sections{}, err
	}
	if err := checkShouldSections(stem, figures, &out.Should); err != nil {
		return Sections{}, err
	}
	return out, nil
}

// sectionsOf is the arithmetic of SectionsOf without its checks. Only figures
// SectionsOf already accepted reach it (StemFiguresOf checks every walked RFC),
// so an exclusion kind the annotation declaration does not name cannot reach
// the not-owed count unseen.
func sectionsOf(figures *StemFigures) Sections {
	mandatory, optional := &figures.Mandatory, &figures.Optional
	out := Sections{
		Must: MustSections{Total: mandatory.Total, ZeProven: mandatory.Proven,
			OSVerified: figures.OSVerified, OSGap: figures.OSGap,
			ZeGap: mandatory.Attainable - mandatory.Proven, NotOwedByKind: notOwedByKind(mandatory.Excluded)},
		Should: ShouldSections{Total: optional.Total, Proven: optional.Proven,
			OSVerified: figures.OptionalOSVerified, OSGap: figures.OptionalOSGap,
			NotEver: figures.NotEver, NotYet: figures.NotYet, NotDecided: figures.NotDecided,
			NotOwedByKind: notOwedByKind(optional.Excluded)},
	}
	for _, count := range out.Must.NotOwedByKind {
		out.Must.NotOwed += count
	}
	for _, count := range out.Should.NotOwedByKind {
		out.Should.NotOwed += count
	}
	out.Must.Kept = out.Must.ZeProven + out.Must.OSVerified + out.Must.OSGap + out.Must.ZeGap
	return out
}

// add sums other into s without sharing its maps: a rollup sums counts before
// it divides anything, never percentages.
func (s *Sections) add(other *Sections) {
	s.Must.Total += other.Must.Total
	s.Must.Kept += other.Must.Kept
	s.Must.ZeProven += other.Must.ZeProven
	s.Must.OSVerified += other.Must.OSVerified
	s.Must.OSGap += other.Must.OSGap
	s.Must.ZeGap += other.Must.ZeGap
	s.Must.NotOwed += other.Must.NotOwed
	s.Must.NotOwedByKind = addKinds(s.Must.NotOwedByKind, other.Must.NotOwedByKind)
	s.Should.Total += other.Should.Total
	s.Should.Proven += other.Should.Proven
	s.Should.OSVerified += other.Should.OSVerified
	s.Should.OSGap += other.Should.OSGap
	s.Should.NotOwed += other.Should.NotOwed
	s.Should.NotEver += other.Should.NotEver
	s.Should.NotYet += other.Should.NotYet
	s.Should.NotDecided += other.Should.NotDecided
	s.Should.NotOwedByKind = addKinds(s.Should.NotOwedByKind, other.Should.NotOwedByKind)
}

// addKinds sums per-kind counts into a map of its own.
func addKinds(into, from map[string]int) map[string]int {
	if into == nil {
		into = map[string]int{}
	}
	for kind, count := range from {
		into[kind] += count
	}
	return into
}

// notOwedByKind answers the excluded rows that do not bind Ze: every
// exclusion kind but one a layer under Ze performs (traits OSMet), which is OS
// verified or an OS gap at every level, never "not owed" (owner, 2026-10-31).
func notOwedByKind(excluded map[string]int) map[string]int {
	out := map[string]int{}
	for kind, count := range excluded {
		traits, _ := AnnotationTraitsOf(kind)
		if traits.OSMet {
			continue
		}
		out[kind] += count
	}
	return out
}

// osRowsOf counts the excluded rows a layer under Ze performs (traits OSMet).
func osRowsOf(excluded map[string]int) int {
	rows := 0
	for kind, count := range excluded {
		traits, _ := AnnotationTraitsOf(kind)
		if traits.OSMet {
			rows += count
		}
	}
	return rows
}

// checkExcludedKinds refuses an exclusion kind the annotation declaration does
// not name: its meaning is unknown, so no section can hold it.
func checkExcludedKinds(stem string, excluded map[string]int) error {
	for kind := range excluded {
		if _, known := AnnotationTraitsOf(kind); known {
			continue
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str(stem).Str(": the figures exclude rows of annotation kind `").Str(kind).
			Str("`, which internal/le/rfc does not declare, so no section of the RFC figures can hold them. ").
			Str("Declare the kind in annotationKinds (internal/le/rfc/rfc.go) or fix the summary, then rerun the command").String())
	}
	return nil
}

// checkMustSections refuses a MUST split that does not add back.
func checkMustSections(stem string, figures *StemFigures, must *MustSections) error {
	mandatory := &figures.Mandatory
	osRows := osRowsOf(mandatory.Excluded)
	if osRows != figures.OSVerified+figures.OSGap {
		return sectionsRefusal(stem, "the OS verified and OS gap MUST rows (", figures.OSVerified+figures.OSGap,
			") differ from the MUST rows a lower layer performs (", osRows)
	}
	if mandatory.Proven > mandatory.Attainable {
		return sectionsRefusal(stem, "the Ze-proven MUST rows (", mandatory.Proven,
			") exceed the MUST rows Ze's code owes (", mandatory.Attainable)
	}
	if must.Kept+must.NotOwed != must.Total {
		return sectionsRefusal(stem, "the MUST sections (", must.Kept+must.NotOwed,
			") differ from the MUST rows counted (", must.Total)
	}
	return nil
}

// checkShouldSections refuses a SHOULD / MAY split that does not add back.
func checkShouldSections(stem string, figures *StemFigures, should *ShouldSections) error {
	optional := &figures.Optional
	osRows := osRowsOf(optional.Excluded)
	if osRows != figures.OptionalOSVerified+figures.OptionalOSGap {
		return sectionsRefusal(stem, "the OS verified and OS gap SHOULD rows (", figures.OptionalOSVerified+figures.OptionalOSGap,
			") differ from the SHOULD rows a lower layer performs (", osRows)
	}
	open := optional.Attainable - optional.Proven
	if open != should.NotEver+should.NotYet+should.NotDecided {
		return sectionsRefusal(stem, "the not ever, not yet and not decided SHOULD rows (",
			should.NotEver+should.NotYet+should.NotDecided, ") differ from the open SHOULD rows (", open)
	}
	sum := should.Proven + should.OSVerified + should.OSGap + should.NotOwed + open
	if sum != should.Total {
		return sectionsRefusal(stem, "the SHOULD sections (", sum, ") differ from the SHOULD rows counted (", should.Total)
	}
	return nil
}

// sectionsRefusal names a stem whose figures do not add back to their totals.
// It is a defect in StemFiguresOf, never in the summary, so the fix it names
// is there.
func sectionsRefusal(stem, first string, got int, second string, want int) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str(stem).Str(": ").Str(first).Int(int64(got)).Str(second).Int(int64(want)).
		Str("), so a bar drawn from these figures would disagree with its total. ").
		Str("This is a defect in rfc.StemFiguresOf (internal/le/rfc/profilecoverage.go): compare `./le rfc tree` for the stem, ").
		Str("fix the partition, then rerun the command").String())
}

// String renders the sections, the MUST bar first, then the SHOULD states,
// each segment by name and count.
func (s *Sections) String() string {
	var tb textbuf.Buffer
	must := &s.Must
	tb.Str("MUST ").Int(int64(must.Kept)).Str(" kept: Ze proven ").Int(int64(must.ZeProven)).
		Str(", OS verified ").Int(int64(must.OSVerified)).Str(", OS testing gap ").Int(int64(must.OSGap)).
		Str(", Ze correctness gap ").Int(int64(must.ZeGap))
	writeNotOwed(&tb, "MUST", must.NotOwed, must.NotOwedByKind)
	should := &s.Should
	tb.Str("; SHOULD ").Int(int64(should.Total)).Str(": proven ").Int(int64(should.Proven)).
		Str(", OS verified ").Int(int64(should.OSVerified)).Str(", OS testing gap ").Int(int64(should.OSGap)).
		Str(", not ever ").Int(int64(should.NotEver)).Str(", not yet ").Int(int64(should.NotYet)).
		Str(", not decided ").Int(int64(should.NotDecided))
	writeNotOwed(&tb, "SHOULD", should.NotOwed, should.NotOwedByKind)
	return tb.String()
}

// writeNotOwed renders the rows not owed at one level, by kind in name
// order.
func writeNotOwed(tb *textbuf.Buffer, level string, total int, byKind map[string]int) {
	tb.Str("; ").Str(level).Str(" not owed ").Int(int64(total))
	for _, kind := range AnnotationKinds() {
		if byKind[kind] == 0 {
			continue
		}
		tb.Str(", ").Str(kind).Byte(' ').Int(int64(byKind[kind]))
	}
}
