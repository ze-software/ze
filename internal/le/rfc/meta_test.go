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

// useCaseMeta is the selftest Meta table with its two use-case rows replaced
// by rows, which may be empty to drop both.
func useCaseMeta(rows string) string {
	return "# RFC 9999\n\n" + strings.Replace(selftestMeta, selftestUseCaseRows, rows, 1)
}

// TestParseMetaUseCases: AC-1. The two rows read into two sorted name lists,
// and `-` reads as an empty list. Method: one table with names out of order in
// each row, and the selftest table with `-` in both.
func TestParseMetaUseCases(t *testing.T) {
	meta, err := ParseMeta(useCaseMeta("| Use cases | route-server, looking-glass |\n"+
		"| Optional use cases | lns |\n"), "rfc9999", metaWhere)
	if err != nil {
		t.Fatalf("ParseMeta: %v", err)
	}
	if got := strings.Join(meta.UseCases, ","); got != "looking-glass,route-server" {
		t.Errorf("Use cases read %q, want looking-glass,route-server", got)
	}
	if got := strings.Join(meta.OptionalUseCases, ","); got != "lns" {
		t.Errorf("Optional use cases read %q, want lns", got)
	}
	none, err := ParseMeta("# RFC 9999\n\n"+selftestMeta, "rfc9999", metaWhere)
	if err != nil {
		t.Fatalf("ParseMeta of `-` rows: %v", err)
	}
	if len(none.UseCases) != 0 || len(none.OptionalUseCases) != 0 {
		t.Errorf("`-` read as %v and %v, want two empty lists", none.UseCases, none.OptionalUseCases)
	}
}

// TestParseMetaUseCasesRefusals: AC-2. Every malformed shape of the two rows is
// a parse error naming the summary and the cell, none of them defaulted.
// Method: one table per shape, each checked for the file and a phrase of its fix.
func TestParseMetaUseCasesRefusals(t *testing.T) {
	both := func(required, optional string) string {
		return "| Use cases | " + required + " |\n| Optional use cases | " + optional + " |\n"
	}
	cases := map[string]struct{ rows, want string }{
		"missing required":   {"| Optional use cases | - |\n", "has no `Use cases` row"},
		"missing optional":   {"| Use cases | - |\n", "has no `Optional use cases` row"},
		"empty cell":         {both("", "-"), "`Use cases` is empty"},
		"dash mixed":         {both("-, route-server", "-"), "mixes `-` with profile names"},
		"duplicate":          {both("lns, lns", "-"), "names 'lns' twice"},
		"name in both rows":  {both("lns", "lns"), "names 'lns' in both"},
		"bad token":          {both("Route Server", "-"), "is not one lower-case hyphenated profile name"},
		"near miss hyphen":   {both("-", "-") + "| Use-cases | lns |\n", "Meta field `Use-cases`"},
		"near miss optional": {both("-", "-") + "| Optional usecases | lns |\n", "Meta field `Optional usecases`"},
		"near miss profiles": {both("-", "-") + "| Profiles | lns |\n", "Meta field `Profiles`"},
	}
	for label, one := range cases {
		t.Run(label, func(t *testing.T) {
			_, err := ParseMeta(useCaseMeta(one.rows), "rfc9999", metaWhere)
			if err == nil {
				t.Fatalf("rows %q parsed", one.rows)
			}
			for _, want := range []string{metaWhere, one.want} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestUseCasesNamingUnknownProfileRefused: AC-3. A name in either row that is
// no use-case profile, or that is a technology aggregate, is refused naming
// the summary and listing the use cases. Method: checkUseCases over a profile
// set holding one use case and one aggregate.
func TestUseCasesNamingUnknownProfileRefused(t *testing.T) {
	profiles := Profiles{UseCases: map[string]UseCase{"lns": {Name: "lns"}, "route-server": {Name: "route-server"}},
		Aggregates: map[string]Aggregate{"bgp-core": {Name: "bgp-core"}}}
	cases := map[string]struct {
		meta Meta
		want string
	}{
		"unknown required":   {Meta{UseCases: []string{"nowhere"}}, "no rfc/profiles/nowhere.md"},
		"unknown optional":   {Meta{OptionalUseCases: []string{"nowhere"}}, "no rfc/profiles/nowhere.md"},
		"aggregate required": {Meta{UseCases: []string{"bgp-core"}}, "is a technology aggregate"},
		"aggregate optional": {Meta{OptionalUseCases: []string{"bgp-core"}}, "is a technology aggregate"},
	}
	for label, one := range cases {
		t.Run(label, func(t *testing.T) {
			err := checkUseCases(&one.meta, profiles, metaWhere)
			if err == nil {
				t.Fatal("refused nothing")
			}
			for _, want := range []string{metaWhere, one.want, "'lns', 'route-server'"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
	known := Meta{UseCases: []string{"lns"}, OptionalUseCases: []string{"route-server"}}
	if err := checkUseCases(&known, profiles, metaWhere); err != nil {
		t.Errorf("two known use cases refused: %v", err)
	}
}

// TestBaselineReadsSummariesWithoutUseCaseRows: a HEAD^ summary written before
// the use-case rows existed is read by the baseline as declaring `-` in both,
// in both retired Support shapes, and a summary already carrying the rows is
// left as it is. Method: the selftest table with its rows removed, then the
// retired `<section> <rank>` cell on top.
func TestBaselineReadsSummariesWithoutUseCaseRows(t *testing.T) {
	bare := useCaseMeta("")
	if _, err := ParseMeta(bare, "rfc9999", metaWhere); err == nil {
		t.Fatal("the live reader accepts a summary with no use-case rows")
	}
	for label, text := range map[string]string{
		"current Support": bare,
		"retired rank":    strings.Replace(bare, "| Support | "+FixtureAggregate+" |", "| Support | bgp 3 |", 1),
	} {
		meta, err := ParseMeta(retiredBaselineShapes(text), "rfc9999", metaWhere)
		if err != nil {
			t.Fatalf("%s: the baseline could not read it: %v", label, err)
		}
		if len(meta.UseCases) != 0 || len(meta.OptionalUseCases) != 0 {
			t.Errorf("%s: the baseline read use cases %v and %v, want none", label, meta.UseCases, meta.OptionalUseCases)
		}
	}
	declared := useCaseMeta("| Use cases | lns |\n| Optional use cases | - |\n")
	if retiredBaselineShapes(declared) != declared {
		t.Error("the baseline rewrite changed a summary already carrying its use-case rows")
	}
}

// TestAggregateTierOptional: a summary joins its aggregate's optional tier by
// writing `| Aggregate tier | optional |` beside the `Support` or `Aggregate`
// cell that names the aggregate, and its absence is the required tier.
// Method: parse the rowed and the unrowed shape with and without the row,
// then the refused values: `required`, which the absent row already says,
// and a word that is no tier.
// PREVENTS: a second way to name the aggregate, and a misspelled tier read as
// the required one.
func TestAggregateTierOptional(t *testing.T) {
	where := "rfc/short/rfc9999.md"
	support := "| Support | " + FixtureAggregate + " |"
	required, err := ParseMeta(aggregateMeta(support), "rfc9999", where)
	if err != nil {
		t.Fatal(err)
	}
	if required.AggregateOptional {
		t.Errorf("a summary with no `Aggregate tier` row parsed as optional: %+v", required)
	}
	rowed, err := ParseMeta(aggregateMeta(support+"\n| Aggregate tier | optional |"), "rfc9999", where)
	if err != nil {
		t.Fatal(err)
	}
	if !rowed.AggregateOptional || rowed.Aggregate != FixtureAggregate || !rowed.HasRow() {
		t.Errorf("a rowed optional summary parsed as %+v", rowed)
	}
	unrowed := strings.NewReplacer(support, "| Support | - |\n| Aggregate | tunnels |\n| Aggregate tier | optional |",
		"| Support area | Widgets |\n", "", "| Support status | Partial |\n", "",
		"| Support coverage | unit tests |\n", "", "| Support remaining | Zero MUST gaps. |\n", "").
		Replace("# RFC 9999\n\n" + selftestMeta)
	bare, err := ParseMeta(unrowed, "rfc9999", where)
	if err != nil {
		t.Fatal(err)
	}
	if !bare.AggregateOptional || bare.Aggregate != "tunnels" || bare.HasRow() {
		t.Errorf("an unrowed optional summary parsed as %+v", bare)
	}
	for value, want := range map[string]string{
		"required": "Delete the row",
		"Optional": "not a tier",
		"":         "not a tier",
	} {
		t.Run("value "+value, func(t *testing.T) {
			_, err := ParseMeta(aggregateMeta(support+"\n| Aggregate tier | "+value+" |"), "rfc9999", where)
			if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), where) {
				t.Fatalf("want a refusal naming %q and the file, got %v", want, err)
			}
		})
	}
}
