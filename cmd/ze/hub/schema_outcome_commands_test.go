package hub

import (
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSchemaOutcomeOfACommandOnlyPlugin: a plugin whose declaration brings a
// -cmd module and an -api module and no config module defines no config
// section, so boot records it as SchemaNone and names it beside a refused
// name; a plugin that brings a config module beside its command module is
// SchemaDeclared and is not named.
//
// VALIDATES: defect (a) of the phase 11 command step: what boot records of a
// command-only plugin, which RecordStartedPlugins, the refusal beside an
// unknown name and the last-stop record read.
// PREVENTS: a command-only plugin recorded as one that defines config, so an
// operator whose section it cannot hold is never told so.
func TestSchemaOutcomeOfACommandOnlyPlugin(t *testing.T) {
	commandModules := []rpc.SchemaDecl{
		{Module: "acme-monitor-cmd", Namespace: "urn:acme:cmd", YANGText: "module acme-monitor-cmd {}"},
		{Module: "acme-monitor-api", Namespace: "urn:acme:api", YANGText: "module acme-monitor-api {}"},
	}
	withConfig := append([]rpc.SchemaDecl{
		{Module: "acme-monitor-conf", Namespace: "urn:acme:conf", YANGText: "module acme-monitor-conf {}"},
	}, commandModules...)
	declarations := []pluginserver.EarlyDeclaration{
		{Plugin: "commands-only", Schemas: commandModules},
		{Plugin: "config-and-commands", Schemas: withConfig},
	}

	if outcome := schemaOutcome("commands-only", declarations); outcome != zeconfig.SchemaNone {
		t.Errorf("commands-only has outcome %v, want SchemaNone", outcome)
	}
	if outcome := schemaOutcome("config-and-commands", declarations); outcome != zeconfig.SchemaDeclared {
		t.Errorf("config-and-commands has outcome %v, want SchemaDeclared", outcome)
	}

	external := []zePlugin.PluginConfig{
		{Name: "commands-only", Run: "./commands-only"},
		{Name: "config-and-commands", Run: "./config-and-commands"},
	}
	schemaless := schemalessPlugins(external, declarations)
	if len(schemaless) != 1 || schemaless[0].Name != "commands-only" {
		t.Errorf("schemalessPlugins = %+v, want commands-only alone", schemaless)
	}
}
