package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// firstPassRun is the run string every first-pass test declares for the plugin
// named acme. It holds a space, so the set form has to quote it.
const firstPassRun = "/opt/acme/monitor --fast"

// firstPassPluginBlock is the one declaration the brace-form tests look for
// after the roots they expect skipped.
const firstPassPluginBlock = `plugin {
    external acme {
        run "/opt/acme/monitor --fast";
    }
}
`

// requireAcmeOnly asserts that the first pass answered exactly the plugin
// named acme, with the run string the config wrote.
func requireAcmeOnly(t *testing.T, result *FirstPassResult) {
	t.Helper()
	require.Len(t, result.External, 1)
	assert.Equal(t, "acme", result.External[0].Name)
	assert.Equal(t, firstPassRun, result.External[0].Run)
	assert.True(t, result.External[0].RunsExternalProgram())
}

// TestFirstPassSkipsUnknownRoots parses a whole config whose other roots the
// first-pass schema does not hold: a block a plugin would bring, a block of a
// compiled-in protocol, and both before and after the plugin block.
//
// VALIDATES: the declared external plugin is found among roots nobody knows yet.
// PREVENTS: the first pass refusing a config at the root its plugin will declare.
func TestFirstPassSkipsUnknownRoots(t *testing.T) {
	input := `acme-monitor {
    target 192.0.2.1;
    interval 5;
}
` + firstPassPluginBlock + `bgp {
    router-id 192.0.2.254;
    peer transit1 {
        connection { remote { ip 192.0.2.1; } }
    }
}
`
	result, err := FirstPass(input, filepath.Join(t.TempDir(), "ze.conf"), nil)
	require.NoError(t, err)
	requireAcmeOnly(t, result)
}

// TestFirstPassSkipsNestedBlock skips a root whose blocks nest three deep and
// which holds statements after its inner blocks close.
//
// VALIDATES: the skip ends at the brace that closes the root, not at the first '}'.
// PREVENTS: an inner '}' ending the skip, so the rest of the root reads as top level.
func TestFirstPassSkipsNestedBlock(t *testing.T) {
	input := `acme-monitor {
    probe one {
        http {
            path /health;
        }
        retries 3;
    }
    plugin {
        external decoy {
            run /bin/false;
        }
    }
    interval 5;
}
` + firstPassPluginBlock
	result, err := FirstPass(input, "", nil)
	require.NoError(t, err)
	requireAcmeOnly(t, result)
}

// TestFirstPassQuotedBraces skips a root whose values hold braces inside
// double and single quotes.
//
// VALIDATES: a brace inside a quoted string neither opens nor closes a block.
// PREVENTS: a description holding '}' ending the skip early, or one holding '{'
// swallowing the plugin block after it.
func TestFirstPassQuotedBraces(t *testing.T) {
	input := `acme-monitor {
    description "closes } nothing";
    banner 'opens { nothing';
    pattern "{ } } {";
}
` + firstPassPluginBlock
	result, err := FirstPass(input, "", nil)
	require.NoError(t, err)
	requireAcmeOnly(t, result)
}

// TestFirstPassSkipsTopLevelLeaf skips unknown top-level statements that hold
// no block: a value, a bracket list, and a bare keyword.
//
// VALIDATES: a skipped statement with no block ends at its terminator.
// PREVENTS: a top-level leaf of a plugin's module swallowing the statement after it.
func TestFirstPassSkipsTopLevelLeaf(t *testing.T) {
	input := `acme-level 5;
acme-targets [ alpha beta ];
acme-flag
` + firstPassPluginBlock + `acme-tail 7
`
	result, err := FirstPass(input, "", nil)
	require.NoError(t, err)
	requireAcmeOnly(t, result)
}

// TestFirstPassRefusesUnclosedSkippedBlock gives the first pass a skipped root
// whose block never closes, so the plugin block after it sits inside it.
//
// VALIDATES: the refusal names the line that opened the block.
// PREVENTS: an unclosed block reading as "no plugin declared".
func TestFirstPassRefusesUnclosedSkippedBlock(t *testing.T) {
	input := `# line 1
acme-monitor {
    target 192.0.2.1;
` + firstPassPluginBlock
	result, err := FirstPass(input, "", nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "line 2:")
	assert.Contains(t, err.Error(), "acme-monitor")
	assert.Contains(t, err.Error(), "never closes")
}

// TestFirstPassRefusesUnfinishedSkippedStatement gives the first pass a
// skipped statement that the end of the file cuts inside a bracket list.
//
// VALIDATES: a skipped statement with no terminator is refused, naming its line.
// PREVENTS: the end of the file standing in for a terminator nobody wrote.
func TestFirstPassRefusesUnfinishedSkippedStatement(t *testing.T) {
	input := firstPassPluginBlock + "acme-targets ["
	result, err := FirstPass(input, "", nil)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "line 6:")
	assert.Contains(t, err.Error(), "acme-targets")
}

// TestFirstPassKeepsKnownRootsStrict writes an unknown keyword inside the
// plugin block, in both forms.
//
// VALIDATES: the skip covers unknown roots only; a root the first pass holds is
// parsed as strictly as the full parse parses it.
// PREVENTS: a mistyped leaf of a plugin declaration being skipped, so the
// plugin runs without what the operator wrote.
func TestFirstPassKeepsKnownRootsStrict(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "brace form", input: "plugin {\n    external acme {\n        runn /bin/true;\n    }\n}\n"},
		{name: "set form", input: "set plugin external acme runn /bin/true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FirstPass(tt.input, "", nil)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "runn")
		})
	}
}

// TestFirstPassSetForm parses the same declarations in the brace form and in
// the set form, each beside lines of roots the first pass does not hold.
//
// VALIDATES: both config forms answer the same declared plugins.
// PREVENTS: a set-form config being refused at, or losing plugins to, the root
// a plugin's module will declare.
func TestFirstPassSetForm(t *testing.T) {
	brace := `acme-monitor {
    target 192.0.2.1;
}
plugin {
    external acme {
        run "/opt/acme/monitor --fast";
        encoder json;
    }
    external second {
        run "/opt/second/run";
    }
}
`
	set := `set acme-monitor target 192.0.2.1
set plugin external acme run "/opt/acme/monitor --fast"
set plugin external acme encoder json
set bgp router-id 192.0.2.254
delete acme-monitor interval
set plugin external second run /opt/second/run
`
	fromBrace, err := FirstPass(brace, "", nil)
	require.NoError(t, err)
	fromSet, err := FirstPass(set, "", nil)
	require.NoError(t, err)

	require.Len(t, fromBrace.External, 2)
	assert.ElementsMatch(t, fromBrace.External, fromSet.External)
}

// TestFirstPassSetMetaForm parses set lines that carry authorship metadata,
// the form the daemon writes for a committed config.
//
// VALIDATES: the metadata form skips an unknown root as the plain set form does.
// PREVENTS: a committed config that holds a plugin's root being refused at boot.
func TestFirstPassSetMetaForm(t *testing.T) {
	input := `#thomas @local %2026-11-16T10:00:00Z set acme-monitor target 192.0.2.1
#thomas @local %2026-11-16T10:00:00Z set plugin external acme run "/opt/acme/monitor --fast"
`
	require.Equal(t, FormatSetMeta, DetectFormat(input))
	result, err := FirstPass(input, "", nil)
	require.NoError(t, err)
	requireAcmeOnly(t, result)
}

// TestFirstPassMergesCommandLinePlugins names a plugin on the command line
// for a config that holds no plugin block, only a root the plugin will bring.
//
// VALIDATES: a plugin declared on the command line alone is in the declared list.
// PREVENTS: a plugin with no config section never being run, so its root stays
// unknown at the full parse.
func TestFirstPassMergesCommandLinePlugins(t *testing.T) {
	input := "acme-monitor {\n    target 192.0.2.1;\n}\n"

	result, err := FirstPass(input, "", []string{"/opt/acme/monitor --fast"})
	require.NoError(t, err)

	require.Len(t, result.External, 1)
	assert.Equal(t, "monitor", result.External[0].Name)
	assert.Equal(t, firstPassRun, result.External[0].Run)
}

// TestFirstPassMergesCommandLineWithConfigPlugins names one plugin in the
// config and another on the command line.
//
// VALIDATES: the two sources are merged into one list, each plugin once.
// PREVENTS: the command line replacing the plugin block, or the reverse.
func TestFirstPassMergesCommandLineWithConfigPlugins(t *testing.T) {
	result, err := FirstPass(firstPassPluginBlock, "", []string{"/opt/other/probe", "/elsewhere/acme"})
	require.NoError(t, err)

	names := make([]string, 0, len(result.External))
	for _, declared := range result.External {
		names = append(names, declared.Name)
	}
	assert.ElementsMatch(t, []string{"acme", "probe"}, names,
		"a command-line plugin whose name the config already declares is not added twice")
}

// TestFirstPassLeavesOutCompiledInPlugins declares a plugin block with no run
// string and an inactive external entry beside a running one.
//
// VALIDATES: only a plugin that runs as another program is in the list.
// PREVENTS: the caller forking a plugin the operator deactivated, or one that
// names no program.
func TestFirstPassLeavesOutCompiledInPlugins(t *testing.T) {
	input := `plugin {
    external placeholder { }
    inactive: external parked {
        run "/opt/parked/run";
    }
    external acme {
        run "/opt/acme/monitor --fast";
    }
}
`
	result, err := FirstPass(input, "", nil)
	require.NoError(t, err)
	requireAcmeOnly(t, result)
}

// TestFirstPassAnswersNothingToRun gives the first pass a config and a command
// line that declare no external plugin.
//
// VALIDATES: the answer is an empty list and no error, the case of AC-9.
// PREVENTS: a config with no plugin block being refused by the first pass.
func TestFirstPassAnswersNothingToRun(t *testing.T) {
	result, err := FirstPass("bgp {\n    router-id 192.0.2.254;\n}\n", "", nil)
	require.NoError(t, err)
	assert.Empty(t, result.External)
	assert.Equal(t, plugin.HubConfig{}, result.Hub)
	assert.Nil(t, result.Environment)
}

// TestFirstPassAnswersHubAndEnvironment declares a hub listener and an
// environment block beside a root the first pass skips.
//
// VALIDATES: the hub listener and the environment block are read by the same
// extractors the full load uses, and the work directory is the config's.
// PREVENTS: a declared plugin being forked with no listener to connect back to.
func TestFirstPassAnswersHubAndEnvironment(t *testing.T) {
	input := `environment {
    daemon {
        pid "/run/ze-first-pass.pid";
    }
}
acme-monitor {
    target 192.0.2.1;
}
plugin {
    hub {
        server local {
            ip 127.0.0.1;
            port 1790;
        }
    }
    external acme {
        run "/opt/acme/monitor --fast";
    }
}
`
	dir := t.TempDir()
	result, err := FirstPass(input, filepath.Join(dir, "ze.conf"), nil)
	require.NoError(t, err)

	requireAcmeOnly(t, result)
	require.Len(t, result.Hub.Servers, 1)
	assert.Equal(t, "local", result.Hub.Servers[0].Name)
	assert.Equal(t, "127.0.0.1", result.Hub.Servers[0].Host)
	assert.Equal(t, uint16(1790), result.Hub.Servers[0].Port)
	assert.Equal(t, "/run/ze-first-pass.pid", result.Environment[sectionDaemon]["pid"])
	assert.Equal(t, dir, result.ConfigDir)
}

// TestFirstPassReadsTheEnvironmentOfTheFullParse parses one environment block
// by the first pass and by the full read, and compares the two answers.
//
// VALIDATES: the first pass answers the environment values the full parse
// answers for the same block.
// PREVENTS: a plugin forked from first-pass values seeing settings that differ
// from the ones the daemon later runs with.
func TestFirstPassReadsTheEnvironmentOfTheFullParse(t *testing.T) {
	input := "environment {\n    daemon {\n        pid \"/run/ze-first-pass-shared.pid\";\n    }\n}\n"

	result, err := FirstPass(input, "", nil)
	require.NoError(t, err)
	require.Equal(t, "/run/ze-first-pass-shared.pid", result.Environment[sectionDaemon]["pid"])

	full, err := ReadConfig(input, "", nil)
	require.NoError(t, err)
	assert.Equal(t, ExtractEnvironment(full.Tree), result.Environment,
		"the first pass reads the block the full parse reads")
}

// TestStrictParseStillRefusesUnknownRoot parses a config with an unknown root
// over the first-pass schema with no mode set, in both forms.
//
// VALIDATES: the skip is a mode a caller turns on; the default refuses as before.
// PREVENTS: the first-pass schema quietly accepting every config.
func TestStrictParseStillRefusesUnknownRoot(t *testing.T) {
	schema, err := PluginOnlySchema()
	require.NoError(t, err)

	_, err = NewParser(schema).Parse("acme-monitor {\n    target 192.0.2.1;\n}\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown top-level keyword: acme-monitor")

	_, err = NewSetParser(schema).Parse("set acme-monitor target 192.0.2.1\n")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field: acme-monitor")
}

// TestParseAtNeverSkips parses a fragment at a context with the skip mode on.
//
// VALIDATES: the mode covers the top level of a whole file only.
// PREVENTS: an editor fragment losing an unknown keyword to a mode meant for boot.
func TestParseAtNeverSkips(t *testing.T) {
	schema, err := PluginOnlySchema()
	require.NoError(t, err)

	parser := NewParser(schema)
	parser.SkipUnknownRoots()
	_, err = parser.ParseAt("bogus 1;\n", []string{"plugin"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus")
}

// TestPluginOnlySchemaSharesTheResolvedSet compares the first-pass schema with
// the schema of the whole process, before and after a first pass.
//
// VALIDATES: the first-pass schema is cut from the set yang.DefaultLoader
// already resolved: its roots are the full schema's own nodes, and a first pass
// leaves the resolved set and the cached schema as they were.
// PREVENTS: a second resolve of every module on each start (AC-9, A-11).
func TestPluginOnlySchemaSharesTheResolvedSet(t *testing.T) {
	resolvedBefore, err := yang.DefaultLoader()
	require.NoError(t, err)
	full, err := YANGSchema()
	require.NoError(t, err)

	sub, err := PluginOnlySchema()
	require.NoError(t, err)
	require.NotNil(t, sub.Get("plugin"))
	require.NotNil(t, sub.Get("environment"))
	assert.Same(t, full.Get("plugin"), sub.Get("plugin"))
	assert.Same(t, full.Get("environment"), sub.Get("environment"))
	assert.ElementsMatch(t, []string{"plugin", "environment"}, sub.Children())

	_, err = FirstPass("bgp {\n    router-id 192.0.2.254;\n}\n", "", nil)
	require.NoError(t, err)

	resolvedAfter, err := yang.DefaultLoader()
	require.NoError(t, err)
	assert.Same(t, resolvedBefore, resolvedAfter)
	fullAfter, err := YANGSchema()
	require.NoError(t, err)
	assert.Same(t, full, fullAfter)
}
