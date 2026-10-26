// VALIDATES: AC-4. A summary's `Support` cell names one aggregate or reads
// `-` beside an `Aggregate` cell; the retired `<section> <rank>` form, a
// doubled declaration, a missing aggregate and a cell its aggregate's shape
// does not render are each refused naming the file. A HEAD^ summary in the
// retired shape is still read by the baseline, so the migration commit keeps
// its ratchets.
// PREVENTS: a path that accepts the retired rank after the migration, and a
// row that renders under no heading.

package rfc

import (
	"strings"
	"testing"
)

// aggregateMeta is the selftest Meta table with its Support cell replaced.
func aggregateMeta(support string) string {
	return "# RFC 9999\n\n" + strings.Replace(selftestMeta, "| Support | "+FixtureAggregate+" |", support, 1)
}

func TestSupportNamesAggregate(t *testing.T) {
	aggregates := map[string]Aggregate{
		FixtureAggregate: {Name: FixtureAggregate, Shape: ShapeWide},
		"drafts":         {Name: "drafts", Shape: ShapeBrief},
	}
	meta, err := ParseMeta(aggregateMeta("| Support | "+FixtureAggregate+" |"), "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatal(err)
	}
	if !meta.HasRow() || meta.Aggregate != FixtureAggregate {
		t.Fatalf("parsed %+v", meta)
	}
	if err := checkAggregate(&meta, aggregates, "rfc9999", "rfc/short/rfc9999.md"); err != nil {
		t.Fatal(err)
	}

	unrowed := strings.NewReplacer("| Support | "+FixtureAggregate+" |", "| Support | - |\n| Aggregate | tunnels |",
		"| Support area | Widgets |\n", "", "| Support status | Partial |\n", "",
		"| Support coverage | unit tests |\n", "", "| Support remaining | Zero MUST gaps. |\n", "").
		Replace("# RFC 9999\n\n" + selftestMeta)
	meta, err = ParseMeta(unrowed, "rfc9999", "")
	if err != nil {
		t.Fatal(err)
	}
	if meta.HasRow() || meta.Aggregate != "tunnels" {
		t.Fatalf("a `-` summary parsed as %+v", meta)
	}
	err = checkAggregate(&meta, aggregates, "rfc9999", "rfc/short/rfc9999.md")
	if err == nil || !strings.Contains(err.Error(), "rfc/profiles/tunnels.md") ||
		!strings.Contains(err.Error(), "'drafts'") {
		t.Errorf("an undeclared aggregate is not refused naming the file and the set: %v", err)
	}

	for label, one := range map[string]struct{ text, want string }{
		"retired rank":       {aggregateMeta("| Support | bgp-base 10 |"), "not one aggregate name"},
		"doubled":            {aggregateMeta("| Support | bgp-core |\n| Aggregate | bgp-core |"), "second declaration"},
		"no-row, no aggr":    {strings.Replace(unrowed, "| Aggregate | tunnels |\n", "", 1), "`Aggregate` is ''"},
		"near-miss spelling": {strings.Replace(unrowed, "| Aggregate |", "| Aggregates |", 1), "Aggregates"},
	} {
		t.Run(label, func(t *testing.T) {
			_, err := ParseMeta(one.text, "rfc9999", "rfc/short/rfc9999.md")
			if err == nil || !strings.Contains(err.Error(), one.want) {
				t.Fatalf("want a refusal naming %q, got %v", one.want, err)
			}
		})
	}

	brief, err := ParseMeta(aggregateMeta("| Support | drafts |"), "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkAggregate(&brief, aggregates, "rfc9999", "rfc/short/rfc9999.md"); err == nil ||
		!strings.Contains(err.Error(), "four columns") {
		t.Errorf("a brief row with a Remaining cell is not refused: %v", err)
	}
}

func TestBaselineReadsTheRetiredSectionRank(t *testing.T) {
	rowed := aggregateMeta("| Support | bgp-base 10 |")
	if _, err := ParseMeta(rowed, "rfc9999", ""); err == nil {
		t.Fatal("the working-tree reader accepts the retired rank")
	}
	meta, err := ParseMeta(retiredSectionRank(rowed), "rfc9999", "")
	if err != nil || !meta.HasRow() || meta.Enrolment != enrolmentEnrolled {
		t.Fatalf("a retired row summary is not read by the baseline: %+v %v", meta, err)
	}
	unrowed := strings.NewReplacer("| Support | bgp-base 10 |", "| Support | - |",
		"| Support area | Widgets |\n", "", "| Support status | Partial |\n", "",
		"| Support coverage | unit tests |\n", "", "| Support remaining | Zero MUST gaps. |\n", "").Replace(rowed)
	meta, err = ParseMeta(retiredSectionRank(unrowed), "rfc9999", "")
	if err != nil || meta.HasRow() || meta.Aggregate != retiredShapeAggregate {
		t.Fatalf("a retired no-row summary is not read by the baseline: %+v %v", meta, err)
	}
	current := aggregateMeta("| Support | " + FixtureAggregate + " |")
	if retiredSectionRank(current) != current {
		t.Error("the baseline rewrite changed a summary already in the current shape")
	}
}
