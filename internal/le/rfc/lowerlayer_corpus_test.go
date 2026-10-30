// VALIDATES: annotations never inflate the published whole-proof share.
// Every annotation but {single-polarity} keeps its denominator and earns no
// credit even when a verified discrimination record covers the row, which is
// the only thing that earns credit since 2026-10-30. Rollups remain outside the
// population because they derive other rows.

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
	walked, proven := realCorpusProofSets(t)

	// No real requirement may acquire this annotation without owner approval.
	// Keep the real corpus intact and exercise the newly declared kind in
	// memory, including a verified record that must not earn whole-proof
	// credit.
	const fixtureStem = "rfc999999"
	const fixtureRID = "RFC999999-1-1"
	if _, exists := collected.Metas[fixtureStem]; exists {
		t.Fatal("linux-limit fixture stem collides with the real corpus")
	}
	for _, requirement := range collected.Requirements {
		if requirement.RFC == fixtureStem || requirement.RID == fixtureRID {
			t.Fatal("linux-limit fixture requirement collides with the real corpus")
		}
	}
	fixture, err := ParseSummaryText(
		"- [ ] [RFC999999-1-1] [MUST] Exercise the kernel primitive. (§1) {linux-limit: LL-FIXTURE; kernel has no primitive}\n",
		fixtureStem, "in-memory-linux-limit-fixture",
	)
	if err != nil {
		t.Fatal(err)
	}
	collected.Metas[fixtureStem] = Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true}
	collected.Requirements = append(collected.Requirements, fixture...)
	walked[fixtureStem] = true
	proven[fixtureRID] = true

	stems := map[string]bool{}
	for stem := range collected.Metas {
		stems[stem] = true
	}
	shareOf := func(requirements []Requirement) ProvenShare {
		t.Helper()
		share, err := ProvenShareOf(collected.Metas,
			figuresByStem(requirements, stems, walked, proven, figureEvidence{}))
		if err != nil {
			t.Fatalf("the share over the corpus: %v", err)
		}
		return share
	}
	with := shareOf(collected.Requirements)

	for _, kind := range AnnotationKinds() {
		if kind == AnnotationSinglePolarity {
			continue
		}
		t.Run(kind, func(t *testing.T) {
			// The counterfactual removes only this annotation kind.
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
			excludedCredit := 0
			for _, requirement := range collected.Requirements {
				if requirement.Annotation == nil || requirement.Annotation.Kind != kind {
					stripped = append(stripped, requirement)
					continue
				}
				annotated++
				if kind == AnnotationRollup {
					continue
				}
				meta := collected.Metas[requirement.RFC]
				excludedCredit += annotationWholeCreditDelta(&requirement, &meta, walked, proven)
				requirement.Annotation = nil
				stripped = append(stripped, requirement)
			}
			if annotated == 0 {
				t.Fatalf("no requirement of this corpus carries {%s}, so this case proves nothing", kind)
			}
			without := shareOf(stripped)
			expected := with
			expected.Proven += excludedCredit
			if expected != without {
				t.Errorf("%d {%s} annotation(s) changed the population or whole credit beyond their %d excluded claims: with %+v, without %+v",
					annotated, kind, excludedCredit, with, without)
			}
			if with.Proven > without.Proven {
				t.Errorf("{%s} annotations inflated whole credit: with %+v, without %+v", kind, with, without)
			}
			t.Logf("%d {%s} rows retain their denominator; %d otherwise-whole claims excluded", annotated, kind, excludedCredit)
		})
	}
}

// annotationWholeCreditDelta independently counts the credit a row would earn
// with its annotation stripped, rather than asking the figures whether they
// would grant it: a gated row of a walked RFC Ze implements, which a verified
// record proves. {single-polarity} is the one kind a record already proves
// through, so stripping it moves nothing.
func annotationWholeCreditDelta(requirement *Requirement, meta *Meta, walked, proven map[string]bool) int {
	if requirement.Annotation == nil {
		return 0
	}
	if requirement.Annotation.Kind == AnnotationSinglePolarity {
		return 0
	}
	if !requirement.Gated() {
		return 0
	}
	if !Implements(meta) {
		return 0
	}
	if !walked[requirement.RFC] {
		return 0
	}
	if proven[requirement.RID] {
		return 1
	}
	return 0
}

// TestAnnotationWholeCreditCounterfactual covers record and population
// boundaries with fixed fixtures. A Gap row carrying a verified record models
// a stale binding: accounting must remain honest even before the consistency
// gate refuses it (TestCheckStillRefusesPolarityTagOnGapRow).
func TestAnnotationWholeCreditCounterfactual(t *testing.T) {
	const rid = "RFC2-1-1"
	for _, kind := range []string{AnnotationGap, AnnotationPartial, AnnotationLinuxLimit} {
		for _, evidence := range []struct {
			name   string
			proven bool
			delta  int
		}{
			{name: "no-record"},
			{name: "verified-record", proven: true, delta: 1},
		} {
			for _, scope := range []struct {
				name   string
				meta   Meta
				level  string
				walked bool
				with   ProvenShare
				owes   bool
			}{
				{
					name:   "implemented",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   ProvenShare{Gated: 2, RFCs: 2, Inspected: 2, GatedInspected: 2},
					owes:   true,
				},
				{
					// The owner's 2026-10-30 population: an enrolled RFC
					// whose feature is still to come counts at its figure.
					name:   "feature-still-to-come",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true, Status: "Unsupported"},
					level:  levelMust,
					walked: true,
					with:   ProvenShare{Gated: 2, RFCs: 2, Inspected: 2, GatedInspected: 2},
					owes:   true,
				},
				{
					name:  "unwalked",
					meta:  Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level: levelMust,
					// Unwalked: the target RFC adds to neither side, so only the
					// walked companion's row is gated (the rollup rule).
					with: ProvenShare{Gated: 1, RFCs: 2, NotWalked: 1, Inspected: 2, GatedInspected: 2},
				},
				{
					name:   "unenrolled",
					meta:   Meta{Enrolment: dispositionOutOfScope, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   ProvenShare{Gated: 1, RFCs: 1, Inspected: 1, GatedInspected: 1},
				},
				{
					name:   "no-public-row",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe},
					level:  levelMust,
					walked: true,
					with:   ProvenShare{Gated: 1, RFCs: 1, Inspected: 2, GatedInspected: 2},
				},
				{
					name:   "third-party",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationThirdParty, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   ProvenShare{Gated: 1, RFCs: 1, Inspected: 1, GatedInspected: 1},
				},
				{
					name:   "advisory",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelShould,
					walked: true,
					// An RFC with no gated row is still in both populations
					// (AC-8 names the population, not its rows); it adds no
					// gated requirement to either.
					with: ProvenShare{Gated: 1, RFCs: 2, Inspected: 2, GatedInspected: 1},
				},
			} {
				// Excluded populations need a record: weaker evidence repeats
				// the same exclusion without testing another boundary.
				if !scope.owes && !evidence.proven {
					continue
				}
				t.Run(kind+"/"+evidence.name+"/"+scope.name, func(t *testing.T) {
					metas := map[string]Meta{
						"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
						"rfc2": scope.meta,
					}
					requirements := []Requirement{
						{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
						{RFC: "rfc2", RID: rid, Level: scope.level, Annotation: &Annotation{Kind: kind}},
					}
					walked := map[string]bool{"rfc1": true, "rfc2": scope.walked}
					proven := map[string]bool{rid: evidence.proven}
					delta := 0
					if scope.owes {
						delta = evidence.delta
					}
					if got := annotationWholeCreditDelta(&requirements[1], &scope.meta, walked, proven); got != delta {
						t.Fatalf("independent delta = %d, want %d", got, delta)
					}
					with, err := shareOver(t, metas, requirements, walked, proven)
					if err != nil {
						t.Fatal(err)
					}
					if with != scope.with {
						t.Fatalf("annotated share = %+v, want %+v", with, scope.with)
					}
					requirements[1].Annotation = nil
					without, err := shareOver(t, metas, requirements, walked, proven)
					if err != nil {
						t.Fatal(err)
					}
					want := scope.with
					want.Proven += delta
					if without != want {
						t.Fatalf("stripped share = %+v, want %+v", without, want)
					}
				})
			}
		}
	}
}
