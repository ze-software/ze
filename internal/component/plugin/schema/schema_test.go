package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"

	// Blank import: ze-plugin-conf imports ze-hub-conf.
	_ "codeberg.org/thomas-mangin/ze/internal/component/hub/schema"
)

// TestSchema_ZePluginModule verifies ze-plugin-conf.yang content.
//
// VALIDATES: ze-plugin-conf module defines expected namespace and containers.
// PREVENTS: Missing plugin configuration schema.
func TestSchema_ZePluginModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-plugin-conf")
	require.NotNil(t, mod, "ze-plugin-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:plugin:conf", mod.Namespace.Name)

	// Find plugin container
	var pluginContainer bool
	for _, c := range mod.Container {
		if c.Name == "plugin" {
			pluginContainer = true
			break
		}
	}
	assert.True(t, pluginContainer, "plugin container should exist")
}
