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

// probeCommandsText is a -cmd module of an external plugin with two commands.
const probeCommandsText = `module acme-probe-cmd {
    namespace "urn:acme:probe:cmd";
    prefix probecmd;
    import ze-extensions { prefix ze; }
    container show {
        config false;
        container acme-probe {
            config false;
            container status {
                config false;
                ze:command "probecmd:show-status";
            }
            container target {
                config false;
                ze:command "probecmd:show-target";
                leaf name { type string; }
            }
        }
    }
}`

// The two commands probeCommandsText declares, as an operator types them.
const (
	probeStatus = "show acme-probe status"
	probeTarget = "show acme-probe target"
)

// unservedProblem is the start of the refusal of a node no command reaches.
const unservedProblem = `plugin "acme" brings command `

// probeCommandsModule is the declaration of probeCommandsText by the plugin
// named name.
func probeCommandsModule(name string) yang.ExternalModule {
	return yang.ExternalModule{Plugin: name, Module: "acme-probe-cmd", Namespace: "urn:acme:probe:cmd", Text: probeCommandsText}
}

// probeCommandsDecl is probeCommandsText as a Stage 1 schema declaration.
func probeCommandsDecl() rpc.SchemaDecl {
	module := probeCommandsModule(schemaSubject)
	return rpc.SchemaDecl{Module: module.Module, Namespace: module.Namespace, YANGText: module.Text}
}

// publishProbeCommands makes the process one that booted with schemaSubject
// declaring its config module and probeCommandsText, and schemaSilent
// declaring none.
func publishProbeCommands(t *testing.T) {
	t.Helper()
	t.Cleanup(yang.ResetPublishedModulesForTest())
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	decl := schemaSubjectDecl()
	modules := []yang.ExternalModule{
		{Plugin: schemaSubject, Module: decl.Module, Namespace: decl.Namespace, Text: decl.YANGText},
		probeCommandsModule(schemaSubject),
	}
	require.NoError(t, yang.PublishExternalModules(modules, yang.NewValidatorRegistry()))
}

// declared answers the Stage 1 command declarations of names.
func declared(names ...string) []rpc.CommandDecl {
	commands := make([]rpc.CommandDecl, 0, len(names))
	for _, name := range names {
		commands = append(commands, rpc.CommandDecl{Name: name})
	}
	return commands
}

// declaring answers a Stage 1 declaration that brings probeCommandsText and
// declares the commands names.
func declaring(names ...string) *rpc.DeclareRegistrationInput {
	return &rpc.DeclareRegistrationInput{
		ProtocolVersion: rpc.ProtocolVersion,
		Schemas:         []rpc.SchemaDecl{*schemaSubjectDecl(), probeCommandsDecl()},
		Commands:        declared(names...),
	}
}

// requireUnserved asserts err is the refusal of the node path.
func requireUnserved(t *testing.T, err error, path string) {
	t.Helper()
	require.Error(t, err)
	refusal, ok := errors.AsType[*opererr.Error](err)
	require.True(t, ok, "the refusal is an operator error: %v", err)
	assert.Equal(t, unservedProblem+`"`+path+`" in module "acme-probe-cmd", `+
		`and declares no command at Stage 1 that reaches it`, refusal.Problem())
	assert.Contains(t, refusal.Fix(), `add "`+path+`" to the commands that plugin "acme" declares at Stage 1`)
}

// otherPlugin answers a process of another external plugin, named name.
func otherPlugin(name string) *process.Process {
	return process.NewProcess(externalBlock(name))
}

// TestStage1RefusesACommandNodeNoDeclaredCommandReaches: a plugin whose -cmd
// module holds a ze:command node that none of its Stage 1 commands reaches is
// refused, naming the plugin, the command and the module; a node reached by an
// equal command, or by a declared command it starts with on a word boundary,
// is served.
//
// VALIDATES: the Stage 1 check command-nodes-served holds an external plugin
// to the rule `./le doc yang-contract command-contract` holds the compiled-in
// modules to: every ze:command node has a handler.
// PREVENTS: a command of a plugin's own module that answers "not registered".
// MUTATION: dropping the word boundary in commandReaches makes the
// "show acme-pro" case pass, and this test red.
func TestStage1RefusesACommandNodeNoDeclaredCommandReaches(t *testing.T) {
	publishProbeCommands(t)

	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, probeTarget), programAlone()))
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring("show acme-probe"), programAlone()),
		"a declared command a node starts with reaches it, as the dispatcher routes it")

	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, "show acme-probe targets"),
		programAlone()), probeTarget)
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring("show acme-pro"), programAlone()),
		probeStatus)
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(), programAlone()), probeStatus)
}

// TestStage1CommandNodeNeedsANameRegisterAccepts: a declared name that
// CommandRegistry.Register refuses at Stage 5 serves no node, whatever it
// spells: a name with leading or trailing whitespace, a name the name rule
// refuses (upper case), and a name a builtin holds.
//
// VALIDATES: the check counts only a name the dispatcher will route
// (commandNameRefusal, the predicate Register applies).
// PREVENTS: a node called served by a name Register drops with a warning,
// which then answers "not registered" while the check passed.
func TestStage1CommandNodeNeedsANameRegisterAccepts(t *testing.T) {
	publishProbeCommands(t)

	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(" "+probeStatus, probeTarget),
		programAlone()), probeStatus)
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus+" ", probeTarget),
		programAlone()), probeStatus)
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring("SHOW ACME-PROBE STATUS", probeTarget),
		programAlone()), probeStatus)

	held := newCommandRegistry()
	held.AddBuiltin(probeStatus)
	builtin := stage1Site{routing: held.routingBeside(schemaSubject)}
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, probeTarget), builtin),
		probeStatus)
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, probeTarget), programAlone()),
		"the same declaration is served where no builtin holds the name")
}

// TestStage1CommandNodeGoesToTheLongestCommand: when another plugin
// registered a longer command that reaches a node, the dispatcher routes the
// node there, so the plugin that brought it does not serve it; a shorter
// command of the other plugin leaves the node to the plugin's longer one. The
// registry routingBeside reads leaves the plugin's own commands out.
//
// VALIDATES: the check and matchPluginCommand pick the node's command by one
// rule (longestReach) over the same names.
// PREVENTS: a check that accepts acme's `show acme-probe` while every
// `show acme-probe status` line reaches the other plugin.
func TestStage1CommandNodeGoesToTheLongestCommand(t *testing.T) {
	publishProbeCommands(t)

	longer := newCommandRegistry()
	longer.Register(otherPlugin("other"), []CommandDef{{Name: probeStatus}})
	longer.Register(otherPlugin(schemaSubject), []CommandDef{{Name: "show acme-probe"}})
	site := stage1Site{routing: longer.routingBeside(schemaSubject)}
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring("show acme-probe"), site), probeStatus)
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, probeTarget), site), probeStatus)

	shorter := newCommandRegistry()
	shorter.Register(otherPlugin("other"), []CommandDef{{Name: "show acme-probe"}})
	// The plugin's own earlier registration, as before a restart, holds no
	// name against it.
	shorter.Register(otherPlugin(schemaSubject), []CommandDef{{Name: probeStatus}})
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, probeTarget),
		stage1Site{routing: shorter.routingBeside(schemaSubject)}))

	// The dispatcher routes the same line to the same plugin.
	d := NewDispatcher()
	d.registry = longer
	matched, _ := d.matchPluginCommand(probeStatus)
	require.NotNil(t, matched)
	assert.Equal(t, "other", matched.Process.Config().Name)
}

// TestStage1CommandNodesAtTheDaemonReadItsRegistry: the sink's site holds
// what the daemon's command registry holds beside the plugin, so a node
// another plugin's longer command takes is refused in the daemon.
//
// VALIDATES: engineStartupSink.site feeds the registry to the check.
// PREVENTS: the daemon judging every declaration as a checker does.
func TestStage1CommandNodesAtTheDaemonReadItsRegistry(t *testing.T) {
	publishProbeCommands(t)
	s, _ := newLifecycleStartupServer(t)
	s.dispatcher.Registry().Register(otherPlugin("other"), []CommandDef{{Name: probeStatus}})

	sink := &engineStartupSink{s: s, proc: process.NewProcess(externalBlock(schemaSubject))}
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring("show acme-probe"), sink.site()), probeStatus)

	compiledIn := &engineStartupSink{s: s, proc: process.NewProcess(plugin.PluginConfig{Name: schemaSubject, Internal: true})}
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring(), compiledIn.site()),
		"a compiled-in plugin's modules are judged when ze is built")
}

// TestStage1CommandNodesJoinTheDeclaredModulesWithNoPublish: a process that
// published no external layer, as a checker, joins the modules the
// declaration carries and judges their nodes; a module the join refuses
// answers the join's refusal and claims no unserved node.
//
// VALIDATES: the offline road of commandModules.
// PREVENTS: `ze plugin conformance` passing a declaration whose module has a
// node no command reaches, and a join refusal reported as an unserved node.
func TestStage1CommandNodesJoinTheDeclaredModulesWithNoPublish(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())

	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus, probeTarget), programAlone()))
	requireUnserved(t, refuseUnservedCommandNodes(schemaSubject, declaring(probeStatus), programAlone()), probeTarget)

	broken := &rpc.DeclareRegistrationInput{
		Schemas:  []rpc.SchemaDecl{{Module: "acme-broken-cmd", Namespace: "urn:acme:broken:cmd", YANGText: "module acme-broken-cmd {"}},
		Commands: declared(probeStatus),
	}
	err := refuseUnservedCommandNodes(schemaSubject, broken, programAlone())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "declares no command at Stage 1", "a join refusal is not an unserved node")
}

// TestStage1CommandNodesNeedAModuleThePluginBrought: a plugin compiled into
// ze, an external plugin that brought no module, and a declared command with
// no node of its own are never refused.
//
// VALIDATES: the negative half of the check: what a compiled-in plugin may do
// (a Stage 1 command with no ze:command node) an external one may do too.
// PREVENTS: a check stricter for an external plugin than for a compiled-in one.
func TestStage1CommandNodesNeedAModuleThePluginBrought(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, &rpc.DeclareRegistrationInput{}, programAlone()),
		"a declaration with no module, judged with no publish, has no node")

	publishProbeCommands(t)
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject, declaring(), stage1Site{compiledIn: true}))
	require.NoError(t, refuseUnservedCommandNodes(schemaSilent,
		&rpc.DeclareRegistrationInput{Commands: declared("show beta status")}, programAlone()),
		"a plugin that brought no module declares its commands as before")
	require.NoError(t, refuseUnservedCommandNodes(schemaSubject,
		declaring(probeStatus, probeTarget, "request acme-probe reset"), programAlone()),
		"a declared command with no ze:command node is a Stage 1 command, as a compiled-in plugin may declare")
}

// TestSchemalessRefusalNamesTheCommandModules: for a plugin whose published
// modules add only commands, the refusal beside a name no schema defines says
// which modules it brought and that none defines config; for a plugin that
// brought none it says that it declared none.
//
// VALIDATES: defect (a) of the phase 11 command step, the text an operator and
// `ze doctor` read.
// PREVENTS: a message that says a plugin declared no YANG module when it
// declared a command module.
func TestSchemalessRefusalNamesTheCommandModules(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())
	require.NoError(t, yang.PublishExternalModules([]yang.ExternalModule{probeCommandsModule("commands-only")},
		yang.NewValidatorRegistry()))

	commandsOnly := SchemalessRefusal(externalBlock("commands-only"))
	assert.Equal(t, `external plugin "commands-only" declared YANG modules that define commands and no config `+
		`("acme-probe-cmd"), so it defines no config section`, commandsOnly.Problem())
	assert.Equal(t, commandsOnlyWhy, commandsOnly.Why())

	none := SchemalessRefusal(externalBlock("no-schema"))
	assert.Equal(t, `external plugin "no-schema" declared no YANG module, so it defines no config section`, none.Problem())
}

// TestDeclaresConfigReadsTheDeclaredModules: a declaration with a -conf module
// declares config, one with only a -cmd module does not, and one with no
// module text does not.
//
// VALIDATES: the predicate cmd/ze/hub schemaOutcome reads.
// PREVENTS: a command-only plugin recorded as SchemaDeclared.
func TestDeclaresConfigReadsTheDeclaredModules(t *testing.T) {
	commandDecl := probeCommandsDecl()

	config, err := DeclaresConfig(schemaSubject, []rpc.SchemaDecl{*schemaSubjectDecl(), commandDecl})
	require.NoError(t, err)
	assert.True(t, config, "a -conf module and a -cmd module declare config")

	config, err = DeclaresConfig(schemaSubject, []rpc.SchemaDecl{commandDecl})
	require.NoError(t, err)
	assert.False(t, config, "a -cmd module alone declares no config")

	config, err = DeclaresConfig(schemaSubject, []rpc.SchemaDecl{{Module: "acme-monitor-conf"}})
	require.NoError(t, err)
	assert.False(t, config, "an entry with no text brings no module")
}
