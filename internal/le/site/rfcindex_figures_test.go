package site

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestTheComplianceIndexMarksEveryFigure renders the compliance index over
// the profile fixture's data and proves its headline is the enrolled RFCs'
// sections, every value a data-ze-rfc element filled with the last build's
// value: the enrolled subject leaves out rfc6, which is not enrolled, and
// rfc7, which is enrolled but third-party; the corpus counts sum every RFC's
// requirement rows whether or not it is walked; and the mechanism table and the exclusion coverage carry marked values. The
// retired figures are gone: no percentage, no Gated MUSTs card, no Proven by
// test share, no bucket table and no ceiling, and no mark is left unfilled.
//
// Method: rfcDetailFiguresLedger publishes the fixture data/rfc/ and the
// ledger; renderRFCCompliance writes the index; the page and its mirror are
// read back and searched.
// PREVENTS: a headline summed over RFCs the gate does not hold, a corpus
// count that drops an unwalked summary, and a retired figure coming back.
func TestTheComplianceIndexMarksEveryFigure(t *testing.T) {
	paths, _ := rfcDetailFiguresLedger(t)
	if _, err := renderRFCCompliance(paths); err != nil {
		t.Fatal(err)
	}
	page := readArtifact(t, paths.Output, rfcComplianceDest)
	mirror := readRFCProfileFile(t, paths.Output, rfcComplianceDirectory+"/index.md")

	want := map[string]string{
		"enrolled:rfcs": "5", "enrolled:not-walked": "1", "enrolled:walked": "4",
		"enrolled:must.total": "1,232", "enrolled:must.ze-proven": "18", "enrolled:must.ze-gap": "1,200",
		"enrolled:must.os-gap": "2", "enrolled:must.not-owed": "10",
		"all:rfcs": "7", "all:requirements": "1,332", "all:walked": "5", "all:not-walked": "2",
	}
	for marker, value := range want {
		subject, field, _ := strings.Cut(marker, ":")
		element := `<span data-ze-rfc="` + subject + ":" + field + `">` + value + `</span>`
		if !strings.Contains(page, element) {
			t.Errorf("the index does not carry %s", element)
		}
	}
	for _, line := range []string{"| MUST rows of the walked enrolled RFCs | 1,232 |", "| Enrolled RFCs | 5 |"} {
		if !strings.Contains(mirror, line) {
			t.Errorf("the mirror does not carry %q", line)
		}
	}
	for _, section := range []string{`id="rfc-enrolled-sections"`, `data-ze-rfc-bar="enrolled:must"`,
		`data-ze-rfc-bar="enrolled:overview"`, `id="rfc-enrolled-check"`} {
		if !strings.Contains(page, section) {
			t.Errorf("the index headline lacks %s", section)
		}
	}
	main := visibleText(mainContent(t, page))
	if found := regexp.MustCompile(`\d%`).FindString(main); found != "" {
		t.Errorf("the index still states a percentage (%q)", found)
	}
	for _, retired := range []string{"Gated MUSTs\n", "Proven by test", "Requirement buckets", "Share of gated",
		"Ceiling", "ceiling", "{{ze-rfc"} {
		if strings.Contains(main, retired) {
			t.Errorf("the index still shows %q", retired)
		}
		if strings.Contains(mirror, retired) {
			t.Errorf("the mirror still shows %q", retired)
		}
	}
}

// TestRFCDataCountsEveryRequirementRow proves rfcRequirementCounts counts
// each summary's requirement rows at every level, and refuses by id a row
// whose RFC has no figures rather than publishing a corpus total without it.
//
// Method: three rows over two stems, one SHOULD; then a row naming a stem the
// figures do not hold.
// PREVENTS: a level or an unwalked RFC left out of the corpus count.
func TestRFCDataCountsEveryRequirementRow(t *testing.T) {
	figures := map[string]rfc.StemFigures{"rfc1": {Stem: "rfc1", Walked: true}, "rfc2": {Stem: "rfc2"}}
	rows := []rfc.Requirement{{RFC: "rfc1", RID: "RFC1-1-1", Level: "MUST"},
		{RFC: "rfc1", RID: "RFC1-1-2", Level: "SHOULD"}, {RFC: "rfc2", RID: "RFC2-1-1", Level: "MUST"}}
	counts, err := rfcRequirementCounts(rows, figures)
	if err != nil {
		t.Fatal(err)
	}
	if counts["rfc1"] != 2 {
		t.Errorf("rfc1 counts %d rows, want 2", counts["rfc1"])
	}
	if counts["rfc2"] != 1 {
		t.Errorf("rfc2, not walked, counts %d rows, want 1", counts["rfc2"])
	}
	rows = append(rows, rfc.Requirement{RFC: "rfc9", RID: "RFC9-1-1", Level: "MUST"})
	_, err = rfcRequirementCounts(rows, figures)
	if err == nil {
		t.Fatal("a row of an RFC with no figures was counted in no file and not refused")
	}
	if !strings.Contains(err.Error(), "RFC9-1-1") {
		t.Errorf("the refusal does not name the row: %v", err)
	}
}

// TestRFCDataSaysWhoImplementsEachRFC proves each file's implemented flag is
// rfc.Implements: enrolled with an Implementation kind that counts, AND
// rendering a public row, the population the retired proof share counted.
//
// Method: four metas, each failing one condition but the first.
// PREVENTS: an enrolled summary with no public row, or one another layer
// implements, counted among the RFCs Ze implements.
func TestRFCDataSaysWhoImplementsEachRFC(t *testing.T) {
	for _, one := range []struct {
		name string
		meta rfc.Meta
		want bool
	}{
		{"enrolled ze with a row", rfc.Meta{Enrolment: "enrolled", Implementation: "ze", Row: true}, true},
		{"enrolled ze with no row", rfc.Meta{Enrolment: "enrolled", Implementation: "ze"}, false},
		{"enrolled third-party with a row", rfc.Meta{Enrolment: "enrolled", Implementation: "third-party", Row: true}, false},
		{"backlog mixed with a row", rfc.Meta{Enrolment: "backlog", Implementation: "mixed", Row: true}, false},
	} {
		var data rfcStemData
		rfcStemImplementer(&data, &one.meta)
		if data.Implemented != one.want {
			t.Errorf("%s: implemented is %v, want %v", one.name, data.Implemented, one.want)
		}
		if data.Implementer != one.meta.Implementation {
			t.Errorf("%s: implementer is %q, want %q", one.name, data.Implementer, one.meta.Implementation)
		}
	}
}

// TestTheIndexGatesOnlyWhatTheGateHolds proves index.rfcs[].gated is
// rfc.Meta.Enrolled and not the enrolment word: a summary whose enrolment row
// says enrolled but whose Implementation is third-party is never gated, so the
// enrolled subject, which both readers sum over the gated entries, leaves it
// out.
//
// Method: rfcDataOf over a corpus of two metas, both enrolled, one ze and one
// third-party, with no requirement rows and no extraction.
// PREVENTS: the enrolled sums counting an RFC another layer implements.
func TestTheIndexGatesOnlyWhatTheGateHolds(t *testing.T) {
	in := rfc.RenderInput{Deriver: rfc.NewDeriver(t.TempDir()), Metas: map[string]rfc.Meta{
		"rfc1": {Enrolment: "enrolled", Implementation: "ze", Row: true},
		"rfc2": {Enrolment: "enrolled", Implementation: "third-party", Row: true},
	}}
	data, err := rfcDataOf(&in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"rfc1": true, "rfc2": false}
	if len(data.index.RFCs) != len(want) {
		t.Fatalf("the index lists %d RFCs, want %d: %+v", len(data.index.RFCs), len(want), data.index.RFCs)
	}
	for _, entry := range data.index.RFCs {
		if entry.Enrolment != "enrolled" {
			t.Errorf("%s: enrolment is %q, want enrolled", entry.Stem, entry.Enrolment)
		}
		if entry.Gated != want[entry.Stem] {
			t.Errorf("%s: gated is %v, want %v", entry.Stem, entry.Gated, want[entry.Stem])
		}
	}
}
