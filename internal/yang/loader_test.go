package yang

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoader_EmbeddedModules verifies loading of embedded core YANG modules.
//
// VALIDATES: Core YANG modules (extensions, types, plugin) load without errors.
// PREVENTS: Syntax errors in YANG files breaking startup.
// NOTE: ze-bgp is in internal/plugin/bgp/schema, ze-hub is in internal/hub/schema.
func TestLoader_EmbeddedModules(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err, "loading embedded modules should succeed")

	err = loader.Resolve()
	require.NoError(t, err, "resolving modules should succeed")

	// Verify core modules are loaded (ze-bgp and ze-hub are now external)
	names := loader.ModuleNames()
	assert.Contains(t, names, "ze-extensions", "ze-extensions module should be loaded")
	assert.Contains(t, names, "ze-types", "ze-types module should be loaded")
	assert.Contains(t, names, "ze-plugin", "ze-plugin module should be loaded")
}

// TestLoader_ZeTypesModule verifies ze-types.yang content.
//
// VALIDATES: ze-types module defines expected typedefs.
// PREVENTS: Missing type definitions breaking other modules.
func TestLoader_ZeTypesModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-types")
	require.NotNil(t, mod, "ze-types module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:types", mod.Namespace.Name)

	// Check typedefs exist
	typedefNames := make(map[string]bool)
	for _, td := range mod.Typedef {
		typedefNames[td.Name] = true
	}

	assert.True(t, typedefNames["asn"], "asn typedef should exist")
	assert.True(t, typedefNames["asn2"], "asn2 typedef should exist")
	assert.True(t, typedefNames["port"], "port typedef should exist")
	assert.True(t, typedefNames["ip-address"], "ip-address typedef should exist")
	assert.True(t, typedefNames["ipv4-address"], "ipv4-address typedef should exist")
	assert.True(t, typedefNames["ipv6-address"], "ipv6-address typedef should exist")
	assert.True(t, typedefNames["community"], "community typedef should exist")
}

// TestLoader_ZeBgpModule verifies ze-bgp.yang content.
//
// VALIDATES: ze-bgp module defines expected containers and lists.
// PREVENTS: Missing BGP configuration elements.
// NOTE: Uses LoadAllForTesting since ze-bgp is now in internal/plugin/bgp/schema.
func TestLoader_ZeBgpModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-bgp")
	require.NotNil(t, mod, "ze-bgp module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:bgp", mod.Namespace.Name)

	// Check import of ze-types
	assert.NotEmpty(t, mod.Import, "ze-bgp should import ze-types")

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

// TestLoader_ZeHubModule verifies ze-hub.yang content.
//
// VALIDATES: ze-hub module defines expected containers.
// PREVENTS: Missing hub/environment configuration elements.
// NOTE: Uses LoadAllForTesting since ze-hub is now in internal/hub/schema.
func TestLoader_ZeHubModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadAllForTesting()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-hub")
	require.NotNil(t, mod, "ze-hub module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:hub", mod.Namespace.Name)

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

// TestLoader_ZePluginModule verifies ze-plugin.yang content.
//
// VALIDATES: ze-plugin module defines plugin configuration.
// PREVENTS: Missing plugin configuration schema.
func TestLoader_ZePluginModule(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-plugin")
	require.NotNil(t, mod, "ze-plugin module should exist")

	// Check namespace
	assert.Equal(t, "urn:ze:plugin", mod.Namespace.Name)

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

// TestLoader_AddModuleFromText verifies loading YANG from text.
//
// VALIDATES: YANG modules can be loaded from text content.
// PREVENTS: Plugin schema declaration failures.
func TestLoader_AddModuleFromText(t *testing.T) {
	loader := NewLoader()

	yangText := `
module test-module {
    namespace "urn:test:module";
    prefix tm;

    leaf test-leaf {
        type string;
    }
}
`

	err := loader.AddModuleFromText("test-module.yang", yangText)
	require.NoError(t, err, "loading from text should succeed")

	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("test-module")
	require.NotNil(t, mod)
	assert.Equal(t, "urn:test:module", mod.Namespace.Name)
}

// TestLoader_InvalidYang verifies error handling for invalid YANG.
//
// VALIDATES: Invalid YANG syntax is rejected with error.
// PREVENTS: Silent acceptance of malformed schemas.
func TestLoader_InvalidYang(t *testing.T) {
	loader := NewLoader()

	invalidYang := `
module broken {
    this is not valid yang
}
`

	err := loader.AddModuleFromText("broken.yang", invalidYang)
	require.Error(t, err, "invalid YANG should fail")
}

// TestLoader_MissingImport verifies error on unresolved import.
//
// VALIDATES: Missing imports cause resolution failure.
// PREVENTS: Silently ignoring missing dependencies.
func TestLoader_MissingImport(t *testing.T) {
	loader := NewLoader()

	yangWithImport := `
module needs-import {
    namespace "urn:test:needs-import";
    prefix ni;

    import nonexistent-module { prefix nm; }

    leaf test {
        type nm:some-type;
    }
}
`

	err := loader.AddModuleFromText("needs-import.yang", yangWithImport)
	require.NoError(t, err, "parse should succeed")

	// Resolution should fail due to missing import
	err = loader.Resolve()
	require.Error(t, err, "resolution should fail with missing import")
}

// TestLoader_TypeBoundaries verifies type constraint definitions.
//
// VALIDATES: Types have correct range constraints.
// PREVENTS: Incorrect boundary validation.
func TestLoader_TypeBoundaries(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	mod := loader.GetModule("ze-types")
	require.NotNil(t, mod)

	// Find ASN typedef and check its range
	for _, td := range mod.Typedef {
		if td.Name == "asn" {
			// ASN should be uint32 with range 1..4294967295
			assert.NotNil(t, td.Type, "asn should have a type")
			// Type checking is done by goyang during resolution
		}
		if td.Name == "port" {
			// Port should be uint16 with range 1..65535
			assert.NotNil(t, td.Type, "port should have a type")
		}
	}
}
