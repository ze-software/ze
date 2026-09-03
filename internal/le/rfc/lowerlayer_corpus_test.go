// VALIDATES: over THIS checkout's corpus, removing every {lower-layer}
// annotation changes no field of the published proof share.
// PREVENTS: a headline percentage moved by classifying requirements. The kind
// says a requirement is MET, by a layer under Ze, and NOT proven by Ze, so it
// belongs in the denominator and outside the numerator whichever way the rows
// are annotated. The unit test beside this one holds the property over a
// three-row fixture; this one holds it over the real corpus, where a reader
// meets the number.

package rfc

import (
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

func TestNoLowerLayerAnnotationMovesThePublishedShare(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("collect the corpus: %v", err)
	}

	// The counterfactual corpus: the same requirements with the kind removed,
	// which is what the tree held before the annotations were written.
	stripped := make([]Requirement, len(collected.Requirements))
	copy(stripped, collected.Requirements)
	annotated := 0
	for index := range stripped {
		if stripped[index].Annotation == nil ||
			stripped[index].Annotation.Kind != AnnotationLowerLayer {
			continue
		}
		stripped[index].Annotation = nil
		annotated++
	}
	if annotated == 0 {
		t.Fatal("no requirement of this corpus carries {lower-layer}, so this test proves nothing")
	}

	with, err := ProvenShareOf(collected.Metas, collected.Requirements, collected.Tags, nil)
	if err != nil {
		t.Fatalf("the share over the corpus: %v", err)
	}
	without, err := ProvenShareOf(collected.Metas, stripped, collected.Tags, nil)
	if err != nil {
		t.Fatalf("the share over the counterfactual corpus: %v", err)
	}
	if with != without {
		t.Errorf("%d {lower-layer} annotation(s) moved the published share: %+v, was %+v",
			annotated, with, without)
	}
	t.Logf("%d {lower-layer} requirement(s); share unmoved at %s%% (%d of %d)",
		annotated, with.Percent(), with.Proven, with.Gated)
}
