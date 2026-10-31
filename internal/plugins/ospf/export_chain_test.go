package ospf

import (
	"strings"
	"testing"
)

// TestParseExportChain: `ospf filter export` compiles from the delivered ospf
// section, a config without it compiles to no chain, and a chain without a
// final is refused with the fix. Method: parse sections shaped as the engine
// delivers them (each root wrapped in its own name).
func TestParseExportChain(t *testing.T) {
	cfg, err := parseExportChain([]configSection{{Root: Namespace, Data: `{"ospf":{"filter":{"export":{"final":"reject"}}}}`}})
	if err != nil {
		t.Fatalf("export with final reject: %v", err)
	}
	if cfg.chain == nil {
		t.Fatal("export with final reject compiled to no chain")
	}

	cfg, err = parseExportChain([]configSection{{Root: Namespace, Data: `{"ospf":{"router-id":"10.0.0.1"}}`}})
	if err != nil {
		t.Fatalf("no export: %v", err)
	}
	if cfg.chain != nil {
		t.Fatal("a config with no export container compiled a chain")
	}

	_, err = parseExportChain([]configSection{{Root: Namespace, Data: `{"ospf":{"filter":{"export":{"filters":["loopbacks"]}}}}`}})
	if err == nil {
		t.Fatal("export without final compiled, want a refusal")
	}
	if !strings.Contains(err.Error(), "final accept") {
		t.Fatalf("refusal %q does not name the fix", err)
	}

	_, err = parseExportChain([]configSection{{Root: Namespace, Data: `{"ospf":{"filter":{"export":{"filters":["missing"],"final":"accept"}}}}`}})
	if err == nil {
		t.Fatal("export naming an undefined filter compiled, want a refusal")
	}
}
