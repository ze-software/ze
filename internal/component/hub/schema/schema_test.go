package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
)

// TestSchema_ZeHubModule verifies ze-hub-conf.yang content.
//
// VALIDATES: ze-hub-conf module defines expected namespace and containers.
// PREVENTS: Missing hub/environment configuration elements.
func TestSchema_ZeHubModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-hub-conf")
	require.NotNil(t, mod, "ze-hub-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:hub:conf", mod.Namespace.Name)

	// Find environment container
	var envContainer bool
	for _, c := range mod.Container {
		if c.Name == "environment" {
			envContainer = true
			break
		}
	}
	assert.True(t, envContainer, "environment container should exist")
}
