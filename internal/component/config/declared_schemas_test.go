package config

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// These tests exercise SchemaWithDeclaredPlugins in both offline query mode
// (S-3 / AC-18: ZE_PLUGIN_MODE=declare without starting the run loop, failing
// closed when a program cannot declare) and daemon mode (fixed module set,
// never starting external programs). None runs in parallel: each resets the
// module set the process published.

// offlineProcess puts the process back to one that has published nothing and
// keeps no module set, and restores what it found.
func offlineProcess(t *testing.T) {
	t.Helper()
	t.Cleanup(ResetDeclaredSchemasForTest())
}

// markingPlugin writes, in a directory of its own, a program that appends a
// line to its `runs` file each time it is started, in any mode, and exits 0
// without writing a Stage 1 declaration. It answers the program's run string,
// the program's path and the path of the `runs` file.
func markingPlugin(t *testing.T) (run, program, runs string) {
	t.Helper()
	dir := t.TempDir()
	runs = filepath.Join(dir, "runs")
	program = filepath.Join(dir, "plugin.sh")
	script := "#!/bin/sh\necho run >> " + runs + "\n"
	require.NoError(t, os.WriteFile(program, []byte(script), 0o600))
	return "sh " + program, program, runs
}

// declaringPlugin writes a program that honors ZE_PLUGIN_MODE=declare by
// writing one valid Stage 1 declare-registration line for decl and exiting 0
// without touching its `runs` marker file. Any non-declare invocation appends
// to `runs` and exits 1, proving the run loop never ran.
func declaringPlugin(t *testing.T, decl rpc.DeclareRegistrationInput) (run, program, runs string) {
	t.Helper()
	if decl.ProtocolVersion == 0 {
		decl.ProtocolVersion = 1
	}
	payload, err := json.Marshal(decl)
	require.NoError(t, err)
	dir := t.TempDir()
	runs = filepath.Join(dir, "runs")
	declFile := filepath.Join(dir, "decl.line")
	line := append([]byte("#1 ze-plugin-engine:declare-registration "), payload...)
	line = append(line, '\n')
	require.NoError(t, os.WriteFile(declFile, line, 0o600))
	program = filepath.Join(dir, "plugin.sh")
	script := "#!/bin/sh\nif [ \"$ZE_PLUGIN_MODE\" = \"declare\" ]; then\n  cat " + declFile + "\n  exit 0\nfi\necho run >> " + runs + "\nexit 1\n"
	require.NoError(t, os.WriteFile(program, []byte(script), 0o600))
	return "sh " + program, program, runs
}

// runCount answers how many times a markingPlugin program was started.
func runCount(t *testing.T, runs string) int {
	t.Helper()
	data, err := os.ReadFile(runs) //nolint:gosec // a path this test built under t.TempDir
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	require.NoError(t, err)
	return strings.Count(string(data), "run\n")
}

// configDeclaring answers a config that declares external plugin name with
// run, followed by section.
func configDeclaring(name, run, section string) string {
	return "bus {\n\tendpoint " + name + " {\n\t\texternal {\n\t\t\trun \"" + run + "\"\n\t\t}\n\t\tencoding json\n\t}\n}\n\n" + section
}

// TestDeclaredSchemasStartsNoDeclaredProgram verifies S-3 / AC-18 offline query
// mode: a command that parses a config with no daemon queries each declared
// external program in declare mode (ZE_PLUGIN_MODE=declare), joins its declared
// YANG module, and NEVER starts its run loop.
//
// VALIDATES: S-3 / AC-18 — query mode publishes the plugin's declared schema,
// records Queried, leaves WithoutSchema empty, accepts valid values and refuses
// out-of-range values, and never touches the run-loop marker (with a non-declare
// control invocation proving the marker catches a real run-loop start).
// PREVENTS: offline commands starting an external program's run loop or failing
// to join a declare-mode plugin's YANG module offline.
func TestDeclaredSchemasStartsNoDeclaredProgram(t *testing.T) {
	offlineProcess(t)
	run, program, runs := declaringPlugin(t, rpc.DeclareRegistrationInput{
		Schemas: []rpc.SchemaDecl{{
			Module:    "acme-monitor-conf",
			Namespace: "urn:acme:monitor",
			YANGText:  joinedMonitorModule,
		}},
	})

	text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 5\n}\n")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)

	require.NotNil(t, schema.Get("acme-monitor"), "the schema must hold the queried plugin's module root")
	assert.Equal(t, []string{"acme"}, declared.Queried())
	assert.Empty(t, declared.WithoutSchema())
	modules, published := yang.PublishedExternalModules()
	assert.True(t, published, "the process's module set is recorded as fixed")
	require.Len(t, modules, 1)
	assert.Equal(t, "acme-monitor-conf", modules[0].Module)

	_, parseErr := NewParser(schema).Parse(text)
	require.NoError(t, parseErr, "a valid section of the queried plugin must parse offline")
	_, badErr := NewParser(schema).Parse(configDeclaring("acme", run, "acme-monitor {\n\tinterval 11\n}\n"))
	require.Error(t, badErr, "an out-of-range value in the queried plugin's module must be refused")
	assert.Contains(t, badErr.Error(), "interval")

	assert.Zero(t, runCount(t, runs), "query mode must never start the program's run loop")

	// Control: a non-declare invocation writes the marker and exits non-zero.
	require.Error(t, exec.CommandContext(t.Context(), "sh", program).Run())
	assert.Equal(t, 1, runCount(t, runs), "the marker must catch a non-declare start, or its absence above proves nothing")
}

// TestParseRefusalSaysTheSectionIsNotAvailableOffline verifies S-3 / AC-18
// offline failure handling: when a declared external program cannot declare in
// query mode (exits without a Stage 1 line, exits non-zero, or is missing),
// SchemaWithDeclaredPlugins refuses closed naming the endpoint, its run line,
// the cause, and `fix: ze show plugin conformance run "<run>"`. When a program
// declares no schema, its query succeeds without starting its run loop and an
// unknown root is refused as a plain unknown keyword.
//
// VALIDATES: S-3 / AC-18 / S16 — fail-closed offline query refusals carry one
// conformance fix line, and a schema-less declare-mode plugin is not blamed for
// unrelated unknown keywords.
// PREVENTS: silently accepting undeclared external programs offline or omitting
// the endpoint, run line, cause, or conformance fix.
func TestParseRefusalSaysTheSectionIsNotAvailableOffline(t *testing.T) {
	failDir := t.TempDir()
	failProg := filepath.Join(failDir, "fail.sh")
	require.NoError(t, os.WriteFile(failProg, []byte("#!/bin/sh\nexit 7\n"), 0o600))

	t.Run("no_declaration_line", func(t *testing.T) {
		offlineProcess(t)
		run, _, _ := markingPlugin(t)
		text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 5\n}\n")
		_, _, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
		require.Error(t, err)
		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok, "query failure must return *opererr.Error")
		msg := opErr.Error()
		lines := strings.Split(msg, "\n")
		assert.Contains(t, msg, `bus endpoint "acme"`)
		assert.Contains(t, msg, run)
		assert.Contains(t, msg, "without writing a Stage 1 declaration")
		assert.Equal(t, `fix: ze show plugin conformance run "`+run+`"`, lines[len(lines)-1])
		assert.Equal(t, 1, strings.Count(msg, "\nfix: "), "one fix line: %s", msg)
	})

	t.Run("nonzero_exit", func(t *testing.T) {
		offlineProcess(t)
		run := "sh " + failProg
		text := configDeclaring("acme", run, "")
		_, _, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
		require.Error(t, err)
		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok)
		assert.Contains(t, opErr.Error(), `bus endpoint "acme"`)
		assert.Contains(t, opErr.Error(), run)
		assert.Contains(t, opErr.Error(), "status 7")
		assert.Equal(t, `ze show plugin conformance run "`+run+`"`, opErr.Fix())
	})

	t.Run("missing_binary", func(t *testing.T) {
		offlineProcess(t)
		const run = "./no-such-acme-binary"
		text := configDeclaring("acme", run, "")
		_, _, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
		require.Error(t, err)
		opErr, ok := errors.AsType[*opererr.Error](err)
		require.True(t, ok)
		assert.Contains(t, opErr.Error(), `bus endpoint "acme"`)
		assert.Contains(t, opErr.Error(), run)
		assert.Contains(t, opErr.Error(), "not found")
		assert.Equal(t, `ze show plugin conformance run "`+run+`"`, opErr.Fix())
	})

	t.Run("declared_without_schema", func(t *testing.T) {
		offlineProcess(t)
		run, _, runs := declaringPlugin(t, rpc.DeclareRegistrationInput{})
		text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 5\n}\n")
		schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
		require.NoError(t, err)
		assert.Equal(t, []string{"acme"}, declared.Queried())
		assert.Empty(t, declared.WithoutSchema())
		assert.Zero(t, runCount(t, runs))

		_, parseErr := NewParser(schema).Parse(text)
		require.Error(t, parseErr)
		assert.Contains(t, parseErr.Error(), "unknown top-level keyword: acme-monitor")
		assert.Same(t, parseErr, declared.Explain(parseErr), "a queried plugin that declared no schema must not explain unknown keywords")
	})
}

// TestParseRefusalNamesEveryDeclaredPluginAndNoFile: inside a daemon whose
// module set is fixed without these plugins, the message lists each declared
// plugin once in config order and names no file it was not given.
//
// VALIDATES: both names in config order, the plural form, and "the config does
// not parse" for a text with no file (`Path: "-"`).
// PREVENTS: a message that names one plugin as the cause when the config
// declares several, and naming `-` as a file path.
func TestParseRefusalNamesEveryDeclaredPluginAndNoFile(t *testing.T) {
	startedDaemon(t)
	text := "bus {\n\tendpoint acme {\n\t\texternal {\n\t\t\trun \"./acme\"\n\t\t}\n\t\tencoding json\n\t}\n" +
		"\tendpoint acme-shell {\n\t\texternal {\n\t\t\trun \"./acme-shell\"\n\t\t}\n\t\tencoding json\n\t}\n}\n\nacme-monitor {\n\tinterval 5\n}\n"
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "-"})
	require.NoError(t, err)
	assert.Equal(t, []string{"acme", "acme-shell"}, declared.WithoutSchema())

	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr)
	message := declared.Explain(parseErr).Error()
	assert.Contains(t, message, `the config declares external plugins "acme", "acme-shell"`)
	assert.True(t, strings.HasSuffix(message, "\nfix: restart ze"), "daemon refusal ends with restart ze: %s", message)
	assert.True(t, strings.HasPrefix(message, "the config does not parse"), "a text with no file is named as a file: %s", message)
}

// TestDeclaredSchemasNoExternalPluginRunsNothing: a config that declares no
// external plugin costs nothing and changes nothing. Method: compare the
// schema with the one YANGSchema answered before.
//
// VALIDATES: AC-9 for the offline commands: the schema is the compiled-in one,
// the same value YANGSchema answered, and the process's set is recorded as
// holding no external module.
// PREVENTS: a second schema build, or a changed verdict, for every config
// that uses no external plugin.
func TestDeclaredSchemasNoExternalPluginRunsNothing(t *testing.T) {
	offlineProcess(t)
	before, err := YANGSchema()
	require.NoError(t, err)

	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: "set environment log level info\n"})
	require.NoError(t, err)

	assert.Same(t, before, schema, "a config with no external plugin got another schema")
	assert.Empty(t, declared.WithoutSchema())
	assert.Nil(t, declared.Unchecked("ze.conf"))
	modules, published := yang.PublishedExternalModules()
	assert.True(t, published, "the process's module set is not recorded as fixed")
	assert.Empty(t, modules)
	_, refusal := NewParser(schema).Parse("acme-monitor {\n\tinterval 5\n}\n")
	require.Error(t, refusal)
	assert.Same(t, refusal, declared.Explain(refusal), "a refusal gained text with no plugin to name")
	assert.NoError(t, declared.Explain(nil))
}

// TestDeclaredSchemasRunsNothingOnceTheSetIsFixed: a process whose module set
// is fixed, as a running daemon's is, starts no plugin to answer a command and
// names the restart. Method: record a keeper, as the daemon's plugin server
// does, then give the step a config that declares a plugin.
//
// VALIDATES: inside a daemon the step runs no program and publishes nothing;
// beside the refusal of an unknown name the declared plugin the daemon holds
// no module of is named with the restart, and the offline wording is absent.
// PREVENTS: a command served by a running daemon starting an operator's
// plugin, and a daemon telling its operator to ask a running ze.
func TestDeclaredSchemasRunsNothingOnceTheSetIsFixed(t *testing.T) {
	offlineProcess(t)
	run, _, runs := markingPlugin(t)
	yang.KeepsModuleSet("the plugin server of a test")
	require.True(t, yang.ModuleSetFixed())

	text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 5\n}\n")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)

	assert.Zero(t, runCount(t, runs), "a process whose module set is fixed ran the plugin")
	assert.Nil(t, schema.Get("acme-monitor"))
	_, published := yang.PublishedExternalModules()
	assert.False(t, published, "a process whose module set is fixed published")

	assert.Equal(t, []string{"acme"}, declared.WithoutSchema())
	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr)
	refusal := declared.ParseRefusal("ze.conf", parseErr, "the command did nothing")
	assert.Equal(t, "restart ze", refusal.Fix())
	assert.Contains(t, refusal.Error(), `external plugin "acme"`)
	assert.Contains(t, refusal.Error(), "this ze holds no config schema")
	assert.Contains(t, refusal.Error(), "unknown top-level keyword: acme-monitor")
	assert.NotContains(t, refusal.Error(), "starts no plugin", "a daemon answered with the wording of a command with no daemon")
	assert.NotContains(t, refusal.Error(), "ze cli")
}

// TestDeclaredSchemasAnswersFromThePublishedSet: in a process that published a
// declared plugin's module, as a daemon does at boot from the plugin's live
// Stage 1, the step names nothing and the section is judged by that module.
// Method: publish joinedMonitorModule for plugin acme, then the boundary
// values of its range 1..10.
//
// VALIDATES: AC-2 and AC-10 at the shared step for a served command: the
// schema holds the plugin's root, accepts 1 and 10 and refuses 0 and 11, and
// the plugin is not listed as one ze holds no schema of.
// PREVENTS: a daemon refusing, or naming the restart for, a section of a
// plugin it started with.
func TestDeclaredSchemasAnswersFromThePublishedSet(t *testing.T) {
	offlineProcess(t)
	run, _, runs := markingPlugin(t)
	require.NoError(t, PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: joinedMonitorModule,
	}}))

	text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 10\n}\n")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)

	assert.Zero(t, runCount(t, runs))
	assert.Empty(t, declared.WithoutSchema(), "a plugin whose module the process holds is listed as without schema")
	require.NotNil(t, schema.Get("acme-monitor"), "the schema misses the root of the published module")
	for _, value := range []string{"1", "10"} {
		_, parseErr := NewParser(schema).Parse(configDeclaring("acme", run, "acme-monitor {\n\tinterval "+value+"\n}\n"))
		assert.NoError(t, parseErr, "interval %s is inside range 1..10", value)
	}
	for _, value := range []string{"0", "11"} {
		_, parseErr := NewParser(schema).Parse(configDeclaring("acme", run, "acme-monitor {\n\tinterval "+value+"\n}\n"))
		require.Error(t, parseErr, "interval %s is outside range 1..10", value)
		assert.Contains(t, parseErr.Error(), "interval")
		assert.Same(t, parseErr, declared.Explain(parseErr), "a range refusal gained a plugin")
	}
}

// mandatoryMonitorModule is an external module whose root, once present, owes
// the leaf target.
const mandatoryMonitorModule = `module acme-probe-conf {
    namespace "urn:acme:probe";
    prefix probe;
    container acme-probe {
        presence "The acme probe is configured";
        leaf target { type string; mandatory true; }
        leaf interval { type uint8; }
    }
}`

// TestPublishedExternalModuleOwesItsMandatoryLeaf: the full load refuses a
// section of an external plugin that lacks a leaf the plugin's module marks
// mandatory, as it does for a compiled-in module. The check is the YANG
// validator's (yang/validator.go, validator_mandatory.go), which the load runs
// after the parse, so the parser alone does not show it. Method: publish the
// module as boot does, then ReadConfig, the load a daemon-served command takes,
// with and without the leaf.
//
// VALIDATES: AC-10 for a mandatory leaf: the section with target loads, and
// the section without it is refused naming the leaf.
// PREVENTS: the mandatory check not reaching a joined external module, so a
// plugin is configured without a value its module requires.
func TestPublishedExternalModuleOwesItsMandatoryLeaf(t *testing.T) {
	offlineProcess(t)
	require.NoError(t, PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme-probe", Module: "acme-probe-conf", Namespace: "urn:acme:probe", Text: mandatoryMonitorModule,
	}}))

	_, err := ReadConfig("acme-probe {\n\ttarget probe-a\n\tinterval 5\n}\n", "ze.conf", nil)
	require.NoError(t, err, "a section that holds its mandatory leaf is refused")

	_, err = ReadConfig("acme-probe {\n\tinterval 5\n}\n", "ze.conf", nil)
	require.Error(t, err, "a section without the leaf its module marks mandatory was loaded")
	assert.Contains(t, err.Error(), `mandatory field "target" is missing`)
}

// TestDeclaredSchemasSecondCallAnswersTheSame: a command that reads its config
// twice gets the same answer twice without re-querying or starting the run loop.
//
// VALIDATES: the second call answers the same schema, Offline()==true, and the
// same Queried list, although the first call fixed the process's module set.
// PREVENTS: `ze doctor`, which reads its config twice, losing offline state or
// re-executing external plugins on the second read.
func TestDeclaredSchemasSecondCallAnswersTheSame(t *testing.T) {
	offlineProcess(t)
	run, _, runs := declaringPlugin(t, rpc.DeclareRegistrationInput{})
	source := ConfigSource{Text: configDeclaring("acme", run, ""), Path: "ze.conf"}

	first, _, err := SchemaWithDeclaredPlugins(source)
	require.NoError(t, err)
	second, declared, err := SchemaWithDeclaredPlugins(source)
	require.NoError(t, err)

	assert.Same(t, first, second)
	assert.Equal(t, []string{"acme"}, declared.Queried())
	assert.Empty(t, declared.WithoutSchema())
	assert.True(t, declared.Offline(), "the second read answers as offline")
	assert.Zero(t, runCount(t, runs))
}

// TestDeclaredSchemasTakesPluginsOfEverySource: a command that reads two
// configs queries the external plugins of both in source order without starting
// either run loop.
//
// VALIDATES: both plugins are queried in source order and joined into the
// schema so both sides of `ze config diff` parse cleanly offline.
// PREVENTS: the second side of a diff missing its plugin's schema.
func TestDeclaredSchemasTakesPluginsOfEverySource(t *testing.T) {
	offlineProcess(t)
	monitorRun, _, monitorRuns := declaringPlugin(t, rpc.DeclareRegistrationInput{
		Schemas: []rpc.SchemaDecl{{
			Module:    "acme-monitor-conf",
			Namespace: "urn:acme:monitor",
			YANGText:  joinedMonitorModule,
		}},
	})
	shellRun, _, shellRuns := declaringPlugin(t, rpc.DeclareRegistrationInput{})

	schema, declared, err := SchemaWithDeclaredPlugins(
		ConfigSource{Text: configDeclaring("acme", monitorRun, ""), Path: "before.conf"},
		ConfigSource{Text: configDeclaring("acme-shell", shellRun, ""), Path: "after.conf"},
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"acme", "acme-shell"}, declared.Queried())
	assert.Empty(t, declared.WithoutSchema())
	require.NotNil(t, schema.Get("acme-monitor"))
	assert.Zero(t, runCount(t, monitorRuns))
	assert.Zero(t, runCount(t, shellRuns))
}

// TestDeclaredSchemasSkipsSourceTheFirstPassRefuses: a config whose `plugin`
// block the first pass refuses declares nothing to the step, and the full
// parse gives the refusal. Method: an unknown leaf inside the plugin block.
//
// VALIDATES: the step answers no error and the compiled-in schema, and the
// parser over that schema refuses the same block.
// PREVENTS: two wordings for one mistake, one from the step and one from the
// parse, and a config mistake reported as a schema failure.
func TestDeclaredSchemasSkipsSourceTheFirstPassRefuses(t *testing.T) {
	offlineProcess(t)
	const text = "bus {\n\tno-such-leaf yes\n}\n"

	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)

	assert.Empty(t, declared.WithoutSchema())
	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr, "the full parse accepts what the first pass refused")
	assert.Contains(t, parseErr.Error(), "no-such-leaf")
}

// TestDeclaredSchemasExplainsOnlyUnknownNames: the declared plugins are named
// only beside the refusal of a name the schema does not define. Method: one
// declared plugin, then three refusals through Explain: an unknown top-level
// keyword, an unknown field of a known section, and a refusal that is about
// no unknown name.
//
// VALIDATES: RefusesUnknownNode is true for the parser's refusal of an unknown
// root and of an unknown field, and Explain adds the plugins to those two
// alone.
// PREVENTS: every parse refusal of a config that declares an external plugin
// naming that plugin, which sends the operator to a plugin the mistake has
// nothing to do with.
func TestDeclaredSchemasExplainsOnlyUnknownNames(t *testing.T) {
	startedDaemon(t)
	declaring := configDeclaring("acme", "./acme-monitor", "")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: declaring, Path: filepath.Join(t.TempDir(), "ze.conf")})
	require.NoError(t, err)
	require.Len(t, declared.WithoutSchema(), 1)

	_, unknownRoot := NewParser(schema).Parse("acme-monitor {\n\tinterval 5\n}\n")
	require.Error(t, unknownRoot)
	assert.True(t, RefusesUnknownNode(unknownRoot), "an unknown top-level keyword is not marked: %v", unknownRoot)
	assert.Contains(t, declared.Explain(unknownRoot).Error(), `external plugin "acme"`)

	_, unknownField := NewParser(schema).Parse("bus {\n\tno-such-leaf yes\n}\n")
	require.Error(t, unknownField)
	assert.True(t, RefusesUnknownNode(unknownField), "an unknown field of a known section is not marked: %v", unknownField)
	assert.Contains(t, declared.Explain(unknownField).Error(), `external plugin "acme"`)

	other := errors.New("line 2: missing closing brace")
	assert.False(t, RefusesUnknownNode(other))
	assert.Same(t, other, declared.Explain(other), "a refusal about no unknown name gained a plugin")
}

// TestFreeformUnknownFieldIsAnUnknownName: an unknown field in the block of a
// flex node, and in the block of an inline list entry, is marked as the
// refusal of a name the schema does not define, like one in a container.
// Method: a hand-built schema with one flex node and one inline list, and one
// block of each that holds a field the node does not define.
//
// VALIDATES: RefusesUnknownNode is true for both refusals, and each keeps the
// text the parser gave it.
// PREVENTS: DeclaredSchemas.Explain staying silent about a plugin ze holds no
// schema of when the name ze does not know sits in a flex block or in an
// inline list entry.
func TestFreeformUnknownFieldIsAnUnknownName(t *testing.T) {
	schema := NewSchema()
	schema.Define("vpls", Flex(Field("rd", Leaf(TypeString))))
	schema.Define("route", InlineList(TypeString, Field("next-hop", Leaf(TypeString))))

	_, flexRefusal := NewParser(schema).Parse("vpls {\n\tno-such-field yes;\n}\n")
	require.Error(t, flexRefusal)
	assert.True(t, RefusesUnknownNode(flexRefusal), "an unknown field of a flex block is not marked: %v", flexRefusal)
	assert.Contains(t, flexRefusal.Error(), "line 2: unknown field in vpls: no-such-field")

	_, listRefusal := NewParser(schema).Parse("route alpha {\n\tno-such-field yes;\n}\n")
	require.Error(t, listRefusal)
	assert.True(t, RefusesUnknownNode(listRefusal), "an unknown field of an inline list entry is not marked: %v", listRefusal)
	assert.Contains(t, listRefusal.Error(), "line 2: unknown field in route: no-such-field")
}

// TestParseRefusalNamesOneFix: the operator error a command prints for a
// config that does not parse ends with one fix, chosen by what was refused.
// Method: one declared plugin, then three refusals through ParseRefusal: an
// unknown top-level keyword, a refusal that is about no unknown name, and a
// refusal that already holds an operator error.
//
// VALIDATES: beside an unknown name the fix is the command that reaches a
// running ze and the plugin is named; any other refusal gets `ze config
// validate <path>` and names no plugin; an operator error is answered as it
// is.
// PREVENTS: a parse refusal printed with two fix lines, or with a plugin the
// mistake has nothing to do with.
func TestParseRefusalNamesOneFix(t *testing.T) {
	startedDaemon(t)
	declaring := configDeclaring("acme", "./acme-monitor", "")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: declaring, Path: filepath.Join(t.TempDir(), "ze.conf")})
	require.NoError(t, err)

	_, unknownRoot := NewParser(schema).Parse("acme-monitor {\n\tinterval 5\n}\n")
	require.Error(t, unknownRoot)
	explained := declared.ParseRefusal("ze.conf", unknownRoot, "the command did nothing")
	assert.Equal(t, "restart ze", explained.Fix())
	assert.Contains(t, explained.Error(), "config ze.conf does not parse")
	assert.Contains(t, explained.Error(), `external plugin "acme"`)
	assert.Contains(t, explained.Error(), "acme-monitor")
	assert.Equal(t, 1, strings.Count(explained.Error(), "\nfix: "), "one fix line: %s", explained.Error())

	other := errors.New("line 2: missing closing brace")
	plain := declared.ParseRefusal("ze.conf", other, "the command did nothing")
	assert.Equal(t, "ze config validate ze.conf", plain.Fix())
	assert.Contains(t, plain.Error(), "the command did nothing: line 2: missing closing brace")
	assert.NotContains(t, plain.Error(), `"acme"`)

	held := opererr.New("the value is out of range", "ze accepts 1 to 10", "set interval 5")
	assert.Same(t, held, declared.ParseRefusal("ze.conf", held, "the command did nothing"))
}

// startedDaemon makes the process one a daemon booted in: its module set is
// kept, and the external plugins it started are recorded with what each
// declared. Each plugin is recorded as the first pass answers it for
// configDeclaring.
func startedDaemon(t *testing.T, started ...StartedPlugin) {
	t.Helper()
	offlineProcess(t)
	yang.KeepsModuleSet("the plugin server of a test")
	require.True(t, yang.ModuleSetFixed())
	RecordStartedPlugins(started)
}

// declarationOf answers the declaration the first pass reads for the plugin
// configDeclaring declares with name and run.
func declarationOf(t *testing.T, name, run string) plugin.PluginConfig {
	t.Helper()
	firstPass, err := FirstPass(configDeclaring(name, run, ""), "ze.conf", nil)
	require.NoError(t, err)
	require.Len(t, firstPass.External, 1)
	return firstPass.External[0]
}

// TestDeclaredSchemasDoesNotNameAStartedPluginThatDeclaredNoModule: inside a
// daemon, a plugin it started that declared no YANG module defines no config
// section, so it explains no refused name. Method: record such a plugin, then
// give the step a config that declares it and holds an unknown root.
//
// VALIDATES: the plugin is not listed, and the refusal of the unknown name is
// the parser's own, with no "restart ze" beside what is a plain mistake.
// PREVENTS: a daemon answering every typo in a config that declares a
// schema-less plugin with the restart of ze.
func TestDeclaredSchemasDoesNotNameAStartedPluginThatDeclaredNoModule(t *testing.T) {
	startedDaemon(t, StartedPlugin{Declaration: declarationOf(t, "observer", "./observer"), Schema: SchemaNone})

	text := configDeclaring("observer", "./observer", "acme-monitor {\n\tinterval 5\n}\n")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)

	assert.Empty(t, declared.WithoutSchema(), "a started plugin that declared no module was named")
	assert.Nil(t, declared.Unchecked("ze.conf"))
	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr)
	assert.Equal(t, parseErr, declared.Explain(parseErr), "the refusal was explained by a plugin that defines no section")
	refusal := declared.ParseRefusal("ze.conf", parseErr, "the command did nothing")
	assert.NotEqual(t, "restart ze", refusal.Fix())
	assert.NotContains(t, refusal.Error(), `external plugin "observer"`)
}

// TestDeclaredSchemasNamesThePluginsThatCouldExplain: the record narrows the
// list to the plugins a restart could bring a schema from. Method: one daemon
// that started three plugins, one for each outcome, and a config that
// declares them, a fourth the daemon never started, and the schema-less one
// under another run line.
//
// VALIDATES: named are the plugin the daemon did not start, the one that gave
// no declaration, and the schema-less one when its declaration is not the one
// the daemon started with; not named are the schema-less one as started and
// the one whose module the daemon holds.
// PREVENTS: the record hiding a plugin whose declaration the config changed,
// which a restart would run and which may then declare a schema.
func TestDeclaredSchemasNamesThePluginsThatCouldExplain(t *testing.T) {
	startedDaemon(t,
		StartedPlugin{Declaration: declarationOf(t, "observer", "./observer"), Schema: SchemaNone},
		StartedPlugin{Declaration: declarationOf(t, "silent", "./silent"), Schema: SchemaUnread},
	)

	_, declared, err := SchemaWithDeclaredPlugins(
		ConfigSource{Text: configDeclaring("observer", "./observer", ""), Path: "observer.conf"},
		ConfigSource{Text: configDeclaring("silent", "./silent", ""), Path: "silent.conf"},
		ConfigSource{Text: configDeclaring("late", "./late", ""), Path: "late.conf"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"silent", "late"}, declared.WithoutSchema())

	otherRun := configDeclaring("observer", "./observer --with-schema", "")
	_, declared, err = SchemaWithDeclaredPlugins(ConfigSource{Text: otherRun, Path: "ze.conf"})
	require.NoError(t, err)
	assert.Equal(t, []string{"observer"}, declared.WithoutSchema(),
		"a declaration the daemon did not start with was taken for the started plugin")
}

// TestDeclaredSchemasWithoutARecordNamesEveryPlugin: a process that recorded
// no started plugin cannot tell which declared plugin defines a section.
// Method: a process whose set is fixed and that recorded nothing.
//
// VALIDATES: with no record every declared plugin the process holds no module
// of is named, as before the record existed.
// PREVENTS: an absent record read as "every plugin declared no module".
func TestDeclaredSchemasWithoutARecordNamesEveryPlugin(t *testing.T) {
	startedDaemon(t)

	text := configDeclaring("observer", "./observer", "")
	_, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)
	assert.Equal(t, []string{"observer"}, declared.WithoutSchema())
}

// TestRecordCommandLinePluginsClonesAndResets proves the process-wide
// command-line plugin record isolates caller and reader slices, records empty
// lists cleanly, and participates in ResetDeclaredSchemasForTest.
//
// VALIDATES: AC-18 record semantics: input/output slice mutations never alias
// the stored record, nil/empty clears prior entries, and
// ResetDeclaredSchemasForTest clears and restores the recorded list.
// PREVENTS: Shared backing-array mutation across doors or leaked test state.
func TestRecordCommandLinePluginsClonesAndResets(t *testing.T) {
	saved := CommandLinePlugins()
	t.Cleanup(func() { RecordCommandLinePlugins(saved) })

	input := []string{"ze.flowspec-firewall", "ze.hostname"}
	RecordCommandLinePlugins(input)
	input[0] = "mutated-input"

	got := CommandLinePlugins()
	assert.Equal(t, []string{"ze.flowspec-firewall", "ze.hostname"}, got)

	got[1] = "mutated-output"
	assert.Equal(t, []string{"ze.flowspec-firewall", "ze.hostname"}, CommandLinePlugins())

	restore := ResetDeclaredSchemasForTest()
	assert.Empty(t, CommandLinePlugins(), "ResetDeclaredSchemasForTest must clear command-line plugins")
	restore()
	assert.Equal(t, []string{"ze.flowspec-firewall", "ze.hostname"}, CommandLinePlugins(), "restore must put command-line plugins back")

	RecordCommandLinePlugins(nil)
	assert.Empty(t, CommandLinePlugins())

	RecordCommandLinePlugins([]string{"ze.flowspec-firewall"})
	RecordCommandLinePlugins([]string{})
	assert.Empty(t, CommandLinePlugins())
}
