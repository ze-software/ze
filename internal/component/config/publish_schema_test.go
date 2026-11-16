package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// publishedMonitor is the declaration an external plugin makes for
// joinedMonitorModule: its own root `acme-monitor`, a ranged leaf and a leaf
// with a pattern.
func publishedMonitor() yang.ExternalModule {
	return yang.ExternalModule{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: joinedMonitorModule,
	}
}

// TestPublishedSetReachesSchemaCaches: the schema and the validator every
// reader of the process asks for hold an external plugin's module once it is
// published, and not before. Method: ask YANGSchema, YANGValidator and
// DefaultLoader before the publish, after it, and after the test's restore.
//
// VALIDATES: AC-13 at the schema: after PublishExternalModules, YANGSchema
// holds the module's top-level node and parses a block under it, and
// YANGValidator applies its range; before the publish neither knows the root.
// PREVENTS: a published module that reaches DefaultLoader and not the schema
// or the validator cached over it, so one surface accepts a value another
// refuses.
func TestPublishedSetReachesSchemaCaches(t *testing.T) {
	const section = "acme-monitor"
	const leaf = section + "/interval"
	const module = "acme-monitor-conf"

	requireAbsent := func(stage string) {
		t.Helper()
		schema, err := YANGSchema()
		require.NoError(t, err, stage)
		assert.Nil(t, schema.Get(section), "%s: the schema holds the external root", stage)
		validator, err := YANGValidator()
		require.NoError(t, err, stage)
		assert.Error(t, validator.Validate(leaf, 5), "%s: the validator resolves the external leaf", stage)
		resolved, err := yang.DefaultLoader()
		require.NoError(t, err, stage)
		assert.Nil(t, resolved.GetModule(module), "%s: DefaultLoader holds the external module", stage)
	}

	restore := yang.ResetPublishedModulesForTest()
	func() {
		defer restore()
		requireAbsent("before the publish")

		require.NoError(t, PublishExternalModules([]yang.ExternalModule{publishedMonitor()}))

		schema, err := YANGSchema()
		require.NoError(t, err)
		require.NotNil(t, schema.Get(section), "the schema misses the published root")
		_, err = NewParser(schema).Parse(`acme-monitor { interval 5; }`)
		assert.NoError(t, err, "a block under the published root does not parse")
		_, err = NewParser(schema).Parse(`acme-monitor { interval 11; }`)
		assert.Error(t, err, "the parser ignores the range of the published leaf")

		validator, err := YANGValidator()
		require.NoError(t, err)
		assert.NoError(t, validator.Validate(leaf, 5), "the validator misses the published leaf")
		assert.Error(t, validator.Validate(leaf, 11), "the validator ignores the range of the published leaf")

		resolved, err := yang.DefaultLoader()
		require.NoError(t, err)
		assert.NotNil(t, resolved.GetModule(module), "DefaultLoader misses the published module")

		again, err := YANGSchema()
		require.NoError(t, err)
		assert.Same(t, schema, again, "the schema of the published set is built on every call")
	}()

	requireAbsent("after the restore")
}

// TestPublishWithNoExternalModuleKeepsTheSharedSet: a start that declares no
// external plugin publishes nothing and builds nothing twice. Method: pointer
// identity of the Resolved, the schema and the validator around a publish of
// no module.
//
// VALIDATES: AC-9: no extra resolve and no second schema or validator build.
// PREVENTS: every start paying a join it does not need.
func TestPublishWithNoExternalModuleKeepsTheSharedSet(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	resolvedBefore, err := yang.DefaultLoader()
	require.NoError(t, err)
	schemaBefore, err := YANGSchema()
	require.NoError(t, err)
	validatorBefore, err := YANGValidator()
	require.NoError(t, err)

	require.NoError(t, PublishExternalModules(nil))

	resolvedAfter, err := yang.DefaultLoader()
	require.NoError(t, err)
	assert.Same(t, resolvedBefore, resolvedAfter)
	schemaAfter, err := YANGSchema()
	require.NoError(t, err)
	assert.Same(t, schemaBefore, schemaAfter)
	validatorAfter, err := YANGValidator()
	require.NoError(t, err)
	assert.Same(t, validatorBefore, validatorAfter)

	_, published := yang.PublishedExternalModules()
	assert.True(t, published, "the publish of no module is not recorded")
}

// TestPublishRefusedModuleKeepsTheCompiledInSchema: a module the join refuses
// publishes nothing, and the compiled-in set still answers. Method: publish a
// module whose name a compiled-in module holds.
//
// VALIDATES: the refusal names the plugin, and DefaultLoader and YANGSchema
// answer what they answered before; nothing is recorded as published.
// PREVENTS: a refused module leaving the process on a half-joined set.
func TestPublishRefusedModuleKeepsTheCompiledInSchema(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	resolvedBefore, err := yang.DefaultLoader()
	require.NoError(t, err)
	schemaBefore, err := YANGSchema()
	require.NoError(t, err)

	err = PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "ze-plugin-conf", Namespace: "urn:acme:plugin",
		Text: `module ze-plugin-conf {
    namespace "urn:acme:plugin";
    prefix acmeplug;
    container acme-plugin {
        leaf note { type string; }
    }
}`,
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"acme"`)
	assert.Contains(t, err.Error(), "ze-plugin-conf")

	resolvedAfter, err := yang.DefaultLoader()
	require.NoError(t, err)
	assert.Same(t, resolvedBefore, resolvedAfter)
	schemaAfter, err := YANGSchema()
	require.NoError(t, err)
	assert.Same(t, schemaBefore, schemaAfter)
	_, published := yang.PublishedExternalModules()
	assert.False(t, published, "a refused publish is recorded as a publish")
}

// TestPublishTwiceThroughTheConfigEntry: the config entry builds a new
// validator registry on each call, and that MUST NOT make the same modules
// read as another set. Method: publish the same module twice, then another.
//
// VALIDATES: the second publish of the same modules returns nil and keeps the
// schema; a publish of other modules is refused naming the restart.
// PREVENTS: the once rule comparing the registry and not the modules.
func TestPublishTwiceThroughTheConfigEntry(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	require.NoError(t, PublishExternalModules([]yang.ExternalModule{publishedMonitor()}))
	schema, err := YANGSchema()
	require.NoError(t, err)

	require.NoError(t, PublishExternalModules([]yang.ExternalModule{publishedMonitor()}))
	again, err := YANGSchema()
	require.NoError(t, err)
	assert.Same(t, schema, again)

	err = PublishExternalModules(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "restart ze")
}
