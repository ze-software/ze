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

// TestSchemaAcceptOwnDefaultsDisabled proves RFC 7611 Section 2.3 at the YANG
// schema boundary: bgp/accept-own is a boolean leaf with default "false".
func TestSchemaAcceptOwnDefaultsDisabled(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)

	mod := schema.GetModule("ze-bgp-conf")
	require.NotNil(t, mod)
	var found bool
	for _, c := range mod.Container {
		if c.Name != "bgp" {
			continue
		}
		for _, leaf := range c.Leaf {
			if leaf.Name != "accept-own" {
				continue
			}
			found = true
			require.NotNil(t, leaf.Type)
			assert.Equal(t, "boolean", leaf.Type.Name)
			require.NotNil(t, leaf.Default)
			assert.Equal(t, "false", leaf.Default.Name)
		}
	}
	require.True(t, found, "ze-bgp-conf must declare bgp/accept-own leaf")

	validator := yang.NewValidator(schema)
	require.NoError(t, validator.Validate("bgp/accept-own", false))
	require.NoError(t, validator.Validate("bgp/accept-own", true))
	require.Error(t, validator.Validate("bgp/accept-own", "not-a-bool"))
}

// TestSchemaAcceptOriginValidationStateDefaultsFalse proves RFC 8097 Section 2
// at the YANG schema boundary: session/accept-origin-validation-state is a
// boolean leaf defaulting to "false" across global, group, and peer scopes.
func TestSchemaAcceptOriginValidationStateDefaultsFalse(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)

	mod := schema.GetModule("ze-bgp-conf")
	require.NotNil(t, mod)
	var found bool
	for _, g := range mod.Grouping {
		if g.Name != "peer-fields" {
			continue
		}
		for _, c := range g.Container {
			if c.Name != "session" {
				continue
			}
			for _, leaf := range c.Leaf {
				if leaf.Name != "accept-origin-validation-state" {
					continue
				}
				found = true
				require.NotNil(t, leaf.Type)
				assert.Equal(t, "boolean", leaf.Type.Name)
				require.NotNil(t, leaf.Default)
				assert.Equal(t, "false", leaf.Default.Name)
			}
		}
	}
	require.True(t, found, "ze-bgp-conf peer-fields/session must declare accept-origin-validation-state leaf")

	validator := yang.NewValidator(schema)
	for _, path := range []string{
		"bgp/group/session/accept-origin-validation-state",
		"bgp/group/peer/session/accept-origin-validation-state",
		"bgp/peer/session/accept-origin-validation-state",
	} {
		require.NoError(t, validator.Validate(path, false), path)
		require.NoError(t, validator.Validate(path, true), path)
		require.Error(t, validator.Validate(path, "not-a-bool"), path)
	}
}

// TestSchemaFlowSpecRedirectNextHopDefaultsFalse proves RFC 8955 Section 4 at
// the YANG schema boundary: session/flowspec-redirect-next-hop is a boolean
// leaf defaulting to "false" across group and peer scopes.
func TestSchemaFlowSpecRedirectNextHopDefaultsFalse(t *testing.T) {
	loader := yang.NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.LoadRegistered())
	schema, err := loader.Resolve()
	require.NoError(t, err)

	mod := schema.GetModule("ze-bgp-conf")
	require.NotNil(t, mod)
	var found bool
	for _, g := range mod.Grouping {
		if g.Name != "peer-fields" {
			continue
		}
		for _, c := range g.Container {
			if c.Name != "session" {
				continue
			}
			for _, leaf := range c.Leaf {
				if leaf.Name != "flowspec-redirect-next-hop" {
					continue
				}
				found = true
				require.NotNil(t, leaf.Type)
				assert.Equal(t, "boolean", leaf.Type.Name)
				require.NotNil(t, leaf.Default)
				assert.Equal(t, "false", leaf.Default.Name)
			}
		}
	}
	require.True(t, found, "ze-bgp-conf peer-fields/session must declare flowspec-redirect-next-hop leaf")

	validator := yang.NewValidator(schema)
	for _, path := range []string{
		"bgp/group/session/flowspec-redirect-next-hop",
		"bgp/group/peer/session/flowspec-redirect-next-hop",
		"bgp/peer/session/flowspec-redirect-next-hop",
	} {
		require.NoError(t, validator.Validate(path, false), path)
		require.NoError(t, validator.Validate(path, true), path)
		require.Error(t, validator.Validate(path, "not-a-bool"), path)
	}
}
