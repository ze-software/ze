package rfc

// VALIDATES: `./le rfc tree` and `./le rfc next` over a small corpus (AC-10,
// AC-11, AC-12, AC-13, A-4), and the status page's level order (AC-6).
// PREVENTS: a closure that misses an updater, an order that is not roots to
// leaves, a gap named at the wrong level, a mandatory figure read complete
// while an RFC is absent or unwalked.

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// priorityFixture is a seven-text corpus:
//
//	rfc1            level 0, updated by rfc4
//	rfc2 -> rfc1    level 1
//	rfc4 -> rfc1    level 1, updates rfc1
//	rfc5 <-> rfc6   level 2, a citation cycle; rfc5 -> rfc2; rfc6 has no summary
//	rfc3 -> rfc2, rfc5   level 3
//	rfc9            level 0, outside every closure but lab's
//
// Profiles: edge (release, root rfc3, absent RFC 7948), core (release, root
// rfc2), lab (not release, root rfc9), far (not release, root rfc9, absent
// RFC 8212), and the aggregate base. proven names the rows a recorded break
// proved; unwalked the summaries with no extraction sign-off.
func priorityFixture(t *testing.T, proven map[string]bool, unwalked ...string) *priorityCorpus {
	t.Helper()
	g := graph.Build(map[string]graph.Text{
		"rfc1": {},
		"rfc2": {Cites: []string{"rfc1"}},
		"rfc3": {Cites: []string{"rfc2", "rfc5"}},
		"rfc4": {Updates: []string{"rfc1"}, Cites: []string{"rfc1"}},
		"rfc5": {Cites: []string{"rfc6", "rfc2"}},
		"rfc6": {Cites: []string{"rfc5"}},
		"rfc9": {},
	})
	profiles := Profiles{
		UseCases: map[string]UseCase{
			"edge": {Name: "edge", Release: true, Roots: []string{"rfc3"}, Absent: []int{7948}},
			"core": {Name: "core", Release: true, Roots: []string{"rfc2"}},
			"lab":  {Name: "lab", Roots: []string{"rfc9"}},
			"far":  {Name: "far", Roots: []string{"rfc9"}, Absent: []int{8212}},
		},
		Aggregates: map[string]Aggregate{"base": {Name: "base"}},
	}
	metas := map[string]Meta{}
	stems := map[string]bool{}
	for _, stem := range []string{"rfc1", "rfc2", "rfc3", "rfc4", "rfc5", "rfc9"} {
		metas[stem] = Meta{Title: "Title of " + stem, Aggregate: "base"}
		stems[stem] = true
	}
	requirements := []Requirement{
		{RFC: "rfc1", RID: "RFC1-1-1", Level: "MUST"},
		{RFC: "rfc1", RID: "RFC1-2-1", Level: "MUST", UpdatedBy: &UpdatedBy{Target: "RFC4-1-1", Reason: "rewritten"}},
		{RFC: "rfc2", RID: "RFC2-1-1", Level: "MUST"},
		{RFC: "rfc2", RID: "RFC2-1-2", Level: "MUST"},
		{RFC: "rfc3", RID: "RFC3-1-1", Level: "MUST"},
		{RFC: "rfc4", RID: "RFC4-1-1", Level: "MUST"},
		{RFC: "rfc5", RID: "RFC5-1-1", Level: "MUST"},
		{RFC: "rfc9", RID: "RFC9-1-1", Level: "MUST"},
		{RFC: "rfc9", RID: "RFC9-1-2", Level: "SHOULD"},
	}
	walked := map[string]bool{}
	for stem := range stems {
		walked[stem] = !slices.Contains(unwalked, stem)
	}
	figures := figuresByStem(requirements, stems, walked, proven)
	return newPriorityCorpus(g, profiles, metas, requirements, figures, proven)
}

// TestTreeActionRendersLevelsAndFigures holds AC-13: a profile's tree is its
// closure over builds-on and updated-by edges, roots to leaves, each row with
// stem, title, level, aggregate, edges, both figures, walked, and the path
// that put it in; absent RFCs come last at 0%; the JSON carries every field.
// Method: the fixture corpus, profile edge.
func TestTreeActionRendersLevelsAndFigures(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{"RFC1-1-1": true, "RFC2-1-1": true}, "rfc5")
	report, err := corpus.tree("edge")
	if err != nil {
		t.Fatalf("tree edge: %v", err)
	}
	var order []string
	for _, row := range report.Rows {
		order = append(order, row.Stem)
	}
	if want := []string{"rfc1", "rfc2", "rfc4", "rfc5", "rfc6", "rfc3", "rfc7948"}; !slices.Equal(order, want) {
		t.Fatalf("rows = %v, want %v (roots to leaves, absent last)", order, want)
	}
	byStem := map[string]*TreeRow{}
	for _, row := range report.Rows {
		byStem[row.Stem] = row
	}
	one, two, four, five, six := byStem["rfc1"], byStem["rfc2"], byStem["rfc4"], byStem["rfc5"], byStem["rfc6"]
	if *one.Level != 0 || *two.Level != 1 || *five.Level != 2 || *byStem["rfc3"].Level != 3 {
		t.Errorf("levels rfc1 %d rfc2 %d rfc5 %d rfc3 %d, want 0 1 2 3",
			*one.Level, *two.Level, *five.Level, *byStem["rfc3"].Level)
	}
	if !slices.Equal(one.UpdatedBy, []string{"rfc4"}) || !slices.Equal(four.BuildsOn, []string{"rfc1"}) {
		t.Errorf("rfc1 updated-by %v, rfc4 builds-on %v", one.UpdatedBy, four.BuildsOn)
	}
	if want := "edge: rfc3 builds-on rfc2 builds-on rfc1 updated-by rfc4"; four.Path != want {
		t.Errorf("rfc4 path %q, want %q", four.Path, want)
	}
	if want := "edge: rfc3 (root)"; byStem["rfc3"].Path != want {
		t.Errorf("rfc3 path %q, want %q", byStem["rfc3"].Path, want)
	}
	if !slices.Equal(five.Component, []string{"rfc6"}) {
		t.Errorf("rfc5 component %v, want [rfc6]", five.Component)
	}
	if one.MandatoryProven != 1 || one.MandatoryTotal != 2 || !one.Walked || one.Aggregate != "base" || one.Title != "Title of rfc1" {
		t.Errorf("rfc1 row %+v", one)
	}
	if five.Walked || six.Summary || !byStem["rfc7948"].Absent {
		t.Errorf("rfc5 walked %v, rfc6 summary %v, rfc7948 absent %v", five.Walked, six.Summary, byStem["rfc7948"].Absent)
	}
	if !slices.Equal(two.Profiles, []string{"core", "edge"}) {
		t.Errorf("rfc2 profiles %v, want [core edge]", two.Profiles)
	}
	rollup := report.Rollup
	if rollup.RFCs != 7 || rollup.Absent != 1 || rollup.NotWalked != 2 || rollup.MandatoryComplete {
		t.Errorf("rollup %+v, want 7 RFCs, 1 absent, 2 not walked (rfc5, rfc6), incomplete", rollup)
	}

	text := report.Text()
	for _, want := range []string{"L0 rfc1", "L2 rfc5 (cycle with rfc6)", "mandatory not walked",
		"mandatory no summary", "L? rfc7948  mandatory 0% (absent from the corpus)"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	for _, key := range []string{`"stem":`, `"title":`, `"level":`, `"aggregate":`, `"builds-on":`,
		`"updated-by":`, `"mandatory-proven":`, `"mandatory-total":`, `"optional-proven":`,
		`"optional-total":`, `"walked":`, `"not-walked":`, `"mandatory-complete":`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("json lacks %s", key)
		}
	}

	if _, err := corpus.tree("nowhere"); err == nil || !strings.Contains(err.Error(), "edge") {
		t.Errorf("tree nowhere: %v, want a refusal naming the valid profiles", err)
	}
	whole, err := corpus.tree("")
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if last := whole.Rows[len(whole.Rows)-1].Stem; last != "rfc9" {
		t.Errorf("whole tree ends at %s, want rfc9: no release profile reaches it", last)
	}
}

// TestStatusPageOrdersByLevel holds AC-6's order: inside one aggregate the
// rows go by level, then RFC number, and a summary with no text sorts last.
// Method: rfc2 cites rfc10, so rfc10 (level 0) precedes rfc2 (level 1)
// although 2 < 10; rfc3 has no text.
func TestStatusPageOrdersByLevel(t *testing.T) {
	metas := map[string]Meta{
		"rfc2":  {Aggregate: "base", Status: "Supported", Row: true},
		"rfc3":  {Aggregate: "base", Status: "Supported", Row: true},
		"rfc10": {Aggregate: "base", Status: "Supported", Row: true},
	}
	aggregates := map[string]Aggregate{"base": {Name: "base", Heading: "Base", Shape: ShapeWide}}
	levels := graph.Build(map[string]graph.Text{"rfc2": {Cites: []string{"rfc10"}}, "rfc10": {}})
	placed, err := placeRows(metas, aggregates, levels)
	if err != nil {
		t.Fatalf("placeRows: %v", err)
	}
	var order []string
	for _, row := range placed["base"] {
		order = append(order, row.Stem)
	}
	if want := []string{"rfc10", "rfc2", "rfc3"}; !slices.Equal(order, want) {
		t.Errorf("order %v, want %v", order, want)
	}
	if _, err := placeRows(metas, aggregates, nil); err == nil {
		t.Error("placeRows with no graph ordered the page by number alone")
	}
}
