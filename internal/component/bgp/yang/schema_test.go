package yang_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"

	// Blank imports trigger init() registration of YANG modules.
	// bgp/schema registers automatically (same package).
	// hub/schema needed because ze-bgp-conf imports ze-hub-conf.
	_ "github.com/ze-software/ze/internal/component/hub/yang"
)

// TestSchema_ZeBgpModule verifies ze-bgp-conf.yang content.
//
// VALIDATES: ze-bgp-conf module defines expected namespace and containers.
// PREVENTS: Missing BGP configuration elements.
func TestSchema_ZeBgpModule(t *testing.T) {
	loader := yang.NewLoader()

	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)

	mod := schema.GetModule("ze-bgp-conf")
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

// TestSchemaDelayOpenRange proves the inherited timer leaf accepts the
// complete configured boundary and refuses a value beyond RFC 4271's maximum.
func TestSchemaDelayOpenRange(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)
	validator := yang.NewValidator(schema)
	for _, path := range []string{"bgp/timer/delay-open", "bgp/group/timer/delay-open", "bgp/group/peer/timer/delay-open", "bgp/peer/timer/delay-open"} {
		for _, seconds := range []uint16{0, 1, 240} {
			require.NoError(t, validator.Validate(path, seconds), "%s=%d", path, seconds)
		}
		err := validator.Validate(path, uint16(241))
		require.Error(t, err, "%s accepted 241", path)
		var rangeError *yang.ValidationError
		require.ErrorAs(t, err, &rangeError)
		assert.Equal(t, yang.ErrTypeRange, rangeError.Type, "%s must reject the value, not fail schema lookup", path)
	}
}
