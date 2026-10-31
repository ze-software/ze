// VALIDATES: annotations never inflate the published Ze-proven count. Every
// annotation but {single-polarity} earns no credit even when a verified
// discrimination record covers the row, which is the only thing that earns
// credit since 2026-10-30. A kind the sections count not owed (excluded from
// what Ze's code owes and performed by no layer under Ze) moves exactly its own
// MUST rows whose evidence passes its check out of the rows owed; a row of it
// lacking that evidence, and every row of every other kind, stays owed. Rollups
// remain outside the population because they derive other rows.

package rfc

import (
	"slices"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func TestAnnotationsNeverInflateThePublishedSections(t *testing.T) {
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

	// No row of the corpus carries {feature-declined}: the owner rules that
	// only a feature he will never implement leaves the count, and every row
	// that carried the kind became a {gap} owed after the first release. The
	// kind is still declared and judged, so its case runs on a fixture row of
	// a real RFC whose quote and producer the evidence reader checks against
	// this checkout: the line RFC8907-5.4-2 carried before it became a {gap},
	// under an id the corpus does not hold, its text narrowed to the one
	// sentence the quote holds, since featureShapeGap judges a conditional
	// quote only as the row's own sentence.
	const declinedStem = "rfc8907"
	const declinedRID = "RFC8907-5.4-90"
	if !walked[declinedStem] {
		t.Fatalf("the {feature-declined} fixture needs %s walked, and this checkout has not walked it", declinedStem)
	}
	for _, requirement := range collected.Requirements {
		if requirement.RID == declinedRID {
			t.Fatal("feature-declined fixture requirement collides with the real corpus")
		}
	}
	declined, err := ParseSummaryText(
		"- [ ] [RFC8907-5.4-90] [MUST NOT] When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered. (§5.4, Description of Authentication Process) {feature-declined: \"When the client queries the user for the information, the response MUST NOT be reflected in the user interface as it is entered.\"; ze never queries the user for the information: internal/component/tacacs/client.go::Authenticate builds a PAP START with NewPAPAuthenStart from the username and password already supplied through aaa.AuthRequest and returns the REPLY, so it never prompts the user in response to a TACACS+ reply}\n",
		declinedStem, "in-memory-feature-declined-fixture",
	)
	if err != nil {
		t.Fatal(err)
	}
	collected.Requirements = append(collected.Requirements, declined...)

	stems := map[string]bool{}
	for stem := range collected.Metas {
		stems[stem] = true
	}
	// One reader over this checkout judges every not-owed row, for the
	// figures and for the independent count alike.
	reader := newEvidenceReader(root)
	sectionsOf := func(requirements []Requirement) headline {
		t.Helper()
		sections, err := ImplementedSectionsOf(collected.Metas,
			figuresByStem(requirements, stems, walked, proven, figureEvidence{exclusions: reader}))
		if err != nil {
			t.Fatalf("the sections over the corpus: %v", err)
		}
		return headlineOf(&sections)
	}
	with := sectionsOf(collected.Requirements)

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
			// MUST nobody tests, which re-enters the owed rows and moves them:
			// that would prove the row is out of the population, which
			// TestRollupMovesNeitherSectionsNorGatedCount already holds, and
			// nothing about the annotation.
			stripped := make([]Requirement, 0, len(collected.Requirements))
			annotated := 0
			excludedCredit := 0
			leavingOwed := 0
			// sheltered counts the rows whose kind leaves the owed rows on its
			// traits but whose evidence fails its check: they stay owed.
			sheltered := 0
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
				owed := annotationOwedDelta(&requirement, &meta, walked, reader)
				leavingOwed += owed
				if owed == 0 && leavesOwedOnTraits(&requirement, &meta, walked) {
					sheltered++
				}
				requirement.Annotation = nil
				stripped = append(stripped, requirement)
			}
			if annotated == 0 {
				t.Fatalf("no requirement of this corpus carries {%s}, so this case proves nothing", kind)
			}
			without := sectionsOf(stripped)
			expected := with
			expected.ZeProven += excludedCredit
			expected.Kept += leavingOwed
			if expected != without {
				t.Errorf("%d {%s} annotation(s) changed the population or whole credit beyond their %d excluded claims and %d not-owed rows: with %+v, without %+v",
					annotated, kind, excludedCredit, leavingOwed, with, without)
			}
			if with.ZeProven > without.ZeProven {
				t.Errorf("{%s} annotations inflated whole credit: with %+v, without %+v", kind, with, without)
			}
			if kind == AnnotationNotApplicable {
				// The bare copy drops the quote and the role it binds, as the
				// backlog's rows are written.
				evidencedRowBothWays(t, kind, &collected, walked, reader, leavingOwed, with, sectionsOf,
					func(annotation *Annotation) { annotation.Quote, annotation.BoundRole = "", "" })
			}
			if kind == AnnotationFeatureDeclined {
				// The bare copy drops the optionality quote, the evidence the
				// parser demands of every such row.
				evidencedRowBothWays(t, kind, &collected, walked, reader, leavingOwed, with, sectionsOf,
					func(annotation *Annotation) { annotation.Quote, annotation.Heading = "", "" })
			}
			t.Logf("%d {%s} rows; %d leave the owed rows; %d stay owed for want of evidence; %d otherwise-whole claims excluded",
				annotated, kind, leavingOwed, sheltered, excludedCredit)
		})
	}
}

// headline is the comparable projection of ImplementedSections the published
// figures read: Ze proven over the MUST rows owed, and the populations beside
// them. Total is every MUST row counted, so an annotation that moves a row out
// of the owed rows leaves it unchanged.
type headline struct {
	ZeProven, Kept, Total, NotWalked, RFCs, Inspected, MustInspected int
}

func headlineOf(s *ImplementedSections) headline {
	return headline{ZeProven: s.Sections.Must.ZeProven, Kept: s.Sections.Must.Kept, Total: s.Sections.Must.Total,
		NotWalked: s.NotWalked, RFCs: s.RFCs, Inspected: s.Inspected, MustInspected: s.MustInspected}
}

// annotationOwedDelta independently counts the owed MUST row an annotation
// takes away: a row leavesOwedOnTraits names whose evidence passes the check a
// new row must pass (notOwedGap). The sections count such a row not owed;
// stripped of its annotation it is owed. A row whose evidence fails stays
// owed either way, so it moves nothing.
func annotationOwedDelta(requirement *Requirement, meta *Meta, walked map[string]bool, reader *evidenceReader) int {
	if !leavesOwedOnTraits(requirement, meta, walked) {
		return 0
	}
	if _, lacks := reader.notOwedGap(requirement); lacks {
		return 0
	}
	return 1
}

// leavesOwedOnTraits answers whether a row would leave the owed rows on its
// annotation's traits alone: a gated row of a walked RFC Ze implements whose
// kind its declared traits exclude from what Ze's code owes and no layer
// under Ze performs.
func leavesOwedOnTraits(requirement *Requirement, meta *Meta, walked map[string]bool) bool {
	if requirement.Annotation == nil {
		return false
	}
	traits, _ := AnnotationTraitsOf(requirement.Annotation.Kind)
	if !traits.CeilingExcluded {
		return false
	}
	if traits.OSMet {
		return false
	}
	if !requirement.Gated() {
		return false
	}
	if !Implements(meta) {
		return false
	}
	if !walked[requirement.RFC] {
		return false
	}
	return true
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
				with   headline
				owes   bool
			}{
				{
					name:   "implemented",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   headline{Kept: 2, Total: 2, RFCs: 2, Inspected: 2, MustInspected: 2},
					owes:   true,
				},
				{
					// The owner's 2026-10-30 population: an enrolled RFC
					// whose feature is still to come counts at its figure.
					name:   "feature-still-to-come",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   headline{Kept: 2, Total: 2, RFCs: 2, Inspected: 2, MustInspected: 2},
					owes:   true,
				},
				{
					name:  "unwalked",
					meta:  Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level: levelMust,
					// Unwalked: the target RFC adds to neither side, so only the
					// walked companion's row is gated (the rollup rule).
					with: headline{Kept: 1, Total: 1, RFCs: 2, NotWalked: 1, Inspected: 2, MustInspected: 2},
				},
				{
					name:   "unenrolled",
					meta:   Meta{Enrolment: dispositionOutOfScope, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   headline{Kept: 1, Total: 1, RFCs: 1, Inspected: 1, MustInspected: 1},
				},
				{
					name:   "no-public-row",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe},
					level:  levelMust,
					walked: true,
					with:   headline{Kept: 1, Total: 1, RFCs: 1, Inspected: 2, MustInspected: 2},
				},
				{
					name:   "third-party",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationThirdParty, Aggregate: "bgp-core", Row: true},
					level:  levelMust,
					walked: true,
					with:   headline{Kept: 1, Total: 1, RFCs: 1, Inspected: 1, MustInspected: 1},
				},
				{
					name:   "advisory",
					meta:   Meta{Enrolment: enrolmentEnrolled, Implementation: implementationZe, Aggregate: "bgp-core", Row: true},
					level:  levelShould,
					walked: true,
					// An RFC with no gated row is still in both populations
					// (AC-8 names the population, not its rows); it adds no
					// gated requirement to either.
					with: headline{Kept: 1, Total: 1, RFCs: 2, Inspected: 2, MustInspected: 1},
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
					owed := 0
					if scope.owes {
						// The bare {linux-limit} names no register entry, so its
						// evidence fails and the row stays owed (owed is 0).
						owed = annotationOwedDelta(&requirements[1], &scope.meta, walked, newEvidenceReader(t.TempDir()))
					}
					annotated, err := sectionsOver(t, metas, requirements, walked, proven)
					if err != nil {
						t.Fatal(err)
					}
					// The table's Kept is the stripped population: a not-owed
					// kind takes its own row out of it while annotated.
					wantWith := scope.with
					wantWith.Kept -= owed
					if with := headlineOf(&annotated); with != wantWith {
						t.Fatalf("annotated sections = %+v, want %+v", with, wantWith)
					}
					requirements[1].Annotation = nil
					stripped, err := sectionsOver(t, metas, requirements, walked, proven)
					if err != nil {
						t.Fatal(err)
					}
					want := scope.with
					want.ZeProven += delta
					if without := headlineOf(&stripped); without != want {
						t.Fatalf("stripped sections = %+v, want %+v", without, want)
					}
				})
			}
		}
	}
}

// evidencedRowBothWays holds a kind the sections count not owed to both sides
// of the evidence rule on the corpus. The exact comparison of the
// counterfactual already keeps every row lacking evidence in the owed rows;
// this adds that the corpus carries rows of the kind whose evidence passes,
// which leave (leavingOwed), and that a bare copy of one of them, its evidence
// dropped by strip, does not move the denominator: it joins the owed rows,
// one more Kept and one more Total. The copy is made in memory, so the proof
// holds whatever the backlog holds.
func evidencedRowBothWays(t *testing.T, kind string, collected *Collected, walked map[string]bool, reader *evidenceReader,
	leavingOwed int, with headline,
	sectionsOf func([]Requirement) headline,
	strip func(*Annotation),
) {
	t.Helper()
	if leavingOwed == 0 {
		t.Fatalf("no {%s} row of this corpus carries passing evidence, so nothing shows an evidenced row leaves", kind)
	}
	for _, requirement := range collected.Requirements {
		if requirement.Annotation == nil || requirement.Annotation.Kind != kind {
			continue
		}
		meta := collected.Metas[requirement.RFC]
		if annotationOwedDelta(&requirement, &meta, walked, reader) != 1 {
			// A row outside the published population, or one whose
			// evidence fails: it proves nothing about leaving.
			continue
		}
		bare := requirement
		annotation := *requirement.Annotation
		strip(&annotation)
		bare.Annotation, bare.RID = &annotation, requirement.RID+"-bare"
		got := sectionsOf(append(slices.Clone(collected.Requirements), bare))
		want := with
		want.Kept++
		want.Total++
		want.MustInspected++
		if got != want {
			t.Fatalf("a bare copy of %s moved the sections beyond one owed row: with %+v, with the copy %+v", requirement.RID, with, got)
		}
		return
	}
	t.Fatalf("no evidenced {%s} row of the published population was found to copy", kind)
}
