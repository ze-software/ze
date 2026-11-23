package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// joinedMonitorModule is the module an external plugin declares in these
// tests: its own root, one ranged leaf and one leaf with a pattern.
const joinedMonitorModule = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8 { range "1..10"; } }
        leaf target { type string { pattern '[a-z][a-z0-9-]*'; } }
    }
}`

// joinedMonitorSchema answers the config schema built over the compiled-in
// modules joined to joinedMonitorModule, through the join's entry and the
// builder every compiled-in module goes through.
func joinedMonitorSchema(t *testing.T) *Schema {
	t.Helper()
	resolved, err := JoinExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: joinedMonitorModule,
	}})
	require.NoError(t, err)
	schema, err := schemaFromResolved(resolved)
	require.NoError(t, err)
	return schema
}

// TestJoinedModuleRangeRefusesOutOfRangeValue: the range of a leaf an external
// module declares reaches the parser, as a compiled-in leaf's does. Method:
// the boundary values of range 1..10.
//
// VALIDATES: AC-10 for a range: 1 and 10 parse, 0 and 11 are refused naming
// the leaf.
// PREVENTS: an external plugin's config being accepted with a value its own
// module forbids.
func TestJoinedModuleRangeRefusesOutOfRangeValue(t *testing.T) {
	schema := joinedMonitorSchema(t)

	for _, value := range []string{"1", "10"} {
		_, err := NewParser(schema).Parse(`acme-monitor { interval ` + value + `; }`)
		assert.NoError(t, err, "interval %s is inside range 1..10", value)
	}
	for _, value := range []string{"0", "11"} {
		_, err := NewParser(schema).Parse(`acme-monitor { interval ` + value + `; }`)
		require.Error(t, err, "interval %s is outside range 1..10", value)
		assert.Contains(t, err.Error(), "interval")
	}
}

// TestJoinedModulePatternRefusesMismatch: the pattern of a leaf an external
// module declares reaches the parser the same way.
//
// VALIDATES: AC-10 for a pattern: a matching value parses and a mismatch is
// refused naming the leaf.
// PREVENTS: the join carrying an external module's types without their
// restrictions.
func TestJoinedModulePatternRefusesMismatch(t *testing.T) {
	schema := joinedMonitorSchema(t)

	_, err := NewParser(schema).Parse(`acme-monitor { target probe-1; }`)
	require.NoError(t, err)

	_, err = NewParser(schema).Parse(`acme-monitor { target Bad; }`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
}

// TestJoinedAugmentReachesCompiledInContainer: a child an external module
// augments into a compiled-in container is in the schema of that container,
// beside its compiled-in children.
//
// VALIDATES: AC-6 at the schema: the augmented child parses under the
// compiled-in root, and the plugin-free schema does not hold it.
// PREVENTS: an augment that resolves and never reaches the config schema.
func TestJoinedAugmentReachesCompiledInContainer(t *testing.T) {
	resolved, err := JoinExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-plugin-extra", Namespace: "urn:acme:plugin-extra",
		Text: `module acme-plugin-extra {
    namespace "urn:acme:plugin-extra";
    prefix extra;
    import ze-system-conf { prefix sys; }
    augment "/sys:system" {
        container acme-extra {
            leaf note { type string; }
        }
    }
}`,
	}})
	require.NoError(t, err)
	schema, err := schemaFromResolved(resolved)
	require.NoError(t, err)

	root, ok := schema.Get("system").(*ContainerNode)
	require.True(t, ok, "the system root must be a container in the joined schema")
	assert.True(t, root.Has("acme-extra"), "the system root lacks the augmented child")
	assert.True(t, root.Has("host"), "the system root lost its compiled-in child")

	_, err = NewParser(schema).Parse(`system { acme-extra { note probe; } }`)
	require.NoError(t, err)

	shipped, err := YANGSchema()
	require.NoError(t, err)
	shippedRoot, ok := shipped.Get("system").(*ContainerNode)
	require.True(t, ok)
	assert.False(t, shippedRoot.Has("acme-extra"), "the join changed the schema of the compiled-in set")
}

// TestJoinExternalModulesWithNoneAnswersTheSharedSet: with no external module
// the entry answers the Resolved DefaultLoader shares. Method: pointer
// identity, because every resolve allocates a new Resolved.
//
// VALIDATES: AC-9 at the entry: no second resolve, and the schema cache keyed
// on the Resolved still answers.
// PREVENTS: a start with no external plugin resolving every module twice.
func TestJoinExternalModulesWithNoneAnswersTheSharedSet(t *testing.T) {
	shared, err := yang.DefaultLoader()
	require.NoError(t, err)
	joined, err := JoinExternalModules(nil)
	require.NoError(t, err)
	assert.Same(t, shared, joined)
}

// registeredModuleText answers the text the process registered for one module
// file, so a test can pass the very text the compiled-in set holds.
func registeredModuleText(t *testing.T, file string) string {
	t.Helper()
	for _, module := range yang.Modules() {
		if module.Name == file {
			return module.Content
		}
	}
	t.Fatalf("no registered YANG module %s in this test binary", file)
	return ""
}

// TestModuleTextRoadLeavesOutIdenticalRegisteredModule: a text that is, byte
// for byte, a module the process registered is that module, so the text road
// leaves it out and answers the shared set. Method: pointer identity.
//
// VALIDATES: an identical text of a registered module is accepted with no
// resolve, and the schema built from it is the compiled-in one.
// PREVENTS: a caller that passes a compiled-in module's own text being
// refused, or paying a second resolve for a module that adds nothing.
func TestModuleTextRoadLeavesOutIdenticalRegisteredModule(t *testing.T) {
	texts := map[string]string{"ze-plugin-conf.yang": registeredModuleText(t, "ze-plugin-conf.yang")}

	shared, err := yang.DefaultLoader()
	require.NoError(t, err)
	resolved, err := loadYANGModules(texts)
	require.NoError(t, err)
	assert.Same(t, shared, resolved)

	_, err = YANGSchemaWithPlugins(texts)
	require.NoError(t, err)
}

// TestJoinExternalModulesRefusesIdenticalCompiledInModule: the leave-out rule
// is the text road's alone. A module an external plugin declares under a
// compiled-in name is refused even when its text is identical.
//
// VALIDATES: JoinExternalModules refuses a compiled-in module's own text
// declared by a plugin, as a name clash naming the plugin.
// PREVENTS: the identical-text rule leaking into the external join, where the
// owner ruled a compiled-in name is always refused.
func TestJoinExternalModulesRefusesIdenticalCompiledInModule(t *testing.T) {
	_, err := JoinExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "ze-plugin-conf", Namespace: "urn:ze:plugin:conf",
		Text: registeredModuleText(t, "ze-plugin-conf.yang"),
	}})
	require.Error(t, err)
	var refusal *opererr.Error
	require.True(t, errors.As(err, &refusal), "the refusal is %T, want an operator error", err)
	assert.Contains(t, err.Error(), `plugin "acme"`)
	assert.Contains(t, err.Error(), `"ze-plugin-conf"`)
	assert.Contains(t, err.Error(), "compiled into ze")
}

// TestModuleTextRoadRefusesRegisteredModule: a module text that carries the
// name of a module the process registered and a different content is refused
// by name, where the old road dropped any duplicate whatever its content.
//
// VALIDATES: YANGSchemaWithPlugins passes its texts through the join, so a
// text with a registered name and another content is a name clash.
// PREVENTS: a module that shadows a compiled-in one being discarded with no
// message, and the caller reading a schema it did not ask for.
func TestModuleTextRoadRefusesRegisteredModule(t *testing.T) {
	_, err := YANGSchemaWithPlugins(map[string]string{
		"ze-plugin-conf.yang": `module ze-plugin-conf {
    namespace "urn:acme:shadow";
    prefix shadow;
    container acme-shadow { leaf enabled { type boolean; } }
}`,
	})
	require.Error(t, err)
	var refusal *opererr.Error
	require.True(t, errors.As(err, &refusal), "the refusal is %T, want an operator error", err)
	assert.Contains(t, err.Error(), `"ze-plugin-conf"`)
	assert.Contains(t, err.Error(), "compiled into ze")
}

// TestYANGSchemaAndValidatorWithPluginsReuseCacheForSharedSet proves that when
// loadYANGModules answers the shared DefaultLoader set (nil map or only texts
// already in the registered set), YANGSchemaWithPlugins and
// YANGValidatorWithPlugins return the cached YANGSchema and YANGValidator
// pointers rather than rebuilding, while MigrationSchema still returns a fresh
// schema and an unregistered module text builds a distinct schema and validator.
//
// VALIDATES: spec-plugin-external-yang-at-startup row 7 (G-4): cache reuse on
// the command-line text road when the module set is the shared one.
// PREVENTS: paying an uncached schema and validator build on every start or
// reload with an in-tree `--plugin`, or mutating the shared cache in
// MigrationSchema.
func TestYANGSchemaAndValidatorWithPluginsReuseCacheForSharedSet(t *testing.T) {
	cachedSchema, err := YANGSchema()
	require.NoError(t, err)
	cachedValidator, err := YANGValidator()
	require.NoError(t, err)

	nilSchema, err := YANGSchemaWithPlugins(nil)
	require.NoError(t, err)
	assert.Same(t, cachedSchema, nilSchema)

	nilValidator, err := YANGValidatorWithPlugins(nil)
	require.NoError(t, err)
	assert.Same(t, cachedValidator, nilValidator)

	registeredTexts := map[string]string{"ze-plugin-conf.yang": registeredModuleText(t, "ze-plugin-conf.yang")}
	regSchema, err := YANGSchemaWithPlugins(registeredTexts)
	require.NoError(t, err)
	assert.Same(t, cachedSchema, regSchema)

	regValidator, err := YANGValidatorWithPlugins(registeredTexts)
	require.NoError(t, err)
	assert.Same(t, cachedValidator, regValidator)

	migSchema, err := MigrationSchema()
	require.NoError(t, err)
	assert.NotSame(t, cachedSchema, migSchema)

	externalTexts := map[string]string{"acme-monitor-conf.yang": joinedMonitorModule}
	extSchema, err := YANGSchemaWithPlugins(externalTexts)
	require.NoError(t, err)
	assert.NotSame(t, cachedSchema, extSchema)
	assert.NotNil(t, extSchema.Get("acme-monitor"))

	extValidator, err := YANGValidatorWithPlugins(externalTexts)
	require.NoError(t, err)
	assert.NotSame(t, cachedValidator, extValidator)
}
