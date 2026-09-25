// VALIDATES: over THIS checkout's corpus, a {rollup} row moves neither the
// published proof share nor any coverage count (AC-9 of
// spec-rfc-ledger-rollup-annotation).
// PREVENTS: a document billed twice. A rollup names rows that are each counted
// once under their own id, so the rollup itself sits outside Gated, Proven and
// Annotated, and a fully conformant RFC whose rollup is met reads 100% rather
// than one row short. The method is the counterfactual in both directions: the
// corpus with every rollup line removed, and the corpus with one synthetic
// rollup added on an implemented RFC, so the property holds on a day the
// corpus carries no rollup and on a day it carries several.

package rfc

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/le/path"
)

func TestRollupMovesNeitherShareNorGatedCount(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("collect the corpus: %v", err)
	}
	// The corpus with every rollup line removed is the baseline both
	// directions are held against.
	var bare []Requirement
	removed := 0
	for _, requirement := range collected.Requirements {
		if requirement.Rollup() {
			removed++
			continue
		}
		bare = append(bare, requirement)
	}
	baseShare, err := ProvenShareOf(collected.Metas, bare, collected.Tags, nil)
	if err != nil {
		t.Fatalf("the share over the corpus without rollups: %v", err)
	}
	baseRows := CoverageRows(bare, collected.Tags, nil)

	// A synthetic rollup on the first implemented RFC, naming that RFC's
	// first gated row, so the added row is one the share's population would
	// count if a rollup were counted.
	var synthetic Requirement
	for _, requirement := range bare {
		if !requirement.Gated() || !Implements(collected.Metas[requirement.RFC]) {
			continue
		}
		synthetic = Requirement{RFC: requirement.RFC, RID: requirement.RFC + "-rollup-1",
			Level: levelMust, Source: requirement.Source, Line: requirement.Line,
			Annotation: &Annotation{Kind: AnnotationRollup, Targets: []string{requirement.RID},
				Reason: requirement.RID + "; the synthetic rollup of this test"}}
		break
	}
	if synthetic.RID == "" {
		t.Fatal("no implemented RFC carries a gated row, so the synthetic rollup has nowhere to land")
	}

	for name, requirements := range map[string][]Requirement{
		"this checkout's corpus":        collected.Requirements,
		"the corpus plus one synthetic": append(slices.Clone(bare), synthetic),
	} {
		t.Run(name, func(t *testing.T) {
			share, err := ProvenShareOf(collected.Metas, requirements, collected.Tags, nil)
			if err != nil {
				t.Fatalf("the share: %v", err)
			}
			if share != baseShare {
				t.Errorf("the rollup rows moved the published share: %+v, without them %+v",
					share, baseShare)
			}
			rows := CoverageRows(requirements, collected.Tags, nil)
			if !slices.Equal(rows, baseRows) {
				t.Errorf("the rollup rows moved a coverage count:\n%+v\nwithout them\n%+v", rows, baseRows)
			}
		})
	}
	t.Logf("%d rollup row(s) in the corpus, one synthetic on %s; share unmoved at %s%% (%d of %d)",
		removed, synthetic.RFC, baseShare.Percent(), baseShare.Proven, baseShare.Gated)
}

// TestRollupRowsInThisCorpusDeriveAsStated holds AC-10 over this checkout's
// corpus: RFC4302-5-2 derives met from the three multicast rows, RFC4302-5-1
// derives gap with its first cause the sequence-counter reset row
// RFC4302-2.5-5, which precedes RFC4302-4-1 in its target list, and the gate
// names neither row. The method is the real corpus through Collect,
// checkRollupTargets and evaluate, so the rows are held exactly as
// `./le rfc check` holds them.
func TestRollupRowsInThisCorpusDeriveAsStated(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	collected, err := Collect(root)
	if err != nil {
		t.Fatalf("collect the corpus: %v", err)
	}
	if refused := checkRollupTargets(collected.Requirements, collected.Enrolled); len(refused) != 0 {
		t.Fatalf("the corpus's rollup targets are refused: %v", refused)
	}
	findings := evaluate(collected.Requirements, collected.Tags, collected.Enrolled)

	const multicast, unicast = "RFC4302-5-2", "RFC4302-5-1"
	if state := derivedOf(t, collected.Requirements, multicast); state != RollupMet {
		t.Errorf("%s derives %s, want met: %+v", multicast, state, findingsFor(findings, multicast))
	}
	if state := derivedOf(t, collected.Requirements, unicast); state != RollupGap {
		t.Errorf("%s derives %s, want gap", unicast, state)
	}
	const cause = "RFC4302-2.5-5 is annotated {gap}"
	if got := derivedCauseOf(t, collected.Requirements, unicast); got != cause {
		t.Errorf("%s's derived cause reads %q, want %q", unicast, got, cause)
	}
	for _, rid := range []string{multicast, unicast} {
		if own := findingsFor(findings, rid); len(own) != 0 {
			t.Errorf("the gate names %s, want it silent: %+v", rid, own)
		}
	}
}
