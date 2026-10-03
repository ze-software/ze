// VALIDATES: annotating an obligation never inflates its whole-proof share.
// Non-testing annotations change neither numerator nor denominator; partial
// retains clause-only tags and excludes exactly their otherwise-whole credit.
// Rollups remain outside the population because they derive other rows.

package rfc

import (
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func TestAnnotationsNeverInflateThePublishedShare(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("collect the corpus: %v", err)
	}
	with, err := ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
	if err != nil {
		t.Fatalf("the share over the corpus: %v", err)
	}

	for _, kind := range AnnotationKinds() {
		if kind == AnnotationSinglePolarity {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			// The counterfactual corpus: the same requirements with this kind
			// removed, which is what the tree held before the annotations were
			// written.
			//
			// A {rollup} row is the one kind whose counterfactual is the LINE
			// removed rather than the annotation stripped (AC-9 of
			// spec-rfc-ledger-rollup-annotation). Stripped, the row is a bare
			// MUST nobody tests, which re-enters Gated and moves the share:
			// that would prove the row is out of the denominator, which
			// TestRollupMovesNeitherShareNorGatedCount already holds, and
			// nothing about the annotation.
			stripped := make([]Requirement, 0, len(collected.Requirements))
			annotated := 0
			scopedCredit := 0
			byRID := tagsByRID(collected.Tags)
			for _, requirement := range collected.Requirements {
				if requirement.Annotation == nil || requirement.Annotation.Kind != kind {
					stripped = append(stripped, requirement)
					continue
				}
				annotated++
				if kind == AnnotationRollup {
					continue
				}
				requirement.Annotation = nil
				if kind == AnnotationPartial && requirement.Gated() && Implements(collected.Metas[requirement.RFC]) && polarityCovered(requirement, byRID[requirement.RID]) {
					scopedCredit++
				}
				stripped = append(stripped, requirement)
			}
			if annotated == 0 {
				t.Fatalf("no requirement of this corpus carries {%s}, so this case proves nothing", kind)
			}
			without, err := ProvenShareOf(collected.Metas, stripped, collected.Tags, nil)
			if err != nil {
				t.Fatalf("the share over the counterfactual corpus: %v", err)
			}
			expected := with
			expected.Proven += scopedCredit
			if expected != without {
				t.Errorf("%d {%s} annotation(s) changed the population or whole credit beyond their %d scoped claims: with %+v, without %+v",
					annotated, kind, scopedCredit, with, without)
			}
			t.Logf("%d {%s} rows retain their denominator; %d clause-only rows excluded from whole credit", annotated, kind, scopedCredit)
		})
	}
}
