package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// publishedModuleText is the module the published-set tests declare for the
// external plugin acme-monitor.
const publishedModuleText = `module acme-monitor-conf {
    namespace "urn:acme:monitor";
    prefix acme;
    container acme-monitor {
        leaf interval { type uint8; }
    }
}`

// markerProgram writes a program that leaves a marker file when it is
// started, whatever its arguments, and answers its path and the marker's.
func markerProgram(t *testing.T) (program, marker string) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "started")
	program = filepath.Join(dir, "external-plugin")
	script := "#!/bin/sh\necho started > " + marker + "\necho 'module acme-monitor-conf {}'\n"
	require.NoError(t, os.WriteFile(program, []byte(script), 0o700)) //nolint:gosec // a test program the test itself runs
	return program, marker
}

// TestSchemaRegistryStartsNoExternalProgram proves that the schema CLI never
// starts a plugin the command line names as an external program: the registry
// builds from the modules compiled into ze, and the program's marker is
// absent. The control shows the marker is what a start leaves.
//
// VALIDATES: the owner ruling of 2026-11-16 at the schema CLI (AC-13's
// survivor of spec-plugin-external-yang-at-startup): with no daemon running
// ze starts no external plugin's program.
// PREVENTS: `ze --plugin ./program schema list` running ./program with
// `--yang`, a flag only ze's own plugin CLI knows, so a third-party program
// ran its live start.
func TestSchemaRegistryStartsNoExternalProgram(t *testing.T) {
	program, marker := markerProgram(t)

	registry, err := buildSchemaRegistry([]string{program})

	require.NoError(t, err)
	assert.NoFileExists(t, marker, "the schema CLI started the external program")
	_, lookupErr := registry.GetByModule("acme-monitor-conf")
	require.Error(t, lookupErr, "the registry holds a module only the program could have given")
	assert.NotEmpty(t, registry.ListModules(), "the compiled-in modules are still answered")

	// Control: the marker is what a start of the program leaves.
	require.NoError(t, exec.Command(program).Run()) //nolint:gosec // the test's own program
	assert.FileExists(t, marker)
}

// TestExternalNotShownNamesThePluginAndTheFix proves what `ze schema` says of
// an external plugin named on its command line: which plugin its answer shows
// nothing of, why, and the command that reaches a running ze. A command line
// with compiled-in plugins alone says nothing.
//
// VALIDATES: the ruling's "the refusal says so and names what to run".
// PREVENTS: an answer of compiled-in modules alone that reads as the whole
// schema, with no word about the plugin the operator named.
func TestExternalNotShownNamesThePluginAndTheFix(t *testing.T) {
	notShown := externalNotShown([]string{"ze.bgp", "./acme-monitor --verbose", "ze-gr", "/opt/other"})

	require.NotNil(t, notShown)
	assert.Equal(t, `ze schema answered from the modules compiled into ze alone: the config schema of external plugins `+
		`"./acme-monitor --verbose", "/opt/other" is not available with no running ze`, notShown.Problem())
	assert.Contains(t, notShown.Why(), "ze starts no external plugin's program")
	// The two commands a running ze serves for it. `show schema show` is not
	// one: the daemon answers it `unknown command`.
	assert.Contains(t, notShown.Why(), "typed in a `ze cli` session, `show schema list` lists that module and "+
		"`show yang tree config` shows its nodes")
	assert.NotContains(t, notShown.Why(), "show schema show")
	assert.Equal(t, "ze cli", notShown.Fix())

	one := externalNotShown([]string{"./acme-monitor"})
	require.NotNil(t, one)
	assert.Contains(t, one.Problem(), `of external plugin "./acme-monitor" is not available`)

	assert.Nil(t, externalNotShown([]string{"ze.bgp", "ze-gr"}), "compiled-in plugins are shown")
	assert.Nil(t, externalNotShown(nil))
}

// TestSchemaRegistryHoldsPublishedExternalModule proves that inside a process
// that published an external plugin's module, as a running daemon does at
// start, the schema registry answers that module: its text, the plugin that
// declared it and the config roots that plugin wants. Before the publish the
// registry does not hold it.
//
// VALIDATES: AC-13 of spec-plugin-external-yang-at-startup at the schema
// command the daemon serves (`show schema list`), from the published set and
// with no second read of the plugin. The module's text is held too, though
// the daemon has no command that prints it.
// PREVENTS: the daemon's schema commands answering compiled-in modules alone
// while the config it runs holds a section the plugin's module defines.
func TestSchemaRegistryHoldsPublishedExternalModule(t *testing.T) {
	t.Cleanup(yang.ResetPublishedModulesForTest())
	t.Cleanup(pluginserver.SetExternalConfigRootsForTest(map[string][]string{"acme-monitor": {"acme-monitor"}}))

	before, err := buildSchemaRegistry(nil)
	require.NoError(t, err)
	_, lookupErr := before.GetByModule("acme-monitor-conf")
	require.Error(t, lookupErr, "no module is published yet")

	require.NoError(t, yang.PublishExternalModules([]yang.ExternalModule{{
		Plugin: "acme-monitor", Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", Text: publishedModuleText,
	}}, yang.NewValidatorRegistry()))

	registry, err := buildSchemaRegistry(nil)
	require.NoError(t, err)
	schema, err := registry.GetByModule("acme-monitor-conf")
	require.NoError(t, err)
	assert.Equal(t, publishedModuleText, schema.Yang)
	assert.Equal(t, "acme-monitor", schema.Plugin)
	assert.Equal(t, []string{"acme-monitor"}, schema.WantsConfig)
	assert.Contains(t, registry.ListModules(), "acme-monitor-conf")
}
