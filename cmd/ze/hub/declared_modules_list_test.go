package hub

import (
	"errors"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestDeclaredModulesTakesEveryModuleOfEachPlugin: boot publishes every module
// a plugin lists, in the order it lists them, after the modules of the plugins
// before it, and skips an entry with no text and a plugin that did not declare.
// Method: three early declarations, read by declaredModules and schemaOutcome.
//
// VALIDATES: the command step of spec-plugin-external-yang-at-startup: a
// config module and a command module of one plugin both reach yang.Join.
// PREVENTS: the second module of a plugin dropped before the join, so its
// commands never exist.
func TestDeclaredModulesTakesEveryModuleOfEachPlugin(t *testing.T) {
	declarations := []pluginserver.EarlyDeclaration{
		{Plugin: "acme-monitor", Schemas: []rpc.SchemaDecl{
			{Module: "acme-monitor-conf", Namespace: "urn:acme:conf", YANGText: "module acme-monitor-conf {}"},
			{Module: "acme-monitor-empty"},
			{Module: "acme-monitor-cmd", Namespace: "urn:acme:cmd", YANGText: "module acme-monitor-cmd {}"},
		}},
		{Plugin: "silent", Err: errors.New("it sent nothing")},
		{Plugin: "beta", Schemas: []rpc.SchemaDecl{{Module: "beta-api", Namespace: "urn:beta", YANGText: "module beta-api {}"}}},
	}
	got := declaredModules(declarations)
	want := []yang.ExternalModule{
		{Plugin: "acme-monitor", Module: "acme-monitor-conf", Namespace: "urn:acme:conf", Text: "module acme-monitor-conf {}"},
		{Plugin: "acme-monitor", Module: "acme-monitor-cmd", Namespace: "urn:acme:cmd", Text: "module acme-monitor-cmd {}"},
		{Plugin: "beta", Module: "beta-api", Namespace: "urn:beta", Text: "module beta-api {}"},
	}
	if len(got) != len(want) {
		t.Fatalf("declaredModules answered %d modules, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("module %d is %+v, want %+v", i, got[i], want[i])
		}
	}
	if outcome := schemaOutcome("acme-monitor", declarations); outcome != zeconfig.SchemaDeclared {
		t.Errorf("acme-monitor has outcome %v, want SchemaDeclared", outcome)
	}
	none := []pluginserver.EarlyDeclaration{{Plugin: "bare", Schemas: []rpc.SchemaDecl{{Module: "bare-conf"}}}}
	if outcome := schemaOutcome("bare", none); outcome != zeconfig.SchemaNone {
		t.Errorf("a plugin whose only entry has no text has outcome %v, want SchemaNone", outcome)
	}
}
