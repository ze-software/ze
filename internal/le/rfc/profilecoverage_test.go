// VALIDATES: AC-7, AC-8 and AC-5's never-complete empty set. The
// mandatory figure is proven rows over gated rows minus {rollup}; a
// not-applicable row stays in the denominator; a tagged row without a verified
// proof, a stale proof, an escape and a {gap} row are all unproven; a
// {single-polarity} row is proven by its verified record and by nothing else; the
// optional figure counts the same way over the other levels. An unwalked RFC
// renders "not walked", and a rollup holding one, or holding no RFC, is never
// complete. No absent state exists.
// PREVENTS: a figure that counts tags rather than recorded breaks, an
// annotation that raises a figure, and a profile that reads complete while a
// member is unwalked or while it has no member.

package rfc

import (
	"encoding/json"
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
		row("RFC1-1-1", levelMust, ""),                        // proven
		row("RFC1-1-2", levelMust, ""),                        // tagged, no record
		row("RFC1-1-3", levelMust, ""),                        // stale record
		row("RFC1-1-4", levelMust, ""),                        // no-break escape
		row("RFC1-1-5", levelMust, AnnotationNotApplicable),   // denominator only, its record proves nothing
		row("RFC1-1-6", levelMust, AnnotationRollup),          // out of both
		row("RFC1-1-7", levelMust, AnnotationGap),             // a record cannot prove a gap
		row("RFC1-1-8", levelMust, AnnotationLowerLayer),      // denominator only, its record proves nothing
		row("RFC1-1-9", levelMust, AnnotationSinglePolarity),  // proven by its verified record
		row("RFC1-1-10", levelMust, AnnotationSinglePolarity), // no record: the annotation alone proves nothing
		row("RFC1-2-1", levelShould, ""),                      // optional, proven
		row("RFC1-2-2", "MAY", ""),                            // optional, unproven
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
		verdict("RFC1-1-9", RouteRevert, ProofVerified),
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
	if one.Mandatory != (Figure{Proven: 2, Total: 9}) {
		t.Errorf("mandatory = %+v, want 2 proven of 9 owed", one.Mandatory)
	}
	if one.Optional != (Figure{Proven: 1, Total: 2}) {
		t.Errorf("optional = %+v, want 1 proven of 2 owed", one.Optional)
	}
	if got := one.mandatoryCell(); got != "2 of 9 (22.2%)" {
		t.Errorf("mandatory cell = %q", got)
	}

	// A {single-polarity} row is owed either way, and proven only by its
	// verified record: the annotation alone proves nothing.
	proven := provenRequirements(verdicts)
	for index := range requirements {
		requirement := &requirements[index]
		if requirement.Annotation == nil || requirement.Annotation.Kind != AnnotationSinglePolarity {
			continue
		}
		owed, met := rowCounts(requirement, proven)
		want := requirement.RID == "RFC1-1-9"
		if !owed || met != want {
			t.Errorf("%s: owed=%v proven=%v, want owed and proven=%v", requirement.RID, owed, met, want)
		}
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

	rollup := rollupOf(figures, []string{"rfc1", "rfc2", "rfc3"})
	want := Rollup{RFCs: 3, NotWalked: 1,
		Mandatory: Figure{Proven: 2, Total: 9}, Optional: Figure{Proven: 1, Total: 2}}
	if rollup != want {
		t.Errorf("rollup = %+v, want %+v", rollup, want)
	}
	for _, phrase := range []string{"3 RFCs", "1 not walked"} {
		if !strings.Contains(rollup.String(), phrase) {
			t.Errorf("rollup %q does not state %q", rollup.String(), phrase)
		}
	}
}

// TestRollupNeverCompleteWithUnwalkedOrEmpty: a set whose walked rows are all
// proven is complete only when no member is unwalked and the set has a member:
// 0 of 0 over no RFC is not a measurement.
func TestRollupNeverCompleteWithUnwalkedOrEmpty(t *testing.T) {
	figures := map[string]StemFigures{
		"rfc1": {Stem: "rfc1", Walked: true, Mandatory: Figure{Proven: 3, Total: 3}},
		"rfc2": {Stem: "rfc2"},
	}
	if !rollupOf(figures, []string{"rfc1"}).mandatoryComplete() {
		t.Error("a fully proven walked set is not complete")
	}
	if rollupOf(figures, []string{"rfc1", "rfc2"}).mandatoryComplete() {
		t.Error("a set with an unwalked RFC reads complete")
	}
	if rollupOf(figures, nil).mandatoryComplete() {
		t.Error("a set with no member reads complete")
	}
}

// TestRollupHasNoAbsentState: the absent state is deleted (owner, "all RFC
// should have a valid summary"). Method: a rollup and a tree row, encoded and
// rendered, carry no `absent` key and no "absent at 0%" text.
func TestRollupHasNoAbsentState(t *testing.T) {
	rollup := ScopeRollup{Rollup: Rollup{RFCs: 1, NotWalked: 1}}
	row := &TreeRow{Stem: "rfc1", Summary: true}
	for label, value := range map[string]any{"rollup": rollup, "row": row} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), `"absent"`) {
			t.Errorf("%s JSON carries an absent key: %s", label, encoded)
		}
	}
	if text := rollup.String(); strings.Contains(text, "absent") {
		t.Errorf("rollup text %q still speaks of absent RFCs", text)
	}
}
