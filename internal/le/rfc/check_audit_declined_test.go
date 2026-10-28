// Design: docs/contributing/rfc-conformance-gates.md -- the feature-declined annotation
// Related: check_audit.go -- verdictClaims, the verdict and annotation agreement

package rfc

import (
	"strings"
	"testing"
)

// TestNotApplicableVerdictStandsOverFeatureDeclined proves that a requirement
// annotated {feature-declined} has a legal audit verdict. Coverage is
// monotonic, so a row re-annotated {feature-declined} after it was judged keeps
// its verdict, and before this ruling no verdict agreed with the kind. Method:
// hand verdictClaims one requirement per annotation kind with the same
// complete 'not-applicable' verdict and compare the refusals.
func TestNotApplicableVerdictStandsOverFeatureDeclined(t *testing.T) {
	verdict := map[string]any{
		"verdict":      VerdictNotApplicable,
		"note":         "judged by an independent auditor",
		"no_code_path": "the optional feature is declined, so no path reaches the obligation",
	}
	for _, kind := range []string{AnnotationNotApplicable, AnnotationFeatureDeclined} {
		t.Run(kind, func(t *testing.T) {
			req := Requirement{RID: "RFC9999-2-1", Source: "rfc/short/rfc9999.md", Line: 7,
				Annotation: &Annotation{Kind: kind}}
			if errs := verdictClaims("rfc9999", req.RID, verdict, req, nil); len(errs) != 0 {
				t.Fatalf("'not-applicable' over {%s} refused: %v", kind, errs)
			}
		})
	}
	for _, kind := range []string{AnnotationGap, AnnotationLowerLayer} {
		t.Run("refused over "+kind, func(t *testing.T) {
			req := Requirement{RID: "RFC9999-2-1", Source: "rfc/short/rfc9999.md", Line: 7,
				Annotation: &Annotation{Kind: kind}}
			errs := verdictClaims("rfc9999", req.RID, verdict, req, nil)
			if len(errs) != 1 || !strings.Contains(errs[0], "{feature-declined} checklist line") {
				t.Fatalf("'not-applicable' over {%s} answered %v, want the one agreement refusal", kind, errs)
			}
		})
	}
}

// TestUnimplementedVerdictRefusedOverFeatureDeclined proves the exclusion does
// not leak into the other direction. {feature-declined} says nothing is owed,
// and 'unimplemented' says Ze owes behavior it does not deliver, so the two
// committed facts disagree. Method: a complete 'unimplemented' verdict with a
// code map over a {feature-declined} line must draw the annotation refusal.
func TestUnimplementedVerdictRefusedOverFeatureDeclined(t *testing.T) {
	verdict := map[string]any{
		"verdict": VerdictUnimplemented,
		"note":    "the code does not comply",
		"code":    map[string]any{"internal/x/x.go::Build": "sha"},
	}
	req := Requirement{RID: "RFC9999-2-1", Source: "rfc/short/rfc9999.md", Line: 7,
		Annotation: &Annotation{Kind: AnnotationFeatureDeclined}}
	errs := verdictClaims("rfc9999", req.RID, verdict, req, nil)
	if len(errs) != 1 {
		t.Fatalf("'unimplemented' over {feature-declined} answered %v, want the one annotation refusal", errs)
	}
	if !strings.Contains(errs[0], req.RID) {
		t.Fatalf("annotation refusal does not identify the requirement: %v", errs)
	}
	if !strings.Contains(errs[0], req.Source) {
		t.Fatalf("annotation refusal does not identify the summary: %v", errs)
	}
	req.Annotation = &Annotation{Kind: AnnotationGap}
	if errs := verdictClaims("rfc9999", req.RID, verdict, req, nil); len(errs) != 0 {
		t.Fatalf("the same unimplemented verdict over a gap was refused: %v", errs)
	}
}
