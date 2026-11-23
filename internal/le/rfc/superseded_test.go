package rfc

import (
	"slices"
	"testing"
)

// VALIDATES: superseded obligations resolve to validated and complete (or not
// owed) when Ze implements and proves their successor requirement, or when an
// implemented successor drops the old obligation, while unresolved,
// unextracted, or still-unproven successors stay open gaps.
// PREVENTS: double-billing obsoleted RFC gaps whose superseding obligations Ze
// already proves, or silently excusing gaps whose successor is still unmet.
func TestSupersededResolutionInFiguresByStem(t *testing.T) {
	t.Parallel()

	requirements := []Requirement{
		// Old RFC (rfc100) obsoleted by rfc200.
		{
			RFC: "rfc100", RID: "RFC100-1-1", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "old version not spoken"},
			Superseded: &Successor{Disposition: successorRestated, Target: "RFC200-1-1", Reason: "restated"},
		},
		{
			RFC: "rfc100", RID: "RFC100-1-2", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "old version not spoken"},
			Superseded: &Successor{Disposition: successorRestated, Target: "RFC200-1-2", Reason: "restated"},
		},
		{
			RFC: "rfc100", RID: "RFC100-1-3", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "dropped by rfc200"},
			Superseded: &Successor{Disposition: successorDropped, Reason: "dropped"},
		},
		{
			RFC: "rfc100", RID: "RFC100-1-4", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "operator rule in successor"},
			Superseded: &Successor{Disposition: successorRestated, Target: "RFC200-1-3", Reason: "restated"},
		},
		{
			RFC: "rfc100", RID: "RFC100-1-5", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "not extracted yet"},
			Superseded: &Successor{Disposition: successorUnextracted, Target: "§2", Reason: "debt"},
		},
		// Successor RFC (rfc200), implemented by Ze.
		{RFC: "rfc200", RID: "RFC200-1-1", Level: levelMust},
		{
			RFC: "rfc200", RID: "RFC200-1-2", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "still missing"},
		},
		{
			RFC: "rfc200", RID: "RFC200-1-3", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationNotApplicable, Quote: "Operators MUST assign unique IDs.", BoundRole: "operator", Reason: "operator rule"},
		},
	}

	stems := map[string]bool{"rfc100": true, "rfc200": true}
	proven := map[string]bool{"RFC200-1-1": true}
	evidence := figureEvidence{
		exclusions:  acceptEveryExclusion{},
		successors:  map[string]string{"rfc100": "rfc200"},
		implemented: map[string]bool{"rfc200": true},
	}

	figures := figuresByStem(requirements, stems, stems, proven, evidence)
	old := figures["rfc100"]

	// RFC100-1-1 (restated -> proven) and RFC100-1-3 (dropped by implemented rfc200) are proven.
	if old.Mandatory.Proven != 2 {
		t.Errorf("rfc100 Mandatory.Proven = %d, want 2", old.Mandatory.Proven)
	}
	if old.MandatoryByKind[kindSuperseded] != 2 {
		t.Errorf("rfc100 MandatoryByKind[%s] = %d, want 2", kindSuperseded, old.MandatoryByKind[kindSuperseded])
	}
	// RFC100-1-4 (restated -> not-applicable) is excluded as not-applicable.
	if old.Mandatory.Excluded[AnnotationNotApplicable] != 1 {
		t.Errorf("rfc100 Mandatory.Excluded[not-applicable] = %d, want 1", old.Mandatory.Excluded[AnnotationNotApplicable])
	}
	// RFC100-1-2 (restated -> gap) and RFC100-1-5 (unextracted) remain NotBuilt and Unmet.
	if old.NotBuilt != 2 {
		t.Errorf("rfc100 NotBuilt = %d, want 2", old.NotBuilt)
	}
	wantUnmet := []string{"RFC100-1-2", "RFC100-1-5"}
	if !slices.Equal(old.Unmet, wantUnmet) {
		t.Errorf("rfc100 Unmet = %v, want %v", old.Unmet, wantUnmet)
	}
	if _, err := SectionsOf("rfc100", &old); err != nil {
		t.Fatalf("SectionsOf(rfc100) rejected split: %v", err)
	}
}

// VALIDATES: a {superseded: dropped} row stays an open gap when its successor
// RFC is NOT implemented by Ze.
// PREVENTS: excusing dropped obligations of an obsoleted RFC when Ze does not
// actually implement the successor document.
func TestSupersededDroppedStaysGapWhenSuccessorUnimplemented(t *testing.T) {
	t.Parallel()

	requirements := []Requirement{
		{
			RFC: "rfc100", RID: "RFC100-1-1", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "dropped"},
			Superseded: &Successor{Disposition: successorDropped, Reason: "dropped"},
		},
	}
	stems := map[string]bool{"rfc100": true}
	evidence := figureEvidence{
		exclusions:  acceptEveryExclusion{},
		successors:  map[string]string{"rfc100": "rfc200"},
		implemented: map[string]bool{},
	}

	old := figuresByStem(requirements, stems, stems, map[string]bool{}, evidence)["rfc100"]
	if old.Mandatory.Proven != 0 {
		t.Errorf("Mandatory.Proven = %d, want 0 when successor is unimplemented", old.Mandatory.Proven)
	}
	if old.NotBuilt != 1 {
		t.Errorf("NotBuilt = %d, want 1 when successor is unimplemented", old.NotBuilt)
	}
}
