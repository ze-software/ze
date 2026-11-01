//go:build ze_bgp

package cli

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	// The test-only plugins the functional ze links under the zetest tag
	// (cmd/ze/plugins_zetest.go). A .ci body configures them the way an
	// operator configures a shipped plugin, so the sweep validates against
	// the same composition the .ci run uses.
	_ "github.com/ze-software/ze/internal/test/plugins/all"
	"github.com/ze-software/ze/internal/test/plugins/fakedrain"
	"github.com/ze-software/ze/internal/test/plugins/fakeredist"
)

// withoutTestPlugins removes the test-only plugins the repository's .ci
// bodies configure from the plugin registry for the rest of t, and restores
// the registry when t ends. It is how TestSweepDerivesEachExclusion turns the
// test-plugin rule off: the composition is linked, so the rule can only be
// removed by removing its registrations.
func withoutTestPlugins(t *testing.T) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	kept := registry.All()
	registry.Reset()
	for _, reg := range kept {
		if reg.Name == fakedrain.Name || reg.Name == fakeredist.Name {
			continue
		}
		if err := registry.Register(*reg); err != nil {
			t.Fatalf("re-register %s: %v", reg.Name, err)
		}
	}
}
