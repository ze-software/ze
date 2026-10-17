package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// probeConsistentModule is a -conf module, so the schema build defines its
// top-level container, with a ranged leaf the value validator can judge.
const probeConsistentModule = `module ze-probe-consistent-conf {
    namespace "urn:ze:probe-consistent-conf";
    prefix probec;
    container probe-consistent {
        leaf size { type uint8 { range "1..10"; } }
    }
}`

// TestPluginFreeCachesFollowTheRegisteredModuleSet proves YANGSchema,
// YANGValidator and yang.DefaultLoader answer from one module set: a module a
// test registers is seen by all three while it is registered, and by none once
// the registration is restored.
//
// VALIDATES: the schema and validator caches are keyed by DefaultLoader's
// Resolved, so cli.NewConfigValidator, which pairs YANGSchema with a validator
// over DefaultLoader, never holds two different module sets.
// PREVENTS: a process-wide sync.Once freezing whichever set its first caller
// saw while DefaultLoader moves on to the registered one.
func TestPluginFreeCachesFollowTheRegisteredModuleSet(t *testing.T) {
	const section = "probe-consistent"
	const leaf = section + "/size"
	const module = "ze-probe-consistent-conf"

	requireAbsent := func(stage string) {
		t.Helper()
		schema, err := YANGSchema()
		require.NoError(t, err, stage)
		assert.Nil(t, schema.Get(section), "%s: the schema holds the probe section", stage)
		validator, err := YANGValidator()
		require.NoError(t, err, stage)
		assert.Error(t, validator.Validate(leaf, 5), "%s: the validator resolves the probe leaf", stage)
		resolved, err := yang.DefaultLoader()
		require.NoError(t, err, stage)
		assert.Nil(t, resolved.GetModule(module), "%s: DefaultLoader holds the probe module", stage)
	}

	requireAbsent("before registering")

	restore := yang.RegisterModuleForTest(module+".yang", probeConsistentModule)
	func() {
		defer restore()
		schema, err := YANGSchema()
		require.NoError(t, err)
		assert.NotNil(t, schema.Get(section), "the schema misses the registered probe section")
		validator, err := YANGValidator()
		require.NoError(t, err)
		assert.NoError(t, validator.Validate(leaf, 5), "the validator misses the registered probe leaf")
		assert.Error(t, validator.Validate(leaf, 11), "the validator ignores the probe leaf's range")
		resolved, err := yang.DefaultLoader()
		require.NoError(t, err)
		assert.NotNil(t, resolved.GetModule(module), "DefaultLoader misses the registered probe module")
	}()

	requireAbsent("after restoring")
}
