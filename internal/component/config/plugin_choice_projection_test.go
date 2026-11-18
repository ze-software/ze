//go:build ze_bgp

// Design: docs/architecture/config/yang-config-design.md -- internal plugin completion.
package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy"
	_ "github.com/ze-software/ze/internal/component/bgp/reactor/filter"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	_ "github.com/ze-software/ze/internal/plugins/crashes"
)

// TestInternalPluginCompletionOmitsNonSelectableRegistrations asks the real
// validator after init registrations; the SR-Policy engine remains selectable.
func TestInternalPluginCompletionOmitsNonSelectableRegistrations(t *testing.T) {
	validator := config.InternalPluginNameValidator()
	choices := validator.CompleteFn()
	require.Contains(t, choices, "bgp-nlri-srpolicy")
	for _, name := range []string{"loop", "crashes"} {
		require.True(t, registry.Has(name), "catalog and boot-effect registration must survive")
		require.NotContains(t, choices, name, "an engine that immediately exits is not a choice")
	}
}
