package isis

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
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
