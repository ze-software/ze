// Design: docs/functional-tests.md -- one composition for test-only plugins.
package all_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/test/plugins/all"
)

// TestFilterWireCasesRegisteredForBothTransports verifies the shared test
// composition exposes runnable filter declarations to config validation and DUTs.
func TestFilterWireCasesRegisteredForBothTransports(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"filter-wire-cases", "filter-wire-cases-forked"} {
		registration := registry.Lookup(name)
		if registration == nil {
			t.Errorf("test plugin composition does not register %q", name)
			continue
		}
		if registration.CLIHandler == nil {
			t.Errorf("test plugin %q has no CLI handler for forked execution", name)
		}
		if registration.RunEngine == nil {
			t.Errorf("test plugin %q has no engine handler for internal execution", name)
		}
		if len(registration.Filters) == 0 {
			t.Errorf("test plugin %q declares no filters for config validation", name)
		}
	}
}
