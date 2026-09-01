package rfc

import "testing"

// VALIDATES: the per-RFC polarity counters are readable from another package,
// and answer what the private walk answered.
//
// internal/le/site recomputed its own buckets over the same requirements until
// 2026-09-01, because this walk was private. Two counts of one population is a
// future disagreement with nothing to arbitrate it (ai/rules/principles.md), so
// the exported row is what the published page reads. The method is a stated
// corpus whose every bucket is reachable.
func TestTheCoverageRowIsReadableFromAnotherPackage(t *testing.T) {
	requirements := []Requirement{
		{RFC: "rfc9999", RID: "RFC9999-1-1", Level: levelMust},
		{RFC: "rfc9999", RID: "RFC9999-1-2", Level: levelMust},
		{RFC: "rfc9999", RID: "RFC9999-1-3", Level: levelMust,
			Annotation: &Annotation{Kind: AnnotationGap, Reason: "not implemented yet"}},
		{RFC: "rfc9999", RID: "RFC9999-1-4", Level: levelMust},
		{RFC: "rfc9999", RID: "RFC9999-1-5", Level: "SHOULD"},
	}
	tags := []Tag{
		{RID: "RFC9999-1-1", Polarity: PolarityPositive, File: "internal/a_test.go"},
		{RID: "RFC9999-1-1", Polarity: PolarityNegative, File: "internal/a_test.go"},
		{RID: "RFC9999-1-2", Polarity: PolarityPositive, File: "internal/a_test.go"},
	}
	table := []Carrier{{Name: "unit", Kind: kindUnit, Tier: tierVerify,
		Prefix: "internal/", Suffix: "_test.go"}}

	rows := CoverageRows(requirements, tags, table)
	if len(rows) != 1 {
		t.Fatalf("the walk answered %d row(s), want one for rfc9999", len(rows))
	}
	row := rows[0]
	for _, one := range []struct {
		field string
		got   int
		want  int
	}{
		{"gated", row.Gated, 4},
		{"both", row.Both, 1},
		{"one", row.One, 1},
		{"annotated", row.Annotated, 1},
		{"missing", row.Missing, 1},
		{"nightly-only", row.NightlyOnly, 0},
	} {
		if one.got != one.want {
			t.Errorf("%s is %d, want %d", one.field, one.got, one.want)
		}
	}
	if row.Outstanding() != 2 {
		t.Errorf("the outstanding work is %d, want the one-polarity plus the missing row", row.Outstanding())
	}
}
