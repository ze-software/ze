// VALIDATES: the {rollup} annotation kind -- its format, the targets its body
// must name, and the parse-time refusals that keep it from asserting a status
// nothing derives.
// PREVENTS: a second {not-applicable}. That kind says "this never bound Ze" and
// nothing in the tree can contradict it; this one says "these rows together ARE
// this row", and the gate goes and reads those rows. Drop the target demand and
// the difference is gone.

package rfc

import (
	"slices"
	"strings"
	"testing"
)

// rollupLine is one well-formed {rollup} checklist line naming two ids and a
// stem.
const rollupLine = "- [ ] [RFC9999-5-2] [MUST] An implementation MUST comply with the multicast " +
	"requirements (§5) {rollup: RFC9999-2.4-2, RFC9999-2.4-3, rfc4301; the three multicast rows " +
	"and the architecture document together are this sentence}"

func TestRollupParsesTargetsAndReason(t *testing.T) {
	req, err := oneLine(t, rollupLine)
	if err != nil {
		t.Fatalf("a well-formed rollup annotation: %v", err)
	}
	if req.Annotation == nil || req.Annotation.Kind != AnnotationRollup {
		t.Fatalf("the annotation was lost: %+v", req.Annotation)
	}
	want := []string{"RFC9999-2.4-2", "RFC9999-2.4-3", "rfc4301"}
	if !slices.Equal(req.Annotation.Targets, want) {
		t.Errorf("targets: %q, want %q", req.Annotation.Targets, want)
	}
	// Reason keeps the whole body, so a renderer that knows only `{kind} reason`
	// still publishes the rows the rollup rests on.
	if !strings.HasPrefix(req.Annotation.Reason, "RFC9999-2.4-2, RFC9999-2.4-3, rfc4301; ") ||
		!strings.HasSuffix(req.Annotation.Reason, "together are this sentence") {
		t.Errorf("reason: %q, want the targets and the why", req.Annotation.Reason)
	}
	if strings.Contains(req.Text, "{") {
		t.Errorf("the marker was left inside the requirement text: %q", req.Text)
	}
	if req.Section != "5" {
		t.Errorf("the marker moved the anchor: %q", req.Section)
	}
}

// TestRollupRefusesEmptyMalformedAndDuplicateTargets drives the parse-time
// refusals: a rollup over nothing, a rollup with no reason, a target shaped
// like neither an id nor a stem, and a target written twice. Self-reference and
// a target the corpus cannot show are checkRollupTargets's refusals, over the
// loaded corpus, and are tested beside it.
func TestRollupRefusesEmptyMalformedAndDuplicateTargets(t *testing.T) {
	cases := []struct {
		name, body, refused string
	}{
		{
			name:    "no target before the reason",
			body:    "{rollup: ; the rows together are this sentence}",
			refused: "needs at least one target",
		},
		{
			name:    "targets with no reason after them",
			body:    "{rollup: RFC9999-2.4-2, RFC9999-2.4-3}",
			refused: "needs at least one target, then a reason",
		},
		{
			name:    "targets with an empty reason",
			body:    "{rollup: RFC9999-2.4-2;}",
			refused: "needs at least one target, then a reason",
		},
		{
			name:    "a target that is neither an id nor a stem",
			body:    "{rollup: RFC9999-2.4, RFC9999-2.4-3; why}",
			refused: "target 'RFC9999-2.4' is neither a requirement id nor a summary stem",
		},
		{
			name:    "two ids with no comma between them",
			body:    "{rollup: RFC9999-2.4-2 RFC9999-2.4-3; why}",
			refused: "is neither a requirement id nor a summary stem",
		},
		{
			name:    "an empty target from a trailing comma",
			body:    "{rollup: RFC9999-2.4-2, ; why}",
			refused: "target '' is neither a requirement id nor a summary stem",
		},
		{
			name:    "one target named twice",
			body:    "{rollup: RFC9999-2.4-2, RFC9999-2.4-3, RFC9999-2.4-2; why}",
			refused: "names 'RFC9999-2.4-2' twice",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := oneLine(t, "- [ ] [RFC9999-5-2] [MUST] A speaker MUST comply (§5) "+one.body)
			if err == nil {
				t.Fatalf("%s parsed, want a refusal", one.body)
			}
			if !strings.Contains(err.Error(), one.refused) {
				t.Errorf("refusal %q does not say %q", err.Error(), one.refused)
			}
			if !strings.HasSuffix(err.Error(), rollupFormat) {
				t.Errorf("refusal %q does not end in the format sentence", err.Error())
			}
		})
	}
}

// TestRollupCannotDisplaceAGap holds the property the note above SupersededKind
// records: a way OUT of the gated population must not be creatable by adding a
// marker beside the one already there. A {gap} and a {rollup} on one line is a
// contradiction the parser refuses rather than a relabeling.
func TestRollupCannotDisplaceAGap(t *testing.T) {
	for _, line := range []string{
		"- [ ] [RFC9999-5-2] [MUST] A speaker MUST comply (§5) {gap: no producer yet} " +
			"{rollup: RFC9999-2.4-2; the rows are this sentence}",
		"- [ ] [RFC9999-5-2] [MUST] A speaker MUST comply (§5) " +
			"{rollup: RFC9999-2.4-2; the rows are this sentence} {gap: no producer yet}",
	} {
		req, err := oneLine(t, line)
		if err == nil {
			t.Fatalf("a line carrying both annotations parsed as %+v, want a refusal", req.Annotation)
		}
		if !strings.Contains(err.Error(), "two coverage annotations on one line") {
			t.Errorf("refusal %q does not name the collision", err.Error())
		}
	}
}

// rollupRow builds one {rollup} requirement of rfc1 in code, the way
// TestCheckLowerLayerProducerReadsTheTree builds its rows: the parser is
// proven above, and the check under test reads Requirement values.
func rollupRow(rid string, line int, targets ...string) Requirement {
	return Requirement{RFC: "rfc1", RID: rid, Level: levelMust,
		Source: "rfc/short/rfc1.md", Line: line, Annotation: &Annotation{
			Kind: AnnotationRollup, Targets: targets,
			Reason: strings.Join(targets, ", ") + "; the rows together are this sentence",
		}}
}

// plainRow builds one requirement with no annotation, a target for a rollup.
func plainRow(stem, rid string) Requirement {
	return Requirement{RFC: stem, RID: rid, Level: levelMust, Source: "rfc/short/" + stem + ".md", Line: 3}
}

// TestRollupRefusesATargetTheCorpusCannotShow is the check that makes the kind
// checkable rather than assertable: every target has to be a row of an
// enrolled summary or an enrolled summary itself, so a rollup over a row nobody
// can open is refused, exactly as a {lower-layer} producer the tree cannot show
// is. Driven through checkRollupTargets, the entry the gate calls
// (ai/rules/evidence.md).
func TestRollupRefusesATargetTheCorpusCannotShow(t *testing.T) {
	// rfc1 is enrolled, rfc2 is a summary the corpus holds but has not
	// enrolled, and rfc7 is no summary at all.
	enrolled := map[string]bool{"rfc1": true}
	cases := []struct {
		name, target, refused string
	}{
		{name: "an id of an enrolled summary", target: "RFC1-1-1"},
		{name: "an enrolled stem", target: "rfc1"},
		{
			name: "an id no summary holds", target: "RFC1-9-9",
			refused: "names 'RFC1-9-9', which no summary holds",
		},
		{
			name: "an id of a summary that is not enrolled", target: "RFC2-1-1",
			refused: "names 'RFC2-1-1', a row of rfc2, which is not enrolled",
		},
		{
			name: "a stem that is not enrolled", target: "rfc2",
			refused: "names 'rfc2', a summary that is not enrolled",
		},
		{
			name: "a stem no summary answers to", target: "rfc7",
			refused: "names 'rfc7', which no summary holds",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			requirements := []Requirement{
				plainRow("rfc1", "RFC1-1-1"),
				plainRow("rfc2", "RFC2-1-1"),
				rollupRow("RFC1-5-1", 7, one.target),
			}
			errs := checkRollupTargets(requirements, enrolled)
			if one.refused == "" {
				if len(errs) != 0 {
					t.Fatalf("a target the corpus holds was refused: %v", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("checkRollupTargets answered %d errors, want 1: %v", len(errs), errs)
			}
			if !strings.HasPrefix(errs[0], "rfc/short/rfc1.md:7: RFC1-5-1 ") {
				t.Errorf("refusal %q does not open with the row", errs[0])
			}
			if !strings.Contains(errs[0], one.refused) {
				t.Errorf("refusal %q does not say %q", errs[0], one.refused)
			}
			if !strings.HasSuffix(errs[0], rollupFormat) {
				t.Errorf("refusal %q does not end in the format sentence", errs[0])
			}
		})
	}
}

// TestRollupRefusesACycle holds R-1: a rollup whose targets lead back to it
// has no state to derive, so the walk over rollup-to-rollup targets refuses
// the repeat before derivation ever runs. A rollup naming itself is the
// shortest cycle and carries its own sentence, because the author's fix is
// different: drop one target rather than untangle two rows.
func TestRollupRefusesACycle(t *testing.T) {
	enrolled := map[string]bool{"rfc1": true}
	t.Run("two rollups naming each other", func(t *testing.T) {
		requirements := []Requirement{
			plainRow("rfc1", "RFC1-1-1"),
			rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC1-5-2"),
			rollupRow("RFC1-5-2", 8, "RFC1-5-1"),
		}
		errs := checkRollupTargets(requirements, enrolled)
		if len(errs) != 2 {
			t.Fatalf("checkRollupTargets answered %d errors, want one per row of the cycle: %v", len(errs), errs)
		}
		for index, want := range []string{
			"rfc/short/rfc1.md:7: RFC1-5-1 is annotated {rollup} and names 'RFC1-5-2', a rollup whose targets lead back to RFC1-5-1",
			"rfc/short/rfc1.md:8: RFC1-5-2 is annotated {rollup} and names 'RFC1-5-1', a rollup whose targets lead back to RFC1-5-2",
		} {
			if !strings.HasPrefix(errs[index], want) {
				t.Errorf("refusal %q does not open %q", errs[index], want)
			}
			if !strings.HasSuffix(errs[index], rollupFormat) {
				t.Errorf("refusal %q does not end in the format sentence", errs[index])
			}
		}
	})
	t.Run("a rollup naming itself", func(t *testing.T) {
		requirements := []Requirement{
			plainRow("rfc1", "RFC1-1-1"),
			rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC1-5-1"),
		}
		errs := checkRollupTargets(requirements, enrolled)
		if len(errs) != 1 {
			t.Fatalf("checkRollupTargets answered %d errors, want 1: %v", len(errs), errs)
		}
		if !strings.HasPrefix(errs[0], "rfc/short/rfc1.md:7: RFC1-5-1 is annotated {rollup} and names itself") {
			t.Errorf("refusal %q does not say the row names itself", errs[0])
		}
		if !strings.HasSuffix(errs[0], rollupFormat) {
			t.Errorf("refusal %q does not end in the format sentence", errs[0])
		}
	})
	t.Run("a chain that ends in a plain row", func(t *testing.T) {
		requirements := []Requirement{
			plainRow("rfc1", "RFC1-1-1"),
			rollupRow("RFC1-5-1", 7, "RFC1-5-2"),
			rollupRow("RFC1-5-2", 8, "RFC1-1-1"),
		}
		if errs := checkRollupTargets(requirements, enrolled); len(errs) != 0 {
			t.Fatalf("a rollup over a rollup over a row was refused: %v", errs)
		}
	})
}

// TestRollupCannotStandBesideATag checks the arm of evaluate that {rollup}
// joined (AC-8). A tagged test on a rollup row would claim more than its body
// checks, which is the vacuity the kind exists to prevent, so the tag makes
// the annotation stale exactly as it does beside {lower-layer}.
func TestRollupCannotStandBesideATag(t *testing.T) {
	requirements := []Requirement{
		plainRow("rfc1", "RFC1-1-1"),
		rollupRow("RFC1-5-1", 7, "RFC1-1-1"),
	}
	tags := []Tag{
		{RID: "RFC1-1-1", Polarity: PolarityPositive, File: "a_test.go", Line: 3},
		{RID: "RFC1-1-1", Polarity: PolarityNegative, File: "a_test.go", Line: 4},
		{RID: "RFC1-5-1", Polarity: PolarityPositive, File: "a_test.go", Line: 5},
	}
	findings := evaluate(requirements, tags, map[string]bool{"rfc1": true})
	if len(findings) != 1 {
		t.Fatalf("evaluate answered %d findings, want 1: %+v", len(findings), findings)
	}
	if findings[0].RID != "RFC1-5-1" || !strings.Contains(findings[0].Message, "the annotation is stale") {
		t.Errorf("finding %+v does not say the rollup's annotation is stale", findings[0])
	}
}

// annotatedRow builds one requirement of rfc1 carrying an annotation of the
// given kind, a target whose contribution to a rollup is the kind's own.
func annotatedRow(rid, kind string) Requirement {
	row := plainRow("rfc1", rid)
	row.Annotation = &Annotation{Kind: kind, Reason: "the reason " + rid}
	return row
}

// pairTags answers a positive and a negative tag for one requirement.
func pairTags(rid string) []Tag {
	return []Tag{
		{RID: rid, Polarity: PolarityPositive, File: "a_test.go", Line: 3},
		{RID: rid, Polarity: PolarityNegative, File: "a_test.go", Line: 4},
	}
}

// findingsFor answers the findings evaluate raised against one row.
func findingsFor(findings []Finding, rid string) []Finding {
	var out []Finding
	for _, finding := range findings {
		if finding.RID == rid {
			out = append(out, finding)
		}
	}
	return out
}

// derivedCauseOf answers the cause evaluate filled on one row.
func derivedCauseOf(t *testing.T, requirements []Requirement, rid string) string {
	t.Helper()
	for _, req := range requirements {
		if req.RID == rid {
			return req.DerivedCause
		}
	}
	t.Fatalf("no row %s", rid)
	return ""
}

// derivedOf answers what evaluate filled on one row.
func derivedOf(t *testing.T, requirements []Requirement, rid string) RollupState {
	t.Helper()
	for _, req := range requirements {
		if req.RID == rid {
			return req.Derived
		}
	}
	t.Fatalf("no row %s", rid)
	return RollupNone
}

// TestRollupIsMetWhenEveryTargetIs holds AC-4 through evaluate, the producer
// that fills Requirement.Derived: every met kind in the spec's table, a stem
// target that expands to the other summary's gated rows, and a rollup over a
// met rollup each contribute met, so the row is silent and reads met.
func TestRollupIsMetWhenEveryTargetIs(t *testing.T) {
	single := annotatedRow("RFC1-1-2", AnnotationSinglePolarity)
	single.Annotation.Polarity = PolarityPositive
	requirements := []Requirement{
		plainRow("rfc1", "RFC1-1-1"),
		single,
		annotatedRow("RFC1-1-3", AnnotationLowerLayer),
		annotatedRow("RFC1-1-4", AnnotationFeatureDeclined),
		annotatedRow("RFC1-1-5", AnnotationNotApplicable),
		rollupRow("RFC1-6-1", 9, "RFC1-1-1"),
		plainRow("rfc2", "RFC2-1-1"),
		// A row of rfc2 the stem target does not reach: not gated.
		{RFC: "rfc2", RID: "RFC2-1-2", Level: "SHOULD", Source: "rfc/short/rfc2.md", Line: 4},
		rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC1-1-2", "RFC1-1-3", "RFC1-1-4", "RFC1-1-5", "RFC1-6-1", "rfc2"),
	}
	tags := append(pairTags("RFC1-1-1"), pairTags("RFC2-1-1")...)
	tags = append(tags, Tag{RID: "RFC1-1-2", Polarity: PolarityPositive, File: "a_test.go", Line: 5})
	findings := evaluate(requirements, tags, map[string]bool{"rfc1": true, "rfc2": true})
	if len(findings) != 0 {
		t.Fatalf("evaluate answered %d findings over a corpus whose every row is met: %+v", len(findings), findings)
	}
	for _, rid := range []string{"RFC1-5-1", "RFC1-6-1"} {
		if got := derivedOf(t, requirements, rid); got != RollupMet {
			t.Errorf("%s derives as %q, want met", rid, got)
		}
	}
	if got := derivedOf(t, requirements, "RFC1-1-1"); got != RollupNone {
		t.Errorf("a row that is not a rollup carries derived state %q", got)
	}
	// The stem expansion is not vacuous: with the one gated row of rfc2
	// unproven, the rollup that names the stem is unproven too.
	withoutRFC2 := append(append([]Tag{}, tags[:2]...), tags[4:]...)
	evaluate(requirements, withoutRFC2, map[string]bool{"rfc1": true, "rfc2": true})
	if got := derivedOf(t, requirements, "RFC1-5-1"); got != RollupUnproven {
		t.Errorf("a stem whose row lost its tests leaves the rollup %q", got)
	}
	if cause := derivedCauseOf(t, requirements, "RFC1-5-1"); !strings.HasPrefix(cause, "RFC2-1-1 is not proven") {
		t.Errorf("the derived cause reads %q, want it to name RFC2-1-1", cause)
	}
}

// TestRollupIsAGapWhileOneTargetIs holds AC-5 through evaluate and
// checkGapCountAgreement: one {gap} target makes the rollup a gap whatever
// else it names, the derived cause names that target, a rollup over the gap
// rollup carries the same cause, no rollup raises a finding, and the Remaining
// cell still counts the one {gap} annotation because a derived gap is not one.
func TestRollupIsAGapWhileOneTargetIs(t *testing.T) {
	requirements := []Requirement{
		plainRow("rfc1", "RFC1-1-1"),
		annotatedRow("RFC1-1-2", AnnotationGap),
		plainRow("rfc1", "RFC1-1-3"),
		// The unproven row comes first, and the gap still decides.
		rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC1-1-3", "RFC1-1-2"),
		rollupRow("RFC1-5-2", 8, "RFC1-5-1"),
	}
	findings := evaluate(requirements, pairTags("RFC1-1-1"), map[string]bool{"rfc1": true})
	for _, rid := range []string{"RFC1-5-1", "RFC1-5-2"} {
		if got := derivedOf(t, requirements, rid); got != RollupGap {
			t.Errorf("%s derives as %q, want gap", rid, got)
		}
		if cause := derivedCauseOf(t, requirements, rid); cause != "RFC1-1-2 is annotated {gap}" {
			t.Errorf("%s's derived cause reads %q", rid, cause)
		}
		// The gap is declared under RFC1-1-2's own id and disclosed by the
		// Remaining cell, so the rollup reports it a second time nowhere.
		if own := findingsFor(findings, rid); len(own) != 0 {
			t.Errorf("%s raised %d finding(s), want none: %+v", rid, len(own), own)
		}
	}
	if own := findingsFor(findings, "RFC1-1-3"); len(own) != 1 {
		t.Errorf("the untested row's own finding is gone: %+v", findings)
	}
	rows := map[string]LedgerRow{"rfc1": {Status: "Partial", Remaining: "one MUST-level gap"}}
	if errs := checkGapCountAgreement(requirements, rows); len(errs) != 0 {
		t.Errorf("two derived gaps moved the Remaining count: %v", errs)
	}
	rows["rfc1"] = LedgerRow{Status: "Partial", Remaining: "three MUST-level gaps"}
	if errs := checkGapCountAgreement(requirements, rows); len(errs) != 1 {
		t.Errorf("a cell counting the derived gaps was accepted, so the check above proves nothing: %v", errs)
	}
}

// TestRollupIsUnprovenWhileOneTargetIs holds AC-6 through evaluate: a target
// with no test, a target with one polarity, a {single-polarity} target
// missing its one test, a target the gate holds no state for, a stem with no
// gated row, and a rollup leading into a cycle it is not on each make the
// rollup unproven, its derived cause naming the first such target and the
// rollup itself raising no finding.
func TestRollupIsUnprovenWhileOneTargetIs(t *testing.T) {
	enrolled := map[string]bool{"rfc1": true, "rfc3": true}
	single := annotatedRow("RFC1-1-4", AnnotationSinglePolarity)
	single.Annotation.Polarity = PolarityNegative
	cases := []struct {
		name  string
		rows  []Requirement
		cause string
	}{
		{name: "no test", rows: []Requirement{rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC1-1-2")},
			cause: "RFC1-1-2 is not proven"},
		{name: "one polarity", rows: []Requirement{rollupRow("RFC1-5-1", 7, "RFC1-1-3")},
			cause: "RFC1-1-3 is not proven"},
		{name: "single-polarity without its test", rows: []Requirement{rollupRow("RFC1-5-1", 7, "RFC1-1-4")},
			cause: "RFC1-1-4 is not proven"},
		{name: "a row the gate holds no state for", rows: []Requirement{rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC2-1-1")},
			cause: "'RFC2-1-1' is neither a gated row of an enrolled summary nor an enrolled summary"},
		{name: "a stem with no gated row", rows: []Requirement{rollupRow("RFC1-5-1", 7, "rfc3")},
			cause: "rfc3 holds no gated row to derive from"},
		{name: "a cycle the row is not on", rows: []Requirement{
			rollupRow("RFC1-5-1", 7, "RFC1-1-1", "RFC1-5-2"),
			rollupRow("RFC1-5-2", 8, "RFC1-5-3"),
			rollupRow("RFC1-5-3", 9, "RFC1-5-2"),
		}, cause: "RFC1-5-2 is a rollup whose derivation leads back into a cycle"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			requirements := append([]Requirement{
				plainRow("rfc1", "RFC1-1-1"),
				plainRow("rfc1", "RFC1-1-2"),
				plainRow("rfc1", "RFC1-1-3"),
				single,
				plainRow("rfc2", "RFC2-1-1"),
				{RFC: "rfc3", RID: "RFC3-1-1", Level: "SHOULD", Source: "rfc/short/rfc3.md", Line: 3},
			}, one.rows...)
			tags := append(pairTags("RFC1-1-1"),
				Tag{RID: "RFC1-1-3", Polarity: PolarityPositive, File: "a_test.go", Line: 5})
			findings := evaluate(requirements, tags, enrolled)
			if got := derivedOf(t, requirements, "RFC1-5-1"); got != RollupUnproven {
				t.Errorf("RFC1-5-1 derives as %q, want unproven", got)
			}
			if cause := derivedCauseOf(t, requirements, "RFC1-5-1"); !strings.HasPrefix(cause, one.cause) {
				t.Errorf("the derived cause reads %q, want it to name %q", cause, one.cause)
			}
			// An unproven target is reported under its own id by evaluate's
			// "has no test and no annotation" arm, so the rollup is silent.
			if own := findingsFor(findings, "RFC1-5-1"); len(own) != 0 {
				t.Errorf("RFC1-5-1 raised %d finding(s), want none: %+v", len(own), own)
			}
		})
	}
}

// TestRollupInAnUnenrolledSummaryStillDerives holds the renderer's contract
// for a summary nobody enrolled: every summary is printed, so a {rollup} row
// there carries a derived state too, or DerivedMark would stop the render.
// Its own rows hold no state, so a rollup over them is unproven with the
// cause naming the row, and a rollup over enrolled rows derives from them.
func TestRollupInAnUnenrolledSummaryStillDerives(t *testing.T) {
	requirements := []Requirement{
		plainRow("rfc1", "RFC1-1-1"),
		plainRow("rfc2", "RFC2-1-1"),
		{RFC: "rfc2", RID: "RFC2-5-1", Level: levelMust, Source: "rfc/short/rfc2.md", Line: 7,
			Annotation: &Annotation{Kind: AnnotationRollup, Targets: []string{"RFC1-1-1", "RFC2-1-1"},
				Reason: "RFC1-1-1, RFC2-1-1; the rows together are this sentence"}},
		{RFC: "rfc2", RID: "RFC2-5-2", Level: levelMust, Source: "rfc/short/rfc2.md", Line: 8,
			Annotation: &Annotation{Kind: AnnotationRollup, Targets: []string{"RFC1-1-1"},
				Reason: "RFC1-1-1; the row is this sentence"}},
	}
	evaluate(requirements, pairTags("RFC1-1-1"), map[string]bool{"rfc1": true})
	if got := derivedOf(t, requirements, "RFC2-5-1"); got != RollupUnproven {
		t.Errorf("RFC2-5-1 derives as %q, want unproven", got)
	}
	if cause := derivedCauseOf(t, requirements, "RFC2-5-1"); !strings.HasPrefix(cause, "'RFC2-1-1' is neither") {
		t.Errorf("the derived cause reads %q, want it to name RFC2-1-1", cause)
	}
	if got := derivedOf(t, requirements, "RFC2-5-2"); got != RollupMet {
		t.Errorf("RFC2-5-2 derives as %q, want met", got)
	}
	for _, requirement := range requirements {
		if requirement.Rollup() && requirement.DerivedMark() == "" {
			t.Errorf("%s prints an empty derived mark", requirement.RID)
		}
	}
}
