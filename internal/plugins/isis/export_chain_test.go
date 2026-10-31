package isis

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
)

// TestParseExportChain: `isis filter export` compiles from the delivered isis
// section, a config without it compiles to no chain, and a chain without a
// final is refused with the fix. Method: parse sections shaped as the engine
// delivers them (each root wrapped in its own name).
func TestParseExportChain(t *testing.T) {
	cfg, err := parseExportChain([]configSection{{Root: configRoot, Data: `{"isis":{"filter":{"export":{"final":"reject"}}}}`}})
	if err != nil {
		t.Fatalf("export with final reject: %v", err)
	}
	if cfg.chain == nil {
		t.Fatal("export with final reject compiled to no chain")
	}

	cfg, err = parseExportChain([]configSection{{Root: configRoot, Data: `{"isis":{"level":"level-2"}}`}})
	if err != nil {
		t.Fatalf("no export: %v", err)
	}
	if cfg.chain != nil {
		t.Fatal("a config with no export container compiled a chain")
	}

	_, err = parseExportChain([]configSection{{Root: configRoot, Data: `{"isis":{"filter":{"export":{"filters":["loopbacks"]}}}}`}})
	if err == nil {
		t.Fatal("export without final compiled, want a refusal")
	}
	if !strings.Contains(err.Error(), "final accept") {
		t.Fatalf("refusal %q does not name the fix", err)
	}

	_, err = parseExportChain([]configSection{{Root: configRoot, Data: `{"isis":{"filter":{"export":{"filters":["missing"],"final":"accept"}}}}`}})
	if err == nil {
		t.Fatal("export naming an undefined filter compiled, want a refusal")
	}
}

// TestExportChainRefusesMatchLevel: the level a redistributed route takes is
// where IS-IS injects it, not a field the route arrives with, so a filter
// that matches on level at `isis filter export` is refused at load with the
// field it reads, instead of silently rejecting every route. Method: compile
// an export chain naming a `match` instance with a level condition.
func TestExportChainRefusesMatchLevel(t *testing.T) {
	_, err := parseExportChain([]configSection{
		{Root: configRoot, Data: `{"isis":{"filter":{"export":{"filters":["L2"],"final":"accept"}}}}`},
		{Root: policy.RootName, Data: `{"policy":{"match":{"L2":{"level":["2"]}}}}`},
	})
	if err == nil {
		t.Fatal("export chain matching level compiled, want a refusal")
	}
	if !strings.Contains(err.Error(), "level") {
		t.Fatalf("refusal %q does not name the level field", err)
	}
}

// TestExportChainMatchesTag: a redistributed route arrives with its own tag
// (a static route's tag leaf), so `isis filter export` carries the tag field
// and a match on it judges the route. Method: compile an export chain whose
// match passes only tag 7, with final accept, then evaluate a route tagged 7, one
// tagged 8 and an untagged one.
func TestExportChainMatchesTag(t *testing.T) {
	cfg, err := parseExportChain([]configSection{
		{Root: configRoot, Data: `{"isis":{"filter":{"export":{"filters":["T7"],"final":"accept"}}}}`},
		{Root: policy.RootName, Data: `{"policy":{"match":{"T7":{"tag":["7"]}}}}`},
	})
	if err != nil {
		t.Fatalf("export chain matching tag: %v", err)
	}
	if cfg.chain == nil {
		t.Fatal("export chain matching tag compiled to no chain")
	}
	prefix := netip.MustParsePrefix("192.0.2.1/32")
	cases := []struct {
		route ribpolicy.Route
		want  ribpolicy.Action
	}{
		{ribpolicy.Route{Prefix: prefix, Tag: 7, HasTag: true}, ribpolicy.ActionAccept},
		{ribpolicy.Route{Prefix: prefix, Tag: 8, HasTag: true}, ribpolicy.ActionReject},
		{ribpolicy.Route{Prefix: prefix}, ribpolicy.ActionReject},
	}
	for _, c := range cases {
		if got := cfg.chain.Evaluate(c.route).Action; got != c.want {
			t.Errorf("route tag %d (has %v): action %v, want %v", c.route.Tag, c.route.HasTag, got, c.want)
		}
	}
}
