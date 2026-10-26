package graph

// VALIDATES: edges, redirects, cycle components and levels (AC-1, AC-3).
// PREVENTS: a cycle or an obsoleted target mis-ordering the tree.

import (
	"slices"
	"strings"
	"testing"
)

// TestGraphRedirectsObsoleted holds AC-1: one edge per in-corpus relation, no
// RFC 2119 or 8174 edge, no self-edge, no edge out of the corpus, and a
// builds-on edge to an obsoleted document lands on its successor while the
// header edge keeps the document the header names.
// Method: a four-text corpus whose citations hit each case.
func TestGraphRedirectsObsoleted(t *testing.T) {
	g := Build(map[string]Text{
		"rfc4271": {Obsoletes: []string{"rfc1771"}, Cites: []string{"rfc2119", "rfc4271"}},
		"rfc1771": {},
		"rfc7606": {Updates: []string{"rfc4271", "rfc9999"}, Cites: []string{"rfc1771", "rfc8174", "rfc5555"}},
		"rfc9000": {Cites: []string{"rfc7606", "rfc7606"}},
	})
	want := []Edge{
		{From: "rfc4271", Kind: KindObsoletes, To: "rfc1771"},
		{From: "rfc7606", Kind: KindBuildsOn, To: "rfc4271"},
		{From: "rfc7606", Kind: KindUpdates, To: "rfc4271"},
		{From: "rfc9000", Kind: KindBuildsOn, To: "rfc7606"},
	}
	if got := g.Edges(); !slices.Equal(got, want) {
		t.Errorf("Edges = %v, want %v", got, want)
	}
	if got := g.Updaters("rfc4271"); !slices.Equal(got, []string{"rfc7606"}) {
		t.Errorf("Updaters(rfc4271) = %v", got)
	}
	if got, _ := g.HeaderUpdates("rfc7606"); !slices.Equal(got, []string{"rfc4271", "rfc9999"}) {
		t.Errorf("HeaderUpdates keeps out-of-corpus RFCs: got %v", got)
	}
	tsv := g.TSV()
	if !strings.Contains(tsv, "rfc7606\tupdates\trfc4271\n") || strings.Contains(tsv, "rfc2119") {
		t.Errorf("TSV rows wrong:\n%s", tsv)
	}
}

// TestGraphCycleSharesLevel holds AC-3: A and B cite each other, so they are
// one component at one level; C cites A, so C is one level above; D cites
// nothing in the corpus, so it is level 0.
func TestGraphCycleSharesLevel(t *testing.T) {
	g := Build(map[string]Text{
		"rfc1": {Cites: []string{"rfc2", "rfc4"}},
		"rfc2": {Cites: []string{"rfc1"}},
		"rfc3": {Cites: []string{"rfc1"}},
		"rfc4": {},
	})
	levels := map[string]int{}
	for _, stem := range g.Nodes() {
		level, ok := g.Level(stem)
		if !ok {
			t.Fatalf("no level for %s", stem)
		}
		levels[stem] = level
	}
	if levels["rfc4"] != 0 || levels["rfc1"] != 1 || levels["rfc2"] != 1 || levels["rfc3"] != 2 {
		t.Errorf("levels = %v, want rfc4 0, rfc1 and rfc2 1, rfc3 2", levels)
	}
	if members, _ := g.Component("rfc2"); !slices.Equal(members, []string{"rfc1", "rfc2"}) {
		t.Errorf("Component(rfc2) = %v", members)
	}
	if _, ok := g.Level("rfc77"); ok {
		t.Error("a stem outside the corpus answered a level")
	}
}
