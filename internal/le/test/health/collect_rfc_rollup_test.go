// VALIDATES: a {rollup} row is counted by the annotation split, so the page can
// state how many rows derive, and left out of the split's sum, which the
// partition check compares against the ledger's Annotated column.
// PREVENTS: the divergence the first rollup row raised on 2026-09-15.
// rfc.CoverageRows leaves a rollup out of every count, so a split that summed
// it compared against a population one row wider than the ledger's, and
// collectRFC refuses that divergence by taking the whole page down.

package testhealth

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

func TestARollupRowIsCountedApartFromTheSplitSum(t *testing.T) {
	line := "- [ ] [RFC9998-5-1] [MUST] A speaker MUST do all of it (§5) " +
		"{rollup: RFC9998-2-1, RFC9998-2-2; the row names the other rows}"
	kind, annotated := annotationOf(line)
	if !annotated || kind != rfc.AnnotationRollup {
		t.Fatalf("the rollup line reads as kind %q (annotated=%t), want %q", kind, annotated,
			rfc.AnnotationRollup)
	}

	kinds := annotationCounts{counts: map[string]int{}}
	kinds.add(rfc.AnnotationGap)
	kinds.add(rfc.AnnotationRollup)
	kinds.add(rfc.AnnotationRollup)
	if kinds.get(rfc.AnnotationRollup) != 2 {
		t.Errorf("the split counts %d rollup(s), want 2", kinds.get(rfc.AnnotationRollup))
	}
	if total := kinds.total(); total != 1 {
		t.Errorf("the split sums to %d, want 1: the rollup rows entered the sum the ledger's "+
			"Annotated column is compared against", total)
	}
	if !strings.Contains(pythonDict(kinds), rfc.AnnotationRollup) {
		t.Errorf("the divergence message leaves the rollup count out: %s", pythonDict(kinds))
	}
}
