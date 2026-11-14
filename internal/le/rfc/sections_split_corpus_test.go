package rfc

// VALIDATES: spec-rfc-figures-built-and-proven A-1 and A-7 over THIS
// checkout's corpus, under the owner's rule "Keep the number real."
// (2026-11-13): an open row is proof pending only when a non-gap tag covers
// it, and not assessed otherwise. Every open row of a walked RFC lands, at its
// level, in exactly one of proof pending, exclusion unchecked, not assessed
// and not built, exactly as the published sections count them, every proof
// pending row carries a tagged test, and the MUST rows in not assessed are
// named.
// PREVENTS: "proof pending" published over a row with no test behind it, and
// a corpus count that differs from what the figures publish.

import (
	"slices"
	"testing"
)

// openSectionOfCorpusRow answers the section the figures place one row of the
// real corpus in, and false for a row that is in no open section: not in a
// walked RFC's figures, not owed, proven, or excluded. It asks the figures'
// own predicates, so a disagreement with the published counts is a defect.
func openSectionOfCorpusRow(corpus *priorityCorpus, requirement *Requirement) (openSection, bool) {
	figures, held := corpus.figures[requirement.RFC]
	if !held {
		return openUnspecified, false
	}
	if !figures.Walked {
		return openUnspecified, false
	}
	owed, met := rowCounts(requirement, corpus.proven)
	if !owed {
		return openUnspecified, false
	}
	if met {
		return openUnspecified, false
	}
	if corpus.evidence.excluded(requirement) {
		return openUnspecified, false
	}
	unmet := corpus.evidence.unmet(requirement)
	return openSectionOf(requirement, unmet, corpus.evidence.tested(requirement.RID)), true
}

// carriesATest answers whether a row's tags hold one that is not a gap
// demonstration: a gap tag shows the obligation unmet, so it is no test of it.
func carriesATest(tags []Tag) bool {
	for _, tag := range tags {
		if !tag.Gap {
			return true
		}
	}
	return false
}

// TestEveryProofPendingRowCarriesATaggedTest checks A-1: every MUST row of a
// walked RFC that the sections count proof pending carries a tagged test. The
// method walks the real corpus's rows with the figures' own predicates, holds
// the count it finds equal to the published ProofPending sum (so the walk is
// the figures' split, not a second one), and fails naming every such row with
// no tag. Since a row no tagged test covers is not assessed (openSectionOf),
// this holds by construction; TestNotAssessedMustRowsAreNamed names those
// rows instead.
func TestEveryProofPendingRowCarriesATaggedTest(t *testing.T) {
	corpus, err := realPriorityCorpus()
	if err != nil {
		t.Fatalf("load the real corpus: %v", err)
	}
	counted := 0
	var untested []string
	for index := range corpus.requirements {
		requirement := &corpus.requirements[index]
		if !requirement.Gated() {
			continue
		}
		section, open := openSectionOfCorpusRow(corpus, requirement)
		if !open {
			continue
		}
		if section != openProofPending {
			continue
		}
		counted++
		if !carriesATest(corpus.evidence.tags[requirement.RID]) {
			untested = append(untested, requirement.RID)
		}
	}
	published := 0
	for stem := range corpus.figures {
		if corpus.figures[stem].Walked {
			published += corpus.figures[stem].ProofPending
		}
	}
	if counted != published {
		t.Fatalf("the corpus walk finds %d proof-pending MUST rows, the figures publish %d", counted, published)
	}
	if len(untested) != 0 {
		slices.Sort(untested)
		t.Errorf("%d of %d proof-pending MUST rows carry no tagged test; tag a test or annotate each row: %v",
			len(untested), counted, untested)
	}
}

// TestNotAssessedMustRowsAreNamed checks A-1 under the not assessed rule: it
// logs every MUST row of a walked RFC that no tagged test covers (not
// assessed), by stem and id, and asserts that the corpus walk counts each of
// the four open MUST sections as the figures publish it, and that the four sum
// to the MUST rows owed and not proven (attainable less proven).
func TestNotAssessedMustRowsAreNamed(t *testing.T) {
	corpus, err := realPriorityCorpus()
	if err != nil {
		t.Fatalf("load the real corpus: %v", err)
	}
	walk := map[openSection]int{}
	for index := range corpus.requirements {
		requirement := &corpus.requirements[index]
		if !requirement.Gated() {
			continue
		}
		section, open := openSectionOfCorpusRow(corpus, requirement)
		if !open {
			continue
		}
		walk[section]++
		if section == openNotAssessed {
			t.Logf("not assessed MUST row: %s %s", requirement.RFC, requirement.RID)
		}
	}
	published := map[openSection]int{}
	openRows := 0
	for stem := range corpus.figures {
		figures := corpus.figures[stem]
		if !figures.Walked {
			continue
		}
		published[openProofPending] += figures.ProofPending
		published[openExclusionUnchecked] += figures.ExclusionUnchecked
		published[openNotAssessed] += figures.NotAssessed
		published[openNotBuilt] += figures.NotBuilt
		openRows += figures.Mandatory.Attainable - figures.Mandatory.Proven
	}
	t.Logf("open MUST rows %d: proof pending %d, exclusion unchecked %d, not assessed %d, not built %d",
		openRows, published[openProofPending], published[openExclusionUnchecked], published[openNotAssessed],
		published[openNotBuilt])
	sum := 0
	for _, section := range []openSection{openProofPending, openExclusionUnchecked, openNotAssessed, openNotBuilt} {
		if walk[section] != published[section] {
			t.Errorf("section %d: the corpus walk counts %d MUST rows, the figures publish %d",
				section, walk[section], published[section])
		}
		sum += published[section]
	}
	if sum != openRows {
		t.Errorf("the four open MUST sections sum to %d, the MUST rows owed and not proven are %d", sum, openRows)
	}
	if len(walk) > 4 {
		t.Errorf("the corpus walk placed MUST rows in a section that is none of the four: %v", walk)
	}
}

// TestOpenOptionalRowsWithNoTestAreCounted checks A-7 and R-2: it logs how
// many open optional rows are proof pending and how many are not assessed (no
// tagged test), and asserts that every proof pending optional row carries a
// tagged test, and that the four optional sections, counted by the corpus walk
// and by the figures, agree and sum to the open optional rows (attainable less
// proven) of the walked RFCs.
func TestOpenOptionalRowsWithNoTestAreCounted(t *testing.T) {
	corpus, err := realPriorityCorpus()
	if err != nil {
		t.Fatalf("load the real corpus: %v", err)
	}
	walk := map[openSection]int{}
	untested := 0
	for index := range corpus.requirements {
		requirement := &corpus.requirements[index]
		if requirement.Gated() {
			continue
		}
		section, open := openSectionOfCorpusRow(corpus, requirement)
		if !open {
			continue
		}
		walk[section]++
		if section != openProofPending {
			continue
		}
		if !carriesATest(corpus.evidence.tags[requirement.RID]) {
			untested++
			t.Errorf("optional row %s %s is proof pending with no tagged test", requirement.RFC, requirement.RID)
		}
	}
	published := map[openSection]int{}
	openRows := 0
	for stem := range corpus.figures {
		figures := corpus.figures[stem]
		if !figures.Walked {
			continue
		}
		published[openProofPending] += figures.OptionalProofPending
		published[openExclusionUnchecked] += figures.OptionalExclusionUnchecked
		published[openNotAssessed] += figures.OptionalNotAssessed
		published[openNotBuilt] += figures.OptionalNotBuilt
		openRows += figures.Optional.Attainable - figures.Optional.Proven
	}
	t.Logf("open optional rows %d: proof pending %d (%d of them with no tagged test), not assessed %d, "+
		"exclusion unchecked %d, not built %d", openRows, published[openProofPending], untested,
		published[openNotAssessed], published[openExclusionUnchecked], published[openNotBuilt])
	for _, section := range []openSection{openProofPending, openExclusionUnchecked, openNotAssessed, openNotBuilt} {
		if walk[section] != published[section] {
			t.Errorf("section %d: the corpus walk counts %d optional rows, the figures publish %d",
				section, walk[section], published[section])
		}
	}
	sum := published[openProofPending] + published[openExclusionUnchecked] + published[openNotAssessed] +
		published[openNotBuilt]
	if sum != openRows {
		t.Errorf("the four optional sections sum to %d, the open optional rows are %d", sum, openRows)
	}
	if len(walk) > 4 {
		t.Errorf("the corpus walk placed rows in a section that is none of the four: %v", walk)
	}
}
