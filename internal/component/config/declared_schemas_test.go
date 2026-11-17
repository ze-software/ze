package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/opererr"
)

// These tests give the shared step a config that declares an external plugin
// whose run string is a real program: a shell script that appends a line to a
// marker file each time it is started. The step must never start it. None
// runs in parallel: each resets the module set the process published.

// offlineProcess puts the process back to one that has published nothing and
// keeps no module set, and restores what it found.
func offlineProcess(t *testing.T) {
	t.Helper()
	t.Cleanup(ResetDeclaredSchemasForTest())
}

// markingPlugin writes, in a directory of its own, a program that appends a
// line to its `runs` file each time it is started, in any mode. It answers
// the program's run string, the program's path and the path of the `runs`
// file.
func markingPlugin(t *testing.T) (run, program, runs string) {
	t.Helper()
	dir := t.TempDir()
	runs = filepath.Join(dir, "runs")
	program = filepath.Join(dir, "plugin.sh")
	script := "#!/bin/sh\necho run >> " + runs + "\n"
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
	return "plugin {\n\texternal " + name + " {\n\t\trun \"" + run + "\"\n\t\tencoder json\n\t}\n}\n\n" + section
}

// TestDeclaredSchemasStartsNoDeclaredProgram: a command that parses a config
// with no daemon never starts a program the config's plugin block names.
// Method: a run string whose program leaves a marker when started, the step,
// the full parse and the refusal; then the program is started by the test, as
// the control that the marker would have shown a start.
//
// VALIDATES: the owner ruling "plugin command requiring external plugins which
// can not be started from the code when no daemon is running to pass it to are
// not available": the marker is absent after the step and the parse, the
// schema is the compiled-in one, the plugin's section is refused as an unknown
// name, and the plugin is remembered by name.
// PREVENTS: inspecting a config someone else wrote running their command.
func TestDeclaredSchemasStartsNoDeclaredProgram(t *testing.T) {
	offlineProcess(t)
	run, program, runs := markingPlugin(t)
	before, err := YANGSchema()
	require.NoError(t, err)

	text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 5\n}\n")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)

	assert.Same(t, before, schema, "the step answers another schema than the compiled-in one")
	assert.Nil(t, schema.Get("acme-monitor"), "a root only the plugin's schema defines is known with no daemon")
	assert.Equal(t, []string{"acme"}, declared.WithoutSchema())
	modules, published := yang.PublishedExternalModules()
	assert.True(t, published, "the process's module set is not recorded as fixed")
	assert.Empty(t, modules)

	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr, "a section ze could not check was accepted")
	assert.Contains(t, parseErr.Error(), "unknown top-level keyword: acme-monitor")
	require.NotNil(t, declared.ParseRefusal("ze.conf", parseErr, "the command did nothing"))

	assert.Zero(t, runCount(t, runs), "a command with no daemon started the declared program")

	// Control: the marker shows a start when the program is started.
	require.NoError(t, exec.CommandContext(t.Context(), "sh", program).Run())
	assert.Equal(t, 1, runCount(t, runs), "the marker does not show a start, so its absence above proves nothing")
}

// TestParseRefusalSaysTheSectionIsNotAvailableOffline: the refusal of a name
// only an external plugin's schema could define says what is true with no
// daemon, and names the command that reaches a running ze. Method: one
// declared plugin, the refusal of its section through ParseRefusal, Explain
// and Unchecked.
//
// VALIDATES: the message holds the parser's refusal, the declared plugin's
// name, that ze reads that schema when `ze start` runs the plugin, that this
// command starts no plugin, that only a running ze checks such a section and
// that this command asked none; its fix is `ze config validate <file>`, which
// asks one, alone on the last line, once. It claims no plugin as the owner of
// the refused name.
// PREVENTS: a bare "unknown top-level keyword", a fix that runs the plugin, a
// fix (`ze cli -c "show config validation ..."`) that is answered in the
// caller's own process and gives the same refusal again, and the session form
// `show config validation <file>`, which the daemon refuses for a file outside
// its config directories (command.CallerPath).
func TestParseRefusalSaysTheSectionIsNotAvailableOffline(t *testing.T) {
	offlineProcess(t)
	run, _, runs := markingPlugin(t)
	text := configDeclaring("acme", run, "acme-monitor {\n\tinterval 5\n}\n")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "ze.conf"})
	require.NoError(t, err)
	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr)

	refusal := declared.ParseRefusal("ze.conf", parseErr, "the command did nothing")
	require.ErrorIs(t, refusal, parseErr, "the parser's refusal is lost")
	assert.Equal(t, "ze config validate ze.conf", refusal.Fix())
	message := refusal.Error()
	lines := strings.Split(message, "\n")
	assert.Contains(t, lines[0], "config ze.conf does not parse")
	assert.Contains(t, lines[0], "no config schema compiled into ze defines")
	assert.Equal(t, "fix: ze config validate ze.conf", lines[len(lines)-1], "the command is not alone on the last line")
	assert.Equal(t, 1, strings.Count(message, "\nfix: "), "one fix line: %s", message)
	assert.Contains(t, message, `the config declares external plugin "acme"`)
	assert.Contains(t, message, "when `ze start` runs it")
	assert.Contains(t, message, "starts no plugin")
	assert.Contains(t, message, "only a running ze that started with these plugins checks a section")
	assert.Contains(t, message, "asked no running ze", "a command that asked no daemon claims one was asked")
	assert.NotContains(t, message, "none answered", "a command that asked no daemon claims one was asked")
	assert.NotContains(t, message, "show config validation", "the daemon refuses that form for a file outside its config directories")
	assert.Contains(t, message, "unknown top-level keyword: acme-monitor")
	assert.NotContains(t, message, "ze cli -c", "that form is answered in the caller's own process")
	assert.NotContains(t, message, "conformance", "the fix runs the plugin")

	explained := declared.Explain(parseErr)
	require.ErrorIs(t, explained, parseErr)
	assert.Equal(t, message, explained.Error(), "Explain and ParseRefusal word one refusal two ways")

	unchecked := declared.Unchecked("ze.conf")
	require.NotNil(t, unchecked)
	assert.Equal(t, "ze config validate ze.conf", unchecked.Fix())
	assert.Contains(t, unchecked.Error(), `external plugin "acme"`)
	assert.NotContains(t, unchecked.Error(), "unknown top-level keyword", "the entry beside a refusal repeats the refusal")

	assert.Zero(t, runCount(t, runs))
}

// TestParseRefusalNamesEveryDeclaredPluginAndNoFile: the message lists each
// declared plugin once and names no file it was not given. Method: two
// declared plugins, one of them declared twice, in a text read from stdin.
//
// VALIDATES: both names in config order, the plural form, and the placeholder
// `<config-file>` for a text with no file.
// PREVENTS: a message that names one plugin as the cause when the config
// declares several, and a daemon form that names the file `-`.
func TestParseRefusalNamesEveryDeclaredPluginAndNoFile(t *testing.T) {
	offlineProcess(t)
	text := "plugin {\n\texternal acme {\n\t\trun \"./acme\"\n\t\tencoder json\n\t}\n" +
		"\texternal acme-shell {\n\t\trun \"./acme-shell\"\n\t\tencoder json\n\t}\n}\n\nacme-monitor {\n\tinterval 5\n}\n"
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: text, Path: "-"})
	require.NoError(t, err)
	assert.Equal(t, []string{"acme", "acme-shell"}, declared.WithoutSchema())

	_, parseErr := NewParser(schema).Parse(text)
	require.Error(t, parseErr)
	message := declared.Explain(parseErr).Error()
	assert.Contains(t, message, `the config declares external plugins "acme", "acme-shell"`)
	assert.True(t, strings.HasSuffix(message, "\nfix: ze config validate <config-file>"), "the fix names the file `-`: %s", message)
	assert.NotContains(t, message, "validate -", "the fix names the file `-`")
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
// twice gets the same answer twice. Method: the step twice over one config.
//
// VALIDATES: the second call answers the same schema, the same plugin, and
// the wording of a command with no daemon, although the first call fixed the
// process's module set.
// PREVENTS: `ze doctor`, which reads its config twice, telling its operator
// to restart ze on the second read.
func TestDeclaredSchemasSecondCallAnswersTheSame(t *testing.T) {
	offlineProcess(t)
	run, _, runs := markingPlugin(t)
	source := ConfigSource{Text: configDeclaring("acme", run, ""), Path: "ze.conf"}

	first, _, err := SchemaWithDeclaredPlugins(source)
	require.NoError(t, err)
	second, declared, err := SchemaWithDeclaredPlugins(source)
	require.NoError(t, err)

	assert.Same(t, first, second)
	assert.Equal(t, []string{"acme"}, declared.WithoutSchema())
	unchecked := declared.Unchecked("ze.conf")
	require.NotNil(t, unchecked)
	// A read inside a daemon answers `restart ze`; the fix of a command with
	// no daemon is the command that asks a running ze.
	assert.Equal(t, "ze config validate ze.conf", unchecked.Fix(), "the second read answers as a daemon would")
	assert.True(t, declared.Offline(), "the second read answers as a daemon would")
	assert.Zero(t, runCount(t, runs))
}

// TestDeclaredSchemasTakesPluginsOfEverySource: a command that reads two
// configs remembers the plugins of both. Method: two configs, each declaring
// a plugin of its own.
//
// VALIDATES: both plugins are listed, in source order, so the refusal of a
// name on either side of `ze config diff` names the plugins of both.
// PREVENTS: the second side of a diff refused with no word about the plugin
// it declares.
func TestDeclaredSchemasTakesPluginsOfEverySource(t *testing.T) {
	offlineProcess(t)
	monitorRun, _, monitorRuns := markingPlugin(t)
	shellRun, _, shellRuns := markingPlugin(t)

	_, declared, err := SchemaWithDeclaredPlugins(
		ConfigSource{Text: configDeclaring("acme", monitorRun, ""), Path: "before.conf"},
		ConfigSource{Text: configDeclaring("acme-shell", shellRun, ""), Path: "after.conf"},
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"acme", "acme-shell"}, declared.WithoutSchema())
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
	const text = "plugin {\n\tno-such-leaf yes\n}\n"

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
	offlineProcess(t)
	declaring := configDeclaring("acme", "./acme-monitor", "")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: declaring, Path: filepath.Join(t.TempDir(), "ze.conf")})
	require.NoError(t, err)
	require.Len(t, declared.WithoutSchema(), 1)

	_, unknownRoot := NewParser(schema).Parse("acme-monitor {\n\tinterval 5\n}\n")
	require.Error(t, unknownRoot)
	assert.True(t, RefusesUnknownNode(unknownRoot), "an unknown top-level keyword is not marked: %v", unknownRoot)
	assert.Contains(t, declared.Explain(unknownRoot).Error(), `external plugin "acme"`)

	_, unknownField := NewParser(schema).Parse("plugin {\n\tno-such-leaf yes\n}\n")
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
	offlineProcess(t)
	declaring := configDeclaring("acme", "./acme-monitor", "")
	schema, declared, err := SchemaWithDeclaredPlugins(ConfigSource{Text: declaring, Path: filepath.Join(t.TempDir(), "ze.conf")})
	require.NoError(t, err)

	_, unknownRoot := NewParser(schema).Parse("acme-monitor {\n\tinterval 5\n}\n")
	require.Error(t, unknownRoot)
	explained := declared.ParseRefusal("ze.conf", unknownRoot, "the command did nothing")
	assert.Equal(t, "ze config validate ze.conf", explained.Fix())
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
