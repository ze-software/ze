package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"

	// Blank import triggers init() registration of the web YANG module.
	_ "codeberg.org/thomas-mangin/ze/internal/component/web/schema"
)

// TestSchema_ZeWebModule verifies ze-web-conf.yang content.
//
// VALIDATES: ze-web-conf module defines expected namespace and containers.
// PREVENTS: Missing web configuration elements.
func TestSchema_ZeWebModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-web-conf")
	require.NotNil(t, mod, "ze-web-conf module should exist")

	assert.Equal(t, "urn:ze:web:conf", mod.Namespace.Name)

	// Web config lives under environment.web (moved from system.web).
	var envContainer bool
	for _, c := range mod.Container {
		if c.Name != "environment" {
			continue
		}

		envContainer = true

		var webChild bool
		for _, child := range c.Container {
			if child.Name == "web" {
				webChild = true
				break
			}
		}

		assert.True(t, webChild, "environment.web container should exist")

		break
	}
	assert.True(t, envContainer, "environment container should exist")
}
