// VALIDATES: spec-rfc-priority-tree AC-5 -- a row of an updated RFC names the
// requirement of the updating RFC that rewrites it, and the check holds that
// pointer against the updater's own summary.
// PREVENTS: a pointer nobody can follow. An {updated-by} naming an id the
// updater does not declare, or an RFC that does not say it updates this one,
// would order a row with a document that never touched it.

package rfc

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// updatedByRow is a rfc9999 row carrying the marker under test.
func updatedByRow(marker string) string {
	return "- [ ] [RFC9999-6.3-7] [MUST] A speaker MUST reset the session (§6.3) " + marker
}

// TestAnUpdatedByMarkerParsesItsTargetAndItsReason parses one marked row and
// checks the marker leaves the requirement text, keeps its id and its reason,
// and composes with a coverage annotation and a {superseded} marker, one of
// each kind on the line.
func TestAnUpdatedByMarkerParsesItsTargetAndItsReason(t *testing.T) {
	req, err := oneLine(t, updatedByRow("{gap: no producer yet} "+
		"{updated-by: RFC10000-3.d-1; RFC 10000 Section 3.d replaces the reset} "+
		"{superseded: restated RFC10001-2-1; the successor states it}"))
	if err != nil {
		t.Fatalf("a row carrying all three marker kinds: %v", err)
	}
	if req.UpdatedBy == nil {
		t.Fatal("the {updated-by} marker was lost")
	}
	if req.UpdatedBy.Target != "RFC10000-3.d-1" {
		t.Errorf("target: %q", req.UpdatedBy.Target)
	}
	if req.UpdatedBy.Reason != "RFC 10000 Section 3.d replaces the reset" {
		t.Errorf("reason: %q", req.UpdatedBy.Reason)
	}
	if req.Annotation == nil || req.Superseded == nil {
		t.Errorf("a sibling marker was lost: annotation=%+v superseded=%+v", req.Annotation, req.Superseded)
	}
	if strings.Contains(req.Text, "{") {
		t.Errorf("a marker was left inside the requirement text: %q", req.Text)
	}
	if req.Section != "6.3" {
		t.Errorf("a marker left in the text moved the anchor: %q", req.Section)
	}
}

// TestAMalformedUpdatedByMarkerIsRefused walks every way the body can fail to
// name exactly one updater requirement with a reason, and a second marker on
// the same row, and checks each refusal says what is wrong.
func TestAMalformedUpdatedByMarkerIsRefused(t *testing.T) {
	cases := []struct {
		name, marker, refused string
	}{
		{name: "empty body", marker: "{updated-by: }", refused: "{updated-by} has an empty body"},
		{name: "no reason", marker: "{updated-by: RFC10000-3-1}", refused: "has no reason"},
		{name: "empty reason", marker: "{updated-by: RFC10000-3-1; }", refused: "has no reason"},
		{name: "no id", marker: "{updated-by: ; why}", refused: "needs exactly one requirement id"},
		{name: "two ids", marker: "{updated-by: RFC10000-3-1 RFC10000-3-2; why}", refused: "needs exactly one requirement id"},
		{name: "two ids with a comma", marker: "{updated-by: RFC10000-3-1, RFC10000-3-2; why}", refused: "needs exactly one requirement id"},
		{name: "not an id", marker: "{updated-by: RFC 10000; why}", refused: "needs exactly one requirement id"},
		{name: "malformed id", marker: "{updated-by: rfc10000; why}", refused: "is not a requirement id"},
		{
			name:    "two markers",
			marker:  "{updated-by: RFC10000-3-1; why} {updated-by: RFC10000-3-2; why}",
			refused: "two {updated-by} markers on one line",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			_, err := oneLine(t, updatedByRow(one.marker))
			if err == nil || !strings.Contains(err.Error(), one.refused) {
				t.Fatalf("want a refusal naming %q, got %v", one.refused, err)
			}
		})
	}
}

// TestTheUpdatesCellNamesTheStemsItUpdates reads every spelling the corpus
// uses for the Meta `Updates` cell, and refuses an item that names no RFC.
func TestTheUpdatesCellNamesTheStemsItUpdates(t *testing.T) {
	cases := []struct {
		cell    string
		want    []string
		refused string
	}{
		{cell: "RFC 1997, 4271, 4360", want: []string{"rfc1997", "rfc4271", "rfc4360"}},
		{cell: "RFC 4271 (verified errata 7915)", want: []string{"rfc4271"}},
		{cell: "RFC 2328 (OSPF Version 2), RFC 5709", want: []string{"rfc2328", "rfc5709"}},
		{cell: "1350", want: []string{"rfc1350"}},
		{cell: "RFC 777, RFC 760", want: []string{"rfc777", "rfc760"}},
		{cell: "-"},
		{cell: "None"},
		{cell: "None (updates two existing IANA OSPFv3 registries, see §9)"},
		{cell: "- (informationally extends RFC 1332; does not formally update it)"},
		{cell: "RFC 4271, the BGP spec", refused: "names no RFC"},
	}
	for _, one := range cases {
		t.Run(one.cell, func(t *testing.T) {
			meta, err := ParseMeta(selftestMeta+"| Updates | "+one.cell+" |\n", "rfc9999", "")
			if one.refused != "" {
				if err == nil || !strings.Contains(err.Error(), one.refused) {
					t.Fatalf("want a refusal naming %q, got %v", one.refused, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsing %q: %v", one.cell, err)
			}
			if strings.Join(meta.Updates, ",") != strings.Join(one.want, ",") {
				t.Errorf("Updates = %q, want %q", meta.Updates, one.want)
			}
		})
	}
}

// updatedByCorpus answers a two-summary corpus: rfc9999 carries one marked
// row, and rfc10000 is the updater with the row the marker may name. What the
// rfc10000 header updates is the caller's graph text, not a Meta cell.
func updatedByCorpus(target string) ([]Requirement, map[string]Meta) {
	requirements := []Requirement{
		{RFC: "rfc9999", RID: "RFC9999-6.3-7", Level: levelMust, Source: "rfc/short/rfc9999.md", Line: 7,
			UpdatedBy: &UpdatedBy{Target: target, Reason: "RFC 10000 Section 3 replaces the reset"}},
		{RFC: "rfc10000", RID: "RFC10000-3-1", Level: levelMust, Source: "rfc/short/rfc10000.md", Line: 9},
	}
	metas := map[string]Meta{
		"rfc9999":  {Title: "Widgets"},
		"rfc10000": {Title: "Widget errors"},
	}
	return requirements, metas
}

// TestCheckUpdatedByHoldsThePointerAgainstTheUpdater accepts a marker whose
// updater declares both the update and the id, and refuses each of the three
// ways the pointer can dangle, checking every refusal names the fix.
func TestCheckUpdatedByHoldsThePointerAgainstTheUpdater(t *testing.T) {
	cases := []struct {
		name, target string
		updates      []string
		dropUpdater  bool
		noText       bool
		refused      []string
	}{
		{name: "accepted", target: "RFC10000-3-1", updates: []string{"rfc4271", "rfc9999"}},
		{
			name: "the updater does not say it updates this RFC", target: "RFC10000-3-1",
			updates: []string{"rfc4271"},
			refused: []string{"RFC10000-3-1", "rfc/short/rfc10000.md", "Updates", "RFC 9999"},
		},
		{
			name: "the id is not a row of the updater", target: "RFC10000-3-9",
			updates: []string{"rfc9999"},
			refused: []string{"RFC10000-3-9", "rfc/short/rfc10000.md does not declare"},
		},
		{
			name: "the updater has no text, so no header", target: "RFC10000-3-1",
			updates: []string{"rfc9999"}, noText: true,
			refused: []string{"rfc/short/rfc10000.md has no text under rfc/full/", "curl -o rfc/full/rfc10000.txt"},
		},
		{
			name: "the updater summary is absent", target: "RFC10000-3-1",
			updates: []string{"rfc9999"}, dropUpdater: true,
			refused: []string{"rfc/short/rfc10000.md is not in this repository"},
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			requirements, metas := updatedByCorpus(one.target)
			if one.dropUpdater {
				delete(metas, "rfc10000")
				requirements = requirements[:1]
			}
			stems := map[string]bool{}
			for stem := range metas {
				stems[stem] = true
			}
			texts := map[string]graph.Text{"rfc9999": {}}
			if !one.noText {
				texts["rfc10000"] = graph.Text{Updates: one.updates}
			}
			errs := checkUpdatedBy(requirements, graph.Build(texts), stems)
			if len(one.refused) == 0 {
				if len(errs) != 0 {
					t.Fatalf("a well-formed pointer was refused: %q", errs)
				}
				return
			}
			if len(errs) != 1 {
				t.Fatalf("want one refusal, got %d: %q", len(errs), errs)
			}
			for _, want := range one.refused {
				if !strings.Contains(errs[0], want) {
					t.Errorf("refusal %q omits %q", errs[0], want)
				}
			}
		})
	}
}

// TestAnUpdatedByMarkerChangesNoCount parses one summary with and without the
// marker and compares every count the gate publishes for it. The marker orders
// a row; it never takes the row out of its own RFC's population.
func TestAnUpdatedByMarkerChangesNoCount(t *testing.T) {
	const head = "- [ ] [RFC9999-6.3-7] [MUST] A speaker MUST reset the session (§6.3)"
	const other = "\n- [ ] [RFC9999-6.3-8] [SHOULD] A speaker SHOULD log it (§6.3)\n"
	plain, err := ParseSummaryText(head+other, "rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatal(err)
	}
	marked, err := ParseSummaryText(head+" {updated-by: RFC10000-3-1; RFC 10000 Section 3 replaces it}"+other,
		"rfc9999", "rfc/short/rfc9999.md")
	if err != nil {
		t.Fatal(err)
	}
	if marked[0].UpdatedBy == nil {
		t.Fatal("the marker did not parse, so this test compares two identical populations")
	}
	if got, want := gatedCounts(marked)["rfc9999"], gatedCounts(plain)["rfc9999"]; got != want || got != 1 {
		t.Errorf("gated count: marked %d, plain %d, want 1 both", got, want)
	}
	enrolled := map[string]bool{"rfc9999": true}
	if got, want := len(evaluate(marked, nil, enrolled)), len(evaluate(plain, nil, enrolled)); got != want || got != 1 {
		t.Errorf("untested gated rows: marked %d, plain %d, want 1 both", got, want)
	}
	markedRows, plainRows := CoverageRows(marked, nil, nil), CoverageRows(plain, nil, nil)
	if len(markedRows) != 1 || len(plainRows) != 1 {
		t.Fatalf("coverage rows: marked %d, plain %d, want 1 both", len(markedRows), len(plainRows))
	}
	if markedRows[0] != plainRows[0] {
		t.Errorf("coverage row changed under the marker: marked %+v, plain %+v", markedRows[0], plainRows[0])
	}
}
