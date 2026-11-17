package hub

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSchemalessPluginsAreNamedBesideAnUnknownSection proves that boot names,
// for a config load refused for a name no schema defines, each declared
// external plugin whose declaration carries no YANG module, and no other: a
// plugin that declared a module is known to the schema, and one that sent no
// declaration is named by undeclaredPluginLines with its own reason.
//
// VALIDATES: the "declared none" case of doctor-plugin-schema-undeclared
// (AC-14 of spec-plugin-external-yang-at-startup): the plugin, what follows,
// and the conformance check as the fix on the last line.
// PREVENTS: a section written for a plugin that declares no module refused as
// a bare unknown keyword, with nothing that says the plugin brought no schema.
func TestSchemalessPluginsAreNamedBesideAnUnknownSection(t *testing.T) {
	external := []zePlugin.PluginConfig{
		{Name: "with-module", Run: "./with-module"},
		{Name: "no-schema", Run: "./no-schema"},
		{Name: "no-text", Run: "./no-text --flag"},
		{Name: "silent", Run: "./silent"},
	}
	declarations := []pluginserver.EarlyDeclaration{
		{Plugin: "with-module", Schema: &rpc.SchemaDecl{Module: "m-conf", YANGText: "module m-conf {}"}},
		{Plugin: "no-schema"},
		{Plugin: "no-text", Schema: &rpc.SchemaDecl{Module: "n-conf"}},
		{Plugin: "silent", Err: errors.New("sent nothing")},
	}

	schemaless := schemalessPlugins(external, declarations)

	assert.Equal(t, []zePlugin.PluginConfig{external[1], external[2]}, schemaless)
	lines := schemalessPluginLines(schemaless)
	assert.Contains(t, lines, `error: external plugin "no-schema" declared no YANG module, so it defines no config section`)
	assert.Contains(t, lines, `error: external plugin "no-text" declared no YANG module`)
	assert.Contains(t, lines, "refused as an unknown keyword")
	assert.Contains(t, lines, "fix: ze show plugin conformance run \"./no-schema\"\n")
	assert.Contains(t, lines, "fix: ze show plugin conformance run \"./no-text --flag\"\n")
	assert.NotContains(t, lines, "with-module")
	assert.NotContains(t, lines, "silent")

	assert.Empty(t, schemalessPlugins(external[:1], declarations), "a plugin that declared a module is not named")
	assert.Empty(t, schemalessPluginLines(nil))
}
