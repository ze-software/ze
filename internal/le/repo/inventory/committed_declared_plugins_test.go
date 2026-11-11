// Design: docs/architecture/plugin/feature-gates.md -- the declared list refuses an unreadable population.
package repoinventory

import "testing"

// TestDeclaredPluginsRefusesAnUnreadablePopulation keeps the list-only reader
// fail-closed: a root with no committed source answers an error and no list.
func TestDeclaredPluginsRefusesAnUnreadablePopulation(t *testing.T) {
	plugins, err := DeclaredPlugins(t.TempDir())
	if err == nil {
		t.Fatalf("an unreadable root answered %+v", plugins)
	}
	if plugins != nil {
		t.Fatalf("a refusal leaked a partial list: %+v", plugins)
	}
}
