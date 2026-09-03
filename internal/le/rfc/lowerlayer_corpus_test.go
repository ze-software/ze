// VALIDATES: over THIS checkout's corpus, removing every annotation of a kind
// that is not {single-polarity} changes no field of the published proof share.
// PREVENTS: a headline percentage moved by classifying requirements. Only
// {single-polarity} says a requirement IS proven, one side of the pair with a
// reason for the other; every other kind says it is NOT proven by Ze, for a
// different reason, so each belongs in the denominator and outside the
// numerator whichever way the rows are annotated. The population is READ from
// the vocabulary rather than listed, so a kind added tomorrow is held to this
// rule on the day it is added: {lower-layer} arrived on 2026-09-03 and
// {feature-declined} the same day, and a hand list would have covered one of
// them. The unit tests beside this one hold the property over a three-row
// fixture; this one holds it over the real corpus, where a reader meets the
// number.

package rfc

import (
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

func TestNoAnnotationExceptSinglePolarityMovesThePublishedShare(t *testing.T) {
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
			stripped := make([]Requirement, len(collected.Requirements))
			copy(stripped, collected.Requirements)
			annotated := 0
			for index := range stripped {
				if stripped[index].Annotation == nil || stripped[index].Annotation.Kind != kind {
					continue
				}
				stripped[index].Annotation = nil
				annotated++
			}
			if annotated == 0 {
				t.Fatalf("no requirement of this corpus carries {%s}, so this case proves nothing", kind)
			}
			without, err := ProvenShareOf(collected.Metas, stripped, collected.Tags, nil)
			if err != nil {
				t.Fatalf("the share over the counterfactual corpus: %v", err)
			}
			if with != without {
				t.Errorf("%d {%s} annotation(s) moved the published share: %+v, was %+v",
					annotated, kind, with, without)
			}
			t.Logf("%d {%s} requirement(s); share unmoved at %s%% (%d of %d)",
				annotated, kind, with.Percent(), with.Proven, with.Gated)
		})
	}
}
