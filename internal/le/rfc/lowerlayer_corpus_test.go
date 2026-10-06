// VALIDATES: annotations never inflate the published whole-proof share.
// Partial and Gap retain their denominator but suppress whole credit even when
// ordinary polarity tags remain. A gap demonstration proves neither polarity.
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
			// The counterfactual removes only this annotation kind; it does
			// not claim the remaining tags legitimately prove a Gap row.
			// The separate ordinary-tag-versus-Gap consistency gate still
			// refuses that binding.
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
				excludedCredit += annotationWholeCreditDelta(requirement, collected.Metas[requirement.RFC], byRID[requirement.RID])
				requirement.Annotation = nil
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

// annotationWholeCreditDelta independently counts ordinary polarity presence
// rather than asking the coverage producer whether it would grant credit.
func annotationWholeCreditDelta(requirement Requirement, meta Meta, tags []Tag) int {
	if requirement.Annotation == nil {
		return 0
	}
	kind := requirement.Annotation.Kind
	if kind != AnnotationPartial && kind != AnnotationGap {
		return 0
	}
	if !requirement.Gated() {
		return 0
	}
	if !Implements(meta) {
		return 0
	}
	var positive, negative bool
	for _, tag := range tags {
		if tag.Gap {
			continue
		}
		if tag.Polarity == PolarityPositive {
			positive = true
		}
		if tag.Polarity == PolarityNegative {
			negative = true
		}
	}
	if positive && negative {
		return 1
	}
	return 0
}

// TestAnnotationWholeCreditCounterfactual covers tag and population boundaries
// with fixed fixtures. Gap plus ordinary tags deliberately models a stale
// binding: accounting must remain honest even before the consistency gate
// refuses it (TestCheckStillRefusesPolarityTagOnGapRow).
func TestAnnotationWholeCreditCounterfactual(t *testing.T) {
	const rid = "RFC2-1-1"
	for _, kind := range []string{AnnotationGap, AnnotationPartial} {
		for _, evidence := range []struct {
			name  string
			tags  []Tag
			delta int
		}{
			{name: "zero"},
			{name: "positive", tags: []Tag{{RID: rid, Polarity: PolarityPositive}}},
			{name: "negative", tags: []Tag{{RID: rid, Polarity: PolarityNegative}}},
			{name: "both", tags: []Tag{
				{RID: rid, Polarity: PolarityPositive},
				{RID: rid, Polarity: PolarityNegative},
			}, delta: 1},
			{name: "gap-demonstration-only", tags: []Tag{{RID: rid, Gap: true}}},
		} {
			for _, scope := range []struct {
				name  string
				meta  Meta
				level string
				with  ProvenShare
				owes  bool
			}{
				{
					name:  "implemented",
					meta:  Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Support: "bgp-base", Status: "Partial"},
					level: levelMust,
					with:  ProvenShare{Gated: 2, RFCs: 2, Inspected: 2, GatedInspected: 2},
					owes:  true,
				},
				{
					name:  "unsupported",
					meta:  Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Support: "bgp-base", Status: "Unsupported"},
					level: levelMust,
					with:  ProvenShare{Gated: 1, RFCs: 1, Inspected: 2, GatedInspected: 2},
				},
				{
					name:  "unenrolled",
					meta:  Meta{Enrolment: dispositionOutOfScope, Implementation: implementationZe, Support: "bgp-base", Status: "Supported"},
					level: levelMust,
					with:  ProvenShare{Gated: 1, RFCs: 1, Inspected: 1, GatedInspected: 1},
				},
				{
					name:  "no-public-row",
					meta:  Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe},
					level: levelMust,
					with:  ProvenShare{Gated: 1, RFCs: 1, Inspected: 2, GatedInspected: 2},
				},
				{
					name:  "advisory",
					meta:  Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Support: "bgp-base", Status: "Partial"},
					level: levelShould,
					with:  ProvenShare{Gated: 1, RFCs: 1, Inspected: 1, GatedInspected: 1},
				},
			} {
				// Excluded populations need both polarities: weaker evidence
				// repeats the same exclusion without testing another boundary.
				if !scope.owes && evidence.name != "both" {
					continue
				}
				t.Run(kind+"/"+evidence.name+"/"+scope.name, func(t *testing.T) {
					metas := map[string]Meta{
						"rfc1": {Enrolment: enrolmentEnrolled, Implementation: implementationZe, Support: "bgp-base", Status: "Partial"},
						"rfc2": scope.meta,
					}
					requirements := []Requirement{
						{RFC: "rfc1", RID: "RFC1-1-1", Level: levelMust},
						{RFC: "rfc2", RID: rid, Level: scope.level, Annotation: &Annotation{Kind: kind}},
					}
					tags, gaps := splitGapTags(evidence.tags)
					if evidence.name == "gap-demonstration-only" {
						if len(gaps) != 1 {
							t.Fatal("demonstration lost from the gap population")
						}
						if len(tags) != 0 {
							t.Fatal("demonstration entered the proof population")
						}
					}
					delta := 0
					if scope.owes {
						delta = evidence.delta
					}
					if got := annotationWholeCreditDelta(requirements[1], scope.meta, evidence.tags); got != delta {
						t.Fatalf("independent delta = %d, want %d", got, delta)
					}
					with, err := ProvenShareOf(metas, requirements, tags, nil)
					if err != nil {
						t.Fatal(err)
					}
					if with != scope.with {
						t.Fatalf("annotated share = %+v, want %+v", with, scope.with)
					}
					requirements[1].Annotation = nil
					without, err := ProvenShareOf(metas, requirements, tags, nil)
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
