//go:build ze_bgp

// VALIDATES: every command commandDecls names is also in the dispatch table, so
// the summary it publishes is the one the table states.
// PREVENTS: a declaration whose name and table key disagree. commandSummary
// answers "" for a name the table does not hold, and an empty description is
// indistinguishable from a command that states no summary, so the catalog would
// publish a blank line and nothing would go red (ai/rules/principles.md).

package rib

import "testing"

func TestEveryDeclaredCommandCarriesItsTableSummary(t *testing.T) {
	decls := commandDecls()
	if len(decls) == 0 {
		t.Fatal("the plugin declares no command")
	}
	for _, decl := range decls {
		if decl.Description != "" {
			continue
		}
		t.Errorf("%q publishes no summary, so the dispatch table does not name it", decl.Name)
	}
}
