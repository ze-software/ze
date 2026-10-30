package sysrib

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

type candidateChain func(r ribpolicy.Route) ribpolicy.Verdict

func (f candidateChain) Evaluate(r ribpolicy.Route) ribpolicy.Verdict { return f(r) } //nolint:gocritic // hugeParam: the Chain signature passes Route by value

// TestShowRIBCandidatesListsHiddenPaths is the `show rib candidates` contract:
// a prefix the central chain hides still appears, with every source's path
// marked hidden by the central chain and named by its attachment, a selected
// prefix shows its selected path, a tagged path shows its tag (zero included),
// the `prefix` selector narrows the answer to one prefix, and the JSON keys are
// kebab-case.
func TestShowRIBCandidatesListsHiddenPaths(t *testing.T) {
	static := redistevents.RegisterProtocol("static")
	bogon := netip.MustParsePrefix("192.0.2.0/24")
	kept := netip.MustParsePrefix("10.70.0.0/24")
	snap, err := ribpolicy.NewSnapshot(nil, candidateChain(func(r ribpolicy.Route) ribpolicy.Verdict {
		if r.Prefix == bogon {
			return ribpolicy.Verdict{Action: ribpolicy.ActionReject}
		}
		return ribpolicy.Verdict{Action: ribpolicy.ActionAccept}
	}))
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	ribpolicy.Set(snap)
	t.Cleanup(func() { ribpolicy.Set(nil) })

	loc := locrib.NewRIB()
	loc.Insert(family.IPv4Unicast, bogon, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.1")})
	loc.Insert(family.IPv4Unicast, kept, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.1"), HasTag: true})

	data, err := showRIBCandidates(loc, nil)
	if err != nil {
		t.Fatalf("show rib candidates: %v", err)
	}
	entries, ok := data.([]candidateEntry)
	if !ok {
		t.Fatalf("answer is %T, want []candidateEntry", data)
	}
	if len(entries) != 2 {
		t.Fatalf("answer lists %d prefixes, want 2 (the hidden one included): %+v", len(entries), entries)
	}
	hidden := entries[1].Paths
	if entries[1].Prefix != bogon || len(hidden) != 1 {
		t.Fatalf("second entry %+v, want %s with one path", entries[1], bogon)
	}
	if !hidden[0].Hidden || hidden[0].Selected {
		t.Fatalf("bogon path %+v: want hidden and not selected", hidden[0])
	}
	if hidden[0].HiddenBy != "central" || hidden[0].Chain != "rib import" {
		t.Fatalf("bogon path hidden by %q chain %q, want central / rib import", hidden[0].HiddenBy, hidden[0].Chain)
	}
	if hidden[0].Source != "static" {
		t.Fatalf("bogon path source %q, want static", hidden[0].Source)
	}
	sel := entries[0].Paths
	if entries[0].Prefix != kept || len(sel) != 1 || !sel[0].Selected || sel[0].Hidden {
		t.Fatalf("first entry %+v, want %s with its one path selected", entries[0], kept)
	}
	if sel[0].Tag == nil || *sel[0].Tag != 0 {
		t.Fatalf("tag %v, want a present tag 0: zero is a real tag", sel[0].Tag)
	}
	if sel[0].HiddenBy != "" || sel[0].Chain != "" {
		t.Fatalf("selected path names hiding chain %q/%q", sel[0].HiddenBy, sel[0].Chain)
	}

	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"hidden-by":"central"`, `"chain":"rib import"`, `"next-hop":"192.0.2.1"`, `"selected":true`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("JSON %s lacks %s", raw, key)
		}
	}

	one, err := showRIBCandidates(loc, []string{"prefix", "192.0.2.0/24"})
	if err != nil {
		t.Fatalf("show rib candidates prefix: %v", err)
	}
	got, ok := one.([]candidateEntry)
	if !ok {
		t.Fatalf("answer is %T, want []candidateEntry", one)
	}
	if len(got) != 1 || got[0].Prefix != bogon {
		t.Fatalf("prefix selector answered %+v, want only %s", got, bogon)
	}
}

// TestShowRIBCandidatesRefusals proves the grammar refuses what it cannot
// answer, and that a process with no Loc-RIB says so rather than answer an
// empty list.
func TestShowRIBCandidatesRefusals(t *testing.T) {
	loc := locrib.NewRIB()
	// Each refusal also names its own reason, so a case refused by the wrong
	// check (a bare value caught only by the arity check) is red.
	cases := map[string]struct {
		args   []string
		reason string
	}{
		"bare value":       {[]string{"10.0.0.0/24"}, "unknown argument 10.0.0.0/24"},
		"missing value":    {[]string{"prefix"}, "takes exactly one prefix"},
		"not a prefix":     {[]string{"prefix", "10.0.0.1"}, "10.0.0.1 is not a prefix"},
		"trailing garbage": {[]string{"prefix", "10.0.0.0/24", "extra"}, "takes exactly one prefix"},
	}
	for name, tc := range cases {
		_, err := showRIBCandidates(loc, tc.args)
		if err == nil {
			t.Errorf("%s %v: accepted", name, tc.args)
			continue
		}
		if !strings.Contains(err.Error(), "show rib candidates prefix 10.0.0.0/24") {
			t.Errorf("%s: error %q does not show the correct form", name, err)
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("%s: error %q does not give the reason %q", name, err, tc.reason)
		}
	}
	if _, err := showRIBCandidates(nil, nil); err == nil {
		t.Error("no Loc-RIB answered without an error")
	}
}
