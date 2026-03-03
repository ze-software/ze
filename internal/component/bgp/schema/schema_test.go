package schema_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"

	// Blank imports trigger init() registration of YANG modules.
	// bgp/schema registers automatically (same package).
	// hub/schema needed because ze-bgp-conf imports ze-hub-conf.
	_ "codeberg.org/thomas-mangin/ze/internal/hub/schema"
)

// TestSchema_ZeBgpModule verifies ze-bgp-conf.yang content.
//
// VALIDATES: ze-bgp-conf module defines expected namespace and containers.
// PREVENTS: Missing BGP configuration elements.
func TestSchema_ZeBgpModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	require.NoError(t, loader.Resolve())

	mod := loader.GetModule("ze-bgp-conf")
	require.NotNil(t, mod, "ze-bgp-conf module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:bgp:conf", mod.Namespace.Name)

	// Check import of ze-types
	assert.NotEmpty(t, mod.Import, "ze-bgp-conf should import ze-types")

	// Find bgp container
	var bgpContainer bool
	for _, c := range mod.Container {
		if c.Name == "bgp" {
			bgpContainer = true
			break
		}
	}
	assert.True(t, bgpContainer, "bgp container should exist")
}
