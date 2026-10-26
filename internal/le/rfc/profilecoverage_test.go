// VALIDATES: AC-7, AC-8 and the owner ruling of 2026-10-25 on absent RFCs. The
// mandatory figure is proven rows over gated rows minus {rollup}; a
// not-applicable row stays in the denominator; a tagged row without a verified
// proof, a stale proof, an escape and a {gap} row are all unproven; the
// optional figure counts the same way over the other levels. An unwalked RFC
// renders "not walked", an absent one renders 0%, and a rollup holding either
// is never complete.
// PREVENTS: a figure that counts tags rather than recorded breaks, an
// annotation that raises a figure, and a profile that reads complete while an
// RFC it needs is absent.

package rfc

import (
	"strings"
	"testing"
)

// coverageFixture is one walked RFC carrying every row shape the figures
// distinguish, and one unwalked RFC.
func coverageFixture() ([]Requirement, []DiscriminationVerdict) {
	row := func(rid, level string, kind string) Requirement {
		out := Requirement{RFC: "rfc1", RID: rid, Level: level}
		if kind != "" {
			out.Annotation = &Annotation{Kind: kind, Reason: "fixture"}
		}
		return out
	}
	requirements := []Requirement{
		row("RFC1-1-1", levelMust, ""),                      // proven
		row("RFC1-1-2", levelMust, ""),                      // tagged, no record
		row("RFC1-1-3", levelMust, ""),                      // stale record
		row("RFC1-1-4", levelMust, ""),                      // no-break escape
		row("RFC1-1-5", levelMust, AnnotationNotApplicable), // denominator only, its record proves nothing
		row("RFC1-1-6", levelMust, AnnotationRollup),        // out of both
		row("RFC1-1-7", levelMust, AnnotationGap),           // a record cannot prove a gap
		row("RFC1-1-8", levelMust, AnnotationLowerLayer),    // denominator only, its record proves nothing
		row("RFC1-2-1", levelShould, ""),                    // optional, proven
		row("RFC1-2-2", "MAY", ""),                          // optional, unproven
		{RFC: "rfc2", RID: "RFC2-1-1", Level: levelMust},
	}
	verdict := func(rid, route, state string) DiscriminationVerdict {
		return DiscriminationVerdict{Record: DiscriminationRecord{RID: rid, Route: route}, State: state}
	}
	verdicts := []DiscriminationVerdict{
		verdict("RFC1-1-1", RouteRevert, ProofVerified),
		verdict("RFC1-1-3", RouteRevert, "producer-changed"),
		verdict("RFC1-1-4", RouteNoBreak, ProofVerified),
		verdict("RFC1-1-5", RouteRevert, ProofVerified),
		verdict("RFC1-1-6", RouteRevert, ProofVerified),
		verdict("RFC1-1-7", RouteRevert, ProofVerified),
		verdict("RFC1-1-8", RouteRevert, ProofVerified),
		verdict("RFC1-2-1", RouteMutant, ProofVerified),
		verdict("RFC2-1-1", RouteRevert, ProofVerified),
	}
	return requirements, verdicts
}

func TestCoverageMandatoryOptional(t *testing.T) {
	requirements, verdicts := coverageFixture()
	stems := map[string]bool{"rfc1": true, "rfc2": true, "rfc3": true}
	walked := map[string]bool{"rfc1": true, "rfc3": true}
	figures := figuresByStem(requirements, stems, walked, provenRequirements(verdicts))

	one := figures["rfc1"]
	if one.Mandatory != (Figure{Proven: 1, Total: 7}) {
		t.Errorf("mandatory = %+v, want 1 proven of 7 owed", one.Mandatory)
	}
	if one.Optional != (Figure{Proven: 1, Total: 2}) {
		t.Errorf("optional = %+v, want 1 proven of 2 owed", one.Optional)
	}
	if got := one.mandatoryCell(); got != "1 of 7 (14.3%)" {
		t.Errorf("mandatory cell = %q", got)
	}

	// AC-8: a proven row on an unwalked RFC still renders no figure.
	two := figures["rfc2"]
	if two.mandatoryCell() != notWalked || two.optionalCell() != notWalked {
		t.Errorf("unwalked cells = %q, %q, want %q", two.mandatoryCell(), two.optionalCell(), notWalked)
	}
	// A walked RFC with no row renders no percentage, never 0% or 100%.
	if got := figures["rfc3"].mandatoryCell(); got != "no rows" {
		t.Errorf("empty walked cell = %q, want no rows", got)
	}

	// Owner ruling 2026-10-25: an absent RFC renders 0% on both figures.
	absent := absentFigures(7948)
	if absent.Stem != "rfc7948" || absent.mandatoryCell() != absentCell || absent.optionalCell() != absentCell {
		t.Errorf("absent figures = %+v %q %q", absent, absent.mandatoryCell(), absent.optionalCell())
	}
	if !strings.HasPrefix(absent.mandatoryCell(), "0%") {
		t.Errorf("absent cell %q does not read 0%%", absent.mandatoryCell())
	}

	rollup := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3"}, []int{7948, 8212})
	want := Rollup{RFCs: 5, NotWalked: 1, Absent: 2,
		Mandatory: Figure{Proven: 1, Total: 7}, Optional: Figure{Proven: 1, Total: 2}}
	if rollup != want {
		t.Errorf("rollup = %+v, want %+v", rollup, want)
	}
	for _, phrase := range []string{"1 not walked", "2 absent at 0%"} {
		if !strings.Contains(rollup.String(), phrase) {
			t.Errorf("rollup %q does not state %q", rollup.String(), phrase)
		}
	}
}

// TestRollupNeverCompleteWithAbsentOrUnwalked: a set whose walked rows are all
// proven is complete only when no member is absent and none is unwalked.
func TestRollupNeverCompleteWithAbsentOrUnwalked(t *testing.T) {
	figures := map[string]StemFigures{
		"rfc1": {Stem: "rfc1", Walked: true, Mandatory: Figure{Proven: 3, Total: 3}},
		"rfc2": {Stem: "rfc2"},
	}
	if !rollupOf(figures, []string{"rfc1"}, nil).mandatoryComplete() {
		t.Error("a fully proven walked set is not complete")
	}
	if rollupOf(figures, []string{"rfc1"}, []int{7948}).mandatoryComplete() {
		t.Error("a set with an absent RFC reads complete")
	}
	if rollupOf(figures, []string{"rfc1", "rfc2"}, nil).mandatoryComplete() {
		t.Error("a set with an unwalked RFC reads complete")
	}
}
