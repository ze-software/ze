package rfc

// VALIDATES: `./le rfc tree` and `./le rfc next` over a small corpus whose
// summaries declare their use cases (AC-1, AC-4, AC-8, AC-13), and the status
// page's level order.
// PREVENTS: a member reached over a citation or an updates edge rather than
// declared, an optional member counted in the profile figure, an order that is
// not roots to leaves, a candidate counted or hidden.

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc/graph"
)

// priorityFixture is an eight-text corpus:
//
//	rfc1            level 0, updated by rfc4
//	rfc2 -> rfc1    level 1, updated by rfc7
//	rfc4 -> rfc1    level 1, updates rfc1
//	rfc5 <-> rfc6   level 2, a citation cycle; rfc5 -> rfc2; rfc6 has no summary
//	rfc7 -> rfc2    level 2, updates rfc2, listed by no profile
//	rfc3 -> rfc2, rfc5   level 3
//	rfc9            level 0
//
// Membership is declared by each summary's two use-case rows: edge (release)
// requires rfc1, rfc2, rfc3, rfc4 and rfc5 and lists rfc9 as optional; core
// (release) requires rfc2; lab (not release) requires rfc9. The aggregate is
// base. proven names the rows a recorded break proved; unwalked the summaries
// with no extraction sign-off.
func priorityFixture(t *testing.T, proven map[string]bool, unwalked ...string) *priorityCorpus {
	t.Helper()
	g := graph.Build(map[string]graph.Text{
		"rfc1": {},
		"rfc2": {Cites: []string{"rfc1"}},
		"rfc3": {Cites: []string{"rfc2", "rfc5"}},
		"rfc4": {Updates: []string{"rfc1"}, Cites: []string{"rfc1"}},
		"rfc5": {Cites: []string{"rfc6", "rfc2"}},
		"rfc6": {Cites: []string{"rfc5"}},
		"rfc7": {Updates: []string{"rfc2"}, Cites: []string{"rfc2"}},
		"rfc9": {},
	})
	profiles := Profiles{
		UseCases: map[string]UseCase{
			"edge": {Name: "edge", Title: "Edge", Release: true},
			"core": {Name: "core", Title: "Core", Release: true},
			"lab":  {Name: "lab", Title: "Lab"},
		},
		Aggregates: map[string]Aggregate{"base": {Name: "base"}},
	}
	declared := map[string][2][]string{
		"rfc1": {{"edge"}, nil},
		"rfc2": {{"core", "edge"}, nil},
		"rfc3": {{"edge"}, nil},
		"rfc4": {{"edge"}, nil},
		"rfc5": {{"edge"}, nil},
		"rfc7": {nil, nil},
		"rfc9": {{"lab"}, {"edge"}},
	}
	metas := map[string]Meta{}
	stems := map[string]bool{}
	for stem, tiers := range declared {
		metas[stem] = Meta{Title: "Title of " + stem, Aggregate: "base", UseCases: tiers[0], OptionalUseCases: tiers[1]}
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
		{RFC: "rfc7", RID: "RFC7-1-1", Level: "MUST"},
		{RFC: "rfc9", RID: "RFC9-1-1", Level: "MUST"},
		{RFC: "rfc9", RID: "RFC9-1-2", Level: "SHOULD"},
	}
	walked := map[string]bool{}
	for stem := range stems {
		walked[stem] = !slices.Contains(unwalked, stem)
	}
	figures := figuresByStem(requirements, stems, walked, proven, figureEvidence{})
	return newPriorityCorpus(g, profiles, metas, requirements, figures, proven)
}

// rowStems answers the stems of rows, in order.
func rowStems(rows []*TreeRow) []string {
	out := []string{}
	for _, row := range rows {
		out = append(out, row.Stem)
	}
	return out
}

// TestTreeActionRendersLevelsAndFigures: a profile's tree is its declared
// required members, roots to leaves, each row with stem, title, level,
// aggregate, edges, both figures, walked, and the declaration that put it in;
// the JSON carries every field. Method: the fixture corpus, profile edge.
func TestTreeActionRendersLevelsAndFigures(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{"RFC1-1-1": true, "RFC2-1-1": true}, "rfc5")
	report, err := corpus.tree("edge")
	if err != nil {
		t.Fatalf("tree edge: %v", err)
	}
	if want := []string{"rfc1", "rfc2", "rfc4", "rfc5", "rfc3"}; !slices.Equal(rowStems(report.Rows), want) {
		t.Fatalf("rows = %v, want %v (roots to leaves)", rowStems(report.Rows), want)
	}
	byStem := map[string]*TreeRow{}
	for _, row := range report.Rows {
		byStem[row.Stem] = row
	}
	one, two, four, five := byStem["rfc1"], byStem["rfc2"], byStem["rfc4"], byStem["rfc5"]
	if *one.Level != 0 || *two.Level != 1 || *five.Level != 2 || *byStem["rfc3"].Level != 3 {
		t.Errorf("levels rfc1 %d rfc2 %d rfc5 %d rfc3 %d, want 0 1 2 3",
			*one.Level, *two.Level, *five.Level, *byStem["rfc3"].Level)
	}
	if !slices.Equal(one.UpdatedBy, []string{"rfc4"}) || !slices.Equal(four.BuildsOn, []string{"rfc1"}) {
		t.Errorf("rfc1 updated-by %v, rfc4 builds-on %v", one.UpdatedBy, four.BuildsOn)
	}
	if want := "edge: required by rfc/short/rfc4.md"; four.Path != want {
		t.Errorf("rfc4 path %q, want %q", four.Path, want)
	}
	if !slices.Equal(five.Component, []string{"rfc6"}) {
		t.Errorf("rfc5 component %v, want [rfc6]", five.Component)
	}
	if one.Mandatory.Proven != 1 || one.Mandatory.Total != 2 || !one.Walked || one.Aggregate != "base" || one.Title != "Title of rfc1" {
		t.Errorf("rfc1 row %+v", one)
	}
	if five.Walked {
		t.Errorf("rfc5 walked %v, want not walked", five.Walked)
	}
	if !slices.Equal(two.Profiles, []string{"core", "edge"}) {
		t.Errorf("rfc2 profiles %v, want [core edge]", two.Profiles)
	}
	rollup := report.Rollup
	if rollup.RFCs != 5 || rollup.NotWalked != 1 || rollup.MandatoryComplete {
		t.Errorf("rollup %+v, want 5 RFCs, 1 not walked (rfc5), incomplete", rollup)
	}

	text := report.Text()
	for _, want := range []string{"L0 rfc1", "L2 rfc5 (cycle with rfc6)  not walked",
		"L0 rfc1  MUST 2 kept: Ze proven 1, OS verified 0, OS testing gap 0, Ze correctness gap 1",
		"edge (release): MUST 6 kept: Ze proven 2",
		"via edge: required by rfc/short/rfc1.md"} {
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
		`"optional-total":`, `"sections":`, `"walked":`, `"not-walked":`, `"mandatory-complete":`, `"optional":`,
		`"candidates":`, `"optional-profiles":`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("json lacks %s", key)
		}
	}

	// A text no summary covers (rfc6) is still a row of the whole-corpus
	// tree, rendered "no summary" rather than as a figure.
	whole, err := corpus.tree("")
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	for _, row := range whole.Rows {
		if row.Stem == "rfc6" && row.Summary {
			t.Error("rfc6 has no summary, but its whole-corpus row says it does")
		}
	}
	if want := "rfc6 (cycle with rfc5)  no summary"; !strings.Contains(whole.Text(), want) {
		t.Errorf("whole tree text lacks %q:\n%s", want, whole.Text())
	}

	if _, err := corpus.tree("nowhere"); err == nil || !strings.Contains(err.Error(), "edge") {
		t.Errorf("tree nowhere: %v, want a refusal naming the valid profiles", err)
	}
}

// TestTreeProfileMembersAreDeclared: AC-1, AC-4 and AC-13. Core requires rfc2
// alone; rfc2 cites rfc1 and rfc7 updates rfc2 with no marker, and neither is
// listed for core, so neither is a member in either tier, in the rollup or in
// the members another package reads. The whole-corpus tree puts rows a release
// profile requires first and names both tiers on each row. Method: the fixture.
func TestTreeProfileMembersAreDeclared(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{})
	report, err := corpus.tree("core")
	if err != nil {
		t.Fatalf("tree core: %v", err)
	}
	if got := rowStems(report.Rows); !slices.Equal(got, []string{"rfc2"}) {
		t.Fatalf("core rows %v, want [rfc2]: rfc1 and rfc7 are only cited or updating", got)
	}
	if want := "core: required by rfc/short/rfc2.md"; report.Rows[0].Path != want {
		t.Errorf("rfc2 path %q, want %q", report.Rows[0].Path, want)
	}
	if report.Rollup.RFCs != 1 || len(report.Optional.Rows) != 0 || report.Optional.Rollup.RFCs != 0 {
		t.Errorf("core rollup %+v optional %+v, want 1 RFC and no optional member", report.Rollup, report.Optional)
	}
	members := requiredMembers(corpus.profiles, corpus.metas)
	optional := useCaseTiers(corpus.profiles, corpus.metas)["core"].optional
	if got := members["core"]; !slices.Equal(got, []string{"rfc2"}) || len(optional) != 0 {
		t.Errorf("core required %v optional %v, want required [rfc2] and no optional member", got, sortedSet(optional))
	}

	edge, err := corpus.tree("edge")
	if err != nil {
		t.Fatalf("tree edge: %v", err)
	}
	if got := rowStems(edge.Optional.Rows); !slices.Equal(got, []string{"rfc9"}) {
		t.Fatalf("edge optional rows %v, want [rfc9]", got)
	}
	if want := "edge: optional in rfc/short/rfc9.md"; edge.Optional.Rows[0].Path != want {
		t.Errorf("rfc9 optional path %q, want %q", edge.Optional.Rows[0].Path, want)
	}

	whole, err := corpus.tree("")
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	want := []string{"rfc1", "rfc2", "rfc4", "rfc5", "rfc3", "rfc9", "rfc6", "rfc7"}
	if got := rowStems(whole.Rows); !slices.Equal(got, want) {
		t.Errorf("whole tree %v, want %v: rows a release profile requires first", got, want)
	}
	for _, row := range whole.Rows {
		if row.Stem != "rfc9" {
			continue
		}
		if !slices.Equal(row.Profiles, []string{"lab"}) || !slices.Equal(row.OptionalProfiles, []string{"edge"}) {
			t.Errorf("rfc9 profiles %v optional-profiles %v, want [lab] and [edge]", row.Profiles, row.OptionalProfiles)
		}
	}
}

// TestTreeOptionalTierNotCounted: AC-8. The required figure, RFC count and
// completeness are identical with and without optional members, and the
// optional section carries its own rollup. Method: tree edge, then the same
// corpus with rfc9's optional listing removed.
func TestTreeOptionalTierNotCounted(t *testing.T) {
	proven := map[string]bool{"RFC1-1-1": true, "RFC1-2-1": true, "RFC2-1-1": true, "RFC2-1-2": true,
		"RFC3-1-1": true, "RFC4-1-1": true, "RFC5-1-1": true}
	with := priorityFixture(t, proven)
	withReport, err := with.tree("edge")
	if err != nil {
		t.Fatalf("tree edge: %v", err)
	}
	without := priorityFixture(t, proven)
	meta := without.metas["rfc9"]
	meta.OptionalUseCases = nil
	without.metas["rfc9"] = meta
	withoutReport, err := without.tree("edge")
	if err != nil {
		t.Fatalf("tree edge: %v", err)
	}
	if !reflect.DeepEqual(withReport.Rollup, withoutReport.Rollup) {
		t.Errorf("required rollup %+v with optional members, %+v without", *withReport.Rollup, *withoutReport.Rollup)
	}
	if !withReport.Rollup.MandatoryComplete {
		t.Errorf("required rollup %+v, want complete: every required row is proven", withReport.Rollup)
	}
	optional := withReport.Optional.Rollup
	if optional.RFCs != 1 || !reflect.DeepEqual(optional.Mandatory, Figure{Proven: 0, Total: 1, Attainable: 1}) || optional.MandatoryComplete {
		t.Errorf("optional rollup %+v, want rfc9 alone, 0 of 1, incomplete", optional)
	}
	text := withReport.Text()
	for _, want := range []string{"; mandatory complete",
		"optional members, not counted: MUST 1 kept: Ze proven 0, OS verified 0, OS testing gap 0, Ze correctness gap 1",
		"edge: optional in rfc/short/rfc9.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

// TestAggregateOptionalTier: a summary whose `Aggregate tier` is optional
// leaves its aggregate's required tier and joins its optional one, so its rows
// count in the optional figure alone. Method: tree base with every summary
// required, then with rfc9 optional; the required rollup loses rfc9's one
// mandatory row and the optional rollup carries it.
func TestAggregateOptionalTier(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{})
	before, err := corpus.tree("base")
	if err != nil {
		t.Fatalf("tree base: %v", err)
	}
	meta := corpus.metas["rfc9"]
	meta.AggregateOptional = true
	corpus.metas["rfc9"] = meta
	after, err := corpus.tree("base")
	if err != nil {
		t.Fatalf("tree base: %v", err)
	}
	if slices.Contains(rowStems(after.Rows), "rfc9") {
		t.Errorf("required rows %v still hold the optional rfc9", rowStems(after.Rows))
	}
	if got := rowStems(after.Optional.Rows); !slices.Equal(got, []string{"rfc9"}) {
		t.Fatalf("optional rows %v, want [rfc9]", got)
	}
	if want := "base: optional in rfc/short/rfc9.md"; after.Optional.Rows[0].Path != want {
		t.Errorf("rfc9 optional path %q, want %q", after.Optional.Rows[0].Path, want)
	}
	if after.Rollup.RFCs != before.Rollup.RFCs-1 ||
		after.Rollup.Mandatory.Total != before.Rollup.Mandatory.Total-1 {
		t.Errorf("required rollup %+v after, %+v before: want rfc9's RFC and its one MUST row gone",
			*after.Rollup, *before.Rollup)
	}
	optional := after.Optional.Rollup
	if optional.RFCs != 1 || optional.Mandatory.Total != 1 {
		t.Errorf("optional rollup %+v, want rfc9 alone with its one MUST row", optional)
	}
}

// TestTreeProfileListsCandidates: AC-4's candidates. `tree profile` lists each
// one-hop builds-on target and updater of a member that no tier lists, once,
// with the member and edge that name it, by level then number, and counts
// none of them. Method: edge, whose rfc5 cites rfc6 and whose rfc2 rfc7
// updates.
func TestTreeProfileListsCandidates(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{})
	report, err := corpus.tree("edge")
	if err != nil {
		t.Fatalf("tree edge: %v", err)
	}
	var got []string
	for _, candidate := range report.Candidates {
		got = append(got, candidate.Stem+" via "+candidate.Via)
	}
	if want := []string{"rfc6 via rfc5 builds-on rfc6", "rfc7 via rfc2 updated-by rfc7"}; !slices.Equal(got, want) {
		t.Errorf("candidates %v, want %v", got, want)
	}
	if report.Rollup.RFCs != 5 {
		t.Errorf("rollup counts %d RFCs, want 5: a candidate is never counted", report.Rollup.RFCs)
	}
	text := report.Text()
	for _, want := range []string{"candidates, not owed", "L2 rfc6  via rfc5 builds-on rfc6"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	aggregate, err := corpus.tree("base")
	if err != nil {
		t.Fatalf("tree base: %v", err)
	}
	if len(aggregate.Candidates) != 0 {
		t.Errorf("aggregate candidates %v: only a use-case profile lists them", aggregate.Candidates)
	}
}

// TestStatusPageOrdersByLevel holds AC-6's order: inside one aggregate the
// rows go by level, then RFC number, and a summary with no text sorts last.
// Method: rfc2 cites rfc10, so rfc10 (level 0) precedes rfc2 (level 1)
// although 2 < 10; rfc3 has no text.
func TestStatusPageOrdersByLevel(t *testing.T) {
	metas := map[string]Meta{
		"rfc2":  {Aggregate: "base", Row: true},
		"rfc3":  {Aggregate: "base", Row: true},
		"rfc10": {Aggregate: "base", Row: true},
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

// TestTreeRowHoldsStemFiguresWhole guards the row against dropping a count.
// Goal: every StemFigures field figuresByStem answers reaches the TreeRow
// `./le rfc tree` builds, and the published keys survive a JSON round trip.
// Method: a StemFigures with every field set (checked by reflection, so a
// field added later fails here until the fixture sets it) goes through
// treeRow; the row must hold it unchanged, then decode from its own JSON with
// every published count intact.
func TestTreeRowHoldsStemFiguresWhole(t *testing.T) {
	want := StemFigures{Stem: "rfc1", Walked: true,
		Mandatory:       Figure{Proven: 1, Total: 4, Attainable: 3, Excluded: map[string]int{AnnotationLinuxLimit: 1}},
		Optional:        Figure{Proven: 2, Total: 5, Attainable: 4, Excluded: map[string]int{AnnotationLowerLayer: 1}},
		MandatoryByKind: map[string]int{"functional": 1}, Unmet: []string{"RFC1-1-2"},
		OSVerified: 1, OSGap: 2, NotYet: 3, NotEver: 4, NotDecided: 5, OptionalOSVerified: 6, OptionalOSGap: 7}
	fields := reflect.ValueOf(want)
	for index := range fields.NumField() {
		if fields.Field(index).IsZero() {
			t.Fatalf("fixture leaves StemFigures.%s zero: set it so this test can see it dropped",
				fields.Type().Field(index).Name)
		}
	}
	corpus := newPriorityCorpus(graph.Build(map[string]graph.Text{"rfc1": {}}), Profiles{},
		map[string]Meta{"rfc1": {Title: "Title of rfc1"}}, nil, map[string]StemFigures{"rfc1": want}, nil)
	row := corpus.treeRow("rfc1", nil)
	if !reflect.DeepEqual(row.StemFigures, want) {
		t.Fatalf("tree row figures = %+v, want %+v", row.StemFigures, want)
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var decoded TreeRow
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	// The OS and intent splits and the figures' Attainable and Excluded are
	// published only inside the sections, so the decoded figures hold them
	// zero and the decoded sections hold them whole.
	published := want
	published.OSVerified, published.OSGap, published.NotYet, published.NotEver, published.NotDecided = 0, 0, 0, 0, 0
	published.OptionalOSVerified, published.OptionalOSGap = 0, 0
	published.Mandatory = Figure{Proven: want.Mandatory.Proven, Total: want.Mandatory.Total}
	published.Optional = Figure{Proven: want.Optional.Proven, Total: want.Optional.Total}
	if !reflect.DeepEqual(decoded.StemFigures, published) {
		t.Errorf("decoded figures = %+v, want %+v from %s", decoded.StemFigures, published, encoded)
	}
	wantSections := sectionsOf(&want)
	if row.Sections == nil || decoded.Sections == nil || !reflect.DeepEqual(*decoded.Sections, wantSections) {
		t.Errorf("decoded sections = %+v, want %+v from %s", decoded.Sections, wantSections, encoded)
	}
	if decoded.Stem != "rfc1" || decoded.Title != "Title of rfc1" || !decoded.Summary {
		t.Errorf("decoded row lost its own keys: %+v", decoded)
	}
}
