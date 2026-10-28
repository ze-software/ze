// Design: docs/contributing/rfc-conformance-gates.md -- one declaration of annotation meaning.
package rfc

import (
	"strings"
	"testing"
)

// TestAnnotationTraitsPreserveRowBehavior holds the existing kinds against
// independent expected outcomes, exercising the consumers rather than the table.
func TestAnnotationTraitsPreserveRowBehavior(t *testing.T) {
	for _, one := range []struct {
		kind          string
		body          string
		owed          bool
		proven        bool
		unmet         bool
		barsTests     bool
		rollup        RollupState
		unimplemented bool
		notApplicable bool
	}{
		{AnnotationNotApplicable, "another role", true, false, false, true, RollupMet, true, true},
		{AnnotationGap, "not implemented", true, false, true, true, RollupGap, true, false},
		{AnnotationPartial, `tested "MUST send"; gap "MUST log"; internal/example.go::Send does not log`, true, false, true, false, RollupGap, false, false},
		{AnnotationSinglePolarity, "positive; no negative input", true, true, false, false, RollupMet, false, false},
		{AnnotationLowerLayer, "Linux; internal/example.go::Send installs the state", true, false, false, true, RollupMet, false, false},
		{AnnotationFeatureDeclined, `"An implementation MAY offer this optional feature"; internal/example.go::Send offers only the basic form`, true, false, false, true, RollupMet, false, true},
		{AnnotationRollup, "RFC9999-2-2; the constituent owns the obligation", false, false, false, true, RollupUnproven, false, false},
	} {
		t.Run(one.kind, func(t *testing.T) {
			annotation, err := parseAnnotation(one.kind+": "+one.body, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			req := Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust,
				Source: selftestSummaryRel, Line: 1, Text: "MUST send and MUST log (§2)", Annotation: annotation}
			for _, recorded := range []bool{false, true} {
				owed, proven := rowCounts(&req, map[string]bool{req.RID: recorded})
				if owed != one.owed || proven != (recorded && one.proven) {
					t.Errorf("recorded=%v: owed=%v proven=%v, want owed=%v proven=%v", recorded, owed, proven, one.owed, recorded && one.proven)
				}
			}
			positive := Tag{RID: req.RID, Polarity: PolarityPositive, File: "fixture_test.go", Line: 1}
			negative := Tag{RID: req.RID, Polarity: PolarityNegative, File: "fixture_test.go", Line: 2}
			rows := CoverageRows([]Requirement{req}, []Tag{positive, negative}, nil)
			if !one.owed {
				if len(rows) != 0 {
					t.Fatalf("derived row entered the coverage partition: %+v", rows)
				}
			} else {
				if len(rows) != 1 {
					t.Fatalf("coverage rows = %+v, want one", rows)
				}
				row := rows[0]
				wantPartial := 0
				if one.kind == AnnotationPartial {
					wantPartial = 1
				}
				if row.Partial != wantPartial {
					t.Errorf("partial subset = %d, want %d", row.Partial, wantPartial)
				}
				if row.Gated != 1 || row.Annotated != 1 || row.Both != 0 || row.One != 0 || row.Missing != 0 {
					t.Errorf("annotated row changed the partition: %+v", row)
				}
			}
			if gap := annotatedGap(&req); gap != (one.kind == AnnotationGap) {
				t.Errorf("ordinary gap=%v for %s", gap, one.kind)
			}
			if got := rollupTargetState(req, map[string]bool{PolarityPositive: true, PolarityNegative: true}); got != one.rollup {
				t.Errorf("rollup contribution = %v, want %v", got, one.rollup)
			}
			findings := evaluate([]Requirement{req}, []Tag{positive}, map[string]bool{selftestStem: true})
			stale := false
			for _, finding := range findings {
				if strings.Contains(finding.Message, "annotation is stale") {
					stale = true
				}
			}
			if stale != one.barsTests {
				t.Errorf("tag contradicts annotation=%v, want %v; findings=%+v", stale, one.barsTests, findings)
			}
			status := map[string]LedgerRow{selftestStem: {Status: "Supported", Remaining: "none"}}
			if refused := len(checkStatusAgreement([]Requirement{req}, status, map[string]bool{selftestStem: true})) != 0; refused != one.unmet {
				t.Errorf("clean public support refused=%v, want %v", refused, one.unmet)
			}
			status[selftestStem] = LedgerRow{Remaining: "one MUST remains"}
			if refused := len(checkGapCountAgreement([]Requirement{req}, status)) != 0; refused == one.unmet {
				t.Errorf("one unmet row claim refused=%v, unmet=%v", refused, one.unmet)
			}
			for _, audit := range []struct {
				value string
				legal bool
			}{
				{VerdictUnimplemented, one.unimplemented},
				{VerdictNotApplicable, one.notApplicable},
			} {
				verdict := map[string]any{"verdict": audit.value, "no_code_path": "the role is not reachable",
					fingerprintCode: map[string]any{"internal/example.go::Send": "fingerprint"}}
				errs := verdictClaims(selftestStem, req.RID, verdict, req, nil)
				if legal := len(errs) == 0; legal != audit.legal {
					t.Errorf("%s legal=%v, want %v: %v", audit.value, legal, audit.legal, errs)
				}
			}
		})
	}
}

// TestUnknownAnnotationFailsClosed constructs an unparsed row to prove render
// and gate callers cannot turn a missing declaration into proof or an exemption.
func TestUnknownAnnotationFailsClosed(t *testing.T) {
	const unknown = "not-a-declared-kind"
	if _, err := parseAnnotation(unknown+": reason", "fixture"); err == nil {
		t.Fatal("unknown kind parsed")
	}
	if _, known := AnnotationTraitsOf(unknown); known {
		t.Fatal("unknown kind acquired traits")
	}
	req := Requirement{RFC: selftestStem, RID: selftestRIDSend, Level: levelMust,
		Source: selftestSummaryRel, Line: 1, Annotation: &Annotation{Kind: unknown}}
	owed, proven := rowCounts(&req, map[string]bool{req.RID: true})
	if !owed || proven {
		t.Fatalf("unknown row owed=%v proven=%v, want denominator only", owed, proven)
	}
	if req.Rollup() {
		t.Fatal("unknown row became a derived exemption")
	}
	if got := rollupTargetState(req, map[string]bool{PolarityPositive: true, PolarityNegative: true}); got != RollupUnproven {
		t.Fatalf("unknown row contributes %v, want unproven", got)
	}
	if findings := evaluate([]Requirement{req}, nil, map[string]bool{selftestStem: true}); len(findings) == 0 {
		t.Fatal("coverage gate accepted an unknown annotation")
	}
	if errs := verdictClaims(selftestStem, req.RID, map[string]any{"verdict": VerdictNotApplicable}, req, nil); len(errs) == 0 {
		t.Fatal("audit accepted an unknown annotation")
	}
	rows := CoverageRows([]Requirement{req}, nil, nil)
	if len(rows) != 1 || rows[0].Gated != 1 || rows[0].Annotated != 1 {
		t.Fatalf("unknown row vanished from coverage: %+v", rows)
	}
	tags := []Tag{{RID: req.RID, Polarity: PolarityPositive}, {RID: req.RID, Polarity: PolarityNegative}}
	if polarityCovered(req, tags) {
		t.Fatal("unknown annotation gained auditable polarity coverage")
	}
}

// TestPartialStatusStillRequiresExplicitRemaining distinguishes a partial row's
// remaining-work disclosure from an ordinary gap's unsupported public status.
func TestPartialStatusStillRequiresExplicitRemaining(t *testing.T) {
	rows := map[string]LedgerRow{selftestStem: {Status: "Unsupported", Remaining: "none"}}
	enrolled := map[string]bool{selftestStem: true}
	req := Requirement{RFC: selftestStem, RID: selftestRIDSend, Annotation: &Annotation{Kind: AnnotationGap}}
	if errs := checkStatusAgreement([]Requirement{req}, rows, enrolled); len(errs) != 0 {
		t.Fatalf("unsupported gap changed semantics: %v", errs)
	}
	req.Annotation.Kind = AnnotationPartial
	if errs := checkStatusAgreement([]Requirement{req}, rows, enrolled); len(errs) != 1 {
		t.Fatalf("partial without remaining disclosure got %v, want refusal", errs)
	}
	rows[selftestStem] = LedgerRow{Status: "Unsupported", Remaining: "one MUST remains"}
	if errs := checkStatusAgreement([]Requirement{req}, rows, enrolled); len(errs) != 0 {
		t.Fatalf("disclosed partial refused: %v", errs)
	}
}
