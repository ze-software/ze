package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// schemaSubject is the external plugin the schema tests publish a module for,
// and schemaSilent one that declared no module when ze started.
const (
	schemaSubject = "acme"
	schemaSilent  = "beta"
)

const schemaSubjectText = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8; }
    }
}`

// schemaSubjectDecl is the Stage 1 schema schemaSubject declared at boot.
func schemaSubjectDecl() *rpc.SchemaDecl {
	return &rpc.SchemaDecl{Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", YANGText: schemaSubjectText}
}

// externalBlock is the operator's block of an external plugin named name.
func externalBlock(name string) plugin.PluginConfig {
	return plugin.PluginConfig{Name: name, Run: "./" + name}
}

// publishSchemaSubject makes the process one that booted with schemaSubject
// declaring its module and schemaSilent declaring none. It MUST run before
// the test builds a plugin server, which keeps the module set.
func publishSchemaSubject(t *testing.T) []yang.ExternalModule {
	t.Helper()
	t.Cleanup(yang.ResetPublishedModulesForTest())
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	decl := schemaSubjectDecl()
	modules := []yang.ExternalModule{{
		Plugin: schemaSubject, Module: decl.Module, Namespace: decl.Namespace, Text: decl.YANGText,
	}}
	require.NoError(t, yang.PublishExternalModules(modules, yang.NewValidatorRegistry()))
	return modules
}

// schemaChangedWarning answers the doctor-plugin-schema-changed warning about
// the plugin named name.
func schemaChangedWarning(name string) (report.Issue, bool) {
	for _, issue := range report.Warnings() {
		if issue.Code != CodeDoctorPluginSchemaChanged {
			continue
		}
		if issue.Subject == name {
			return issue, true
		}
	}
	return report.Issue{}, false
}

// TestStage1RefusesChangedDeclaration: a plugin started again is compared
// with the module the process published for it at boot. Method: publish one
// module, then judge a declaration for each way a later one can differ.
//
// VALIDATES: AC-7. Another text, name or namespace, a module where there was
// none and none where there was one are each refused with an operator error
// that names the plugin, says what differs and names the restart of ze; the
// warning doctor reads is raised with that fix; the published set is not
// written.
// PREVENTS: a restarted plugin accepted with a module no reader compiled, and
// a refusal that changes the schema the daemon holds.
func TestStage1RefusesChangedDeclaration(t *testing.T) {
	otherText := schemaSubjectDecl()
	otherText.YANGText += "\n"
	otherName := schemaSubjectDecl()
	otherName.Module = "acme-probe-conf"
	otherNamespace := schemaSubjectDecl()
	otherNamespace.Namespace = "urn:acme:probe"

	cases := []struct {
		name   string
		plugin string
		schema *rpc.SchemaDecl
		says   string
	}{
		{"another text", schemaSubject, otherText,
			`the text of YANG module "acme-monitor-conf" is not the text the plugin declared when ze started`},
		{"another module name", schemaSubject, otherName,
			`it declares YANG module "acme-probe-conf" now, and declared module "acme-monitor-conf" when ze started`},
		{"another namespace", schemaSubject, otherNamespace,
			`the namespace of YANG module "acme-monitor-conf" is "urn:acme:probe" now, and was "urn:acme:monitor" when ze started`},
		{"no schema where there was one", schemaSubject, nil,
			`it declares no YANG module now, and declared module "acme-monitor-conf" when ze started`},
		{"no module text where there was one", schemaSubject, &rpc.SchemaDecl{Module: "acme-monitor-conf"},
			`it declares no YANG module now, and declared module "acme-monitor-conf" when ze started`},
		{"a module where there was none", schemaSilent, schemaSubjectDecl(),
			`it declares YANG module "acme-monitor-conf" now, and declared no module when ze started`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			published := publishSchemaSubject(t)

			var schemas []rpc.SchemaDecl
			if tc.schema != nil {
				schemas = []rpc.SchemaDecl{*tc.schema}
			}
			err := refuseChangedSchema(externalBlock(tc.plugin), schemas)
			require.Error(t, err, "a changed declaration was accepted")
			refusal, operator := errors.AsType[*opererr.Error](err)
			require.True(t, operator, "the refusal is no operator error: %v", err)
			assert.Equal(t, `plugin "`+tc.plugin+`" was started again with other YANG modules, and ze refused it: `+tc.says,
				refusal.Problem())
			assert.Equal(t, "restart ze", refusal.Fix())

			warning, raised := schemaChangedWarning(tc.plugin)
			require.True(t, raised, "no doctor-plugin-schema-changed warning names the plugin")
			assert.Equal(t, "restart ze", warning.Fix)
			assert.Contains(t, warning.Message, tc.says)

			after, fixed := yang.PublishedExternalModules()
			require.True(t, fixed)
			assert.Equal(t, published, after, "the refusal wrote the published set")
		})
	}
}

const schemaProbeText = `module acme-probe-conf {
    namespace "urn:acme:probe";
    prefix probe;
    container acme-probe {
        leaf depth { type uint8; }
    }
}`

// schemaProbeDecl is the second module schemaSubject declares in the
// multi-module tests.
func schemaProbeDecl() rpc.SchemaDecl {
	return rpc.SchemaDecl{Module: "acme-probe-conf", Namespace: "urn:acme:probe", YANGText: schemaProbeText}
}

// publishTwoModules makes the process one that booted with schemaSubject
// declaring two modules, in the order schemaSubjectDecl then schemaProbeDecl.
// It MUST run before the test builds a plugin server.
func publishTwoModules(t *testing.T) []yang.ExternalModule {
	t.Helper()
	t.Cleanup(yang.ResetPublishedModulesForTest())
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	modules := DeclaredModules(schemaSubject, []rpc.SchemaDecl{*schemaSubjectDecl(), schemaProbeDecl()})
	require.Len(t, modules, 2)
	require.NoError(t, yang.PublishExternalModules(modules, yang.NewValidatorRegistry()))
	return modules
}

// TestStage1RefusesChangedModuleSet: a plugin that brings several modules is
// compared module by module. Method: publish two modules for one plugin, then
// judge a restart that drops one and a restart that renames one, and read the
// operator error each answers.
//
// VALIDATES: the two refusals schemaDifference holds only for a set of
// modules: a published module the plugin no longer declares, and a module
// declared now beside the published ones that it did not declare at start.
// Each names the plugin, the module and the restart of ze, and leaves the
// published set unwritten.
// PREVENTS: a restart that silently loses one of a plugin's modules (its
// config or its commands), or that brings a module no reader compiled.
func TestStage1RefusesChangedModuleSet(t *testing.T) {
	renamed := schemaProbeDecl()
	renamed.Module = "acme-scan-conf"

	cases := []struct {
		name    string
		schemas []rpc.SchemaDecl
		says    string
	}{
		{"one module no longer declared", []rpc.SchemaDecl{*schemaSubjectDecl()},
			`it does not declare YANG module "acme-probe-conf" now, and declared it when ze started`},
		{"one module renamed", []rpc.SchemaDecl{*schemaSubjectDecl(), renamed},
			`it declares YANG module "acme-scan-conf" now, which it did not declare when ze started`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			published := publishTwoModules(t)

			err := refuseChangedSchema(externalBlock(schemaSubject), tc.schemas)
			require.Error(t, err, "a changed module set was accepted")
			refusal, operator := errors.AsType[*opererr.Error](err)
			require.True(t, operator, "the refusal is no operator error: %v", err)
			assert.Equal(t, `plugin "`+schemaSubject+`" was started again with other YANG modules, and ze refused it: `+tc.says,
				refusal.Problem())
			assert.Equal(t, "restart ze", refusal.Fix())

			warning, raised := schemaChangedWarning(schemaSubject)
			require.True(t, raised, "no doctor-plugin-schema-changed warning names the plugin")
			assert.Contains(t, warning.Message, tc.says)

			after, fixed := yang.PublishedExternalModules()
			require.True(t, fixed)
			assert.Equal(t, published, after, "the refusal wrote the published set")
		})
	}
}

// TestStage1ModuleSetOrderDoesNotMatter: the restart check compares the
// modules by name, so a plugin that lists the same modules in another order
// declares the same schema. Method: publish two modules, then judge the same
// two modules in the order they were published and reversed.
//
// VALIDATES: what schemaDifference promises: each declared module is the
// published one of its name, and each published module is declared, whatever
// the order of the list.
// PREVENTS: a plugin refused at restart because its runtime built the list in
// another order, when its schema did not change.
func TestStage1ModuleSetOrderDoesNotMatter(t *testing.T) {
	publishTwoModules(t)

	require.NoError(t, refuseChangedSchema(externalBlock(schemaSubject),
		[]rpc.SchemaDecl{*schemaSubjectDecl(), schemaProbeDecl()}))
	require.NoError(t, refuseChangedSchema(externalBlock(schemaSubject),
		[]rpc.SchemaDecl{schemaProbeDecl(), *schemaSubjectDecl()}))

	_, raised := schemaChangedWarning(schemaSubject)
	assert.False(t, raised, "an unchanged module set raised the warning")
}

// TestStage1AcceptsUnchangedDeclaration: the declarations a restart may
// bring. Method: publish one module, then judge the same module, no module
// from the plugin that declared none, and any module from a plugin compiled
// into ze.
//
// VALIDATES: AC-7's permitted survivors. A restart that declares the same
// module proceeds, and so does the first Stage 1, which declares what boot
// published; a plugin compiled into ze is not compared.
// PREVENTS: the check refusing the boot it was added to, or every internal
// plugin that sends its registered module.
func TestStage1AcceptsUnchangedDeclaration(t *testing.T) {
	publishSchemaSubject(t)

	require.NoError(t, refuseChangedSchema(externalBlock(schemaSubject), []rpc.SchemaDecl{*schemaSubjectDecl()}))
	require.NoError(t, refuseChangedSchema(externalBlock(schemaSilent), nil))
	require.NoError(t, refuseChangedSchema(externalBlock(schemaSilent), []rpc.SchemaDecl{{Module: "beta-conf"}}))

	compiledIn := plugin.PluginConfig{Name: schemaSubject, Internal: true, Run: schemaSubject}
	changed := schemaSubjectDecl()
	changed.YANGText += "\n"
	require.NoError(t, refuseChangedSchema(compiledIn, []rpc.SchemaDecl{*changed}))

	_, raised := schemaChangedWarning(schemaSubject)
	assert.False(t, raised, "an accepted declaration raised the warning")
}

// TestStage1UnchangedDeclarationClearsTheWarning: the warning is a state, and
// it ends when the plugin declares the published module again. Method: refuse
// a changed declaration, then accept the published one.
//
// VALIDATES: a plugin restarted with the schema ze started with leaves no
// doctor-plugin-schema-changed warning behind.
// PREVENTS: doctor reporting a changed schema for a plugin that runs.
func TestStage1UnchangedDeclarationClearsTheWarning(t *testing.T) {
	publishSchemaSubject(t)

	require.Error(t, refuseChangedSchema(externalBlock(schemaSubject), nil))
	_, raised := schemaChangedWarning(schemaSubject)
	require.True(t, raised)

	require.NoError(t, refuseChangedSchema(externalBlock(schemaSubject), []rpc.SchemaDecl{*schemaSubjectDecl()}))
	_, raised = schemaChangedWarning(schemaSubject)
	assert.False(t, raised, "the warning outlived the declaration that ended it")
}

// TestStage1SchemaCheckNeedsAPublishedSet: a process that published no
// external layer started no external plugin before its full parse. Method:
// judge a declaration with a module in a process that published nothing.
//
// VALIDATES: with no published set there is nothing a declaration differs
// from, and the check refuses nothing.
// PREVENTS: every test harness and every daemon with no declared external
// plugin refusing a plugin for a comparison it cannot make.
func TestStage1SchemaCheckNeedsAPublishedSet(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	_, fixed := yang.PublishedExternalModules()
	require.False(t, fixed)
	require.NoError(t, refuseChangedSchema(externalBlock(schemaSubject), []rpc.SchemaDecl{*schemaSubjectDecl()}))
}

// TestOnRegistrationRefusesChangedSchema: the comparison is reached by the
// Stage 1 of a real sink. Method: publish, build a server, and hand the
// sink's onRegistration a declaration with another module text.
//
// VALIDATES: the schema-unchanged entry of the Stage 1 table runs in
// onRegistration and its refusal is the answer the plugin reads.
// PREVENTS: the comparison existing with no caller in the handshake.
func TestOnRegistrationRefusesChangedSchema(t *testing.T) {
	publishSchemaSubject(t)
	s, _ := newLifecycleStartupServer(t)
	sink := &engineStartupSink{s: s, proc: process.NewProcess(externalBlock(schemaSubject))}

	declaration := validStage1Declaration()
	declaration.Schemas = []rpc.SchemaDecl{*schemaSubjectDecl()}
	declaration.Schemas[0].YANGText += "\n"

	err := sink.onRegistration(declaration)
	require.Error(t, err)
	refusal, operator := errors.AsType[*opererr.Error](err)
	require.True(t, operator, "onRegistration answered another refusal: %v", err)
	assert.Contains(t, refusal.Problem(), `plugin "acme" was started again with other YANG modules`)
	assert.Equal(t, "restart ze", refusal.Fix())
}
